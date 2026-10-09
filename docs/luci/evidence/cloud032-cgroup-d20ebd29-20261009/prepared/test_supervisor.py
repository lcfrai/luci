"""Controlled fixtures only: fake child/pidfd/time. No kernel CPU sampling."""
import json
import os
from pathlib import Path
import signal
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import supervisor as s


class FakeGate:
    def __init__(self, owner): self.owner, self.writes, self.closed = owner, [], False
    def write(self, value):
        self.writes.append(value)
        if self.owner.on_gate: self.owner.on_gate()
        return len(value)
    def flush(self): pass
    def close(self):
        self.closed = True
        if not self.writes: self.owner.exit_at, self.owner.code = self.owner.ops.now, 76


class FakeProcess:
    def __init__(self, ops, exit_at=None, code=0, on_gate=None):
        self.ops, self.pid, self.exit_at, self.code = ops, 4242, exit_at, code
        self.on_gate = on_gate
        self.stdin = FakeGate(self)
        self.waits = []
    def wait(self, timeout):
        self.waits.append(timeout)
        if self.exit_at is not None and self.exit_at <= self.ops.now + timeout:
            self.ops.now = max(self.ops.now, self.exit_at)
            return self.code
        self.ops.now += timeout
        raise subprocess.TimeoutExpired('fake observer', timeout)


class FakeOps:
    def __init__(self, exit_at=None, term_delay=0, kill_delay=0, code=0, on_gate=None):
        self.now, self.signals, self.closed, self.commands = 0.0, [], [], []
        self.term_delay, self.kill_delay, self.bind_error, self.probe_error = term_delay, kill_delay, None, None
        self.process = FakeProcess(self, exit_at, code, on_gate)
    def monotonic(self): return self.now
    def thread_cpu_ns(self): return int(self.now*1000000)
    def probe_pidfd(self):
        if self.probe_error: raise self.probe_error
    def spawn(self, command): self.commands.append(command); return self.process
    def pidfd_open(self, pid):
        if self.bind_error: raise self.bind_error
        assert pid == 4242
        return 77
    def close(self, fd): self.closed.append(fd)
    def signal(self, fd, sig):
        assert fd == 77
        self.signals.append((fd, sig))
        delay = self.term_delay if sig == signal.SIGTERM else self.kill_delay
        if delay is not None:
            self.process.exit_at = self.now + delay
            self.process.code = 0 if sig == signal.SIGTERM else -9


def capture(path, samples=1, reason='owner_stop'):
    rows = [{'type': 'header', 'observerPID': 4242, 'limits': {'totalRows': 4000}}]
    rows += [{'type': 'sample', 'index': i} for i in range(samples)]
    prefix = b''.join((json.dumps(row)+'\n').encode() for row in rows)
    footer = {'type': 'footer', 'stopReason': reason, 'sampleRows': samples,
              'rowsBeforeFooter': len(rows), 'bytesBeforeFooter': len(prefix),
              'metadataStable': False, 'unknownObservations': 1, 'counterResetOrShapeChanges': 0}
    path.write_bytes(prefix + (json.dumps(footer)+'\n').encode())
    path.chmod(0o600)


