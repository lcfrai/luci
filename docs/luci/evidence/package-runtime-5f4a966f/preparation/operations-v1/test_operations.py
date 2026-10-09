"""Inert tests only: no Docker, process discovery, tool/archive byte reads or moves."""
import copy,types,unittest
from pathlib import Path
from unittest.mock import patch
import retire_old_image as ri
import relocate_tools as rt
from operation_common import Refusal,require_idle
NEW='sha256:'+'a'*64
BASELINE={ri.OLD:list(ri.TAGS),ri.BASE:['luci/base:trusted'],NEW:['luci/candidate:new'],'sha256:'+'b'*64:['untouched:tag']}
STAT=types.SimpleNamespace(st_dev=1,st_ino=2,st_size=1013367768,st_mtime_ns=3,st_mode=0o100600,st_uid=1000,st_gid=1000)
class FakeJournal:
 def __init__(self,*args):self.value={};self.path=Path('/inert/no-write');self.events=[]
 def save(self,stage,**v):self.value.update(v);self.value['stage']=stage;self.events.append((stage,copy.deepcopy(v)))
 def finish(self,passed,**v):self.save('finished',passed=passed,**v)
class FakeDocker:
 def __init__(self,images=None,fail_second=False):self.images=copy.deepcopy(BASELINE if images is None else images);self.commands=[];self.fail_second=fail_second
 def inventory(self):return copy.deepcopy(self.images)
 def __call__(self,*cmd):
  self.commands.append(cmd)
  if cmd==('info','--format','{{.ID}}'):return 'fixture-engine'
  if cmd==('ps','--all','--quiet','--no-trunc'):return ''
  if cmd[:3]==('image','inspect','--format'):return ri.OLD
  if cmd[:3]==('image','rm','--no-prune'):
   tag=cmd[3]
   if self.fail_second and tag==ri.TAGS[1]:raise Refusal('simulated-unknown-removal-outcome')
   self.images[ri.OLD].remove(tag)
   if not self.images[ri.OLD]:del self.images[ri.OLD]
   return ''
  raise AssertionError('unexpected fake call')
class Controls(unittest.TestCase):
 def run_fake_retirement(self,docker,scan=None,archive_sha=ri.ARCHIVE_SHA):
  journal=FakeJournal();args=['--execute-retirement','--new-image',NEW,'--candidate-receipt','/fixture/candidate','--candidate-receipt-sha256','0'*64,'--quiescence','/fixture/quiet','--quiescence-sha256','1'*64,'--receipt',str(ri.ROOT/'inert-test-never-created')]
  def pinned(path,pin):return {'status':'passed','candidateImage':NEW,'baseImage':ri.BASE,'sourceStable':True} if str(path)=='/fixture/candidate' else {}
  with patch.object(ri,'Journal',return_value=journal),patch.object(ri,'pinned_json',side_effect=pinned),patch.object(ri,'validate_archive_proof',return_value={'archiveBytes':STAT.st_size}),patch.object(ri,'hash_file',return_value=(archive_sha,STAT)),patch.object(ri,'quiescence',return_value={'exclusive':True}),patch.object(ri,'process_scan',return_value=scan or {'busy':[],'unknown':[]}),patch.object(ri,'Docker',return_value=docker),patch.object(Path,'lstat',return_value=STAT),patch.object(ri.os.path,'lexists',return_value=False):code=ri.main(args)
  return code,journal
 def test_exact_two_no_force_no_prune_removals_preserve_base_candidate_unrelated(self):
  d=FakeDocker();code,j=self.run_fake_retirement(d);self.assertEqual(code,0);self.assertTrue(j.value['passed']);self.assertEqual([x for x in d.commands if x[:2]==('image','rm')],[('image','rm','--no-prune',t) for t in ri.TAGS]);self.assertEqual(d.images,{k:v for k,v in BASELINE.items() if k!=ri.OLD})
 def test_unknown_process_stops_before_any_docker(self):
  d=FakeDocker();code,j=self.run_fake_retirement(d,{'busy':[],'unknown':[{'pid':42}]});self.assertEqual(code,1);self.assertEqual(d.commands,[]);self.assertFalse(j.value['partialMutationPossible'])
 def test_archive_mismatch_stops_before_any_docker(self):
  d=FakeDocker();code,j=self.run_fake_retirement(d,archive_sha='f'*64);self.assertEqual(code,1);self.assertEqual(d.commands,[])
 def test_foreign_extra_tag_refuses_before_removal(self):
  images=copy.deepcopy(BASELINE);images[ri.OLD].append('unowned:keep');d=FakeDocker(images);code,j=self.run_fake_retirement(d);self.assertEqual(code,1);self.assertFalse(any(x[:2]==('image','rm') for x in d.commands))
 def test_partial_second_removal_failure_durably_remains_pending(self):
  d=FakeDocker(fail_second=True);code,j=self.run_fake_retirement(d);self.assertEqual(code,1);self.assertTrue(j.value['partialMutationPossible']);self.assertEqual(j.value['pendingCommand'],ri.removal_command(ri.TAGS[1]));self.assertEqual(j.value['completedRemovals'],[ri.TAGS[0]]);self.assertIn(ri.BASE,d.images);self.assertIn(NEW,d.images)
 def test_same_as_protected_candidate_refused(self):
  for image in [ri.OLD,ri.BASE,'latest']:
   with self.subTest(image=image),self.assertRaises(Refusal):ri.validate_inventory(BASELINE,image)
 def test_unrelated_inventory_change_refused(self):
  after=copy.deepcopy(BASELINE);after[NEW]=['reassigned:tag']
  with self.assertRaises(Refusal):ri.validate_preserved(BASELINE,after,list(ri.TAGS))
 def test_arbitrary_removal_refused(self):
  for value in [ri.OLD,ri.BASE,NEW,'--force','untouched:tag']:
   with self.subTest(value=value),self.assertRaises(Refusal):ri.removal_command(value)
 def test_unknown_tool_and_direction_refused(self):
  for tool,direction in [('go1.26.3','to-root'),('rust-1.99.0','delete'),('../rust-1.99.0','to-root')]:
   with self.subTest(tool=tool),self.assertRaises(Refusal):rt.fixed_paths(tool,direction)
 def test_forward_return_paths_are_fixed_and_compatibility_path_unchanged(self):
  for tool in rt.TOOLS:
   a=rt.fixed_paths(tool,'to-root');b=rt.fixed_paths(tool,'return-to-tmp');self.assertEqual(a['source'],b['tmp']);self.assertEqual(a['destination'],b['source']);self.assertEqual(a['compat'],b['compat']);self.assertEqual(a['scratch'].parent,rt.TMP);self.assertEqual(b['destination'].parent,rt.TMP)
 def test_explicit_operation_flag_required_before_journal(self):
  with patch.object(rt,'Journal') as j,self.assertRaises(Refusal):rt.main(['--tool','rust-1.99.0','--direction','to-root','--quiescence','/inert','--quiescence-sha256','0'*64,'--receipt',str(rt.ROOT/'never-created')])
  j.assert_not_called()
 def test_active_and_unobservable_inactivity_are_not_zero(self):
  for scan in [{'busy':[1],'unknown':[]},{'busy':[],'unknown':[1]}]:
   with self.subTest(scan=scan),self.assertRaises(Refusal):require_idle(scan)
if __name__=='__main__':unittest.main()
