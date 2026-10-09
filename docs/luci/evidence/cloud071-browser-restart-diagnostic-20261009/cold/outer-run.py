#!/usr/bin/env python3
"""CLOUD-054 bounded outer wrapper: only signals its exact owned launcher.

Controller/browser observation is read-only. A failed launcher's remaining
allocations or process families stay explicit for independent reviewed cleanup.
No Docker command, provider call, broad process kill, or resource mutation here.
"""
import argparse
import datetime
import hashlib
import json
import os
from pathlib import Path
import re
import selectors
import signal
import stat
import subprocess
import sys
import time

PREPARATION_SECONDS = 120
WORKFLOW_SECONDS = 480
COOPERATIVE_COMMAND_OVERHANG_SECONDS = 60
CLEANUP_SECONDS = 80
LAUNCHER_SECONDS = PREPARATION_SECONDS + WORKFLOW_SECONDS + COOPERATIVE_COMMAND_OVERHANG_SECONDS + CLEANUP_SECONDS
TERM_SECONDS = 20
KILL_SETTLEMENT_SECONDS = 5
FINAL_OBSERVATION_SECONDS = 5
SAMPLE_INTERVAL_SECONDS = 1
MAX_SAMPLES = 800
MAX_JOURNAL_BYTES = 4 * 1024 * 1024
MAX_OUTPUT_BYTES = 2 * 1024 * 1024
MAX_PROCESS_IDENTITIES = 256
# The child cannot exec the application launcher until the parent has its pidfd.
GATED_LAUNCH = """import os,sys
if os.read(0,1)!=b'G':
 sys.exit(125)
os.close(0)
fd=os.open(os.devnull,os.O_RDONLY)
if fd!=0:
 os.dup2(fd,0);os.close(fd)
os.execv(sys.argv[1],sys.argv[1:])
"""



def utc():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def digest(data):
    return hashlib.sha256(data).hexdigest()


def ordinary(path, maximum=4 * 1024 * 1024):
    before = path.lstat()
    if not stat.S_ISREG(before.st_mode) or before.st_nlink != 1 or before.st_size > maximum:
        raise ValueError('Expected bounded ordinary single-link file')
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    try:
        opened = os.fstat(fd)
        if not stat.S_ISREG(opened.st_mode) or (opened.st_dev, opened.st_ino) != (before.st_dev, before.st_ino):
            raise ValueError('File identity changed before read')
        chunks, size = [], 0
        while True:
            chunk = os.read(fd, min(65536, maximum + 1 - size))
            if not chunk:
                break
            chunks.append(chunk)
            size += len(chunk)
            if size > maximum:
                raise ValueError('File read limit')
        after = os.fstat(fd)
        named = path.lstat()
        fields = lambda value: (value.st_dev, value.st_ino, value.st_size, value.st_mtime_ns)
        if fields(before) != fields(after) or fields(after) != fields(named) or size != after.st_size:
            raise ValueError('File changed during read')
        return b''.join(chunks)
    finally:
        os.close(fd)


def private_dir(path):
    actual = path.resolve(strict=True)
    value = path.lstat()
    if actual != path or not stat.S_ISDIR(value.st_mode) or value.st_uid != os.getuid() or value.st_mode & 0o077:
        raise ValueError('Expected owned private ordinary directory')


