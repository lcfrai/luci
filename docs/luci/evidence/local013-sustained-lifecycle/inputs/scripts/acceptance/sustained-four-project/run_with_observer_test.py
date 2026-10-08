"""Mocked lifecycle faults and in-process self-cost checks; no child launches."""
import contextlib
import importlib.util
import io
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch


spec = importlib.util.spec_from_file_location('sustained_observer_wrapper', Path(__file__).with_name('run-with-observer.py'))
wrapper = importlib.util.module_from_spec(spec)
spec.loader.exec_module(wrapper)


def timeout():
    return subprocess.TimeoutExpired('controlled-child', 1)


class Child:
    def __init__(self, waits=(), signal_error=None, kill_error=None, poll_error=None):
        self.pid = os.getpid()  # Read only this process's metadata; never signal it.
        self.returncode = None
        self.waits = list(waits)
        self.events = []
        self.signal_error, self.kill_error, self.poll_error = signal_error, kill_error, poll_error

    def poll(self):
        if self.poll_error:
            raise self.poll_error
        return self.returncode

    def wait(self, timeout):
        self.events.append(('wait', timeout))
        value = self.waits.pop(0) if self.waits else 0
        if callable(value):
            value = value()
        if isinstance(value, BaseException):
            raise value
        self.returncode = value
        return value

    def send_signal(self, value):
        self.events.append(('signal', value))
        if self.signal_error:
            raise self.signal_error

    def kill(self):
        self.events.append(('kill',))
        if self.kill_error:
            raise self.kill_error


