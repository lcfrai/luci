"""Tiny file/fake-clock fixtures only; no live cgroup, app or benchmark."""
import copy
import errno
import importlib.util
import json
import os
from pathlib import Path
import stat
import sys
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('counter_observer_under_test', Path(__file__).with_name('observer.py'))
obs = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = obs
spec.loader.exec_module(obs)

def counters(n=0, throttle=0):
    return {'usage_usec': n, 'user_usec': n, 'system_usec': 0,
            'nr_periods': n, 'nr_throttled': throttle, 'throttled_usec': throttle}

def metadata():
    good = {'status': 'observed', 'value': 'fixture'}
    return {'heldIdentity': {'device': 1, 'inode': 2}, 'currentPathIdentity': {'device': 1, 'inode': 2},
            'pathStillSame': True, 'cpuMax': good.copy(), 'effectiveCpuset': good.copy(),
            'observerCgroupMembership': good.copy(), 'namespaces': {'cgroup': good.copy(), 'time': good.copy()}}

class FakeClock:
    def __init__(self):self.now = 0;self.wall_offset = 10**18;self.sleeps = []
    def monotonic_ns(self):return self.now
    def wall_ns(self):return self.now + self.wall_offset
    def cpu_ns(self):return self.now // 100
    def sleep(self, seconds):self.sleeps.append(seconds);self.now += round(seconds * 1e9)

class Reader:
    def __init__(self, clock, rows=None, delay=0, metadata_after=None, wall_jump=0):
        self.clock=clock;self.rows=rows;self.delay=delay;self.read_count=0;self.samples=0
        self.after=metadata_after;self.metadata_calls=0;self.wall_jump=wall_jump
    def metadata(self):
        self.read_count+=1;self.metadata_calls+=1
        return copy.deepcopy(self.after if self.after is not None and self.metadata_calls>1 else metadata())
    def stat(self):
        self.read_count+=1;self.clock.now+=self.delay
        if self.samples==1:self.clock.wall_offset+=self.wall_jump
        index=self.samples;self.samples+=1
        if self.rows is not None:return copy.deepcopy(self.rows[min(index,len(self.rows)-1)])
        return {'status':'observed','values':counters(index),'missing':[],'unknownKeys':[]}

