"""Review-only exact Windows data-inspection controller; no Docker/build/parser run."""
import argparse
import ctypes as C
from ctypes import wintypes as W
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import threading
import time

BASE = Path('E:/Luci-local-acceptance')
PYTHON = Path('C:/Users/DylanMackie/.cache/codex-runtimes/codex-primary-runtime/dependencies/python/python.exe')
PYTHON_SHA256 = 'b847bb9f4e48c19f585c8bf7d8b355f3f8c6d2125717ece9c9d7e16fb9f09de8'
WORKER_SHA256 = '6953a9d85c200f3ddf633ad87eac0c475ed2e79e4fd102142536d8930a623451'
MANIFEST_SHA256 = '04d9bf5b016ae904c2a3dc890a9c7977996bd12bc425cdd2502a6ed11a8b1d6a'
PINS = {
    'windows-api-job-2g-pdh-recovery.py': 'c050bd16622be034a4ef8db050a1554c0ee492a4519ff0b581c647af14f5fd6d',
    'windows-codex-auth-only-655035ec.py': '3fa9eb79e8596a6c12f53886ca8c52d59fd2cf2feaf1620862a5657375cc6031',
    'windows-job-inventory.py': '52cf3bd64fc39e8c986cf80eb7022b70e58b59f6e2ad3db443d8ab924f4da844',
    'windows-pdh-cpu-format-recovery.py': '2bc56957918bc577b206b4aede979aab867c929a3f48799fd7be477f564d2b49',
}
WHOLE_SECONDS, WORK_SECONDS = 420, 300
sys.dont_write_bytecode = True


def digest(path, maximum=2 << 20):
    if path.stat().st_size > maximum:
        raise ValueError('source/input metadata byte bound')
    h = hashlib.sha256()
    with path.open('rb') as stream:
        for block in iter(lambda: stream.read(65536), b''):
            h.update(block)
    return h.hexdigest()


