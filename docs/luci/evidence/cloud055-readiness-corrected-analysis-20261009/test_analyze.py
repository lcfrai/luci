import unittest,copy,json
from analyze import analyze

def fixture():
 raw={'interactions':[],'browserEventWindows':[{'latency':{'timeOrigin':1,'rows':[]}}],'autonomousReadiness':{'status':'closed','setup':{'timeOrigin':1,'authorityGeneration':1},'windows':[{'timeOrigin':1,'authorityGeneration':1,'sequence':8,'closed':True,'activeGeneration':None,'overflow':False,'rows':[]}]}}
 for r in range(4):
  t=100+r*1000
  for position,kind in enumerate(['project-switch','input','input','draft-reset']):
   raw['interactions'].append({'at':'2026-10-09T00:00:00.000Z','phase':'idle-interactive-calibration','kind':kind,'projectID':'private-id-'+str(r),'success':True,'elapsedMs':150,'diagnostics':{'nodeStages':[],'browserFrames':{'requestedAt':t+80,'firstFrameAt':t+90,'secondFrameAt':t+100,'references':{'timeOrigin':1,'expectedDOM':{'sequence':r+1,'at':t+70,'actionKind':'project-switch'}}}}})
  raw['browserEventWindows'][0]['latency']['rows'].append({'kind':'expected-dom','actionKind':'project-switch','sequence':r+1,'at':t+70})
  for n,stage,click,observed in [(1,'home',t,t+10),(2,'define',t+20,t+40)]:
   raw['autonomousReadiness']['windows'][0]['rows'].append({'sequence':2*r+n,'generation':2*r+n,'round':r,'projectOrdinal':r,'stage':stage,'status':'ready','reason':'predicate-observed','authorityGeneration':1,'at':observed+.1,'clickAt':click,'observedAt':observed,'samples':2,'maxSampleGapMs':20})
 return raw
class Cases(unittest.TestCase):
 def test_signed_join_and_home_separate(self):
  v=analyze(fixture());self.assertEqual(v['summary']['joinedSwitches'],4);self.assertEqual(v['summary']['homeJoinedSwitches'],4);self.assertEqual(v['summary']['signedObservedToDriverMs']['p95Ms'],30);self.assertFalse(v['summary']['originalTargets']['project-switch']['withinOriginalReference']);self.assertEqual(len(v['interactions']),16)
 def test_negative_signed_lag_not_clamped(self):
  raw=fixture();raw['autonomousReadiness']['windows'][0]['rows'][1]['observedAt']=200;v=analyze(raw);self.assertEqual(v['switchJoins'][0]['defineObservedToDriverMs'],-30)
 def test_censored_define_keeps_only_home_interval(self):
  raw=fixture()
  for row in raw['autonomousReadiness']['windows'][0]['rows']:
   if row['stage']=='define':row.update(status='censored',reason='control-cap',observedAt=None)
  v=analyze(raw);self.assertEqual(v['summary']['joinedSwitches'],0);self.assertEqual(v['summary']['homeJoinedSwitches'],4);self.assertIsNone(v['summary']['signedObservedToDriverMs']['p95Ms'])
 def test_clock_mismatch_no_join(self):
  raw=fixture();raw['interactions'][0]['diagnostics']['browserFrames']['references']['timeOrigin']=2;v=analyze(raw);self.assertEqual(v['switchJoins'][0]['availability'],'unavailable');self.assertEqual(v['switchJoins'][0]['homeAvailability'],'unavailable')
 def test_failure_duration_preserved(self):
  raw=fixture();raw['interactions'][0].update(success=False,elapsedMs=15001,error='private-error');v=analyze(raw);self.assertEqual(v['summary']['cohorts']['idle-interactive-calibration:project-switch']['p95Ms'],15001);self.assertEqual(v['summary']['joinedSwitches'],3);self.assertEqual(len(v['interactions']),16)
 def test_private_values_not_projected(self):
  raw=fixture();raw['private']='private-token';raw['interactions'][0]['url']='private-url';raw['interactions'][0]['error']='private-error';text=json.dumps(analyze(raw));self.assertNotIn('private-id',text);self.assertNotIn('private-token',text);self.assertNotIn('private-error',text);self.assertNotIn('private-url',text)
if __name__=='__main__':unittest.main()
