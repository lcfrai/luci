from pathlib import Path
import importlib.util,subprocess,tempfile,unittest,hashlib,json
ROOT=Path(__file__).resolve().parents[1]
spec=importlib.util.spec_from_file_location('binding',ROOT/'actual/checkout-binding.py');mod=importlib.util.module_from_spec(spec);spec.loader.exec_module(mod)
class BindingTests(unittest.TestCase):
 def setUp(self):
  self.tmp=tempfile.TemporaryDirectory(prefix='c32-guard-unit-',dir='/tmp');self.root=Path(self.tmp.name)
  self.git('init','-q');self.git('config','user.email','fixture@example.invalid');self.git('config','user.name','Fixture')
  (self.root/'helper.mjs').write_text('exact-helper');(self.root/'product.go').write_text('product')
  self.git('add','.');self.git('commit','-qm','base');self.base=self.git('rev-parse','HEAD').strip()
  data=(self.root/'helper.mjs').read_bytes();self.capture={'sources':{'helper.mjs':{'bytes':len(data),'sha256':hashlib.sha256(data).hexdigest()}}}
 def tearDown(self):self.tmp.cleanup()
 def git(self,*args):return subprocess.check_output(['git',*args],cwd=self.root,text=True,stderr=subprocess.PIPE,timeout=5)
 def docs(self):
  (self.root/'docs/luci').mkdir(parents=True);(self.root/'docs/luci/evidence.md').write_text('docs');self.git('add','.');self.git('commit','-qm','docs')
 def inspect(self):return mod.inspect_checkout(self.root,self.base,self.capture)
 def test_original_whole_head_guard_reproduced_and_corrected_docs_pass(self):
  self.docs();self.assertNotEqual(self.base,self.git('rev-parse','HEAD').strip())
  with self.assertRaises(AssertionError):assert self.git('rev-parse','HEAD').strip()==self.base
  got=self.inspect();self.assertEqual(got['observedCheckoutCommit'],self.git('rev-parse','HEAD').strip());self.assertEqual(got['helperSource'],self.base)
 def test_exact_head_pass(self):self.assertEqual(self.inspect()['observedCheckoutCommit'],self.base)
 def test_product_commit_rejected(self):
  (self.root/'product.go').write_text('changed');self.git('add','.');self.git('commit','-qm','product')
  with self.assertRaisesRegex(AssertionError,'Only docs'):self.inspect()
 def test_dirty_docs_rejected(self):
  self.docs();(self.root/'docs/luci/evidence.md').write_text('dirty')
  with self.assertRaisesRegex(AssertionError,'Clean'):self.inspect()
 def test_hidden_selected_mutation_rejected(self):
  self.git('update-index','--assume-unchanged','helper.mjs');(self.root/'helper.mjs').write_text('hidden')
  with self.assertRaisesRegex(AssertionError,'working selected'):self.inspect()
 def test_untracked_rejected(self):
  (self.root/'newfile').write_text('new')
  with self.assertRaisesRegex(AssertionError,'Clean'):self.inspect()
if __name__=='__main__':unittest.main(verbosity=2)