@unittest.skipUnless(sys.platform == 'linux', 'Linux /proc metadata required')
class OwnedCleanupTests(unittest.TestCase):
    def run_fault(self, children, *, thread_failure=False, join_failure=False, fixture=None,
                  receipts=False, elapsed_at_observer=0, read_failure=None, write_failure=None,
                  short_write=None, sync_failure=None, forwarded=(), existing_journal=False):
        self.journal, self.launches, self.handlers, self.closed = [], [], {}, []
        self.clock = 0
        self.stderr = io.StringIO()
        with tempfile.TemporaryDirectory(prefix='ls-') as temp:
            root = Path(temp)
            repo = root / 'repo'
            sampler = repo / 'scripts/acceptance/four-project-queue/sample-linux-owned.py'
            sampler.parent.mkdir(parents=True)
            sampler.write_text('# Controlled preflight placeholder; never executed.\n')
            output = root / 'report'
            journal_path = root / 'report-lifecycle.jsonl'
            if existing_journal:
                journal_path.write_text('{"preserved":true}\n')
            argv = ['wrapper', '--fixture-base', temp, '--observation-dir', str(output),
                    '--', '--expected-image-id', 'sha256:' + 'a' * 64, *forwarded]
            child_iter = iter(children)
            real_read, real_write, real_sync, real_close = Path.read_text, os.write, os.fsync, os.close
            self.write_calls, self.sync_calls = 0, 0

            def launch(command, **kwargs):
                self.launches.append((command, kwargs))
                if command[0] == 'node':
                    self.fixture_parent = Path(command[command.index('--fixture-root') + 1])
                    if fixture:
                        instance = self.fixture_parent / 'luci-sustained-owned/state/instance/instance-id'
                        instance.parent.mkdir(parents=True)
                        if fixture == 'symlink':
                            target = root / 'outside-instance'
                            target.write_text('outside-owned-scope\n')
                            instance.symlink_to(target)
                        else:
                            instance.write_text('controlled-instance\n')
                        self.exact_instance = str(instance)
                        if fixture == 'multiple':
                            another = self.fixture_parent / 'luci-sustained-other/state/instance/instance-id'
                            another.parent.mkdir(parents=True)
                            another.write_text('second-instance\n')
                else:
                    self.clock = elapsed_at_observer
                    if receipts:
                        (root / 'report-observer-self-cost.json').write_text('{}\n')
                value = next(child_iter)
                if isinstance(value, BaseException):
                    raise value
                return value

            def read(path, *args, **kwargs):
                if read_failure and str(path) == read_failure:
                    raise OSError('controlled metadata read failure')
                return real_read(path, *args, **kwargs)

            def write(fd, data):
                self.write_calls += 1
                if self.write_calls == write_failure:
                    raise OSError('controlled journal write failure')
                if self.write_calls == short_write:
                    return 0
                return real_write(fd, data)

            def sync(fd):
                self.sync_calls += 1
                if self.sync_calls == sync_failure:
                    raise OSError('controlled journal sync failure')
                return real_sync(fd)

            def close(fd):
                self.closed.append(fd)
                return real_close(fd)

            def install_handler(signum, callback):
                previous = self.handlers.get(signum, signal.SIG_DFL)
                self.handlers[signum] = callback
                return previous

            class Thread:
                ident = None

                def __init__(self, target, daemon):
                    self.target = target

                def start(self):
                    if thread_failure:
                        raise RuntimeError('thread unavailable')
                    if fixture:
                        self.ident = 1
                        self.target()

                def join(self, timeout):
                    if join_failure:
                        raise RuntimeError('join unavailable')

            old_umask = os.umask(0o077)
            try:
                with contextlib.ExitStack() as stack:
                    stack.enter_context(patch.object(wrapper.sys, 'argv', argv))
                    stack.enter_context(patch.dict(wrapper.os.environ, {'LUCI_ACCEPTANCE_REPO': str(repo)}))
                    stack.enter_context(patch.object(wrapper.signal, 'signal', side_effect=install_handler))
                    stack.enter_context(patch.object(wrapper.subprocess, 'Popen', side_effect=launch))
                    stack.enter_context(patch.object(wrapper.threading, 'Thread', Thread))
                    stack.enter_context(patch.object(wrapper.time, 'monotonic', side_effect=lambda: self.clock))
                    stack.enter_context(patch.object(Path, 'read_text', read))
                    stack.enter_context(patch.object(wrapper.os, 'write', side_effect=write))
                    stack.enter_context(patch.object(wrapper.os, 'fsync', side_effect=sync))
                    stack.enter_context(patch.object(wrapper.os, 'close', side_effect=close))
                    stack.enter_context(contextlib.redirect_stdout(io.StringIO()))
                    stack.enter_context(contextlib.redirect_stderr(self.stderr))
                    result = wrapper.main()
                self.journal_text = journal_path.read_text()
                self.journal = [json.loads(line) for line in self.journal_text.splitlines()]
                self.journal_mode = journal_path.stat().st_mode & 0o777
                pointer = root / 'report-instance-path.txt'
                self.pointer = pointer.read_text().strip() if pointer.exists() else None
                self.fixture_retained = self.fixture_parent.exists() if self.launches else False
                return result
            finally:
                os.umask(old_umask)

    def events(self):
        return [row['event'] for row in self.journal]

    def test_success_uses_exact_worker_identity_and_one_durable_total_budget(self):
        driver, observer = Child(), Child()
        self.assertEqual(self.run_fault([driver, observer], fixture='ordinary', receipts=True,
                                       forwarded=['--controlled-marker', 'not-for-the-journal']), 0)
        command = self.launches[1][0]
        self.assertEqual(command[command.index('--seconds') + 1], '2115')
        self.assertNotIn('--include-checks', command)
        self.assertEqual(command[command.index('--driver-pid') + 1], str(driver.pid))
        self.assertEqual(self.pointer, self.exact_instance)
        self.assertEqual(self.journal[0]['preparationSeconds'], 120)
        self.assertEqual(self.journal[0]['workloadSeconds'], 1800)
        self.assertEqual(self.journal[0]['cleanupSeconds'], 180)
        self.assertEqual(self.journal[0]['totalSeconds'], 2100)
        self.assertEqual(self.journal[0]['observerSeconds'], 2115)
        self.assertEqual(self.journal[-1]['returnCode'], 0)
        self.assertEqual(self.journal_mode, 0o600)
        self.assertEqual(self.sync_calls, len(self.journal) + 1)
        self.assertNotIn('not-for-the-journal', self.journal_text)
        self.assertNotIn(self.exact_instance, self.journal_text)
        self.assertEqual(driver.events[0], ('wait', 2100))
        self.assertTrue(all(value == signal.SIG_DFL for value in self.handlers.values()))

    def test_observer_start_failure_stops_and_reaps_owned_driver(self):
        driver = Child()
        self.assertEqual(self.run_fault([driver, OSError('observer unavailable')]), 1)
        self.assertEqual(driver.events, [('signal', signal.SIGTERM), ('wait', 180)])
        self.assertIn('wrapper-failure', self.events())
        self.assertTrue(self.fixture_retained)

    def test_watcher_start_failure_cannot_skip_both_children(self):
        driver, observer = Child(), Child()
        self.assertEqual(self.run_fault([driver, observer], thread_failure=True), 1)
        self.assertEqual(driver.events, [('signal', signal.SIGTERM), ('wait', 180)])
        self.assertEqual(observer.events, [('wait', 30)])

    def test_total_timeout_gets_only_term_five_kill_reap_five(self):
        driver = Child([timeout(), timeout(), 0], kill_error=ProcessLookupError())
        observer = Child([timeout(), 0], signal_error=ProcessLookupError())
        self.assertEqual(self.run_fault([driver, observer]), 1)
        self.assertEqual(driver.events, [('wait', 2100), ('signal', signal.SIGTERM),
                                         ('wait', 5), ('kill',), ('wait', 5)])
        self.assertIn('wrapper-wait-timeout', self.events())
        self.assertIn('driver-forced-exit-request', self.events())
        self.assertEqual(observer.events, [('wait', 30), ('signal', signal.SIGTERM), ('wait', 5)])
        self.assertIn('Docker cleanup unknown', self.stderr.getvalue())

    def test_early_failure_cleanup_cannot_extend_remaining_total_budget(self):
        driver = Child([timeout(), 0])
        self.assertEqual(self.run_fault([driver, OSError('observer unavailable')], elapsed_at_observer=2098), 1)
        self.assertEqual(driver.events, [('signal', signal.SIGTERM), ('wait', 2), ('kill',), ('wait', 5)])
        self.assertNotIn('wrapper-wait-timeout', self.events())

    def test_timeout_before_main_wait_uses_early_failure_cleanup(self):
        driver = Child()
        self.assertEqual(self.run_fault([driver, timeout()]), 1)
        self.assertIn(('wait', 180), driver.events)
        self.assertNotIn('wrapper-wait-timeout', self.events())

    def test_early_signal_interrupts_main_wait_and_bounds_cleanup(self):
        def interrupt():
            self.handlers[signal.SIGTERM](signal.SIGTERM, None)
        driver, observer = Child([interrupt, 0]), Child()
        self.assertEqual(self.run_fault([driver, observer]), 1)
        self.assertIn('wrapper-signal', self.events())
        self.assertIn(('wait', 180), driver.events)
        self.assertEqual(observer.events, [('wait', 30)])

    def test_signal_exception_still_reaps_owned_driver_and_observer(self):
        driver, observer = Child(signal_error=OSError('signal failed')), Child()
        self.assertEqual(self.run_fault([driver, observer], thread_failure=True), 1)
        self.assertIn(('wait', 180), driver.events)
        self.assertEqual(observer.events, [('wait', 30)])
        self.assertIn('Driver signaling exception', self.stderr.getvalue())

    def test_driver_kill_exception_still_attempts_reap_and_observer_cleanup(self):
        driver = Child([timeout(), timeout(), 0], kill_error=OSError('kill failed'))
        observer = Child()
        self.assertEqual(self.run_fault([driver, observer]), 1)
        self.assertEqual(driver.events[-1], ('wait', 5))
        self.assertEqual(observer.events, [('wait', 30)])

    def test_throwing_driver_waits_force_bounded_reap(self):
        driver = Child([OSError('main wait failed'), OSError('cleanup wait failed'), 0])
        observer = Child()
        self.assertEqual(self.run_fault([driver, observer]), 1)
        self.assertEqual(driver.events[-2:], [('kill',), ('wait', 5)])
        self.assertEqual(observer.events, [('wait', 30)])

    def test_throwing_driver_poll_cannot_skip_owned_reaping(self):
        driver, observer = Child(poll_error=OSError('poll failed')), Child()
        self.assertEqual(self.run_fault([driver, observer], thread_failure=True), 1)
        self.assertIn(('wait', 180), driver.events)
        self.assertEqual(observer.events, [('wait', 30)])

    def test_throwing_observer_operations_still_attempt_bounded_reap(self):
        driver = Child()
        observer = Child([OSError('wait failed'), timeout(), 0],
                         signal_error=OSError('signal failed'), kill_error=OSError('kill failed'))
        self.assertEqual(self.run_fault([driver, observer]), 1)
        self.assertEqual(observer.events, [('wait', 30), ('signal', signal.SIGTERM),
                                           ('wait', 5), ('kill',), ('wait', 5)])
        self.assertIn('observer-forced-exit-request', self.events())

    def test_unreaped_children_fail_without_more_waits(self):
        driver, observer = Child([timeout(), timeout(), timeout()]), Child([timeout(), timeout(), timeout()])
        self.assertEqual(self.run_fault([driver, observer]), 1)
        self.assertEqual([event[1] for event in driver.events if event[0] == 'wait'], [2100, 5, 5])
        self.assertEqual([event[1] for event in observer.events if event[0] == 'wait'], [30, 5, 5])
        self.assertIn('Owned driver could not be reaped', self.stderr.getvalue())
        self.assertIn('Owned observer could not be reaped', self.stderr.getvalue())

    def test_join_failure_preserves_completed_child_cleanup(self):
        driver, observer = Child(), Child()
        self.assertEqual(self.run_fault([driver, observer], fixture='ordinary', receipts=True, join_failure=True), 1)
        self.assertEqual(observer.events, [('wait', 30)])
        self.assertEqual(self.journal[-1]['returnCode'], 1)

    def test_multiple_instances_cannot_publish_ambiguous_identity(self):
        driver, observer = Child(), Child()
        self.assertEqual(self.run_fault([driver, observer], fixture='multiple', receipts=True), 1)
        self.assertIsNone(self.pointer)
        self.assertIn(('signal', signal.SIGTERM), driver.events)
        self.assertIn('Multiple fixture instances', self.stderr.getvalue())

    def test_symlink_instance_cannot_publish_unconfined_identity(self):
        driver, observer = Child(), Child()
        self.assertEqual(self.run_fault([driver, observer], fixture='symlink', receipts=True), 1)
        self.assertIsNone(self.pointer)
        self.assertIn('ordinary confined file', self.stderr.getvalue())

    def test_missing_observer_receipt_cannot_claim_success(self):
        self.assertEqual(self.run_fault([Child(), Child()], fixture='ordinary'), 1)
        self.assertIn('Observer self-cost receipt missing', self.stderr.getvalue())
        self.assertEqual(self.journal[-1]['returnCode'], 1)

    def test_nonzero_driver_status_is_preserved(self):
        self.assertEqual(self.run_fault([Child([7]), Child()], fixture='ordinary', receipts=True), 7)
        self.assertEqual(self.journal[-1]['returnCode'], 7)

    def test_own_process_metadata_read_failure_closes_journal_before_launch(self):
        self.assertEqual(self.run_fault([], read_failure='/proc/self/stat'), 1)
        self.assertEqual(self.launches, [])
        self.assertEqual(self.events(), ['wrapper-failure', 'wrapper-settled'])
        self.assertEqual(len(self.closed), 2)

    def test_driver_metadata_read_failure_reaps_driver(self):
        driver = Child()
        self.assertEqual(self.run_fault([driver], read_failure='/proc/' + str(driver.pid) + '/stat'), 1)
        self.assertEqual(len(self.launches), 1)
        self.assertEqual(driver.events, [('signal', signal.SIGTERM), ('wait', 180)])

    def test_initial_journal_write_failure_prevents_launch_and_closes_descriptor(self):
        self.assertEqual(self.run_fault([], write_failure=1), 1)
        self.assertEqual(self.launches, [])
        self.assertEqual(self.journal, [])
        self.assertEqual(len(self.closed), 2)
        self.assertIn('journal failed', self.stderr.getvalue())

    def test_journal_short_write_after_launch_stops_both_owned_children(self):
        driver, observer = Child(), Child()
        self.assertEqual(self.run_fault([driver, observer], short_write=2), 1)
        self.assertEqual(driver.events, [('signal', signal.SIGTERM), ('wait', 180)])
        self.assertEqual(observer.events, [('wait', 30)])
        self.assertEqual(self.events(), ['wrapper-start'])

    def test_journal_sync_failure_cannot_claim_success(self):
        self.assertEqual(self.run_fault([], sync_failure=2), 1)
        self.assertEqual(self.launches, [])
        self.assertIn('journal failed', self.stderr.getvalue())

    def test_existing_journal_rejected_before_launch(self):
        with self.assertRaises(SystemExit) as raised:
            self.run_fault([], existing_journal=True)
        self.assertEqual(raised.exception.code, 2)
        self.assertEqual(self.launches, [])

    def test_forwarded_fixture_override_is_rejected(self):
        for value in ('--fixture-root', '--fixture-root=/tmp/unowned'):
            with self.subTest(value=value), self.assertRaises(SystemExit):
                self.run_fault([], forwarded=[value])
            self.assertEqual(self.launches, [])