def load(name):
    path = BASE/'scripts'/name
    if digest(path) != PINS[name]:
        raise ValueError('primitive pin differs')
    spec = importlib.util.spec_from_file_location('extractor_'+name.replace('-', '_'), path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class AtomicOwnedJob:
    """Reviewed OwnedJob launch shape, with atomic JOB_LIST and checked closure.

    No process exists outside the Job: unsupported JOB_LIST is a refusal. The
    Python loader cannot run before atomic assignment and suspended readback.
    """
    def __init__(self, m, auth, win, output, argv, env, abort, register):
        self.m, self.auth, self.w = m, auth, win
        self.job = self.attrs = None
        self.process = m.PROCESS()
        self.handles, self.initialized, self.assigned = [], False, False
        self.inherit_values = self.job_values = None
        self.closed = False
        self.stdout, self.stderr = output/'worker.stdout.log', output/'worker.stderr.log'
        register(self)  # Parent owns partial construction and its cleanup evidence.
        try:
            self.job = win.k.CreateJobObjectW(None, None)
            win.check(self.job, 'CreateJobObjectW')
            flags = W.DWORD()
            win.check(win.k.GetHandleInformation(self.job, C.byref(flags)), 'job inheritance')
            if flags.value & 1:
                raise ValueError('inheritable Job')
            limits = m.EXTENDED_LIMIT()
            limits.BasicLimitInformation.LimitFlags = m.LIMIT_FLAGS
            limits.BasicLimitInformation.ActiveProcessLimit = m.MAX_PROCESSES
            limits.ProcessMemoryLimit, limits.JobMemoryLimit = m.PROCESS_MEMORY, m.JOB_MEMORY
            win.check(win.k.SetInformationJobObject(self.job, 9, C.byref(limits), C.sizeof(limits)), 'Job limits')
            win.limits(self.job)
            security = m.SECURITY(C.sizeof(m.SECURITY), None, True)
            for path, access, mode in ((Path('NUL'), 0x80000000, 3), (self.stdout, 0x40000000, 1), (self.stderr, 0x40000000, 1)):
                handle = win.k.CreateFileW(str(path), access, 1, C.byref(security), mode, 0x80, None)
                if handle in (None, C.c_void_p(-1).value):
                    raise OSError(C.get_last_error(), 'private stream')
                self.handles.append(handle)
            size = m.S()
            win.k.InitializeProcThreadAttributeList(None, 2, 0, C.byref(size))
            if not 0 < size.value <= 65536:
                raise ValueError('attribute size bound')
            self.attrs = C.create_string_buffer(size.value)
            win.check(win.k.InitializeProcThreadAttributeList(self.attrs, 2, 0, C.byref(size)), 'attribute initialization')
            self.initialized = True
            # Windows requires attribute values to remain alive until the list
            # is destroyed, not merely until CreateProcessW returns.
            self.inherit_values = (W.HANDLE*3)(*self.handles)
            self.job_values = (W.HANDLE*1)(self.job)
            win.check(win.k.UpdateProcThreadAttribute(self.attrs, 0, 0x20002, self.inherit_values, C.sizeof(self.inherit_values), None, None), 'explicit HANDLE_LIST')
            win.check(win.k.UpdateProcThreadAttribute(self.attrs, 0, 0x2000D, self.job_values, C.sizeof(self.job_values), None, None), 'atomic JOB_LIST; no fallback')
            startup = m.STARTUP_EX()
            startup.StartupInfo.cb = C.sizeof(startup)
            startup.StartupInfo.dwFlags = 0x100
            startup.StartupInfo.hStdInput, startup.StartupInfo.hStdOutput, startup.StartupInfo.hStdError = self.handles
            startup.lpAttributeList = C.cast(self.attrs, W.LPVOID)
            block = C.create_unicode_buffer('\0'.join(k+'='+v for k, v in sorted(env.items(), key=lambda row: row[0].upper()))+'\0\0')
            if abort():
                raise ValueError('admission/whole deadline changed before atomic creation')
            win.check(win.k.CreateProcessW(str(argv[0]), C.create_unicode_buffer(subprocess.list2cmdline(argv)), None, None, True,
                0x4 | 0x400 | 0x80000 | 0x08000000, block, str(output), C.byref(startup), C.byref(self.process)), 'atomic contained suspended process')
            self.assigned = True
            owned = W.BOOL()
            win.check(win.k.IsProcessInJob(self.process.hProcess, self.job, C.byref(owned)), 'membership')
            if not owned.value or win.members(self.job) != [self.process.dwProcessId]:
                raise ValueError('atomic suspended membership uncertain')
            win.limits(self.job)
        except BaseException:
            # Caller first stops the no-I/O watchdog, then retires/closes this exact
            # partially constructed owner. Never close a handle under that reader.
            raise

    def resume(self):
        # Reuse the frozen normal runner's exact suspended-thread resume check.
        self.auth.OwnedJob.resume(self)

    def kill_now(self):
        if self.job and not self.closed:
            self.w.check(self.w.k.TerminateJobObject(self.job, 125), 'emergency terminate exact Job')

    def close(self):
        if self.closed:
            raise ValueError('double close refused')
        result = {'terminal': not bool(self.process.hProcess), 'jobEmpty': not bool(self.job), 'closedHandlesAcknowledged': False}
        error = None
        try:
            # Always stop before inventory logging, waits or evidence publication.
            self.kill_now()
            if self.process.hProcess:
                result['terminal'] = self.w.k.WaitForSingleObject(self.process.hProcess, 5000) == 0
            if self.job:
                end = time.monotonic()+5
                while self.w.members(self.job) and time.monotonic() < end:
                    time.sleep(.05)
                result['finalOwnedPIDs'] = self.w.members(self.job)
                result['activeProcesses'] = self.w.query(self.job, 1, self.m.ACCOUNTING).ActiveProcesses
                result['jobEmpty'] = not result['finalOwnedPIDs'] and result['activeProcesses'] == 0
        except BaseException as caught:
            error = caught
        finally:
            if self.initialized:
                self.w.k.DeleteProcThreadAttributeList(self.attrs)
                self.initialized = False
            self.inherit_values = self.job_values = None
            failed = []
            for label, handle in [('thread', self.process.hThread), ('process', self.process.hProcess), *[(f'stdio-{n}', h) for n, h in enumerate(self.handles)], ('job', self.job)]:
                if handle and not self.w.k.CloseHandle(handle):
                    failed.append(label)
            self.closed = True
            result['closedHandlesAcknowledged'] = not failed
            result['handleClosureUnknown'] = failed
            self.process.hThread = self.process.hProcess = None
            self.handles, self.job, self.attrs = [], None, None
        if error or not result['terminal'] or not result['jobEmpty'] or not result['closedHandlesAcknowledged']:
            raise ValueError('exact Job/process/handle retirement uncertain') from error
        return result


class Brake:
    """Only clock, native memory and in-memory PDH state; no disk/network I/O."""
    def __init__(self, work_end, host, cpu):
        self.work_end, self.host, self.cpu = work_end, host, cpu
        self.job = None
        self.stop = threading.Event()
        self.reason = self.kill_error = None
        self.worker = threading.Thread(target=self.watch, name='extractor-input-brake', daemon=True)
        self.timer = threading.Thread(target=self.deadline, name='extractor-input-deadline', daemon=True)

    def deadline(self):
        # Independent of native memory/PDH queries and all filesystem I/O.
        if not self.stop.wait(max(0, self.work_end-time.monotonic())):
            self.reason = self.reason or 'work_deadline_cleanup_reserve'
            if self.job:
                try:
                    self.job.kill_now()
                except BaseException:
                    self.kill_error = 'exact_job_stop_unknown'

    def check(self):
        if self.reason:
            return True
        try:
            if time.monotonic() >= self.work_end:
                self.reason = 'work_deadline_cleanup_reserve'
            else:
                host = self.host()
                if min(host['availableBytes'], host['commitAvailableBytes']) < 4 << 30:
                    self.reason = 'host_reserve_brake'
                self.cpu.snapshot()  # Five-second unknown brake, no I/O in snapshot.
        except BaseException:
            self.reason = 'capacity_or_cpu_unknown'
        if self.reason and self.job:
            try:
                self.job.kill_now()
            except BaseException:
                self.kill_error = 'exact_job_stop_unknown'
        return self.reason is not None

    def watch(self):
        while not self.stop.is_set():
            if self.check():
                break
            self.stop.wait(.05)

    def close(self):
        self.stop.set()
        for worker in (self.timer, self.worker):
            if worker.ident is not None:
                worker.join(timeout=2)
        return not self.worker.is_alive() and not self.timer.is_alive()


def supervise(job, brake, win, output, whole_end):
    """No result/readback/publication before the emergency latch is checked."""
    while True:
        if brake.check() or time.monotonic() >= whole_end:
            job.kill_now()
            raise ValueError('work stopped; preserve evidence without retry')
        wait = win.k.WaitForSingleObject(job.process.hProcess, 0)
        if wait == 0xFFFFFFFF:
            job.kill_now()
            raise ValueError('owned process wait uncertain')
        if wait == 0:
            code = W.DWORD()
            win.check(win.k.GetExitCodeProcess(job.process.hProcess, C.byref(code)), 'worker exit code')
            return code.value
        if any(path.stat().st_size > 65536 for path in (job.stdout, job.stderr)):
            job.kill_now()
            raise ValueError('bounded worker log exceeded')
        time.sleep(.05)


def run(args):
    began = time.monotonic();work_end, whole_end = began+WORK_SECONDS, began+WHOLE_SECONDS
    m = load('windows-api-job-2g-pdh-recovery.py')
    auth = load('windows-codex-auth-only-655035ec.py')
    inventory = load('windows-job-inventory.py')
    load('windows-pdh-cpu-format-recovery.py')
    output = Path(args.output).absolute()
    m.regular(output.parent, directory=True)
    if output.parent != BASE/'artifacts' or output.exists():
        raise ValueError('fresh direct owned artifacts directory required')
    if shutil.disk_usage(output.parent).free < 2 << 30:
        raise ValueError('fresh private-output disk reserve below 2GiB')
    worker = m.regular(Path(__file__).parent/'windows_product_tools_v1.py')
    manifest = m.regular(worker.parent/'windows_product_tools_v1_manifest.json')
    m.regular(PYTHON)
    if digest(PYTHON, 16 << 20) != PYTHON_SHA256 or digest(worker) != WORKER_SHA256 or digest(manifest) != MANIFEST_SHA256:
        raise ValueError('Python/worker/manifest pin differs')
    if args.phase == 'acquire' and args.input or args.phase == 'inspect' and not args.input:
        raise ValueError('phase-specific input required')
    if args.input:
        source = m.regular(Path(args.input).absolute(), directory=True)
        if source.parent != BASE/'artifacts' or source == output:
            raise ValueError('prior direct owned acquisition directory required')
    output.mkdir(mode=0o700)
    win = m.Windows()
    win.inventory_module, win.inventory_path = inventory, output/'raw-job-queries.jsonl'
    cpu = m.PdhMonitor(output)
    job = brake = None
    receipt = {'version': 1, 'phase': args.phase, 'batch': args.batch, 'manifestSha256': MANIFEST_SHA256,
               'workerSha256': WORKER_SHA256, 'controllerSha256': digest(Path(__file__)), 'state': 'preflight',
               'treeSettled': False, 'pdhClosed': False, 'wholeSeconds': WHOLE_SECONDS, 'workSeconds': WORK_SECONDS,
               'stdin': 'NUL', 'credentialSources': [], 'atomicCreationInJobRequired': True}
    error = None
    cpu.worker.start()
    try:
        before = win.host();observed_cpu = cpu.admission();fresh = win.host()
        if min(before['availableBytes'], fresh['availableBytes'], before['commitAvailableBytes'], fresh['commitAvailableBytes']) < 8 << 30 or observed_cpu['cpuBusyPercent'] >= 50:
            raise ValueError('fresh normal capacity admission refused')
        if time.monotonic() >= work_end:
            raise ValueError('preflight exhausted work budget')
        receipt.update(before=fresh, beforeCpu=observed_cpu)
        m.write_json(output/'intent.json', receipt)
        argv = [str(PYTHON), '-I', '-S', '-B', str(worker), '--phase', args.phase, '--batch', args.batch,
                '--output', str(output), '--deadline', str(work_end)]
        if args.input:
            argv += ['--input', str(source)]
        # No HOME, PATH, proxy, SSL override, Docker config or account environment.
        system = Path(os.environ['SystemRoot'])
        if system != Path('C:/Windows'):
            raise ValueError('reviewed Windows system directory differs')
        m.regular(system, directory=True)
        env = {'SystemRoot': str(system), 'WINDIR': str(system), 'TEMP': str(output), 'TMP': str(output)}
        brake = Brake(work_end, win.host, cpu)
        def register(value):
            nonlocal job
            job = value
            brake.job = value
            brake.timer.start()
            brake.worker.start()
        job = AtomicOwnedJob(m, auth, win, output, argv, env, brake.check, register)
        # Fresh suspended readback/capacity; no uncontained runtime executes.
        fresh = win.host();cpu_state = cpu.snapshot()
        if brake.check() or min(fresh['availableBytes'], fresh['commitAvailableBytes']) < 8 << 30 or not cpu_state['known'] or cpu_state['cpuBusyPercent'] >= 50:
            raise ValueError('fresh pre-resume admission changed')
        receipt.update(pid=job.process.dwProcessId, state='contained_data_worker_started')
        print(json.dumps({'state': receipt['state'], 'pid': receipt['pid'], 'phase': args.phase, 'batch': args.batch}), flush=True)
        job.resume()
        receipt['exitCode'] = supervise(job, brake, win, output, whole_end)
    except BaseException as caught:
        # The stop operation precedes error formatting, reader/publication and joins.
        if job:
            try:
                job.kill_now()
            except BaseException:
                receipt['emergencyStopUnknown'] = True
        error = caught
        receipt['errorType'] = type(caught).__name__
    finally:
        if job:
            try:
                job.kill_now()
            except BaseException:
                receipt['emergencyStopUnknown'] = True
        # Stop the no-I/O watchdog before closing handles it may otherwise access.
        if brake:
            receipt['brakeReason'] = brake.reason
            receipt['brakeSettled'] = brake.close()
        try:
            if job and receipt.get('brakeSettled', True):
                cleanup = job.close()
                receipt.update(jobCleanup=cleanup, treeSettled=cleanup['terminal'] and cleanup['jobEmpty'] and cleanup['closedHandlesAcknowledged'])
            elif job:
                receipt['jobHandlesRetainedForUnknownWatchdog'] = True
        except BaseException:
            receipt['jobCleanupUnknown'] = True
        pdh = cpu.close()
        receipt['pdhCleanup'] = pdh
        receipt['pdhClosed'] = bool(pdh['workerSettled'] and pdh['queryCleanup'] and pdh['queryCleanup'].get('closed') and pdh['queryCleanup'].get('queryAndCounterHandlesNull') and not pdh['error'])
    receipt['elapsedSeconds'] = time.monotonic()-began
    if not error and receipt.get('exitCode') == 0 and receipt['treeSettled'] and receipt['pdhClosed'] and receipt['elapsedSeconds'] <= WHOLE_SECONDS:
        # Inspect the bounded worker result only after its exact tree is retired.
        try:
            result_path = m.regular(output/'worker-result.json')
            if result_path.stat().st_size > 2 << 20:
                raise ValueError('worker result bound')
            result = json.loads(result_path.read_text(encoding='utf-8'))
            expected = 'all_transfers_exactly_verified' if args.phase == 'acquire' else 'static_inspection_complete_with_explicit_evidence_gaps'
            if result.get('state') != expected or result.get('manifestSha256') != MANIFEST_SHA256 or result.get('batch') != args.batch or result.get('phase') != args.phase:
                raise ValueError('worker result identity/state differs')
            receipt.update(state='completed_owned_worker_retired', workerResultSha256=digest(result_path))
        except BaseException as caught:
            error = caught
            receipt.update(state='incomplete_or_cleanup_unknown_no_retry', resultErrorType=type(caught).__name__)
    else:
        receipt['state'] = 'incomplete_or_cleanup_unknown_no_retry'
    m.write_json(output/'controller-receipt.json', receipt)
    if receipt['state'] != 'completed_owned_worker_retired':
        raise ValueError('attempt incomplete; no automatic retry') from error
    return receipt


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--phase', choices=('acquire', 'inspect'), required=True)
    parser.add_argument('--batch', choices=('windows-product-tools-v1',), required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--input', type=Path)
    run(parser.parse_args())
