"""Subprocess cleanup fault injection; never starts Node, Docker or an observer."""
import contextlib
import importlib.util
import io
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
            argv = ['wrapper', '--fixture-base', temp, '--observation-dir', str(Path(temp) / 'report'),
                    '--', '--expected-image-id', 'sha256:' + 'a' * 64]
            with contextlib.ExitStack() as stack:
                stack.enter_context(patch.object(wrapper.sys, 'argv', argv))
                stack.enter_context(patch.object(wrapper.signal, 'signal'))
                stack.enter_context(patch.object(wrapper.subprocess, 'Popen', side_effect=children))
                stack.enter_context(contextlib.redirect_stdout(io.StringIO()))
                stack.enter_context(contextlib.redirect_stderr(io.StringIO()))
                # No thread is needed to discover a nonexistent fixture.
                stack.enter_context(patch.object(wrapper.threading.Thread, 'start',
                                                  side_effect=RuntimeError('thread unavailable') if thread_failure else None))
                return wrapper.main()

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
        self.assertIn(('wait', 5), driver.events)
        self.assertIn(('signal', signal.SIGTERM), observer.events)
        self.assertIn(('wait', 5), observer.events)


if __name__ == '__main__':
    unittest.main()
