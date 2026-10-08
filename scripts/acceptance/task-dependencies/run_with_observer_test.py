"""Subprocess cleanup fault injection; never starts Node, Docker or an observer."""
import contextlib
import importlib.util
import io
import json
import sys
import os
from pathlib import Path
import signal
import subprocess
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('observer_wrapper', Path(__file__).with_name('run-with-observer.py'))
wrapper = importlib.util.module_from_spec(spec)
spec.loader.exec_module(wrapper)


class Child:
    def __init__(self, waits=(), signal_race=False, kill_race=False):
        self.pid = os.getpid()  # Only /proc metadata is read; no actual signaling.
        self.returncode = None
        self.waits = list(waits)
        self.events = []
        self.signal_race, self.kill_race = signal_race, kill_race

    def poll(self):
        return self.returncode

    def wait(self, timeout):
        self.events.append(('wait', timeout))
        value = self.waits.pop(0) if self.waits else 0
        if isinstance(value, BaseException):
            raise value
        self.returncode = value
        return value

    def send_signal(self, value):
        self.events.append(('signal', value))
        if self.signal_race:
            raise ProcessLookupError()

    def terminate(self):
        self.send_signal(signal.SIGTERM)

    def kill(self):
        self.events.append(('kill',))
        if self.kill_race:
            raise ProcessLookupError()


@unittest.skipUnless(os.path.isdir('/proc'), 'Linux /proc metadata required')
class OwnedCleanupTests(unittest.TestCase):
    def run_fault(self, children, thread_failure=False):
        with tempfile.TemporaryDirectory(prefix='lq-') as temp:
            # Popen is mocked below. Supply a private placeholder so these
            # cleanup tests do not depend on an external checkout/environment.
            repo = Path(temp) / 'repo'
            sampler = repo / 'scripts/acceptance/four-project-queue/sample-linux-owned.py'
            sampler.parent.mkdir(parents=True)
            sampler.write_text('# Controlled preflight placeholder; never executed.\n')
            argv = ['wrapper', '--fixture-base', temp, '--observation-dir', str(Path(temp) / 'report'),
                    '--', '--expected-image-id', 'sha256:' + 'a' * 64]
            with contextlib.ExitStack() as stack:
                stack.enter_context(patch.object(wrapper.sys, 'argv', argv))
                stack.enter_context(patch.dict(wrapper.os.environ, {'LUCI_ACCEPTANCE_REPO': str(repo)}))
                stack.enter_context(patch.object(wrapper.signal, 'signal'))
                launches = stack.enter_context(patch.object(wrapper.subprocess, 'Popen', side_effect=children))
                stack.enter_context(contextlib.redirect_stdout(io.StringIO()))
                stack.enter_context(contextlib.redirect_stderr(io.StringIO()))
                # No thread is needed to discover a nonexistent fixture.
                stack.enter_context(patch.object(wrapper.threading.Thread, 'start',
                                                  side_effect=RuntimeError('thread unavailable') if thread_failure else None))
                result = wrapper.main()
                if len(launches.call_args_list)>1:self.assertIn('--include-checks',launches.call_args_list[1].args[0])
                self.journal = [json.loads(line) for line in (Path(temp) / 'report-lifecycle.jsonl').read_text().splitlines()]
                return result

    def test_observer_start_failure_stops_and_reaps_owned_driver(self):
        driver = Child()
        self.assertEqual(self.run_fault([driver, OSError('observer unavailable')]), 1)
        self.assertIn(('signal', signal.SIGTERM), driver.events)
        self.assertIn(('wait', 180), driver.events)

    def test_watcher_start_failure_cannot_skip_both_children(self):
        driver, observer = Child(), Child()
        self.assertEqual(self.run_fault([driver, observer], thread_failure=True), 1)
        self.assertIn(('signal', signal.SIGTERM), driver.events)
        self.assertIn(('wait', 30), observer.events)

    def test_signal_lookup_races_still_reap_driver_and_observer(self):
        timeout = lambda: subprocess.TimeoutExpired('controlled-child', 1)
        driver = Child([timeout(), timeout(), 0], kill_race=True)
        observer = Child([timeout(), 0], signal_race=True)
        self.assertEqual(self.run_fault([driver, observer]), 1)
        self.assertIn(('kill',), driver.events)
        self.assertEqual(driver.events[0][0],'wait')
        self.assertLessEqual(driver.events[0][1],wrapper.DRIVER_TOTAL_SECONDS)
        self.assertNotIn(('wait',180),driver.events)
        self.assertIn('wrapper-wait-timeout',[row['event'] for row in self.journal])
        self.assertIn('driver-forced-exit-request',[row['event'] for row in self.journal])
        self.assertEqual(self.journal[0]['totalSeconds'],1200)
        self.assertIn(('wait', 5), driver.events)
        self.assertIn(('signal', signal.SIGTERM), observer.events)
        self.assertIn(('wait', 5), observer.events)


@unittest.skipUnless(sys.platform == 'linux', 'Linux resource units and /proc required')
class OwnProcessCostTests(unittest.TestCase):
    def test_same_process_cost_preserves_success_and_failure_exit_codes(self):
        for code in (0, 7):
            with self.subTest(code=code), tempfile.TemporaryDirectory(prefix='observer-cost-') as temp:
                root = Path(temp)
                script = root / 'tiny-observer.py'
                script.write_text('raise SystemExit(' + str(code) + ')\n')
                cost = root / 'own.json'
                with patch.object(sys,'argv',['launcher',str(script),str(cost)]), patch.object(signal,'signal'), self.assertRaises(SystemExit) as raised:
                    exec(wrapper.SAMPLER_LAUNCHER,{'__name__':'__main__'})
                self.assertEqual(raised.exception.code,code)
                observed = json.loads(cost.read_text())
                self.assertEqual(observed['exitCode'], code)
                self.assertEqual(observed['status'], 'completed' if code == 0 else 'failed')
                self.assertGreater(observed['maxRSSBytes'], 0)
                self.assertGreaterEqual(observed['userCPUSeconds'], 0)
                self.assertIn('excludes spawned Docker CLI', observed['scope'])
                self.assertEqual(cost.stat().st_mode & 0o777, 0o600)


if __name__ == '__main__':
    unittest.main()
