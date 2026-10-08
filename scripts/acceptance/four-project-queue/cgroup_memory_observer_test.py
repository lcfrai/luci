"""Controlled bounded file/link/process fixtures; no native proc reads or Docker."""
import contextlib, copy, importlib.util, io, json, pathlib, sys, tempfile, unittest
from unittest.mock import patch
HERE=pathlib.Path(__file__).parent
spec=importlib.util.spec_from_file_location('cgroup_sampler',HERE/'sample-linux-owned.py')
sampler=importlib.util.module_from_spec(spec);spec.loader.exec_module(sampler)
G=2**30
def process_stat(start=777):return ('123 (controlled) '+' '.join(['S']+['0']*18+[str(start)]+['0']*3)).encode()
class CgroupMemoryTests(unittest.TestCase):
 def setUp(self):
  self.files={'/proc/123/stat':process_stat(),'/proc/123/cgroup':b'0::/tenant/worker\n','/proc/123/mountinfo':b'1 2 0:3 / /cg rw - cgroup2 cgroup rw\n'}
  for path,limit,current in [('/cg/tenant/worker',8*G,3*G),('/cg/tenant',6*G,4*G),('/cg','max',9*G)]:
   self.node(path,limit,current)
  self.targets=[{'pid':123,'startTicks':777,'role':'control-plane','state':'S'}];self.physical={'MemTotal':16*G,'MemAvailable':12*G,'physicalCountersValidated':True};self.calls=[];self.read_counts={};self.clock_value=0
 def node(self,path,limit,current,version=2):
  maximum,usage=('memory.max','memory.current') if version==2 else ('memory.limit_in_bytes','memory.usage_in_bytes')
  self.files[path+'/'+maximum]=str(limit).encode();self.files[path+'/'+usage]=str(current).encode();self.files[path+'/memory.stat']=b'anon 10\nfile 20\ninactive_file 15\nPRIVATE_credential_do_not_emit 99\n'
 def read(self,path,limit):
  self.calls.append(path);self.read_counts[path]=self.read_counts.get(path,0)+1
  if path not in self.files:raise FileNotFoundError(path)
  value=self.files[path]
  if isinstance(value,BaseException):raise value
  if callable(value):value=value(self.read_counts[path])
  return value[:limit+1]
 def link(self,path):return 'cgroup:[11]' if path.endswith('/cgroup') else 'mnt:[22]'
 def capture(self,**kwargs):return sampler.cgroup_memory_observations(self.targets,self.physical,read=self.read,link=kwargs.pop('link',self.link),clock=kwargs.pop('clock',lambda:self.clock_value),**kwargs)
 def observation(self,**kwargs):return self.capture(**kwargs)['observations'][0]
 def unavailable(self,code,**kwargs):
  row=self.observation(**kwargs);self.assertIn(code,row['issues']);self.assertIsNone(row['productComparableEffectiveTotalBytes']);self.assertIsNone(row['productComparableEffectiveAvailableBytes']);return row
 def test_parent_usage_is_paired_with_parent_limit_and_not_leaf_usage(self):
  value=self.capture();row=value['observations'][0]
  self.assertEqual(value['status'],'observed');self.assertEqual(row['productComparableEffectiveTotalBytes'],6*G);self.assertEqual(row['productComparableEffectiveAvailableBytes'],2*G)
  self.assertEqual([n['depth'] for n in row['nodes']],[0,1,2]);self.assertTrue(row['nodes'][-1]['mountRoot']);self.assertEqual(row['nodes'][1]['limitMinusCurrentBytes'],2*G)
  self.assertEqual(row['physicalTotalBytes'],16*G);self.assertEqual(row['physicalAvailableBytes'],12*G);self.assertTrue(row['identityAndTopologyStable'])
  self.assertEqual(row['nodes'][0]['stat'],{'anon':10,'file':20,'inactive_file':15});self.assertNotIn('/tenant',json.dumps(value));self.assertNotIn('PRIVATE',json.dumps(value));self.assertNotIn('/cg',json.dumps(value));self.assertEqual(self.read_counts['/proc/123/stat'],3)
 def test_physical_available_can_bind_independently_of_limit_total(self):
  self.physical['MemAvailable']=G;row=self.observation();self.assertEqual(row['productComparableEffectiveTotalBytes'],6*G);self.assertEqual(row['productComparableEffectiveAvailableBytes'],G)
 def test_unlimited_leaf_still_observes_finite_parent(self):
  self.files['/cg/tenant/worker/memory.max']=b'max';row=self.observation();self.assertEqual(row['nodes'][0]['limitStatus'],'unlimited');self.assertEqual(row['productComparableEffectiveAvailableBytes'],2*G)
 def test_usage_at_or_over_limit_is_zero_not_unavailable(self):
  for current in [6*G,8*G]:
   with self.subTest(current=current):self.files['/cg/tenant/memory.current']=str(current).encode();self.assertEqual(self.observation()['productComparableEffectiveAvailableBytes'],0)
 def test_missing_limit_is_explicit_product_skip_and_not_unlimited(self):
  del self.files['/cg/tenant/memory.max'];row=self.observation();self.assertEqual(row['nodes'][1]['limitStatus'],'missing');self.assertIn('limit-file-missing-product-skips-level',row['issues']);self.assertEqual(row['productComparableEffectiveAvailableBytes'],5*G)
 def test_missing_finite_usage_invalidates_and_later_valid_parent_cannot_restore(self):
  del self.files['/cg/tenant/worker/memory.current'];row=self.unavailable('current-unavailable');self.assertTrue(row['identityAndTopologyStable']);self.assertEqual(row['status'],'partial')
 def test_unlimited_usage_missing_is_diagnostic_gap_not_formula_input(self):
  del self.files['/cg/memory.current'];row=self.observation();self.assertIn('current-unavailable',row['issues']);self.assertEqual(row['productComparableEffectiveAvailableBytes'],2*G)
 def test_invalid_or_unreadable_finite_limit_never_becomes_physical_available(self):
  for raw in [b'0',b'-1',b'+1',b'1.5',str(2**64).encode(),b'',PermissionError('PRIVATE-path')]:
   with self.subTest(raw=type(raw).__name__):self.files['/cg/tenant/worker/memory.max']=raw;row=self.unavailable('limit-unavailable');self.assertNotIn('PRIVATE',json.dumps(row))
 def test_invalid_or_unreadable_usage_is_not_zero(self):
  for raw in [b'-1',b'nan',str(2**64).encode(),PermissionError('PRIVATE')]:
   with self.subTest(raw=type(raw).__name__):self.files['/cg/tenant/worker/memory.current']=raw;self.unavailable('current-unavailable')
 def test_v1_finite_and_large_numeric_sentinel_follow_actual_product(self):
  self.files['/proc/123/cgroup']=b'4:memory:/nested\n3:cpu,cpuacct:/nested\n';self.files['/proc/123/mountinfo']=b'1 2 0:3 / /mem rw - cgroup cgroup rw,memory\n2 2 0:4 / /cpu rw - cgroup cgroup rw,cpu,cpuacct\n'
  self.node('/mem/nested',4*G,3*G,1);self.node('/mem',2**63-4096,5*G,1)
  row=self.observation();self.assertEqual(row['productComparableEffectiveTotalBytes'],4*G);self.assertEqual(row['productComparableEffectiveAvailableBytes'],G);self.assertEqual(row['nodes'][1]['limitStatus'],'finite');self.assertEqual(row['nodes'][1]['limitBytes'],2**63-4096)
  self.files['/mem/nested/memory.limit_in_bytes']=b'max';self.unavailable('limit-unavailable')
 def test_missing_cpu_mount_matches_global_product_location_failure(self):
  self.files['/proc/123/cgroup']=b'4:memory:/nested\n3:cpu:/nested\n';self.files['/proc/123/mountinfo']=b'1 2 0:3 / /mem rw - cgroup cgroup rw,memory\n';self.unavailable('controller-mount-unavailable')
 def test_cpu_only_overdeep_hierarchy_invalidates_memory_like_product(self):
  self.files['/proc/123/cgroup']=('4:memory:/nested\n3:cpu:/'+'/'.join(['deep']*128)).encode();self.files['/proc/123/mountinfo']=b'1 2 0:3 / /mem rw - cgroup cgroup rw,memory\n2 2 0:4 / /cpu rw - cgroup cgroup rw,cpu\n';self.unavailable('hierarchy-depth-bound')
 def test_namespace_root_membership_does_not_walk_above_visible_mount(self):
  self.files['/proc/123/cgroup']=b'0::/\n';self.files['/proc/123/mountinfo']=b'1 2 0:3 /.. /cg rw - cgroup2 cgroup rw\n';row=self.observation();self.assertEqual(len(row['nodes']),1);self.assertTrue(row['nodes'][0]['mountRoot']);self.assertEqual(row['productComparableEffectiveAvailableBytes'],12*G);self.assertNotIn('/memory.max',self.calls)
 def test_escaped_mount_path_and_subtree_mount_root(self):
  self.files['/proc/123/mountinfo']=b'1 2 0:3 /tenant /my\\040group rw - cgroup2 cgroup rw\n';self.node('/my group/worker',8*G,3*G);self.node('/my group',6*G,4*G)
  row=self.observation();self.assertEqual(row['productComparableEffectiveAvailableBytes'],2*G);self.assertEqual(len(row['nodes']),2);self.assertNotIn('/my group',json.dumps(row))
 def test_membership_traversal_and_unresolved_namespace_are_refused(self):
  for group in ['../../private','/../../private','/tenant/../private','/tenant//worker','//tenant/worker']:
   with self.subTest(group=group):self.files['/proc/123/cgroup']=('0::'+group).encode();self.unavailable('membership-invalid')
  self.files['/proc/123/cgroup']=b'0::/different';self.files['/proc/123/mountinfo']=b'1 2 0:3 /tenant /cg rw - cgroup2 cgroup rw';self.unavailable('namespace-mount-mismatch')
 def test_empty_membership_is_visible_absence_not_proven_unlimited(self):
  self.files['/proc/123/cgroup']=b'';row=self.observation();self.assertEqual(row['nodes'],[]);self.assertIn('memory-controller-not-present',row['issues']);self.assertEqual(row['limitSource'],'physical');self.assertEqual(row['productComparableEffectiveAvailableBytes'],12*G)
 def test_different_cgroup_or_mount_namespace_stops_before_counter_reads(self):
  for kind in ['cgroup','mnt']:
   with self.subTest(kind=kind):
    self.calls=[];self.unavailable(kind+'-namespace-differs',link=lambda path:kind+':[99]' if path=='/proc/123/ns/'+kind else self.link(path));self.assertFalse(any('/memory.' in path for path in self.calls))
 def test_pid_reuse_or_exit_after_counters_invalidates_effective_estimate(self):
  self.files['/proc/123/stat']=lambda call:process_stat(777 if call==1 else 778);self.unavailable('process-identity-changed')
 def test_migration_or_mount_change_between_reads_invalidates_estimate(self):
  for path in ['/proc/123/cgroup','/proc/123/mountinfo']:
   with self.subTest(path=path):
    self.setUp();initial=self.files[path];self.files[path]=lambda call:initial if call==1 else initial+b'\n';self.unavailable('process-topology-changed')
 def test_namespace_change_after_counters_invalidates_estimate(self):
  calls=[0]
  def link(path):
   calls[0]+=1
   return ('cgroup:[33]' if path.endswith('/cgroup') else 'mnt:[44]') if calls[0]>4 else self.link(path)
  self.unavailable('process-topology-changed',link=link)
 def test_stat_is_selected_and_never_subtracted_from_formula(self):
  for raw in [b'anon -1',b'anon 1\nanon 2',b'PRIVATE_only 99',b'anon '+b'9'*5000]:
   with self.subTest(raw=raw):self.files['/cg/tenant/memory.stat']=raw;row=self.observation();self.assertIn('stat-unavailable',row['issues']);self.assertEqual(row['productComparableEffectiveAvailableBytes'],2*G);self.assertIsNone(row['nodes'][1]['stat'])
 def test_invalid_physical_measurement_cannot_be_repaired_by_cgroup(self):
  for physical in [None,{}, {'MemTotal':0,'MemAvailable':0},{'MemTotal':1,'MemAvailable':2},{'MemTotal':True,'MemAvailable':0},{'MemTotal':2**64,'MemAvailable':0}]:
   with self.subTest(physical=physical):self.physical=physical;self.unavailable('physical-memory-unavailable')
 def test_physical_probe_requires_kb_complete_counters_and_no_overflow(self):
  self.assertEqual(sampler.cgroup_memory_physical('MemTotal: 100 kB\nMemAvailable: 20 kB\nMemFree: 19 kB'),{'MemTotal':102400,'MemAvailable':20480})
  for raw in ['MemTotal: 100 kB\nMemFree: 20 kB','MemTotal: 100 kB\nMemAvailable: 101 kB','MemTotal: 18446744073709551615 kB\nMemAvailable: 10 kB','MemTotal: 100 MB\nMemAvailable: 10 kB','MemTotal: +100 kB\nMemAvailable: 10 kB']:
   with self.subTest(raw=raw):
    with self.assertRaises(sampler.CgroupMemoryUnavailable):sampler.cgroup_memory_physical(raw)
  self.physical.pop('physicalCountersValidated');self.unavailable('physical-memory-unavailable')
 def test_file_byte_read_node_depth_time_and_output_bounds_fail_explicitly(self):
  for key,value,code in [('reads',3,'observation-read-bound'),('bytes',10,'observation-byte-bound'),('nodeRows',1,'observation-node-bound'),('hierarchyLevels',1,'hierarchy-depth-bound')]:
   with self.subTest(bound=key):bounds=dict(sampler.CGROUP_MEMORY_BOUNDS,**{key:value});self.unavailable(code,bounds=bounds)
  self.files['/proc/123/cgroup']=b'x'*65537;self.unavailable('observation-file-bound');self.setUp()
  clock=[0]
  def time_tick():clock[0]+=.02;return clock[0]
  self.unavailable('observation-time-bound',clock=time_tick)
  output=self.capture(bounds=dict(sampler.CGROUP_MEMORY_BOUNDS,outputBytes=1));self.assertEqual(output['observations'],[]);self.assertEqual(output['issues'],['observation-output-bound'])
 def test_target_bound_and_no_control_plane_does_not_claim_zero_memory(self):
  self.targets=[];value=self.capture();self.assertEqual(value['status'],'not-observed');self.assertEqual(value['observations'],[]);self.assertEqual(self.calls,[])
  self.targets=[{'pid':123,'startTicks':777,'role':'worker','state':'S'},{'pid':123,'startTicks':777,'role':'control-plane','state':'Z'}];self.assertEqual(self.capture()['observations'],[])
  self.targets=[{'pid':123,'startTicks':777,'role':'control-plane','state':'S'}]*9;value=self.capture();self.assertEqual(len(value['observations']),8);self.assertIn('control-plane-target-bound',value['issues'])
 def test_permission_or_disappeared_proc_has_code_only_and_no_estimate(self):
  for error in [PermissionError('PRIVATE credential'),FileNotFoundError('PRIVATE mount')]:
   with self.subTest(kind=type(error).__name__):self.files['/proc/123/cgroup']=error;row=self.unavailable('process-or-topology-unavailable');self.assertNotIn('PRIVATE',json.dumps(row))
 def test_long_leading_zero_numeric_counter_remains_product_compatible(self):
  self.files['/cg/tenant/memory.current']=b'0'*5000+str(4*G).encode();self.assertEqual(self.observation()['productComparableEffectiveAvailableBytes'],2*G)
 def test_actual_main_adds_diagnostics_and_captures_standalone_sampler(self):
  with tempfile.TemporaryDirectory() as directory:
   output=pathlib.Path(directory)/'output';clock=[0.0]
   argv=['sample-linux-owned.py','--driver-pid','1234','--driver-start-ticks','1','--instance-file',str(pathlib.Path(directory)/'not-read'),'--image-id','sha256:'+'a'*64,'--output',str(output),'--seconds','1']
   processes=[dict(self.targets[0],residentBytes=1024),{'pid':1234,'startTicks':1,'role':'test-driver','state':'S','residentBytes':2048}]
   with patch.object(sys,'argv',argv),patch.object(sampler.time,'monotonic',side_effect=lambda:clock[0]),patch.object(sampler.time,'sleep',side_effect=lambda seconds:clock.__setitem__(0,clock[0]+seconds)),patch.object(sampler,'processes',return_value=(processes,0)),patch.object(sampler,'containers',return_value=([],[])),patch.object(sampler,'host',return_value={'memory':copy.deepcopy(self.physical)}),patch.object(sampler,'cgroup_memory_read',side_effect=self.read),patch.object(sampler.os,'readlink',side_effect=self.link),contextlib.redirect_stdout(io.StringIO()):
    sampler.main()
   sample=json.loads((output/'samples.jsonl').read_text());metadata=json.loads((output/'metadata.json').read_text())
   self.assertEqual(sample['host']['memory'],self.physical);self.assertEqual(sample['summedProcessRSSBytes'],3072);self.assertEqual(sample['controlPlaneCgroupMemory']['observations'][0]['productComparableEffectiveAvailableBytes'],2*G)
   self.assertEqual(metadata['controlPlaneCgroupMemory']['productBasisSHA256'],sampler.CGROUP_MEMORY_PRODUCT_BASIS);self.assertEqual((output/'accepted-sampler.py').read_bytes(),(HERE/'sample-linux-owned.py').read_bytes());self.assertNotIn('containerMeasurementStatus',sample)

if __name__=='__main__':unittest.main()