class Settlement(unittest.TestCase):
    def run_watch(self, ops, reason=None, wall=.2):
        return s.watch_child(ops.process, 77, ops, lambda: reason,
                             started_at=0, wall_seconds=wall, term_grace=.5, kill_grace=.5)
    def test_normal_exact_child_exit_no_signal(self):
        ops=FakeOps(exit_at=.1);r=self.run_watch(ops)
        self.assertTrue(r['settled']);self.assertEqual(r['processOutcome'],'clean_exit');self.assertEqual(ops.signals,[])
        self.assertIsNone(r['observationComplete'])
    def test_wall_timeout_is_partial_even_when_term_exits_zero(self):
        ops=FakeOps(term_delay=.1);r=self.run_watch(ops)
        self.assertEqual(r['stopReason'],'wall_deadline');self.assertEqual(r['processOutcome'],'failed_or_partial');self.assertLessEqual(ops.now,.7)
    def test_ordinary_owner_stop_may_exit_normally(self):
        ops=FakeOps(term_delay=.1);r=self.run_watch(ops,'owner_finished')
        self.assertEqual(r['processOutcome'],'clean_exit');self.assertEqual(ops.signals,[(77,signal.SIGTERM)])
    def test_owner_error_stays_partial(self):
        r=self.run_watch(FakeOps(),'owner_error');self.assertEqual(r['processOutcome'],'failed_or_partial')
    def test_kill_settlement_never_success(self):
        ops=FakeOps(term_delay=None,kill_delay=.1);r=self.run_watch(ops,'owner_finished')
        self.assertTrue(r['forced']);self.assertTrue(r['settled']);self.assertEqual(r['processOutcome'],'failed_or_partial');self.assertLessEqual(ops.now,1.0)
    def test_never_settles_bounded_unknown(self):
        ops=FakeOps(term_delay=None,kill_delay=None);r=self.run_watch(ops,'owner_finished')
        self.assertFalse(r['settled']);self.assertEqual(r['processOutcome'],'settlement_unknown');self.assertEqual(ops.now,1.0)
    def test_signalling_denied_retains_uncertainty(self):
        ops=FakeOps()
        def denied(fd,sig): raise PermissionError('fixture')
        ops.signal=denied;r=self.run_watch(ops,'owner_finished')
        self.assertFalse(r['settled']);self.assertEqual(len(r['errors']),3)
    def test_exited_at_stop_never_signalled(self):
        ops=FakeOps(exit_at=0);r=self.run_watch(ops,'owner_finished');self.assertEqual(ops.signals,[]);self.assertTrue(r['settled'])
    def test_numeric_pid_change_cannot_redirect_signal(self):
        ops=FakeOps();ops.process.pid=99999;self.run_watch(ops,'owner_finished');self.assertEqual(ops.signals,[(77,signal.SIGTERM)])
    def test_invalid_limits_refused(self):
        for wall,term,kill in [(401,1,1),(1,2,2),(float('nan'),1,1),(1,-1,1),(True,1,1)]:
            with self.subTest(values=(wall,term,kill)),self.assertRaises(ValueError):s.limits(wall,term,kill)


class Serialization(unittest.TestCase):
    def setUp(self): self.tmp=tempfile.TemporaryDirectory();self.root=Path(self.tmp.name);self.path=self.root/'capture.jsonl'
    def tearDown(self): self.tmp.cleanup()
    def test_valid_bounded_footer_not_coverage_pass(self):
        capture(self.path);r=s.validate_observation(self.path);self.assertTrue(r['serializationComplete']);self.assertFalse(r['metadataStable']);self.assertEqual(r['unknownObservations'],1)
    def test_missing_footer_rejected(self):
        self.path.write_text('{"type":"header"}\n');self.path.chmod(0o600)
        with self.assertRaises(ValueError):s.validate_observation(self.path)
    def test_partial_json_line_rejected(self):
        capture(self.path);self.path.write_bytes(self.path.read_bytes()[:-1])
        with self.assertRaises(ValueError):s.validate_observation(self.path)
    def test_mismatched_footer_rejected(self):
        capture(self.path);self.path.write_bytes(self.path.read_bytes().replace(b'"sampleRows": 1',b'"sampleRows": 2'))
        with self.assertRaises(ValueError):s.validate_observation(self.path)
    def test_row_limit(self):
        capture(self.path,samples=3999)
        with self.assertRaises(ValueError):s.validate_observation(self.path)
    def test_byte_limit(self):
        self.path.write_bytes(b' '*(2*1024*1024+1));self.path.chmod(0o600)
        with self.assertRaises(ValueError):s.validate_observation(self.path)
    def test_symlink_and_permissive_file_refused(self):
        capture(self.path);link=self.root/'link';link.symlink_to(self.path)
        with self.assertRaises(OSError):s.validate_observation(link)
        self.path.chmod(0o644)
        with self.assertRaises(ValueError):s.validate_observation(self.path)
    def test_fifo_refused_without_blocking_open(self):
        os.mkfifo(self.path,0o600)
        with self.assertRaises(ValueError):s.validate_observation(self.path)
    def test_duplicate_json_key_rejected(self):
        capture(self.path);self.path.write_bytes(self.path.read_bytes().replace(b'"type": "header"',b'"type":"header", "type":"header"'))
        with self.assertRaises(ValueError):s.validate_observation(self.path)
    def test_existing_private_receipt_never_overwritten(self):
        s.private_receipt(self.path,{'one':1});before=self.path.read_bytes()
        with self.assertRaises(FileExistsError):s.private_receipt(self.path,{'two':2})
        self.assertEqual(self.path.read_bytes(),before);self.assertEqual(self.path.stat().st_mode&0o777,0o600)


