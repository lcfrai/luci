#!/usr/bin/env python3
"""Controlled wrapper tests. Popen and signal registration are always mocked.

Run only in an authorized test window: python3 -B wrapper.test.py
The frozen sampler is hashed but never imported or executed. Fixture files are
temporary controller doubles; no installed controller, Docker or browser runs.
"""
import contextlib
import hashlib
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
from unittest import mock

sys.dont_write_bytecode = True
HERE = Path(__file__).resolve().parent
WRAPPER = HERE / 'run-with-observer.py'
SAMPLER = HERE / 'runtime/four-project-queue/sample-linux-owned.py'
WRAPPER_HELPER_COMMIT = '81f2ba5b194a81a4f2ad167e466793f59162b387'
SAMPLER_HELPER_COMMIT = '5d306b6dae3cc9d0a4d9cbf95f8142c37958964f'
BASE_WRAPPER_SHA256 = '5a4d13eefb245a5040e727d689a61aad5e18c2a945f2c6cee10658d6810eebdd'
SAMPLER_SHA256 = '1eb5f48c3b9b1e248e15ed2d058181b96e36485d914304397f35427c9cac725d'


def load_wrapper():
    spec = importlib.util.spec_from_file_location('recovery_observer_wrapper', WRAPPER)
    module = importlib.util.module_from_spec(spec)
    with mock.patch.object(subprocess, 'Popen', side_effect=AssertionError('import started a process')):
        spec.loader.exec_module(module)
    return module


wrapper = load_wrapper()


class ProcessDouble:
    def __init__(self, role, waits, signal_error=False):
        self.role = role
        # A real existing PID permits read-only /proc stat; no child is created.
        self.pid = os.getpid() if role == 'driver' else os.getpid() + 1
        self.waits = list(waits)
        self.wait_timeouts = []
        self.returncode = None
        self.signals = []
        self.signal_error = signal_error

    def poll(self):
        return self.returncode

    def wait(self, timeout):
        self.wait_timeouts.append(timeout)
        value = self.waits.pop(0) if self.waits else 0
        if value == 'timeout':
            raise subprocess.TimeoutExpired(self.role, timeout)
        self.returncode = value
        return value

    def send_signal(self, signum):
        self.signals.append(signum)
        if self.signal_error:
            raise PermissionError('controlled signal refusal')

    def terminate(self):
        self.send_signal(signal.SIGTERM)

    def kill(self):
        self.send_signal(signal.SIGKILL)


class WrapperTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='rw-')
        self.addCleanup(self.temp.cleanup)
        self.base = Path(self.temp.name)
        self.output = self.base / 'obs'
        self.pointer = self.base / 'obs-instance-path.txt'
        self.cost = self.base / 'obs-observer-self-cost.json'
        self.lifecycle = self.base / 'obs-lifecycle.jsonl'
        self.binding = self.base / 'binding.json'
        self.binding.write_text('{"controlledWrapperInput":true}\n')
        self.calls = []
        self.processes = []
        self.pointer_before = None
        self.handlers = {}

    def run_wrapper(self, extra=(), *, driver_waits=(0,), observer_waits=(0,),
                    publish_pointer=True, publish_cost=True, signal_error=False,
                    launch_failure=None, remove_window=False, image=None, helper='1' * 40, remove_binding=False):
        driver_args = ['--window-granted', '--repo', str(self.base),
                       '--expected-image-id', image or wrapper.EXPECTED_IMAGE_ID,
                       '--helper-commit', helper, '--source-binding', str(self.binding)]
        if remove_binding:
            driver_args = driver_args[:-2]
        if remove_window:
            driver_args.remove('--window-granted')
        args = [str(WRAPPER), '--observation-dir', str(self.output),
                '--fixture-base', str(self.base), '--', *driver_args, *extra]

        def popen(command, *, cwd):
            role = 'driver' if not self.calls else 'observer'
            self.calls.append((list(command), Path(cwd)))
            if role == launch_failure:
                raise OSError('controlled launch refusal')
            process = ProcessDouble(role, driver_waits if role == 'driver' else observer_waits,
                                    signal_error=signal_error and role == 'driver')
            self.processes.append(process)
            if role == 'driver' and publish_pointer:
                fixture = Path(command[command.index('--fixture-root') + 1]) / 'luci-check-recovery-mock'
                # Both scopes exist. Only the driver supplies the selected team.
                gateway = fixture / 'state/instance/instance-id'
                team = fixture / 'state/control-plane/teams/team-double/runtime/state/instance/instance-id'
                for path, value in ((gateway, 'gateway-double'), (team, 'team-double')):
                    path.parent.mkdir(parents=True, exist_ok=True)
                    path.write_text(value + '\n')
                pointer = Path(command[command.index('--observer-instance-pointer') + 1])
                with pointer.open('x') as stream:
                    stream.write(str(team) + '\n')
                state = pointer.stat()
                self.pointer_before = (pointer.read_bytes(), state.st_ino, state.st_mtime_ns)
            if role == 'observer' and publish_cost:
                # Double only: deliberately not a real sampler cost receipt.
                self.cost.write_text('{"controlledTestDouble":true}\n')
            return process

        self.stdout, self.stderr = io.StringIO(), io.StringIO()
        with contextlib.ExitStack() as stack:
            stack.enter_context(mock.patch.object(sys, 'argv', args))
            stack.enter_context(mock.patch.object(wrapper.subprocess, 'Popen', side_effect=popen))
            stack.enter_context(mock.patch.object(wrapper.signal, 'signal', side_effect=lambda s, h: self.handlers.update({s: h})))
            stack.enter_context(mock.patch.object(wrapper.os, 'umask', return_value=0o022))
            stack.enter_context(mock.patch.object(Path, 'glob', side_effect=AssertionError('instance discovery forbidden')))
            stack.enter_context(mock.patch.object(Path, 'rglob', side_effect=AssertionError('instance discovery forbidden')))
            stack.enter_context(contextlib.redirect_stdout(self.stdout))
            stack.enter_context(contextlib.redirect_stderr(self.stderr))
            return wrapper.main()

    def assert_refused(self, *args, **kwargs):
        with self.assertRaises(SystemExit) as raised:
            self.run_wrapper(*args, **kwargs)
        self.assertEqual(raised.exception.code, 2)
        self.assertEqual(self.calls, [])

    def test_import_has_no_launch_and_exact_frozen_sampler(self):
        load_wrapper()
        self.assertEqual(hashlib.sha256(SAMPLER.read_bytes()).hexdigest(), SAMPLER_SHA256)

    def test_dedicated_driver_frozen_sampler_and_exact_team_pointer(self):
        self.assertEqual(self.run_wrapper(), 0)
        driver, observer = [entry[0] for entry in self.calls]
        self.assertEqual(driver[:2], ['node', str(HERE / 'installed-recovery.mjs')])
        self.assertEqual(observer[3], str(SAMPLER))
        self.assertEqual(driver.count('--observer-instance-pointer'), 1)
        driver_pointer = driver[driver.index('--observer-instance-pointer') + 1]
        self.assertEqual(observer[observer.index('--instance-path-file') + 1], driver_pointer)
        self.assertIn('/state/control-plane/teams/team-double/runtime/state/instance/instance-id', self.pointer.read_text())
        state = self.pointer.stat()
        self.assertEqual((self.pointer.read_bytes(), state.st_ino, state.st_mtime_ns), self.pointer_before)
        self.assertEqual([cwd for _, cwd in self.calls], [self.base, self.base])
        self.assertIn('--include-checks', observer)
        self.assertEqual(observer[observer.index('--image-id') + 1], wrapper.EXPECTED_IMAGE_ID)
        self.assertEqual(observer[observer.index('--driver-pid') + 1], str(os.getpid()))

    def test_workflow_and_total_budgets_are_explicit(self):
        self.assertEqual((wrapper.PREPARATION_SECONDS, wrapper.WORKFLOW_SECONDS,
                          wrapper.CLEANUP_SECONDS, wrapper.DRIVER_TOTAL_SECONDS), (120, 600, 180, 900))
        self.assertEqual(self.run_wrapper(), 0)
        observer = self.calls[1][0]
        self.assertEqual(observer[observer.index('--seconds') + 1], '915')
        rows = [json.loads(line) for line in self.lifecycle.read_text().splitlines()]
        self.assertEqual(rows[0]['workloadSeconds'], 600)
        self.assertEqual(rows[0]['totalSeconds'], 900)
        self.assertEqual(rows[-1]['event'], 'wrapper-settled')

    def test_root_window_required_before_either_child(self):
        self.assert_refused(remove_window=True)

    def test_caller_pointer_override_refused(self):
        self.assert_refused(['--observer-instance-pointer', str(self.base / 'other')])

    def test_equals_pointer_override_refused(self):
        self.assert_refused(['--observer-instance-pointer=other'])

    def test_caller_fixture_root_override_refused(self):
        self.assert_refused(['--fixture-root', str(self.base)])

    def test_equals_fixture_root_override_refused(self):
        self.assert_refused(['--fixture-root=other'])

    def test_wrong_image_refused(self):
        self.assert_refused(image='sha256:' + '0' * 64)

    def test_duplicate_image_refused(self):
        self.assert_refused(['--expected-image-id', wrapper.EXPECTED_IMAGE_ID])

    def test_equals_image_refused(self):
        self.assert_refused(['--expected-image-id=' + wrapper.EXPECTED_IMAGE_ID])

    def test_duplicate_repo_refused(self):
        self.assert_refused(['--repo', str(self.base)])

    def test_malformed_helper_refused_before_children(self):
        self.assert_refused(helper='main')

    def test_duplicate_helper_refused_before_children(self):
        self.assert_refused(['--helper-commit', '2' * 40])

    def test_missing_binding_refused_before_children(self):
        self.assert_refused(remove_binding=True)

    def test_duplicate_binding_refused_before_children(self):
        self.assert_refused(['--source-binding', str(self.binding)])

    def test_absent_binding_refused_before_children(self):
        self.binding.unlink()
        self.assert_refused()

    def test_linked_binding_refused_before_children(self):
        other = self.base / 'other-binding.json'
        self.binding.rename(other)
        self.binding.symlink_to(other)
        self.assert_refused()

    def test_explicit_product_source_is_forwarded_unchanged(self):
        expected = '3' * 40
        self.assertEqual(self.run_wrapper(['--expected-source', expected]), 0)
        command = self.calls[0][0]
        self.assertEqual(command[command.index('--expected-source') + 1], expected)

    def test_malformed_product_source_refused_before_children(self):
        self.assert_refused(['--expected-source', 'main'])

    def test_missing_product_source_value_refused_before_children(self):
        self.assert_refused(['--expected-source'])

    def test_duplicate_product_source_refused_before_children(self):
        self.assert_refused(['--expected-source', '3' * 40, '--expected-source', '4' * 40])

    def test_equals_product_source_refused_before_children(self):
        self.assert_refused(['--expected-source=' + '3' * 40])

    def test_fresh_observation_directory_required(self):
        self.output.mkdir()
        self.assert_refused()

    def test_existing_driver_pointer_never_overwritten(self):
        self.pointer.write_text('retained-controller-pointer\n')
        self.assert_refused()
        self.assertEqual(self.pointer.read_text(), 'retained-controller-pointer\n')

    def test_dangling_driver_pointer_refused_without_following(self):
        target = self.base / 'absent'
        self.pointer.symlink_to(target)
        self.assert_refused()
        self.assertFalse(target.exists())
        self.assertTrue(self.pointer.is_symlink())

    def test_existing_self_cost_is_preserved(self):
        self.cost.write_text('prior cost\n')
        self.assert_refused()
        self.assertEqual(self.cost.read_text(), 'prior cost\n')

    def test_existing_lifecycle_is_preserved(self):
        self.lifecycle.write_text('prior lifecycle\n')
        self.assert_refused()
        self.assertEqual(self.lifecycle.read_text(), 'prior lifecycle\n')

    def test_missing_pointer_fails_without_discovering_gateway(self):
        self.assertEqual(self.run_wrapper(publish_pointer=False), 1)
        self.assertFalse(self.pointer.exists())
        self.assertIn('No driver-owned team instance pointer', self.stderr.getvalue())

    def test_missing_observer_self_cost_fails(self):
        self.assertEqual(self.run_wrapper(publish_cost=False), 1)
        self.assertIn('self-cost receipt missing', self.stderr.getvalue())

    def test_driver_failure_status_is_preserved(self):
        self.assertEqual(self.run_wrapper(driver_waits=(7, 7)), 7)

    def test_observer_failure_cannot_pass(self):
        self.assertEqual(self.run_wrapper(observer_waits=(9,)), 1)

    def test_total_timeout_has_only_five_second_interrupt_grace(self):
        self.assertEqual(self.run_wrapper(driver_waits=('timeout', 'timeout', -9)), 1)
        driver = self.processes[0]
        self.assertEqual(driver.wait_timeouts[1:], [5, 5])
        self.assertEqual(driver.signals, [signal.SIGTERM, signal.SIGKILL])
        self.assertIn('Docker cleanup unknown', self.stderr.getvalue())

    def test_observer_settlement_timeout_is_bounded_and_fails(self):
        self.assertEqual(self.run_wrapper(observer_waits=('timeout', 'timeout', -9)), 1)
        observer = self.processes[1]
        self.assertEqual(observer.wait_timeouts, [30, 5, 5])
        self.assertEqual(observer.signals, [signal.SIGTERM, signal.SIGKILL])

    def test_observer_cleanup_runs_when_driver_signal_fails(self):
        self.assertEqual(self.run_wrapper(driver_waits=('timeout',), signal_error=True), 1)
        self.assertEqual(self.processes[1].wait_timeouts, [30])
        self.assertIn('Driver cleanup exception', self.stderr.getvalue())

    def test_driver_launch_failure_starts_no_observer(self):
        self.assertEqual(self.run_wrapper(launch_failure='driver'), 1)
        self.assertEqual(len(self.calls), 1)

    def test_observer_launch_failure_cleans_owned_driver(self):
        self.assertEqual(self.run_wrapper(launch_failure='observer'), 1)
        self.assertEqual(self.processes[0].signals, [signal.SIGTERM])
        self.assertLessEqual(self.processes[0].wait_timeouts[0], 180)


