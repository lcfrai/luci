"""Fake process/signal/write controls only; no child or signals are executed."""
from pathlib import Path
import importlib.util,os,subprocess,tempfile,unittest
from unittest.mock import patch
import operation_common as c
source=Path(os.environ.get('RESTORE_WATCHDOG_SOURCE',str(Path(__file__).with_name('run.py'))))
spec=importlib.util.spec_from_file_location('restoration_watchdog_control',source);w=importlib.util.module_from_spec(spec);spec.loader.exec_module(w)
class FakeProcess:
 pid=987654
 def __init__(self,force=False):self.returncode=None;self.waits=[];self.force=force
 def poll(self):return self.returncode
 def wait(self,timeout):
  self.waits.append(timeout)
  if timeout==400 or (self.force and timeout==10 and self.waits.count(10)==1):raise subprocess.TimeoutExpired('fake',timeout)
  self.returncode=-15;return self.returncode
class Watchdog(unittest.TestCase):
 def setUp(self):
  self.tmp=tempfile.TemporaryDirectory(prefix='go-watchdog-control-');self.addCleanup(self.tmp.cleanup);self.root=Path(self.tmp.name);self.pins={}
  for n in ['operation_common.py','restore_go.py','tree_support.py']:
   (self.root/n).write_bytes(b'fixture only\n');self.pins[n]=c.sha((self.root/n).read_bytes())
 def exercise(self,write_fail,force=False):
  proc=FakeProcess(force);actual=w.save;writes=[];signals=[]
  def saver(p,value):
   writes.append(value.get('status','coordination'))
   if len(writes)==write_fail:raise OSError('synthetic checkpoint write failed')
   return actual(p,value)
  with patch.object(w,'ROOT',self.root),patch.object(w,'PINS',self.pins),patch.object(c,'process_start',return_value='42'),patch.object(w,'save',side_effect=saver),patch.object(w.subprocess,'Popen',return_value=proc),patch.object(w.os,'killpg',side_effect=lambda pid,sig:signals.append((pid,int(sig)))):
   with self.assertRaises(OSError):w.main(['--acknowledge-exclusive-window'])
  return proc,writes,signals
 def test_initial_child_checkpoint_failure_still_settles(self):
  proc,writes,signals=self.exercise(3);self.assertEqual(signals,[(987654,15)]);self.assertEqual(proc.waits,[10]);self.assertIsNotNone(proc.returncode)
 def test_timeout_checkpoint_failure_still_settles(self):
  proc,writes,signals=self.exercise(4);self.assertEqual(signals,[(987654,15)]);self.assertEqual(proc.waits,[400,10]);self.assertIsNotNone(proc.returncode)
 def test_failed_checkpoint_force_path_is_bounded(self):
  proc,writes,signals=self.exercise(3,force=True);self.assertEqual(signals,[(987654,15),(987654,9)]);self.assertEqual(proc.waits,[10,10]);self.assertIsNotNone(proc.returncode)
if __name__=='__main__':unittest.main(verbosity=2)
