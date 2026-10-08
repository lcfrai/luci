"""Controlled JSON/process/cgroup fixtures only; never contact Docker or spawn."""
import contextlib, copy, datetime, importlib.util, io, json, pathlib, sys, tempfile, types, unittest
from unittest.mock import patch
HERE=pathlib.Path(__file__).parent
spec=importlib.util.spec_from_file_location('owned_sampler',HERE/'sample-linux-owned.py')
sampler=importlib.util.module_from_spec(spec);spec.loader.exec_module(sampler)
CID='a'*64;FOREIGN='b'*64;IMAGE='sha256:'+'c'*64;INSTANCE='fixture-instance';ENGINE='fixture-engine'
LABELS={'luci.check':'true','luci.check-protocol':'1','luci.instance':INSTANCE,'luci.check-engine':ENGINE,'luci.check-allocation':'checkexec-'+'d'*24,'luci.check-id':'check-'+'e'*24,'luci.check-command-index':'0','luci.check-scope':'1'*64,'luci.check-project':'p-12345678','luci.check-command-digest':'2'*64,'luci.check-source-digest':'3'*64,'luci.check-image':IMAGE}

def inspection(labels=LABELS,cid=CID,pid=123,running=True):
 return {'Id':cid,'Name':'/luci-check-'+labels.get('luci.check-allocation','worker'),'Image':IMAGE,'Config':{'Labels':copy.deepcopy(labels),'Env':['PRIVATE=must-never-appear']},'Mounts':[{'Source':'/private-mount'}],'State':{'Pid':pid,'Running':running},'HostConfig':{'Memory':2**30,'MemorySwap':2**30,'NanoCpus':10**9,'PidsLimit':256}}
def statistics(cid=CID):
 return {'id':cid,'read':datetime.datetime.now(datetime.timezone.utc).isoformat(),'memory_stats':{'usage':4096,'stats':{'inactive_file':1024}},'cpu_stats':{'cpu_usage':{'total_usage':1000000}},'pids_stats':{'current':1},'blkio_stats':{'io_service_bytes_recursive':[{'op':'Read','value':100}]}}