class Managed(unittest.TestCase):
    def setUp(self): self.tmp=tempfile.TemporaryDirectory();self.root=Path(self.tmp.name);self.output=self.root/'samples.jsonl'
    def tearDown(self): self.tmp.cleanup()
    def make(self,ops):return s.ManagedObserver(['--output',str(self.output)],self.root/'supervisor',wall_seconds=.2,term_grace=.2,kill_grace=.2,ops=ops)
    def test_fake_complete_capture_and_own_gate(self):
        ops=FakeOps(exit_at=.1,on_gate=lambda:capture(self.output));m=self.make(ops).start();r=m.wait(1)
        self.assertTrue(r['observationComplete']);self.assertTrue(r['observation']['serializationComplete']);self.assertIn('elapsedNS',r['watchdogThreadCPU']);self.assertGreater(r['waitCallCount'],0);self.assertEqual(ops.process.stdin.writes,[b'G']);self.assertEqual(ops.closed,[77]);self.assertEqual(len(ops.commands),1)
        self.assertIn('observer.py',ops.commands[0][-3]);self.assertTrue((self.root/'supervisor/result.json').exists())
    def test_exit_zero_without_footer_fails(self):
        ops=FakeOps(exit_at=.1);r=self.make(ops).start().wait(1);self.assertFalse(r['observationComplete']);self.assertEqual(r['processOutcome'],'failed_or_partial')
    def test_pidfd_failure_closes_gate_without_observer_exec(self):
        ops=FakeOps();ops.bind_error=PermissionError('fixture');m=self.make(ops)
        with self.assertRaises(RuntimeError):m.start()
        self.assertEqual(ops.process.stdin.writes,[]);self.assertTrue(ops.process.stdin.closed);self.assertEqual(ops.signals,[]);self.assertTrue(m.snapshot()['settled'])
    def test_unsupported_pidfd_starts_nothing(self):
        ops=FakeOps();ops.probe_error=RuntimeError('unsupported')
        with self.assertRaises(RuntimeError):self.make(ops).start()
        self.assertEqual(ops.commands,[]);self.assertFalse((self.root/'supervisor').exists())
    def test_watcher_start_failure_settles_pinned_child(self):
        ops=FakeOps(on_gate=lambda:capture(self.output));m=self.make(ops)
        with patch.object(s.threading.Thread,'start',side_effect=RuntimeError('fixture')),self.assertRaises(RuntimeError):m.start()
        self.assertTrue(m.snapshot()['settled']);self.assertEqual(ops.signals,[(77,signal.SIGTERM)]);self.assertFalse(m.snapshot()['observationComplete'])
    def test_wrong_header_identity_cannot_acknowledge_observer_start(self):
        def wrong_header():
            capture(self.output)
            data=self.output.read_bytes().replace(b'4242',b'4243')
            self.output.write_bytes(data)
        ops=FakeOps(exit_at=.1,on_gate=wrong_header);r=self.make(ops).start().wait(1)
        self.assertFalse(r['observationComplete']);self.assertFalse(r['observerStartedAcknowledgedByHeader'])
    def test_existing_output_starts_nothing(self):
        capture(self.output);ops=FakeOps()
        with self.assertRaises(FileExistsError):self.make(ops).start()
        self.assertEqual(ops.commands,[])
    def test_linked_receipt_parent_refused_before_child(self):
        link=self.root/'parent-link';link.symlink_to(self.root,target_is_directory=True);ops=FakeOps()
        m=s.ManagedObserver(['--output',str(self.output)],link/'receipt',ops=ops)
        with self.assertRaises(ValueError):m.start()
        self.assertEqual(ops.commands,[])
    def test_permissive_receipt_parent_refused_before_child(self):
        parent=self.root/'public';parent.mkdir(mode=0o755);parent.chmod(0o755);ops=FakeOps()
        m=s.ManagedObserver(['--output',str(self.output)],parent/'receipt',ops=ops)
        with self.assertRaises(ValueError):m.start()
        self.assertEqual(ops.commands,[])
    def test_source_drift_refuses_complete_capture(self):
        ops=FakeOps(exit_at=.1,on_gate=lambda:capture(self.output));m=self.make(ops)
        real=s.source_record;calls=[0]
        def drift(path):
            calls[0]+=1;r=real(path)
            if calls[0]>2:r['sha256']='0'*64
            return r
        with patch.object(s,'source_record',side_effect=drift):r=m.start().wait(1)
        self.assertFalse(r['sourceStable']);self.assertFalse(r['observationComplete'])
    def test_ambiguous_output_arguments_refused(self):
        for args in [['--output','relative'],['--output','/a','--output','/b'],['--unknown','x'],['--output']]:
            with self.subTest(args=args),self.assertRaises(ValueError):s.output_argument(args)


if __name__=='__main__':unittest.main(verbosity=2)
