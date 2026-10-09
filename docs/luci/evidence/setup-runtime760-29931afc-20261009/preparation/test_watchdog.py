"""Pure fake-child/signal watchdog controls; no subprocess or signal is executed."""
from pathlib import Path
import tempfile,unittest
from unittest.mock import patch
import run_archive as w
class Controls(unittest.TestCase):
 def test_checkpoint_write_failure_still_settles_known_leader(self):
  class P:
   pid=98765
  with tempfile.TemporaryDirectory(prefix='archive-watchdog-control-')as d:
   root=Path(d);pins={}
   for n in ['archive760.py','budget.py','start.template.md']:(root/n).write_text('fixture\n');pins[n]=w.sha(root/n)
   original=w.write;calls=[]
   def writer(p,v):
    calls.append(v)
    if len(calls)==2:raise OSError('synthetic post-spawn checkpoint refusal')
    original(p,v)
   with patch.object(w,'ROOT',root),patch.object(w,'PINS',pins),patch.object(w.subprocess,'Popen',return_value=P()),patch.object(w,'start',return_value='42'),patch.object(w,'write',side_effect=writer),patch.object(w,'settle',return_value={'leaderAbsent':True,'daemonDrainConfirmed':False})as settled:
    self.assertEqual(w.main(['--window-granted','--expected-source','0'*40,'--expected-engine','synthetic','stage','--reviewed-runtime-receipt','/not-read','--reviewed-runtime-sha256','0'*64]),1);settled.assert_called_once_with(98765,'42')
 def test_reused_identity_never_signalled(self):
  with patch.object(w,'start',return_value='later'),patch.object(w.os,'killpg')as signal:r=w.settle(98765,'original')
  signal.assert_not_called();self.assertTrue(r['identityChanged'])
 def test_unreadable_identity_remains_unknown_without_signal(self):
  with patch.object(w,'start',side_effect=PermissionError('synthetic')),patch.object(w.os,'killpg')as signal:r=w.settle(98765,'42')
  signal.assert_not_called();self.assertTrue(r['settlementUnknown']);self.assertFalse(r['daemonDrainConfirmed'])
 def test_zombie_is_not_pid_absence(self):
  with patch.object(w,"start",side_effect=w.NonLive("Z")),patch.object(w.os,"killpg")as signal:r=w.settle(98765,"42")
  signal.assert_not_called();self.assertFalse(r["leaderAbsent"]);self.assertTrue(r["leaderNonLive"]);self.assertEqual(r["observedState"],"Z")
 def test_forced_settlement_has_fixed_wait_budget(self):
  with patch.object(w,'start',return_value='42'),patch.object(w.os,'killpg')as signal,patch.object(w.time,'sleep')as sleep:r=w.settle(98765,'42')
  self.assertEqual(signal.call_count,2);self.assertEqual(sleep.call_count,200);self.assertTrue(r['settlementUnknown']);self.assertTrue(r['killSent'])
if __name__=='__main__':unittest.main(verbosity=2)
