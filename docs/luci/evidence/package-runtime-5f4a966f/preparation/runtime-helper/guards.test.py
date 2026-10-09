import unittest,importlib.util,tempfile,pathlib,json,os,hashlib
from unittest.mock import patch
BASE=pathlib.Path(__file__).parent

def module(name):
 spec=importlib.util.spec_from_file_location(name.replace('-','_'),BASE/(name+'.py'));m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m);return m
r=module('prepare-runtime');p=module('build-package');SOURCE='1'*40
class Guards(unittest.TestCase):
 def test_exact_clean_source_fence(self):
  r.EXPECTED_SOURCE=SOURCE
  with patch.object(r.subprocess,'check_output',side_effect=[SOURCE+'\n','']):self.assertEqual(r.source_fence()['sourceCommit'],SOURCE)
  for answers in [['2'*40+'\n'],[SOURCE+'\n',' M sandboxd/go.mod\n']]:
   with patch.object(r.subprocess,'check_output',side_effect=answers),self.assertRaises(RuntimeError):r.source_fence()
 def test_no_implicit_revision(self):
  r.EXPECTED_SOURCE=None
  with patch.object(r.subprocess,'check_output')as call,self.assertRaises(RuntimeError):r.source_fence()
  call.assert_not_called()
 def test_corrected_vendor_required(self):
  with tempfile.TemporaryDirectory()as temp:
   root=pathlib.Path(temp);(root/'sandboxd/vendor').mkdir(parents=True)
   with patch.object(r,'ROOT',root):
    for version,ok in [('1.8.12',False),('1.8.15',True),('1.8.15 // indirect',True)]:
     (root/'sandboxd/go.mod').write_text('module local\nrequire github.com/coder/websocket v'+version+'\n');(root/'sandboxd/vendor/modules.txt').write_text('# github.com/coder/websocket v'+version.split()[0]+'\n')
     if ok:r.verify_runtime_module()
     else:
      with self.assertRaises(RuntimeError):r.verify_runtime_module()
 def test_pinned_transport_drops_context_and_tls(self):
  with tempfile.TemporaryDirectory()as t,patch.dict(os.environ,{'DOCKER_CONTEXT':'wrong','DOCKER_TLS_VERIFY':'1','DOCKER_CERT_PATH':'secret'}),patch.object(r,'EXPECTED_ENGINE','engine123'),patch.object(r,'docker_json',return_value='engine123'):
   v=r.configure_docker(pathlib.Path(t));self.assertFalse(v['inheritedContextTLSConfig']);self.assertEqual(r.docker_argv('version')[:3],[r.DOCKER,'--host','unix:///var/run/docker.sock']);self.assertNotIn('DOCKER_CONTEXT',r.DOCKER_ENV);self.assertNotIn('DOCKER_TLS_VERIFY',r.DOCKER_ENV);self.assertNotIn('DOCKER_CERT_PATH',r.DOCKER_ENV)
 def test_wrong_engine_refused(self):
  with tempfile.TemporaryDirectory()as t,patch.object(r,'EXPECTED_ENGINE','engine123'),patch.object(r,'docker_json',return_value='other'),self.assertRaises(RuntimeError):r.configure_docker(pathlib.Path(t))
 def test_runtime_source_must_match_exact_git_set_and_bytes(self):
  with tempfile.TemporaryDirectory()as t:
   root=pathlib.Path(t);(root/'sandboxd').mkdir();(root/'images/sandbox').mkdir(parents=True)
   names=['sandboxd/go.mod','images/sandbox/entrypoint.sh','images/sandbox/Dockerfile']
   for n in names:(root/n).write_bytes(b'x')
   with patch.object(r,'ROOT',root),patch.object(r,'EXPECTED_SOURCE',SOURCE):
    with patch.object(r.subprocess,'check_output',side_effect=['\n'.join(names)+'\n',b'x',b'x',b'x']):self.assertEqual(len(r.source_hashes()),3)
    with patch.object(r.subprocess,'check_output',return_value='sandboxd/go.mod\n'),self.assertRaises(RuntimeError):r.source_hashes()
    with patch.object(r.subprocess,'check_output',side_effect=['\n'.join(names)+'\n',b'wrong']),self.assertRaises(RuntimeError):r.source_hashes()
 def test_old_pins_updated_without_budget_change(self):
  self.assertTrue(r.OLD.endswith('f208e98'));self.assertEqual(r.ARCHIVE_SHA,'7caf7aab012cd4a5bf2ace5c8a2e12e617b4afbaef20f6b3db64586a24e9fcf3');self.assertEqual(r.FLOOR,1<<30);self.assertEqual(r.STOP_RESERVE,(1<<30)+(256<<20));self.assertEqual(r.BASE,'sha256:9e0fd9f731408c4405c4b9e97308bf807c8088cf521a138c0b1921667a831351')
 def test_prior_package_preserved_by_byte_parity(self):
  with tempfile.TemporaryDirectory()as t:
   root=pathlib.Path(t);release=root/'0.5.0';release.mkdir();out=root/'out';out.mkdir();b=b'original';(release/'artifact.tgz').write_bytes(b);(release/'SHA256SUMS').write_text('old\n');(release/'release-manifest.json').write_text(json.dumps({'sourceCommit':p.OLD,'sourceDirty':False,'artifacts':[{'name':'artifact.tgz','sha256':hashlib.sha256(b).hexdigest(),'bytes':len(b)}]}));receipt=p.preserve_release(release,out,SOURCE);self.assertFalse(release.exists());self.assertEqual((pathlib.Path(receipt['preservedPath'])/'artifact.tgz').read_bytes(),b);self.assertEqual(receipt['status'],'preserved-verified')
 def test_wrong_prior_package_refuses_rename(self):
  with tempfile.TemporaryDirectory()as t:
   root=pathlib.Path(t);release=root/'0.5.0';release.mkdir();out=root/'out';out.mkdir();(release/'release-manifest.json').write_text(json.dumps({'sourceCommit':'stale','sourceDirty':False,'artifacts':[]}))
   with self.assertRaises(AssertionError):p.preserve_release(release,out,SOURCE)
   self.assertTrue(release.exists())
if __name__=='__main__':unittest.main()
