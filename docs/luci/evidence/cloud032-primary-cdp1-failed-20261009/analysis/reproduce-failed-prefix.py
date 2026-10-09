#!/usr/bin/env python3
"""Reproducible reconstruction of the prior inline analysis; not run yet.

Read the exact private run1 receipt/profile after the measurement window. Never
overwrite the original projection. Parses complete prefix events only. Missing
events and a partial tail stay explicit; category byte weights are prefix-only.
"""
from pathlib import Path
import argparse,json,collections,hashlib,math

def main():
 p=argparse.ArgumentParser();p.add_argument('--result-dir',type=Path,required=True);p.add_argument('--output',type=Path,required=True);a=p.parse_args();root=a.result_dir
 trace=root/'private-primary-profile/browser-timeline.json';raw=trace.read_bytes();assert len(raw)==33554432
 assert hashlib.sha256(raw).hexdigest()=='87881413924fa94f254a8c56d78827ba56da3bdb3eaa6f29f7bbb9be6b8d08b0'
 text=raw.decode();start=text.index('[',text.index('"traceEvents"'))+1;decoder=json.JSONDecoder();pos=start
 cats=collections.Counter();names=collections.Counter();bytes_by_cat=collections.Counter();markers=[];first=None;last=None;count=0
 while pos<len(text):
  while pos<len(text) and text[pos] in '\r\n\t ,':pos+=1
  if pos>=len(text) or text[pos]==']':break
  try:event,end=decoder.raw_decode(text,pos)
  except json.JSONDecodeError:break
  if isinstance(event,dict):
   count+=1;assert count<=1000000;cat=event.get('cat','<none>');cats[cat]+=1;bytes_by_cat[cat]+=len(text[pos:end].encode());names[event.get('name','<none>')]+=1
   ts=event.get('ts')
   if isinstance(ts,(float,int)):first=ts if first is None else min(first,ts);last=ts if last is None else max(last,ts)
   if isinstance(event.get('name'),str) and event['name'].startswith(('luci-primary:','luci-latency:')):markers.append({'name':event['name'],'ts':event.get('ts'),'pid':event.get('pid'),'tid':event.get('tid')})
  pos=end
 cpu_raw=(root/'private-primary-profile/browser-cpu.cpuprofile').read_bytes();assert len(cpu_raw)==1097185
 assert hashlib.sha256(cpu_raw).hexdigest()=='265646ccaf4ef0742369958494d4c6656369283e8e5988eb331e0a9d0def3fe3'
 cpu=json.loads(cpu_raw);nodes={n['id']:n for n in cpu['nodes']};agg=collections.Counter();classes=collections.Counter()
 for sample,delta in zip(cpu['samples'],cpu['timeDeltas']):
  cf=nodes[sample]['callFrame'];fn=cf['functionName'];key=(fn,cf.get('url',''),cf.get('lineNumber'),cf.get('columnNumber'));agg[key]+=delta;classes[fn if fn in ['(idle)','(program)','(garbage collector)','(root)'] else 'javascript-or-other']+=delta
 report_raw=(root/'report.json').read_bytes();assert len(report_raw)<=64*1024**2;report=json.loads(report_raw);assert report['passed'] is False and len(report['interactions'])==400
 groups={k:[v['elapsedMs'] for v in report['interactions'] if v['kind']==k] for k in ['project-switch','input','draft-reset']}
 summary={'scope':'Preserved FAILED primary-cdp-1; prefix parser reads complete events only. This is not a complete trace or proof absent events did not occur. CPU and primary400 are separate retained artifacts.','trace':{'bytes':len(raw),'sha256':hashlib.sha256(raw).hexdigest(),'completeEventsInPrefix':count,'decodedPrefixBytes':pos,'trailingUnparsedBytes':len(text)-pos,'dataLossOccurred':True,'categoryCounts':cats,'categoryEncodedBytes':bytes_by_cat,'topEventNames':names.most_common(30),'minObservedTraceTs':first,'maxObservedTraceTs':last,'primaryMarkers':len([m for m in markers if m['name'].startswith('luci-primary:')]),'primaryActionIDs':sorted({int(m['name'].split(':')[1]) for m in markers if m['name'].startswith('luci-primary:')}),'latencyMarkers':len(markers)-len([m for m in markers if m['name'].startswith('luci-primary:')])},'cpu':{'sha256':hashlib.sha256(cpu_raw).hexdigest(),'bytes':len(cpu_raw),'nodes':len(nodes),'samples':len(cpu['samples']),'deltas':len(cpu['timeDeltas']),'sampleClassesUs':classes,'wholeWindowUs':cpu['endTime']-cpu['startTime'],'topFrames':[{'functionNameSHA256':hashlib.sha256(k[0].encode()).hexdigest(),'urlSHA256':hashlib.sha256(k[1].encode()).hexdigest(),'line':k[2],'column':k[3],'sampleWeightUs':v,'specialFrame':k[0] if k[0] in ['(idle)','(program)','(garbage collector)','(root)'] else None} for k,v in agg.most_common(15)]},'primary':{k:{'count':len(v),'p95Ms':sorted(v)[math.ceil(.95*len(v))-1],'maxMs':max(v)} for k,v in groups.items()}}
 with a.output.open('x') as f:f.write(json.dumps(summary,indent=2)+'\n')
 a.output.chmod(0o600)

if __name__=='__main__':main()
