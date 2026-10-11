#!/usr/bin/env python3
"""Offline allowlisted numeric projection; never modifies the raw receipt."""
from pathlib import Path
import argparse,hashlib,json,math
p=argparse.ArgumentParser();p.add_argument('raw',type=Path);p.add_argument('output',type=Path);a=p.parse_args();raw=a.raw.read_bytes();v=json.loads(raw);assert not a.output.exists()
kind={1:'handler',2:'handler_to_home_commit',3:'navigation_state',4:'scoped_navigation_write',5:'navigation_encode',6:'scope_scan',7:'navigation_set_item',8:'cockpit_serialize_write',9:'cockpit_partialize',10:'storage_key_count',11:'other_navigation_scope_count',12:'app_shell_render_entry_to_layout_effect'}
allowed=['action-start','all-projects-click-end','overview-heading-ready','open-project-click-end','action-end','verify-start','verify-end','frames-start','frames-end']
def number(x):assert isinstance(x,(float,int)) and not isinstance(x,bool) and math.isfinite(x);return x
def summary(x):
 x=sorted(x)
 return {'count':len(x),**({k:x[max(0,math.ceil(len(x)*q)-1)] for k,q in [('p50',.5),('p95',.95),('p99',.99),('max',1)]} if x else {})}
rows=[]
for window in v.get('browserEventWindows',[]):
 h=window.get('homePhase')
 if not h:continue
 for r in h['rows']:
  assert set(r)<=set(['sequence','home','kind','start','end','ok','count']);assert all(isinstance(x,(int,float)) and math.isfinite(x) for x in r.values());assert r['kind'] in kind
  rows.append({**r,'durationMs':r['end']-r['start']})
actions=[];switch=0
for i,r in enumerate(v.get('interactions',[]),1):
 assert r['kind'] in ['project-switch','input','draft-reset'];assert r['phase']=='idle-interactive-calibration'
 switch+=r['kind']=='project-switch'
 d=r.get('diagnostics',{});stages=d.get('nodeStages',[]);assert all(s['stage'] in allowed for s in stages)
 offsets={s['stage']:number(s['offsetMs']) for s in stages}
 spans={}
 for label,start,end in [('all_projects_command','action-start','all-projects-click-end'),('overview_wait','all-projects-click-end','overview-heading-ready'),('open_project_command','overview-heading-ready','open-project-click-end'),('saved_value_verification','verify-start','verify-end'),('two_frame_command','frames-start','frames-end')]:
  if start in offsets and end in offsets:spans[label]=offsets[end]-offsets[start]
 frames=d.get('browserFrames') or {};browser={k:number(frames[k]) for k in ['requestedAt','firstFrameAt','secondFrameAt'] if k in frames}
 actions.append({'ordinal':i,'kind':r['kind'],'homeOrdinal':switch if r['kind']=='project-switch' else None,'success':bool(r['success']),'elapsedMs':number(r['elapsedMs']),'nodeOffsetsMs':offsets,'nodeSpansMs':spans,'browserFrames':browser})
result={'version':1,'rawPrivateSHA256':hashlib.sha256(raw).hexdigest(),'diagnosticOnly':True,'acceptance':False,'rawCapturePassed':v.get('passed') is True,'completeCapture':v.get('homePhaseCapture',{}).get('complete') is True,'sourceCommit':v.get('sourceCommit'),'binarySHA256':v.get('binarySHA256'),'diagnosticWebBindingSHA256':v.get('diagnosticWebBindingSHA256'),'packagedWebIndexSHA256':v.get('packagedWebIndexSHA256'),'instrumentedWebIndexSHA256':v.get('instrumentedWebIndexSHA256'),'kinds':kind,'actions':actions,'homeRows':rows,'actionSummaryMs':{k:summary([r['elapsedMs'] for r in actions if r['kind']==k]) for k in ['project-switch','input','draft-reset']},'homeSummaryMs':{label:summary([r['durationMs'] for r in rows if r['kind']==k]) for k,label in kind.items() if k not in [10,11]},'limits':['Eight rounds are diagnostic, not the 100-switch acceptance sample.','Node and browser clocks are never subtracted.','Nested phase durations overlap and are not added.','Render entry to layout effect includes scheduling and commit; it is not isolated CPU time.','No causal conclusion from counts or association alone.','Original 972d failure remains unchanged.']}
a.output.write_text(json.dumps(result,indent=2)+'\n');a.output.chmod(0o600);print(json.dumps({'actions':len(actions),'homeRows':len(rows),'completeCapture':result['completeCapture'],'projectionSHA256':hashlib.sha256(a.output.read_bytes()).hexdigest()}))
