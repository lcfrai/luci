"""Pure pathname/source contract checks. No fixture allocation or socket calls."""
import importlib.util,unittest
from pathlib import Path
spec=importlib.util.spec_from_file_location('prepared_launcher',Path(__file__).with_name('launch.py'));launch=importlib.util.module_from_spec(spec);spec.loader.exec_module(launch)

class SocketTemplateTests(unittest.TestCase):
 def test_exact_short_templates(self):
  r=launch.validate_socket_templates('/workspace/.lp-abcdefgh')
  self.assertEqual(r['templates']['public'],'/workspace/.lp-abcdefgh/luci-sustained-XXXXXX/docker-proxy.sock')
  self.assertEqual(r['templates']['private'],'/workspace/.lp-abcdefgh/luci-sustained-XXXXXX/.luci-fence-proxy-XXXXXX/docker.sock')
  self.assertTrue(all(n<108 for n in r['utf8Bytes'].values()))
 def test_original_long_parent_refused(self):
  with self.assertRaises(AssertionError):launch.validate_socket_templates('/workspace/.luci-tools/cloud032-primary-profiling-preparation/runs/primary-cdp-1/private-fixtures')
 def test_exact_boundary_private_path(self):
  tail='/luci-sustained-XXXXXX/.luci-fence-proxy-XXXXXX/docker.sock'
  valid='/'+'a'*(107-len(tail)-1)
  self.assertEqual(launch.validate_socket_templates(valid)['utf8Bytes']['private'],107)
  with self.assertRaises(AssertionError):launch.validate_socket_templates(valid+'a')
 def test_unicode_is_counted_as_bytes(self):
  parent='/'+'é'*30
  self.assertLess(len(parent+'/luci-sustained-XXXXXX/.luci-fence-proxy-XXXXXX/docker.sock'),108)
  with self.assertRaises(AssertionError):launch.validate_socket_templates(parent)
 def test_relative_and_nul_refused(self):
  for path in ['relative','/workspace/bad\0name']:
   with self.subTest(path=repr(path)),self.assertRaises(AssertionError):launch.validate_socket_templates(path)
 def test_matches_frozen_node_call_sites(self):
  base=Path(__file__).parent/'candidate';driver=(base/'sustained.mjs').read_text();proxy=(base/'fenced-docker-proxy.mjs').read_text()
  self.assertIn("mkdtempSync(join(fixtureParent,'luci-sustained-'))",driver)
  self.assertIn("socketPath:join(fixture,'docker-proxy.sock')",driver)
  self.assertIn("Buffer.byteLength(socketPath) >= 108",proxy)
  self.assertIn("Buffer.byteLength(join(dirname(socketPath), '.luci-fence-proxy-XXXXXX', 'docker.sock')) >= 108",proxy)
  self.assertIn("mkdtemp(join(dirname(socketPath), '.luci-fence-proxy-'))",proxy)

if __name__=='__main__':unittest.main(verbosity=2)