class Fixture(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory(prefix='luci-counter-fixture-')
        self.root=Path(self.temp.name);self.root.chmod(0o700)
    def tearDown(self):self.temp.cleanup()
    def capture(self, reader, clock, limits=None, stop=None):
        limits=limits or obs.Limits(duration_ns=350_000_000)
        writer=obs.BoundedWriter(self.root/'report.jsonl',limits)
        try:summary=obs.observe(reader,writer,clock=clock,stop=stop,limits=limits)
        finally:writer.close()
        rows=[json.loads(x) for x in (self.root/'report.jsonl').read_text().splitlines()]
        return summary,rows

    def test_parse_required_optional_and_unknown_numeric_fields(self):
        raw=b''.join(f'{k} {v}\n'.encode() for k,v in counters(7).items())+b'nr_bursts 2\nfuture_counter 9\n'
        got=obs.parse_stat(raw)
        self.assertEqual(got['missing'],[]);self.assertEqual(got['unknownKeys'],['future_counter'])
        self.assertEqual(got['values']['future_counter'],9)
    def test_malformed_missing_duplicate_negative_and_oversize(self):
        for raw in [b'',b'usage_usec -1\n',b'usage_usec 1\nusage_usec 2\n',b'usage_usec 1 2\n',b'bad\xff 1\n',b'x 18446744073709551616\n',b'x 1\n'*1025]:
            with self.subTest(raw=raw[:40]):
                with self.assertRaises(obs.FormatError):obs.parse_stat(raw)
        self.assertIn('nr_throttled',obs.parse_stat(b'usage_usec 0\n')['missing'])
    def test_maximum_bounded_record_size_fits_sample_reserve(self):
        values={'x'+str(n)+'a'*60:2**64-1 for n in range(32)}
        parsed=obs.parse_stat(b''.join(f'{k} {v}\n'.encode() for k,v in values.items()))
        self.assertEqual(len(parsed['values']),32)
        self.assertLess(len(json.dumps({'observation':parsed,'delta':obs.counter_delta(values,{k:0 for k in values})}).encode())+2000,obs.SAMPLE_RESERVE)
    def test_no_catch_up_after_slow_read(self):
        clock=FakeClock();summary,rows=self.capture(Reader(clock,delay=250_000_000),clock,obs.Limits(duration_ns=700_000_000))
        samples=[r for r in rows if r['type']=='sample']
        self.assertEqual([r['plannedMonotonicNS'] for r in samples],[0,300_000_000,600_000_000])
        self.assertEqual(summary['missedSamplingDeadlines'],6)
        self.assertEqual(summary['overrunReads'],3)
        self.assertEqual(summary['cooperativeDeadlineOverrunNS'],150_000_000)
    def test_exact_cadence_and_stop_at_duration(self):
        clock=FakeClock();summary,rows=self.capture(Reader(clock),clock)
        self.assertEqual([r['plannedMonotonicNS'] for r in rows if r['type']=='sample'],[0,100_000_000,200_000_000,300_000_000])
        self.assertEqual(summary['elapsedNS'],350_000_000)
        self.assertEqual(summary['stopReason'],'duration_limit')
    def test_four_thousand_row_bound_includes_metadata_and_footer(self):
        clock=FakeClock();summary,rows=self.capture(Reader(clock),clock,obs.Limits(duration_ns=400_000_000_000))
        self.assertLessEqual(len(rows),4000);self.assertLessEqual((self.root/'report.jsonl').stat().st_size,2*1024*1024)
        self.assertEqual(rows[-1]['type'],'footer')
        self.assertEqual(summary['stopReason'],'report_bound')
    def test_small_row_bound_stops_before_unretained_read(self):
        clock=FakeClock();reader=Reader(clock);summary,rows=self.capture(reader,clock,obs.Limits(rows=4))
        self.assertEqual(len(rows),4);self.assertEqual(reader.samples,2);self.assertEqual(summary['sampleRows'],2)
        self.assertEqual(summary['stopReason'],'report_bound')
    def test_byte_bound_reserves_footer_and_does_not_read_unretained_sample(self):
        clock=FakeClock();reader=Reader(clock);summary,rows=self.capture(reader,clock,obs.Limits(bytes=20_000,footer_reserve=4096))
        self.assertEqual(reader.samples,0);self.assertEqual(len(rows),2)
        self.assertLessEqual((self.root/'report.jsonl').stat().st_size,20_000)
        self.assertFalse(summary['noPositiveThrottleWithinCompleteObservedPairs'])
    def test_counter_decrease_and_field_changes_never_clamp(self):
        self.assertIsNone(obs.counter_delta(counters(10),counters(1))['values'])
        self.assertIn('usage_usec',obs.counter_delta(counters(10),counters(1))['resetKeys'])
        self.assertIsNone(obs.counter_delta(counters(),{'usage_usec':1})['values'])
    def test_unknown_gap_then_delta_retains_gap_and_no_zero_claim(self):
        clock=FakeClock();rows=[{'status':'observed','values':counters(1),'missing':[]},
                               {'status':'unavailable','errno':errno.EACCES},
                               {'status':'observed','values':counters(3),'missing':[]}]
        summary,result=self.capture(Reader(clock,rows),clock)
        sample=[r for r in result if r['type']=='sample'][2]
        self.assertEqual(sample['delta']['interveningUnknownSamples'],1)
        self.assertEqual(sample['delta']['values']['usage_usec'],2)
        self.assertFalse(summary['noPositiveThrottleWithinCompleteObservedPairs'])
    def test_positive_throttling_only_when_valid_counter_delta_positive(self):
        clock=FakeClock();rows=[{'status':'observed','values':counters(1,10),'missing':[]},
                               {'status':'observed','values':counters(2,11),'missing':[]}]
        summary,_=self.capture(Reader(clock,rows),clock)
        self.assertTrue(summary['anyPositiveObservedThrottleDelta'])
        self.assertFalse(summary['noPositiveThrottleWithinCompleteObservedPairs'])
    def test_missing_fields_prevent_no_throttle_conclusion(self):
        clock=FakeClock();summary,_=self.capture(Reader(clock,[{'status':'observed','values':{'usage_usec':0},'missing':['nr_throttled']}]),clock)
        self.assertGreater(summary['unknownObservations'],0)
        self.assertFalse(summary['noPositiveThrottleWithinCompleteObservedPairs'])
    def test_metadata_change_preserves_values_and_invalidates_stable_scope(self):
        clock=FakeClock();after=metadata();after['cpuMax']={'status':'observed','value':'changed'}
        summary,_=self.capture(Reader(clock,metadata_after=after),clock)
        self.assertFalse(summary['metadataStable']);self.assertFalse(summary['noPositiveThrottleWithinCompleteObservedPairs'])
    def test_wall_clock_jump_does_not_change_monotonic_cadence(self):
        clock=FakeClock();summary,rows=self.capture(Reader(clock,wall_jump=-5_000_000_000),clock)
        sample=[r for r in rows if r['type']=='sample'][1]
        self.assertLess(sample['readBracket']['endWallNS'],sample['readBracket']['startWallNS'])
        self.assertEqual(summary['elapsedNS'],350_000_000)
        self.assertEqual(summary['sampleRows'],4)
    def test_owner_stop_only_changes_observer_flow(self):
        clock=FakeClock();reader=Reader(clock);summary,_=self.capture(reader,clock,stop=lambda:reader.samples==2)
        self.assertEqual(summary['stopReason'],'owner_stop');self.assertEqual(reader.samples,2)
    def test_private_append_only_fresh_output_and_symlink_refusal(self):
        path=self.root/'report';w=obs.BoundedWriter(path,obs.Limits());w.append({'initial':1});w.close()
        original=path.read_bytes();self.assertEqual(stat.S_IMODE(path.stat().st_mode),0o600)
        with self.assertRaises(FileExistsError):obs.BoundedWriter(path,obs.Limits())
        alias=self.root/'alias';alias.symlink_to(path)
        with self.assertRaises(OSError):obs.BoundedWriter(alias,obs.Limits())
        self.assertEqual(path.read_bytes(),original)
    def test_nonprivate_parent_refused(self):
        self.root.chmod(0o755)
        with self.assertRaises(ValueError):obs.BoundedWriter(self.root/'x',obs.Limits())
    def test_partial_write_error_preserves_failure_without_footer(self):
        writer=obs.BoundedWriter(self.root/'report',obs.Limits());real_write=os.write;calls=0
        def write(fd,data):
            nonlocal calls
            calls+=1
            if calls==1:return real_write(fd,data[:3])
            raise OSError(errno.ENOSPC,'fixture')
        with patch.object(obs.os,'write',write):
            with self.assertRaises(OSError):writer.append({'type':'sample','value':123})
        self.assertTrue(writer.failed);writer.close()
        self.assertEqual((self.root/'report').read_bytes(),b'{"t')
    def test_kernel_reader_missing_denied_symlink_and_nonregular_are_unknown(self):
        reader=obs.KernelReader(self.root)
        try:
            self.assertEqual(reader.stat()['errno'],errno.ENOENT)
            with patch.object(reader,'_read',return_value={'status':'unavailable','errno':errno.EACCES}):
                self.assertEqual(reader.stat()['status'],'unavailable')
            (self.root/'target').write_text('usage_usec 0\n');(self.root/'cpu.stat').symlink_to(self.root/'target')
            self.assertEqual(reader.stat()['status'],'unavailable');(self.root/'cpu.stat').unlink()
            os.mkfifo(self.root/'cpu.stat');self.assertEqual(reader.stat()['reason'],'not_regular')
        finally:reader.close()
    def test_limits_reject_unbounded_and_nonstandard_sampling(self):
        for values in [{'duration_ns':400_000_000_001},{'rows':4001},{'bytes':2*1024*1024+1},{'interval_ns':1}]:
            with self.subTest(values=values):
                with self.assertRaises(ValueError):obs.Limits(**values).validate()

if __name__=='__main__':unittest.main(verbosity=2)
