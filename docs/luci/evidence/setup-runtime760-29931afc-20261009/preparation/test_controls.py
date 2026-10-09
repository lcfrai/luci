"""Small inert/disposable controls; no Docker, real archive or tool operation."""
from pathlib import Path
import importlib.util,io,json,os,subprocess,tempfile,unittest
from unittest.mock import patch
import archive760 as a,budget,place_archives as m
class Bounds(unittest.TestCase):
 def test_same_export_guard_plus_output(self):
  image=budget.EXPORT_FLOOR-1342177280-268435456
  self.assertEqual(budget.admission(image,budget.EXPORT_FLOOR+budget.GZIP_CAP,budget.EXPORT_FLOOR)['perFilesystemExportFloorBytes'],3905250841)
  for root,tmp in [(budget.EXPORT_FLOOR+budget.GZIP_CAP-1,budget.EXPORT_FLOOR),(budget.EXPORT_FLOOR+budget.GZIP_CAP,budget.EXPORT_FLOOR-1)]:
   with self.assertRaises(ValueError):budget.admission(image,root,tmp)
 def test_output_cap_refuses_before_extra_bytes(self):
  old=a.load(a.PRIMITIVES,a.PRIMITIVES_SHA,'archive_primitive_control');sink=io.BytesIO();w=old.LimitedWriter(sink,4);self.assertEqual(w.write(b'1234'),4)
  with self.assertRaises(RuntimeError):w.write(b'5')
  self.assertEqual(sink.getvalue(),b'1234');self.assertEqual(w.count,4)
 def test_unexpected_settlement_error_is_explicit(self):
  class P:
   pid=98765
   def poll(self):return None
  with patch.object(a.os,'killpg',side_effect=PermissionError('fake-denial')):r=a.stop_owned(P())
  self.assertTrue(r['leaderSettlementUnknown']);self.assertEqual(r['settlementErrorType'],'PermissionError');self.assertFalse(r['daemonDrainConfirmed'])
 def test_settlement_waits_are_bounded(self):
  class P:
   pid=98765
   waits=[]
   def poll(self):return None
   def wait(self,timeout):self.waits.append(timeout);raise subprocess.TimeoutExpired('fake',timeout)
  p=P()
  with patch.object(a.os,'killpg')as signal:r=a.stop_owned(p)
  self.assertEqual(p.waits,[10,10]);self.assertEqual(signal.call_count,2);self.assertTrue(r['leaderSettlementUnknown'])
 def test_final_floor_math_requires_old333_step(self):
  f=budget.forecast(7352782848,2260738048)
  self.assertGreater(f['optionalToRoot']['root'],budget.EXPORT_FLOOR+budget.GZIP_CAP);self.assertGreater(f['optionalToRoot']['tmp'],budget.EXPORT_FLOOR)
  self.assertLess(f['optionalReturned']['root'],budget.RUNTIME_FLOOR);self.assertGreater(f['old333HistoricalToTMP']['root'],budget.RUNTIME_FLOOR)
class Placement(unittest.TestCase):
 def setUp(self):
  self.t=tempfile.TemporaryDirectory(prefix='archive-placement-control-');self.addCleanup(self.t.cleanup);self.root=Path(self.t.name)/'root';self.tmp=Path(self.t.name)/'tmp';self.root.mkdir(mode=0o700);self.tmp.mkdir(mode=0o700)
  self.pair=(self.root/'runtime-archive-7e1b1f608cfa/verified-luci-sandbox-dev.tar.gz',self.root/'runtime-archive-1f845cfd2b45/old-luci-sandbox-dev.tar.gz')
  for p in self.pair:p.parent.mkdir(mode=0o700);(p.parent/'receipt.json').write_text('{}\n')
  self.f=self.tmp/'archive425';self.f.write_bytes(b'exact tiny archive\n');self.f.chmod(0o600);self.pin=m.digest(self.f)
  for p in self.pair:p.symlink_to(self.f)
  self.protected=self.root/'luci-sandbox-dev.tar.gz';self.protected.write_bytes(b'protected current333 fixture\n');self.protected.chmod(0o600)
  self.patches=[]
  for name,value in [('ROOT',self.root),('TMP',self.tmp),('PAIR425',self.pair),('FILE425',self.f),('PIN425',self.pin),('SIZE425',self.f.stat().st_size)]:
   p=patch.object(m,name,value);p.start();self.addCleanup(p.stop)
  original_load=m.load
  def fixture_load():
   primitives=original_load();primitives.free=lambda p:20<<30;return primitives
  p=patch.object(m,"load",side_effect=fixture_load);p.start();self.addCleanup(p.stop)
  p=patch.object(m,"observe",return_value={"observed":0,"busy":[],"unknown":[],"scope":"inert substituted observation"});p.start();self.addCleanup(p.stop)
  self.n=0
 def go(self,mode):
  self.n+=1;m.main(['--window-granted','--mode',mode,'--output',str(self.root/f'runtime760-placement-test{self.n}')])
 def test_425_roundtrip_exact_bytes_aliases_protected(self):
  before=self.protected.read_bytes();self.go('425-to-root');self.assertFalse(self.pair[0].is_symlink());self.assertEqual(self.pair[0].stat().st_nlink,2);self.assertEqual(self.pair[0].stat().st_ino,self.pair[1].stat().st_ino);self.assertEqual(os.readlink(self.f),str(self.pair[0]));self.go('425-to-tmp');self.assertFalse(self.f.is_symlink());self.assertEqual(m.digest(self.f),self.pin)
  for p in self.pair:self.assertEqual(os.readlink(p),str(self.f))
  self.assertEqual(self.protected.read_bytes(),before)
 def test_wrong_original_bytes_refuse(self):
  self.f.write_bytes(b'altered-original\n')
  with self.assertRaises(RuntimeError):self.go('425-to-root')
  self.assertTrue(all(p.is_symlink()for p in self.pair));self.assertEqual(self.f.read_bytes(),b'altered-original\n')
 def test_333_refuses_before_publication_identity(self):
  with self.assertRaises(RuntimeError):self.go('333-to-tmp')
  self.assertEqual(self.protected.read_bytes(),b'protected current333 fixture\n');self.assertFalse(self.f.is_symlink())
if __name__=='__main__':unittest.main(verbosity=2)