if __name__ == '__main__':
    transcript = io.StringIO()
    outcome = unittest.TextTestRunner(stream=transcript, verbosity=2).run(
        unittest.defaultTestLoader.loadTestsFromTestCase(WrapperTest))
    scope = ('Controlled Python tests only; subprocess.Popen and signal registration mocked. '
             'Sampler hashed, never imported/executed. No installed controller, Docker, browser, '
             'compiler, external model, or real child process workload.\n')
    result = {
        'status': 'passed' if outcome.wasSuccessful() else 'failed',
        'testsRun': True, 'tests': outcome.testsRun,
        'failures': len(outcome.failures), 'errors': len(outcome.errors),
        'skipped': len(outcome.skipped), 'scope': scope.strip(),
        'wrapperHelperCommit': WRAPPER_HELPER_COMMIT,
        'samplerHelperCommit': SAMPLER_HELPER_COMMIT,
        'origin': {
            'wrapper': {'commit': WRAPPER_HELPER_COMMIT, 'path': 'scripts/acceptance/task-dependencies/run-with-observer.py', 'sha256': BASE_WRAPPER_SHA256},
            'sampler': {'commit': SAMPLER_HELPER_COMMIT, 'path': 'scripts/acceptance/four-project-queue/sample-linux-owned.py', 'sha256': SAMPLER_SHA256},
        },
        'files': {str(path.relative_to(HERE.parent)): hashlib.sha256(path.read_bytes()).hexdigest()
                  for path in (WRAPPER, SAMPLER, Path(__file__).resolve())},
    }
    print(json.dumps(result, sort_keys=True))
    print(transcript.getvalue(), end='')
    raise SystemExit(0 if outcome.wasSuccessful() else 1)
