"""Tiny disposable same-code controls. No actual tool path or proc scan is used."""
from pathlib import Path
import contextlib,hashlib,json,os,tempfile,unittest
from unittest.mock import patch
import restore_go as r
import tree_support as t
import operation_common as c

class Restoration(unittest.TestCase):
 def setUp(self):
  self.tmp=tempfile.TemporaryDirectory(prefix='go-restore-control-');self.addCleanup(self.tmp.cleanup);self.root=Path(self.tmp.name)
  self.work=self.root/'workspace';self.base=self.root/'tmp';self.out=self.root/'operations'
  for p in [self.work,self.base,self.out]:p.mkdir(mode=0o700)
  self.original=self.base/'go1.26.3';self.original.mkdir(mode=0o700);(self.original/'bin').mkdir();(self.original/'bin/go').write_bytes(b'fixture executable\n');(self.original/'bin/go').chmod(0o755)
  self.final=self.work/'go1.26.3';self.final.symlink_to(self.original);self.stage=self.work/'.go-stage';self.scratch=self.base/'.go-retired'
  self.rows=t.manifest(self.original)['entries'];self.bytes=sum(x.get('bytes',0) for x in self.rows);self.m=self.root/'manifest.json';self.m.write_text(json.dumps(self.rows));self.pin=c.sha(self.m.read_bytes());self.source=Path(r.__file__).resolve()
  self.stack=contextlib.ExitStack();self.addCleanup(self.stack.close)
  vals={'ROOT':self.out,'TMP':self.base,'WORK':self.work,'ORIGINAL':self.original,'FINAL':self.final,'STAGE':self.stage,'SCRATCH':self.scratch,'MANIFEST':self.m,'PIN':self.pin,'BYTES':self.bytes,'COUNT':len(self.rows),'ROOT_RUNTIME_FLOOR':1,'WORKING_GUARD':1,'SOURCES':[self.source]}
  for k,v in vals.items():self.stack.enter_context(patch.object(r,k,v))
  self.stack.enter_context(patch.object(t,'TMP',self.base))
  self.stack.enter_context(patch.object(r,'quiescence',return_value={'scope':'cloud054-exclusive-tools','acceptedUnknownProcesses':[]}))
  self.scan=self.stack.enter_context(patch.object(r,'process_scan',return_value={'observed':0,'busy':[],'unknown':[]}))
 def run_case(self):
  self.receipt=self.out/'operation';code=r.main(['--execute-restoration','--quiescence',str(self.root/'not-read'),'--quiescence-sha256','synthetic-only','--receipt',str(self.receipt)])
  last=sorted(self.receipt.glob('*.json'))[-1];return code,json.loads(last.read_text())
 def original_intact(self):
  self.assertFalse(self.original.is_symlink());self.assertEqual(t.manifest(self.original)['entries'],self.rows)
 def test_promotes_real_tree_preserves_tmp_alias_bytes_and_modes(self):
  code,last=self.run_case();self.assertEqual(code,0);self.assertTrue(last['passed']);self.assertFalse(self.final.is_symlink());self.assertEqual(os.readlink(self.original),str(self.final));self.assertEqual(t.manifest(self.final)['entries'],self.rows);self.assertFalse(self.stage.exists());self.assertFalse(self.scratch.exists());self.assertEqual(last['sourceBefore'],last['sourceAfter'])
 def test_changed_byte_refuses_before_copy(self):
  (self.original/'bin/go').write_bytes(b'changed\n');code,last=self.run_case();self.assertEqual(code,1);self.assertEqual(last['failureCode'],'tool-manifest-mismatch');self.assertFalse(self.stage.exists());self.assertTrue(self.final.is_symlink())
 def test_symlink_surprise_refuses(self):
  (self.original/'linked').symlink_to('/does-not-exist');code,last=self.run_case();self.assertEqual(code,1);self.assertEqual(last['failureCode'],'unexpected-symlink-or-special-entry');self.assertFalse(self.stage.exists())
 def test_hardlink_surprise_refuses(self):
  os.link(self.original/'bin/go',self.original/'second');code,last=self.run_case();self.assertEqual(code,1);self.assertEqual(last['failureCode'],'unexpected-hardlink');self.assertFalse(self.stage.exists())
 def test_busy_reference_even_accepted_unknown_refuses(self):
  self.scan.return_value={'observed':1,'busy':[{'pid':123,'startTicks':'10'}],'unknown':[]};code,last=self.run_case();self.assertEqual(code,1);self.assertEqual(last['failureCode'],'active-tool-reference');self.original_intact();self.assertFalse(self.stage.exists())
 def test_unaccepted_unknown_refuses(self):
  self.scan.return_value={'observed':0,'busy':[],'unknown':[{'pid':123,'startTicks':'10','identityStable':True}]};code,last=self.run_case();self.assertEqual(code,1);self.assertEqual(last['failureCode'],'new-missing-or-reused-unknown-process');self.original_intact()
 def test_existing_stage_refuses_without_removal(self):
  self.stage.mkdir();(self.stage/'retained').write_text('keep');code,last=self.run_case();self.assertEqual(code,1);self.assertEqual(last['failureCode'],'restoration-staging-path-already-occupied');self.assertEqual((self.stage/'retained').read_text(),'keep');self.original_intact()
 def test_insufficient_root_refuses_before_copy(self):
  with patch.object(r,'free',return_value=1):code,last=self.run_case()
  self.assertEqual(code,1);self.assertEqual(last['failureCode'],'root-headroom-before-copy-refused');self.original_intact();self.assertFalse(self.stage.exists())
 def test_failure_between_exchanges_retains_both_complete_trees(self):
  actual=r.exchange
  def failed(a,b,parent):
   if parent==self.base:raise OSError('synthetic second exchange refusal')
   return actual(a,b,parent)
  with patch.object(r,'exchange',side_effect=failed):code,last=self.run_case()
  self.assertEqual(code,1);self.assertFalse(last['automaticRollback']);self.assertEqual(t.manifest(self.final)['entries'],self.rows);self.original_intact();self.assertEqual(os.readlink(self.scratch),str(self.final))
 def test_busy_before_retirement_keeps_duplicate_bytes(self):
  self.scan.side_effect=[{'observed':0,'busy':[],'unknown':[]},{'observed':0,'busy':[],'unknown':[]},{'observed':1,'busy':[{'pid':123,'startTicks':'10'}],'unknown':[]}]
  code,last=self.run_case();self.assertEqual(code,1);self.assertEqual(last['failureCode'],'active-tool-reference');self.assertEqual(t.manifest(self.final)['entries'],self.rows);self.assertEqual(t.manifest(self.scratch)['entries'],self.rows);self.assertEqual(os.readlink(self.original),str(self.final))
if __name__=='__main__':unittest.main(verbosity=2)
