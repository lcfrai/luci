"""Small synthetic source-contract tests, no app/browser/worker data or launches."""
from pathlib import Path
import tempfile,unittest,json,subprocess,sys

class AnalyzerTests(unittest.TestCase):
 def run_fixture(self,mutate=lambda r,t:None):
  with tempfile.TemporaryDirectory(prefix='luci-profile-analysis-pure-') as tmp:
   p=Path(tmp);rows=[];events=[]
   def event(name,at):return{'name':name,'ts':10000000+at*1000,'pid':1,'tid':2,'cat':'blink.user_timing'}
   for i in range(1,401):
    kind=['project-switch','input','input','draft-reset'][(i-1)%4];start=i*100;requested=start+50;end=start+80
    refs={'timeOrigin':1,'input':{'sequence':i,'at':start},'pointer':{'sequence':i,'at':start},'expectedDOM':{'sequence':i+1000,'at':start+20,'actionKind':kind}}
    frames={'requestedAt':requested,'firstFrameAt':requested+10,'secondFrameAt':end,'references':refs,'primaryProfile':{'actionID':i,'markerOverflow':False}}
    rows.append({'phase':'idle-interactive-calibration','kind':kind,'elapsedMs':i,'success':True,'diagnostics':{'nodeStages':[{'stage':'action-start','offsetMs':0}],'browserFrames':frames}})
    events.extend([event(f'luci-primary:{i}:requested',requested),event(f'luci-primary:{i}:frame-2',end)])
    if kind!='draft-reset':events.extend([event('luci-latency:'+('input-dispatch' if kind=='input' else 'pointer-dispatch')+':'+str(i),start),event('luci-latency:expected-dom:'+str(i+1000),start+20)])
   receipt={'interactions':rows,'browserEventWindows':[{'latency':{'timeOrigin':1,'longTaskUnsupported':False,'longAnimationFrameUnsupported':True,'rows':[{'kind':'long-task','at':100,'duration':60}]}},{'latency':{'timeOrigin':999,'longTaskUnsupported':False,'longAnimationFrameUnsupported':True,'rows':[{'kind':'long-task','at':100,'duration':50000}]}}]}
   trace={'traceEvents':events};mutate(receipt,trace)
   for name,value in [('report.json',receipt),('browser-timeline.json',trace),('browser-cpu.cpuprofile',{'nodes':[{'id':1,'callFrame':{'functionName':'(program)','url':'','lineNumber':0,'columnNumber':0}}],'samples':[1],'timeDeltas':[50],'startTime':0,'endTime':50}),('profiling-status.json',{'complete':True,'limits':{'cpuIntervalUs':5000}})]:
    (p/name).write_text(json.dumps(value))
   result=subprocess.run([sys.executable,str(Path(__file__).with_name('analyze.py')),'--receipt',str(p/'report.json'),'--profile-dir',str(p),'--output',str(p/'out.json')],capture_output=True,text=True,timeout=5)
   return result,json.loads((p/'out.json').read_text()) if (p/'out.json').exists() else None
 def test_all400_join_and_excluded_categories_are_not_zero_measurements(self):
  result,data=self.run_fixture();self.assertEqual(result.returncode,0,result.stderr);self.assertEqual(data['primarySummary']['project-switch']['count'],100);self.assertEqual(data['primarySummary']['input']['count'],200);self.assertEqual(data['browserTraceAnchors']['spreadMs'],0);self.assertEqual(data['passiveSupport']['longAnimationFrameUnsupportedWindows'],2);self.assertEqual(data['passiveSupport']['timelineStyleLayoutPaintEvents'],'intentionally not captured; zero work cannot be inferred');self.assertIn('(program) means unknown/native',data['cpuWindow']['scope'])
 def test_missing_marker_refuses_complete_analysis(self):
  result,data=self.run_fixture(lambda r,t:t['traceEvents'].pop());self.assertNotEqual(result.returncode,0);self.assertIsNone(data)
 def test_duplicate_marker_refuses_ambiguous_clock_join(self):
  result,data=self.run_fixture(lambda r,t:t['traceEvents'].append(t['traceEvents'][0]));self.assertNotEqual(result.returncode,0);self.assertIsNone(data)
 def test_incomplete_primary_cohort_refused(self):
  result,data=self.run_fixture(lambda r,t:r['interactions'].pop());self.assertNotEqual(result.returncode,0);self.assertIsNone(data)
 def test_marker_overflow_refuses_join(self):
  def mutate(r,t):r['interactions'][0]['diagnostics']['browserFrames']['primaryProfile']['markerOverflow']=True
  result,data=self.run_fixture(mutate);self.assertNotEqual(result.returncode,0);self.assertIsNone(data)
 def test_different_document_clock_is_not_mixed_into_overlap(self):
  def mutate(r,t):r['interactions'][0]['elapsedMs']=5000
  result,data=self.run_fixture(mutate);self.assertEqual(result.returncode,0,result.stderr);first=next(x for x in data['slowCohorts'] if x['ordinal']==1);self.assertEqual(first['passiveLongObservationUnionMs']['long-task'],60)

if __name__=='__main__':unittest.main(verbosity=2)