@unittest.skipUnless(sys.platform == 'linux', 'Linux resource units and /proc required')
class OwnProcessCostTests(unittest.TestCase):
    def run_launcher(self, script_body, exception, *, signal_number=None):
        with tempfile.TemporaryDirectory(prefix='observer-cost-') as temp:
            root = Path(temp)
            script, cost = root / 'tiny-observer.py', root / 'own.json'
            script.write_text(script_body)
            handlers = {}
            with contextlib.ExitStack() as stack:
                stack.enter_context(patch.object(sys, 'argv', ['launcher', str(script), str(cost)]))
                stack.enter_context(patch.object(signal, 'signal', side_effect=lambda number, handler: handlers.update({number: handler})))
                # In-process execution exercises the real launcher and resource
                # accounting without starting any subprocess or actual sampler.
                if signal_number is not None:
                    import runpy
                    stack.enter_context(patch.object(runpy, 'run_path', side_effect=lambda *args, **kwargs: handlers[signal_number](signal_number, None)))
                with self.assertRaises(exception) as raised:
                    exec(wrapper.SAMPLER_LAUNCHER, {'__name__': '__main__'})
            self.assertEqual(cost.stat().st_mode & 0o777, 0o600)
            receipt = json.loads(cost.read_text())
            self.assertEqual(receipt['pid'], os.getpid())
            self.assertGreater(receipt['maxRSSBytes'], 0)
            self.assertGreaterEqual(receipt['userCPUSeconds'], 0)
            self.assertIn('excludes spawned Docker CLI', receipt['scope'])
            self.assertGreater(receipt['wallSeconds'], 0)
            return receipt, raised.exception

    def test_same_process_cost_preserves_success_and_failure_exit_codes(self):
        for code in (0, 7):
            with self.subTest(code=code):
                receipt, error = self.run_launcher('raise SystemExit(' + str(code) + ')\n', SystemExit)
                self.assertEqual(error.code, code)
                self.assertEqual(receipt['exitCode'], code)
                self.assertEqual(receipt['status'], 'completed' if code == 0 else 'failed')
                self.assertIsNone(receipt['signalNumber'])

    def test_unexpected_sampler_exception_still_writes_self_cost(self):
        receipt, _ = self.run_launcher('raise RuntimeError("controlled failure")\n', RuntimeError)
        self.assertEqual(receipt['exitCode'], 1)
        self.assertEqual(receipt['status'], 'failed')

    def test_observer_termination_records_signal_and_self_cost(self):
        receipt, error = self.run_launcher('# not executed\n', SystemExit, signal_number=signal.SIGTERM)
        self.assertEqual(error.code, 143)
        self.assertEqual(receipt['exitCode'], 143)
        self.assertEqual(receipt['signalNumber'], signal.SIGTERM)
        self.assertEqual(receipt['status'], 'failed')


if __name__ == '__main__':
    unittest.main()
