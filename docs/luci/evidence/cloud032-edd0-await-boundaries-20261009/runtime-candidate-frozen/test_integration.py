import hashlib,importlib.util,json,pathlib,re,unittest
P=pathlib.Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location('staged_launch',P/'launch.py');launch=importlib.util.module_from_spec(spec);spec.loader.exec_module(launch)
sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
class Integration(unittest.TestCase):
 def test_endpoint_override_is_single_explicit_unix_host(self):
  inherited={'PATH':'/bin','HOME':'/synthetic','DOCKER_HOST':'tcp://foreign:2375','DOCKER_CONTEXT':'foreign','DOCKER_TLS_VERIFY':'1','DOCKER_CERT_PATH':'/synthetic-secret','DOCKER_TLS':'1','DOCKER_API_VERSION':'bad'}
  value=launch.controlled_environment(inherited,'unix:///var/run/docker.sock');self.assertEqual(value['DOCKER_HOST'],'unix:///var/run/docker.sock')
  self.assertTrue(all(k not in value for k in ['DOCKER_CONTEXT','DOCKER_TLS_VERIFY','DOCKER_CERT_PATH','DOCKER_TLS','DOCKER_API_VERSION']))
  calls=[];original=launch.command
  try:
   launch.command=lambda argv,**kw:calls.append((argv,kw)) or 'synthetic'
   self.assertEqual(launch.docker_command(['info','--format','{{.ID}}'],value),'synthetic')
  finally:launch.command=original
  self.assertEqual(calls,[(['docker','--host','unix:///var/run/docker.sock','info','--format','{{.ID}}'],{'env':value})]);self.assertEqual(inherited['DOCKER_CONTEXT'],'foreign')
 def test_unsupported_and_ambiguous_endpoints_refused(self):
  for endpoint in ['',None,'tcp://127.0.0.1:2375','ssh://host','unix://relative','unix:///','unix:///tmp/a\n','unix:///tmp/a?x','unix:///tmp/a#x','unix:///tmp/a\t','unix:///tmp/a\0','unix:///tmp/a b']:
   with self.subTest(endpoint=repr(endpoint)),self.assertRaises(AssertionError):launch.controlled_environment({},endpoint)
 def test_complete_local_binding_and_marker_bytes(self):
  binding=json.loads((P/'actual/external-source-binding.json').read_text())
  self.assertEqual(binding['prospectiveProduct'],'edd0cb121ff6847f681ab6bf41509a497f043d41');self.assertEqual(binding['prospectiveHelper'],'8c7bf16aa035b27dbea185b565052495eb60777f')
  for name,expected in binding['localSources'].items():self.assertEqual(sha(P/'actual'/name),expected,name)
  for name in ['interaction-metrics.mjs','latency-diagnostics.mjs']:self.assertEqual((P/'actual'/name).read_bytes(),(P/'mark-candidate/candidate'/name).read_bytes())
  source=(P/'launch.py').read_text();self.assertIn('env=runtime_env;',source);self.assertIn('env=env,stdout=stdout,stderr=stderr,start_new_session=True',source);self.assertIn("args.trace_after=='yes'",source);self.assertIn('assert args.window_granted',source)
 def test_driver_action_guard_and_lifecycle_bytes_unchanged_after_exact_binding_edits(self):
  base=pathlib.Path('/workspace/.luci-tools/cloud055-readiness-stage-gate-v2/capsule/actual/sustained.mjs').read_text();after=(P/'actual/sustained.mjs').read_text()
  after=after.replace("mode:'installed-four-project-await-boundary-edd0-diagnostic',","mode:'installed-four-project-autonomous-readiness-v2-diagnostic',")
  after=after.replace("\nassert.equal(sourceBindingOptions.expectedProduct,externalBinding.prospectiveProduct);assert.equal(sourceBindingOptions.expectedHelper,externalBinding.prospectiveHelper);assert.equal(provenance.sourceCommit,externalBinding.prospectiveProduct);",'')
  start=after.index('const dockerEndpoint=');end=after.index("assert.equal(docker('ps'",start)
  after=after[:start]+"const docker = (...values) => execFileSync('docker', values, { encoding: 'utf8', timeout: Math.ceil(commandBudget(30_000)) }).trim();\n"+after[end:]
  after=after.replace('const upstream=dockerEndpoint;',"const upstream=process.env.DOCKER_HOST||'unix:///var/run/docker.sock';assert(upstream.startsWith('unix://'));")
  self.assertEqual(after,base)
if __name__=='__main__':unittest.main()