class SamplerTests(unittest.TestCase):
 def setUp(self):
  self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup)
  self.identity=pathlib.Path(self.temp.name)/'instance';self.identity.write_text(INSTANCE)
  sampler.a=types.SimpleNamespace(include_checks=True,image_id=IMAGE,instance_file=self.identity,instance_path_file=None)
  sampler.instance=None;sampler.docker_previous={};sampler.ever_containers=set();sampler.container_kinds={}
  self.rows=[{'Id':CID,'Labels':copy.deepcopy(LABELS),'ImageID':IMAGE}];self.observations=[inspection(),inspection()];self.stats=statistics();self.engine={'ID':ENGINE,'OSType':'linux'};self.calls=[];self.inspects=0
  self.stack=contextlib.ExitStack();self.addCleanup(self.stack.close)
  self.stack.enter_context(patch.object(sampler,'get',side_effect=self.get))
  self.stack.enter_context(patch.object(sampler,'stat',side_effect=lambda pid:{'pid':pid,'state':'S','startTicks':777}))
  self.stack.enter_context(patch.object(sampler,'read_cgroup',return_value='0::/docker/'+CID+'\n'))
 def get(self,path):
  self.calls.append(path)
  if path=='/containers/json?all=1':return copy.deepcopy(self.rows)
  if path=='/info':return copy.deepcopy(self.engine)
  if path=='/containers/'+CID+'/json':
   row=self.observations[min(self.inspects,len(self.observations)-1)];self.inspects+=1;return copy.deepcopy(row)
  if path=='/containers/'+CID+'/stats?stream=false&one-shot=true':return copy.deepcopy(self.stats)
  self.fail('Unexpected read-only endpoint '+path)
 def rejected(self,code):
  rows,issues=sampler.containers();self.assertEqual(rows,[]);self.assertIn(code,issues);self.assertEqual(sampler.ever_containers,set());return issues
 def test_opt_in_check_uses_fresh_current_identity_and_keeps_swap_cap_without_private_fields(self):
  rows,issues=sampler.containers();self.assertEqual(issues,[]);self.assertEqual(len(rows),1);row=rows[0]
  self.assertEqual((row['kind'],row['checkId'],row['allocationId']),('check',LABELS['luci.check-id'],LABELS['luci.check-allocation']))
  self.assertEqual(row['memorySwapLimitBytes'],2**30);self.assertEqual(row['memoryUsageBytes'],4096);self.assertEqual(row['workingSetBytes'],3072);self.assertEqual(row['pidStartTicks'],777);self.assertEqual(len(row['cgroupIdentitySHA256']),64)
  self.assertEqual(self.inspects,2);self.assertEqual(sampler.container_kinds,{CID:'check'});self.assertIn('not added to process RSS',row['memoryUsageBasis']);self.assertNotIn('must-never-appear',json.dumps(row));self.assertNotIn('/private',json.dumps(row));self.assertNotIn('/docker/',json.dumps(row))
 def test_legacy_default_excludes_check_without_querying_engine_or_stats(self):
  sampler.a.include_checks=False;self.assertEqual(sampler.containers(),([],[]));self.assertEqual(self.calls,['/containers/json?all=1'])
 def test_foreign_instance_excluded_before_inspection(self):
  self.rows[0]['Labels']['luci.instance']='foreign';self.assertEqual(sampler.containers(),([],[]));self.assertEqual(self.calls,['/containers/json?all=1'])
 def test_worker_still_included_and_tagged_with_opt_in_off(self):
  sampler.a.include_checks=False;labels={'luci.instance':INSTANCE,'luci.managed':'true','luci.sandbox-id':'s-12345678'};self.rows[0]['Labels']=labels;self.observations=[inspection(labels,pid=0,running=False)]
  rows,issues=sampler.containers();self.assertEqual(issues,[]);self.assertNotIn('kind',rows[0]);self.assertNotIn('memorySwapLimitBytes',rows[0]);self.assertNotIn('/info',self.calls)
 def test_malformed_own_candidate_is_explicit_and_not_measured(self):
  for key,value,code in [('luci.check','false','check-protocol-invalid'),('luci.check-protocol','2','check-protocol-invalid'),('luci.check-engine','wrong','check-instance-or-engine-mismatch'),('luci.check-image','sha256:'+'9'*64,'check-label-image-mismatch'),('luci.check-command-index','8','check-command-index-invalid'),('luci.check-source-digest','invalid','check-digest-invalid')]:
   with self.subTest(key=key):
    self.rows[0]['Labels']=copy.deepcopy(LABELS);self.rows[0]['Labels'][key]=value;self.rejected(code)
  self.rows[0]['Labels']=copy.deepcopy(LABELS);del self.rows[0]['Labels']['luci.check'];self.rejected('check-label-set-invalid')
  self.rows[0]['Labels']=dict(LABELS,**{'luci.managed':'true'});self.rejected('check-label-set-invalid')
  self.assertFalse(any('/stats?' in path for path in self.calls))
 def test_inspection_id_image_or_labels_changed_is_rejected(self):
  self.observations[0]['Id']=FOREIGN;self.rejected('check-inspect-id-mismatch');self.inspects=0
  self.observations[0]=inspection();self.observations[0]['Image']='sha256:'+'9'*64;self.rejected('check-inspect-identity-changed')
 def test_stale_stats_or_wrong_stats_id_cannot_be_usage(self):
  self.stats['read']=(datetime.datetime.now(datetime.timezone.utc)-datetime.timedelta(seconds=20)).isoformat();self.rejected('check-stats-not-fresh');self.inspects=0
  self.stats=statistics(FOREIGN);self.rejected('check-stats-id-mismatch')
 def test_reused_pid_is_rejected_and_cpu_counter_not_retained(self):
  sampler.stat.side_effect=[{'pid':123,'state':'S','startTicks':777},{'pid':123,'state':'S','startTicks':778}];self.rejected('check-process-or-cgroup-changed');self.assertEqual(sampler.docker_previous,{})
 def test_current_pid_requires_matching_cgroup_and_stable_cgroup(self):
  sampler.read_cgroup.return_value='0::/docker/'+FOREIGN+'\n';self.rejected('check-cgroup-identity-mismatch');self.assertFalse(any('/stats?' in path for path in self.calls))
 def test_v1_relevant_groups_match_while_unscoped_systemd_group_is_not_counted(self):
  sampler.read_cgroup.return_value='3:memory:/docker/'+CID+'\n2:cpu,cpuacct:/docker/'+CID+'\n1:name=systemd:/\n';rows,issues=sampler.containers();self.assertEqual(issues,[]);self.assertEqual(rows[0]['pidStartTicks'],777)
 def test_restart_or_exit_during_stats_is_explicit(self):
  self.observations[1]['State']['Pid']=456;self.rejected('check-restarted-or-exited-during-observation')
 def test_changed_instance_file_never_retargets_ownership(self):
  sampler.resolve_instance();self.identity.write_text('different-instance');self.rejected('exact-owned-instance-identity-changed');self.assertEqual(self.calls,[])
 def test_created_check_records_identity_and_caps_without_claiming_usage(self):
  self.observations=[inspection(pid=0,running=False)];rows,issues=sampler.containers();self.assertEqual(issues,[]);self.assertEqual(rows[0]['kind'],'check');self.assertNotIn('memoryUsageBytes',rows[0]);self.assertFalse(any('/stats?' in path for path in self.calls))
 def test_check_candidates_retain_eight_container_bound(self):
  self.rows=[copy.deepcopy(self.rows[0]) for _ in range(9)]
  self.rejected('owned-inventory-exceeded-eight-container-bound');self.assertEqual(self.calls,['/containers/json?all=1'])
 def test_duplicate_inventory_cannot_double_count(self):
  self.rows.append(copy.deepcopy(self.rows[0]));rows,issues=sampler.containers();self.assertEqual(len(rows),1);self.assertIn('duplicate-container-id',issues)

 def capture_one_tick(self,include_checks=True):
  output=pathlib.Path(self.temp.name)/'output';clock=[0.0]
  argv=['sampler','--driver-pid','1234','--driver-start-ticks','1','--instance-file',str(self.identity),'--image-id',IMAGE,'--output',str(output),'--seconds','1']+(['--include-checks'] if include_checks else [])
  with patch.object(sys,'argv',argv),patch.object(sampler.time,'monotonic',side_effect=lambda:clock[0]),patch.object(sampler.time,'sleep',side_effect=lambda seconds:clock.__setitem__(0,clock[0]+seconds)),patch.object(sampler,'processes',return_value=([{'pid':1234,'startTicks':1,'state':'S','residentBytes':8192,'role':'test-driver'}],0)),patch.object(sampler,'host',return_value={}),contextlib.redirect_stdout(io.StringIO()):
   sampler.main()
  metadata=json.loads((output/'metadata.json').read_text());summary=json.loads((output/'summary.json').read_text());sample=json.loads((output/'samples.jsonl').read_text())
  return metadata,summary,sample,output
 def test_controlled_one_tick_output_preserves_provenance_and_separate_memory(self):
  metadata,summary,sample,output=self.capture_one_tick()
  self.assertTrue(metadata['includeChecks']);self.assertEqual((output/'accepted-sampler.py').read_bytes(),(HERE/'sample-linux-owned.py').read_bytes());self.assertEqual(len(metadata['helperSHA256']),64)
  self.assertEqual(summary['checkCoverage'],{'enabled':True,'identitySamples':1,'cgroupUsageSamples':1,'status':'cgroup-usage-observed'})
  self.assertEqual(summary['peakObserved']['singleContainerCgroupUsageBytes_check'],4096);self.assertEqual(sample['summedProcessRSSBytes'],8192);self.assertEqual(sample['containerMeasurementStatus'],'observed')
  self.assertEqual((output/'samples.jsonl').stat().st_mode&0o777,0o600)

 def test_identity_only_capture_has_no_fabricated_check_peak(self):
  self.observations=[inspection(pid=0,running=False)]
  metadata,summary,sample,_=self.capture_one_tick()
  self.assertEqual(summary['checkCoverage']['status'],'identity-only');self.assertEqual(summary['checkCoverage']['cgroupUsageSamples'],0);self.assertNotIn('singleContainerCgroupUsageBytes_check',summary['peakObserved'])
 def test_malformed_check_capture_is_partial_with_issue_counts_not_zero_peak(self):
  del self.rows[0]['Labels']['luci.check-source-digest']
  _,summary,sample,_=self.capture_one_tick()
  self.assertEqual(sample['containerMeasurementStatus'],'partial');self.assertEqual(summary['containerIssueCounts'],{'check-label-set-invalid':1});self.assertEqual(summary['checkCoverage']['status'],'not-observed');self.assertNotIn('singleContainerCgroupUsageBytes_check',summary['peakObserved'])
 def test_default_capture_keeps_legacy_metadata_and_summary_shape(self):
  metadata,summary,sample,_=self.capture_one_tick(False)
  self.assertNotIn('includeChecks',metadata);self.assertNotIn('checkCoverage',summary);self.assertNotIn('containerMeasurementStatus',sample)
 def test_malformed_numeric_check_stats_are_unavailable_and_invalid_cache_is_not_zero(self):
  for value in [None,-1,True,'4096',2.0,2**64]:
   with self.subTest(value=value):
    self.stats=statistics();self.stats['memory_stats']['usage']=value;self.inspects=0;self.rejected('check-stats-measurement-unavailable')
  self.stats=statistics();self.stats['memory_stats']['stats']['inactive_file']=-1;self.inspects=0
  rows,issues=sampler.containers();self.assertIn('check-working-set-counter-invalid',issues);self.assertEqual(rows[0]['memoryUsageBytes'],4096);self.assertIsNone(rows[0]['workingSetBytes'])

if __name__=='__main__':unittest.main()