def write_json(path, value):
    data = (json.dumps(value, indent=2) + '\n').encode()
    if len(data) > MAX_JOURNAL_BYTES:
        raise ValueError('Receipt size limit')
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    try:
        offset = 0
        while offset < len(data):
            count = os.write(fd, data[offset:])
            if not count:
                raise OSError('Short durable write')
            offset += count
        os.fsync(fd)
    finally:
        os.close(fd)
    parent = os.open(path.parent, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        os.fsync(parent)
    finally:
        os.close(parent)


def read_identity(pid):
    try:
        raw = Path(f'/proc/{int(pid)}/stat').read_bytes()
        fields = raw[raw.rfind(b') ') + 2:].split()
        if len(fields) < 20:
            raise ValueError('Malformed process identity')
        return {'status': 'observed', 'pid': int(pid), 'startTicks': fields[19].decode('ascii'),
                'parentPid': int(fields[1]), 'state': fields[0].decode('ascii')}
    except FileNotFoundError:
        return {'status': 'absent', 'pid': int(pid)}
    except (PermissionError, OSError, ValueError, UnicodeError) as error:
        return {'status': 'unavailable', 'pid': int(pid), 'errorType': type(error).__name__}


def owned_environment(entries, fixture):
    root = str(fixture)
    state = root + '/state'
    if 'LUCI_C054_BROWSER=' + root in entries:
        return 'browser'
    if any(value == 'LUCI_STATE_DIR=' + state or value.startswith('LUCI_STATE_DIR=' + state + '/control-plane/') for value in entries):
        return 'controller'
    return None


def collect_owned(fixture, known):
    unavailable, oversized, observed = 0, 0, []
    names = os.listdir('/proc')
    if len(names) > 65536:
        raise ValueError('Process enumeration limit')
    for name in names:
        if not name.isdigit():
            continue
        before = read_identity(name)
        if before['status'] != 'observed':
            unavailable += before['status'] == 'unavailable'
            continue
        try:
            with open(f'/proc/{name}/environ', 'rb') as source:
                data = source.read(262145)
            if len(data) > 262144:
                oversized += 1
                continue
            family = owned_environment(data.decode('utf8', errors='replace').split('\0'), fixture)
            after = read_identity(name)
            if not family or after.get('startTicks') != before['startTicks']:
                continue
            key = f"{before['pid']}:{before['startTicks']}"
            if key not in known and len(known) >= MAX_PROCESS_IDENTITIES:
                raise ValueError('Owned identity count limit')
            known.setdefault(key, {**before, 'family': family, 'firstObservedAt': utc(), 'source': 'outer-environment'})
            observed.append({**after, 'family': family})
        except (FileNotFoundError, ProcessLookupError):
            continue
        except PermissionError:
            unavailable += 1
    return {'owned': observed, 'unreadableEnvironmentCount': unavailable, 'oversizedEnvironmentCount': oversized,
            'scope': 'Exact fixture marker/state environment only; unavailable unrelated environments prevent universal process claims.'}


def final_identities(known):
    values = []
    for identity in known.values():
        current = read_identity(identity['pid'])
        if current['status'] == 'observed' and current['startTicks'] != identity['startTicks']:
            current = {'status': 'original-absent-pid-reused', 'pid': identity['pid'], 'currentStartTicks': current['startTicks']}
        values.append({'expected': identity, 'current': current})
    return values


def supervision_action(elapsed, returncode, term_at, kill_at, failure):
    if returncode is not None:
        return 'settled'
    if term_at is None and (failure is not None or elapsed >= LAUNCHER_SECONDS):
        return 'term'
    if term_at is not None and kill_at is None and elapsed >= term_at + TERM_SECONDS:
        return 'kill'
    if kill_at is not None and elapsed >= kill_at + KILL_SETTLEMENT_SECONDS:
        return 'unsettled'
    return 'observe'


def controlled_environment(inherited, private_home, endpoint):
    if not re.fullmatch(r'unix:///[^\x00\r\n?#]+', endpoint):
        raise ValueError('Explicit absolute local Unix Docker endpoint required')
    result = {key: inherited[key] for key in ('PATH', 'LANG', 'LC_ALL', 'TZ') if key in inherited}
    result.update({'HOME': str(private_home), 'DOCKER_HOST': endpoint})
    return result


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ('node', 'launcher', 'repo', 'binary', 'web', 'provenance', 'image-provenance', 'chromium', 'output', 'fixture-parent'):
        parser.add_argument('--' + name, required=True, type=Path)
    for name in ('expected-launcher-sha256', 'expected-scenario-sha256', 'expected-wrapper-sha256', 'expected-source', 'helper-source', 'expected-image-id', 'docker-host'):
        parser.add_argument('--' + name, required=True)
    parser.add_argument('--window-granted', action='store_true')
    args = parser.parse_args(argv)
    if sys.platform != 'linux' or os.getuid() == 0 or not args.window_granted:
        parser.error('Ordinary Linux task user and explicit exclusive runtime window required')
    for name in ('expected_launcher_sha256', 'expected_scenario_sha256', 'expected_wrapper_sha256'):
        if not re.fullmatch('[a-f0-9]{64}', getattr(args, name)):
            parser.error('Exact SHA256 required: ' + name)
    for name in ('expected_source', 'helper_source'):
        if not re.fullmatch('[a-f0-9]{40}', getattr(args, name)):
            parser.error('Exact commit required: ' + name)
    if not re.fullmatch('sha256:[a-f0-9]{64}', args.expected_image_id):
        parser.error('Exact immutable image ID required')
    source = Path(__file__).resolve()
    launcher = args.launcher.resolve(strict=True)
    scenario = launcher.with_name('scenario.mjs')
    for path, expected in [(source, args.expected_wrapper_sha256), (launcher, args.expected_launcher_sha256), (scenario, args.expected_scenario_sha256)]:
        if digest(ordinary(path)) != expected:
            parser.error('Pinned helper source mismatch')
    args.output = args.output.absolute()
    args.fixture_parent = args.fixture_parent.absolute()
    private_dir(args.output.parent)
    private_dir(args.fixture_parent)
    if args.output.exists() or args.output.is_symlink():
        parser.error('Fresh output directory required')
    if not hasattr(os, 'pidfd_open') or not hasattr(signal, 'pidfd_send_signal'):
        parser.error('Exact Linux pidfd signaling support required')
    ownfd = os.pidfd_open(os.getpid())
    os.close(ownfd)
    args.output.mkdir(mode=0o700)
    home = args.output / 'launcher-home'
    home.mkdir(mode=0o700)
    environment = controlled_environment(os.environ, home, args.docker_host)
    report = args.output / 'driver' / 'result.json'
    child_args = [str(args.node.resolve(strict=True)), str(launcher), '--window-granted', '--exclusive-daemon']
    for flag, value in [('repo', args.repo), ('binary', args.binary), ('web', args.web), ('provenance', args.provenance),
                        ('image-provenance', args.image_provenance), ('expected-source', args.expected_source), ('helper-source', args.helper_source),
                        ('expected-image-id', args.expected_image_id), ('expected-launcher-sha256', args.expected_launcher_sha256),
                        ('expected-scenario-sha256', args.expected_scenario_sha256), ('chromium', args.chromium), ('report', report), ('fixture-root', args.fixture_parent)]:
        child_args.extend(['--' + flag, str(value)])
    budgets = {'preparationSeconds': PREPARATION_SECONDS, 'workflowSeconds': WORKFLOW_SECONDS, 'cooperativeCommandOverhangSeconds': COOPERATIVE_COMMAND_OVERHANG_SECONDS,
               'cleanupSeconds': CLEANUP_SECONDS, 'launcherSeconds': LAUNCHER_SECONDS, 'termGraceSeconds': TERM_SECONDS,
               'killSettlementSeconds': KILL_SETTLEMENT_SECONDS, 'finalObservationSeconds': FINAL_OBSERVATION_SECONDS,
               'nominalMaximumSeconds': LAUNCHER_SECONDS + TERM_SECONDS + KILL_SETTLEMENT_SECONDS + FINAL_OBSERVATION_SECONDS,
               'note': 'Monotonic supervisor; synchronous kernel/file reads are cooperative, not preempted. No signal is sent to controllers, browsers, process groups or Docker.'}
    before = {str(path): {'bytes':len(data), 'sha256':digest(data)} for path in (source, launcher, scenario) for data in [ordinary(path)]}
    write_json(args.output/'before-launch.json', {'version':1, 'at':utc(), 'command':child_args, 'budgets':budgets, 'sources':before,
        'environmentNames':sorted(environment), 'dockerEndpoint':args.docker_host, 'status':'prepared-before-launch'})
    journal_fd = os.open(args.output/'identities.jsonl', os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW, 0o600)
    journal_bytes = 0
    def journal(value):
        nonlocal journal_bytes
        data=(json.dumps(value,separators=(',',':'))+'\n').encode()
        if journal_bytes+len(data)>MAX_JOURNAL_BYTES:
            raise ValueError('Identity journal limit')
        position=0
        while position<len(data):
            n=os.write(journal_fd,data[position:])
            if not n:raise OSError('Short identity journal write')
            position+=n
        os.fsync(journal_fd)
        journal_bytes+=len(data)
    started=time.monotonic()
    child=None; pidfd=None; fixture=None; failure=None; term_at=None; kill_at=None; wrapper_signals=[]
    known={}; observations=0; unavailable=0; next_sample=started; exited_at=None; signals=[]; buffers={}; output_counts={}; truncations={}
    selector=selectors.DefaultSelector()
    result={'version':1,'passed':False,'startedAt':utc(),'budgets':budgets,'sourceBefore':before,'limitations':[
        'This wrapper observes exact fixture process identities only, not memory/CPU/performance or whole-machine closure.',
        'Driver receipt may attest Docker/ledger closure; this wrapper does not independently query Docker or repair retained charges.',
        'Only the exact launcher PID is signalled through its pidfd. Surviving controller/browser identities remain open for independent reviewed cleanup.',
        'Absent unobserved ephemeral processes cannot be inferred; unreadable environment scans and zombies stay explicit.']}
    previous_handlers={}
    def interrupted(number,_frame):
        nonlocal failure
        wrapper_signals.append(number)
        failure=failure or 'wrapper-interrupted'
    for number in (signal.SIGINT,signal.SIGTERM):
        previous_handlers[number]=signal.getsignal(number);signal.signal(number,interrupted)
    try:
        child=subprocess.Popen([sys.executable,'-c',GATED_LAUNCH,*child_args],cwd=args.output,env=environment,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE,start_new_session=True)
        pidfd=os.pidfd_open(child.pid)
        initial=read_identity(child.pid)
        if initial['status']!='observed':raise RuntimeError('Launcher identity not observed')
        known[f"{child.pid}:{initial['startTicks']}"]={**initial,'family':'launcher','source':'owned-Popen','firstObservedAt':utc()}
        journal({'kind':'launcher-started','at':utc(),'identity':initial})
        for name,stream in [('stdout',child.stdout),('stderr',child.stderr)]:
            os.set_blocking(stream.fileno(),False);selector.register(stream,selectors.EVENT_READ,name)
            buffers[name]=open(args.output/('launcher.'+name),'xb',buffering=0);os.chmod(args.output/('launcher.'+name),0o600);output_counts[name]=0;truncations[name]=False
        child.stdin.write(b'G');child.stdin.flush();child.stdin.close()
        journal({'kind':'launcher-gate-released','at':utc(),'identity':initial,'pidfdAcquired':True})
        while True:
            now=time.monotonic();elapsed=now-started
            code=child.poll()
            if code is not None and exited_at is None:exited_at=now
            if code is not None and now-exited_at>=5 and selector.get_map():
                failure=failure or 'launcher-pipes-unsettled';break
            if fixture is None:
                pointer=args.output/'driver'/'private-fixture-path'
                if pointer.exists():
                    raw=ordinary(pointer,4096).decode().strip();candidate=Path(raw)
                    if candidate.parent!=args.fixture_parent or not candidate.name.startswith('luci-c054-'):
                        raise ValueError('Unexpected fixture pointer')
                    private_dir(candidate);fixture=candidate;journal({'kind':'fixture-observed','at':utc(),'fixture':raw})
            if now>=next_sample:
                if observations>=MAX_SAMPLES:failure=failure or 'identity-sample-limit'
                else:
                    observations+=1
                    scan=collect_owned(fixture,known) if fixture else {'owned':[],'unreadableEnvironmentCount':0,'fixtureUnavailable':True}
                    unavailable+=scan['unreadableEnvironmentCount']
                    journal({'kind':'identity-observation','at':utc(),'elapsedSeconds':elapsed,'launcher':read_identity(child.pid),'fixture':scan,'known':final_identities(known)})
                next_sample=time.monotonic()+SAMPLE_INTERVAL_SECONDS # no catch-up bursts
            action=supervision_action(elapsed,code,term_at,kill_at,failure)
            if action=='term':
                failure=failure or 'launcher-timeout';term_at=elapsed
                journal({'kind':'before-signal','at':utc(),'signal':'SIGTERM','elapsedSeconds':elapsed,'identity':read_identity(child.pid),'failure':failure})
                try:signal.pidfd_send_signal(pidfd,signal.SIGTERM)
                except ProcessLookupError:pass
                signals.append({'signal':'SIGTERM','elapsedSeconds':elapsed,'scope':'exact launcher pidfd'})
            elif action=='kill':
                kill_at=elapsed;journal({'kind':'before-signal','at':utc(),'signal':'SIGKILL','elapsedSeconds':elapsed,'identity':read_identity(child.pid)})
                try:signal.pidfd_send_signal(pidfd,signal.SIGKILL)
                except ProcessLookupError:pass
                signals.append({'signal':'SIGKILL','elapsedSeconds':elapsed,'scope':'exact launcher pidfd'})
            elif action=='unsettled':
                failure=failure or 'launcher-unsettled';break
            for key,_ in selector.select(timeout=0.1):
                data=os.read(key.fileobj.fileno(),65536)
                if not data:selector.unregister(key.fileobj);continue
                name=key.data;remaining=max(0,MAX_OUTPUT_BYTES-output_counts[name]);buffers[name].write(data[:remaining]);output_counts[name]+=len(data)
                if len(data)>remaining:truncations[name]=True;failure=failure or 'launcher-output-limit'
            if action=='settled' and not selector.get_map():break
        if fixture:
            end=time.monotonic()+FINAL_OBSERVATION_SECONDS
            while True:
                collect_owned(fixture,known)
                statuses=final_identities(known)
                living=[x for x in statuses if x['current']['status']=='observed' and x['current']['state']!='Z']
                if not living or time.monotonic()>=end:break
                time.sleep(0.1)
        final=final_identities(known)
        journal({'kind':'final-known-identities','at':utc(),'identities':final})
        data=None
        if report.exists():
            raw=ordinary(report);data=json.loads(raw);result['driverReport']={'sha256':digest(raw),'bytes':len(raw),'passed':data.get('passed')}
        else:failure=failure or 'driver-report-missing'
        after={str(path):{'bytes':len(raw),'sha256':digest(raw)} for path in (source,launcher,scenario) for raw in [ordinary(path)]}
        if after!=before:failure=failure or 'source-changed'
        unresolved=[x for x in final if x['current']['status']=='unavailable' or x['current']['status']=='observed' and x['current']['state']!='Z']
        if unresolved:failure=failure or 'known-owned-process-unresolved'
        result.update({'launcherExitCode':child.poll(),'failure':failure,'signals':signals,'wrapperSignals':wrapper_signals,'identities':final,
            'sourceAfter':after,'samples':observations,'unreadableEnvironmentObservations':unavailable,'outputBytesObserved':output_counts,'outputTruncated':truncations,
            'passed':child.returncode==0 and data is not None and data.get('passed') is True and failure is None and not signals})
    except BaseException as error:
        result['failure']='wrapper-error:'+type(error).__name__
        # Fail durably before the only fallback action. No controller/browser signal.
        try:journal({'kind':'wrapper-error','at':utc(),'errorType':type(error).__name__,'launcher':read_identity(child.pid) if child else None,'known':final_identities(known)})
        except Exception:pass
        if child is not None and child.stdin and not child.stdin.closed:
            child.stdin.close() # EOF gate refuses exec when pidfd acquisition failed.
        if child is not None and pidfd is None:
            try:child.wait(timeout=KILL_SETTLEMENT_SECONDS)
            except subprocess.TimeoutExpired:pass
        if child is not None and pidfd is not None and child.poll() is None:
            try:signal.pidfd_send_signal(pidfd,signal.SIGTERM);signals.append({'signal':'SIGTERM','scope':'exact launcher pidfd','reason':'wrapper-error'})
            except ProcessLookupError:pass
            try:child.wait(timeout=TERM_SECONDS)
            except subprocess.TimeoutExpired:
                try:signal.pidfd_send_signal(pidfd,signal.SIGKILL);signals.append({'signal':'SIGKILL','scope':'exact launcher pidfd','reason':'wrapper-error'})
                except ProcessLookupError:pass
                try:child.wait(timeout=KILL_SETTLEMENT_SECONDS)
                except subprocess.TimeoutExpired:pass
        result.update({'signals':signals,'wrapperSignals':wrapper_signals,'identities':final_identities(known),'launcherExitCode':child.poll() if child else None})
    finally:
        settlement_errors=[]
        def settle(name,operation):
            try:operation()
            except BaseException as error:
                settlement_errors.append({'operation':name,'errorType':type(error).__name__})
                result['passed']=False
        for number,handler in previous_handlers.items():settle('signal-handler-restore',lambda number=number,handler=handler:signal.signal(number,handler))
        for name,value in buffers.items():
            settle(name+'-flush',value.flush)
            settle(name+'-fsync',lambda value=value:os.fsync(value.fileno()))
            settle(name+'-close',value.close)
        settle('selector-close',selector.close)
        if child:
            for name,stream in [('stdin',child.stdin),('stdout',child.stdout),('stderr',child.stderr)]:
                if stream:settle('child-'+name+'-close',stream.close)
        if pidfd is not None:settle('pidfd-close',lambda:os.close(pidfd))
        settle('journal-fsync',lambda:os.fsync(journal_fd))
        settle('journal-close',lambda:os.close(journal_fd))
        result['settlementErrors']=settlement_errors
        result['finishedAt']=utc();result['elapsedSeconds']=time.monotonic()-started
        result['receiptDurable']=True
        try:write_json(args.output/'wrapper-result.json',result)
        except BaseException as error:
            result['passed']=False;result['receiptDurable']=False
            print(json.dumps({'passed':False,'receiptDurable':False,'failure':'final-receipt-write-failed','errorType':type(error).__name__}),file=sys.stderr)

    print(json.dumps({'passed':result['passed'],'failure':result.get('failure'),'receipt':str(args.output/'wrapper-result.json')}))
    return 0 if result['passed'] else 1


if __name__=='__main__':
    raise SystemExit(main())
