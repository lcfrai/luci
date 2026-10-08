#!/usr/bin/env python3
"""Summarize existing observer data without probing the host or changing the run."""
import argparse,datetime,json,pathlib,statistics
p=argparse.ArgumentParser();p.add_argument('directory',type=pathlib.Path);p.add_argument('--workflow-report',type=pathlib.Path,required=True);a=p.parse_args()
rows=[json.loads(line) for line in (a.directory/'samples.jsonl').read_text().splitlines() if line]
meta=json.loads((a.directory/'metadata.json').read_text());summary=json.loads((a.directory/'summary.json').read_text());workflow=json.loads(a.workflow_report.read_text())
def spread(values):
 v=sorted(float(x) for x in values if x is not None)
 return None if not v else {'samples':len(v),'min':v[0],'p50':statistics.median(v),'p95':v[min(len(v)-1,int(.95*(len(v)-1)))],'max':v[-1]}
containers={}
for row in rows:
 for c in row['containers']:
  item=containers.setdefault(c['id'],{'samples':[],'firstObservedAt':row['observedAt'],'lastObservedAt':row['observedAt']});item['lastObservedAt']=row['observedAt'];item['samples'].append(c)
for cid,item in containers.items():
 observed=item.pop('samples');item['observedSamples']=len(observed);item['memoryUsageBytes']=spread(c.get('memoryUsageBytes') for c in observed);item['workingSetBytes']=spread(c.get('workingSetBytes') for c in observed);item['cpuCoreEquivalent']=spread(c.get('cpuCoreEquivalent') for c in observed);item['observedPIDs']=sorted({(c['pid'],c.get('pidStartTicks')) for c in observed if c.get('pid')});item['memoryLimitBytes']=sorted({c['memoryLimitBytes'] for c in observed if c.get('memoryLimitBytes') is not None});item['nanoCPULimit']=sorted({c['nanoCPULimit'] for c in observed if c.get('nanoCPULimit') is not None})
host_cpu=[]
for before,after in zip(rows,rows[1:]):
 x=before['host'].get('cpuTicks');y=after['host'].get('cpuTicks')
 if x and y and len(x)==len(y):
  d=[b-a for a,b in zip(x,y)];total=sum(d)
  if total>0 and min(d)>=0:host_cpu.append(100*(total-d[3]-d[4])/total)
roles=sorted({k for row in rows for k in row['summedProcessRSSBytesByRole']})
gaps=[(datetime.datetime.fromisoformat(b['observedAt'])-datetime.datetime.fromisoformat(a['observedAt'])).total_seconds() for a,b in zip(rows,rows[1:])]
report={'measurementStartedAt':meta['startedAt'],'measurementFinishedAt':summary['finishedAt'],'driver':meta['driver'],'workflowSourceCommit':workflow.get('sourceCommit'),'workflowPassed':workflow.get('passed'),'workflowChecksPassed':[v['name'] for v in workflow.get('checks',[]) if v.get('passed')],'measurementScope':'actual Linux process family and exact-owned Docker observations; controlled provider; no model-quality or Windows claim','baseline':meta['baseline'],'sampleCount':len(rows),'driverAliveSamples':sum(x['driverAlive'] for x in rows),'containerActiveSamples':sum(any(c.get('running') for c in x['containers']) for x in rows),'sampleDurationSeconds':spread(x['sampleDurationSeconds'] for x in rows),'sampleIntervalSeconds':spread(gaps),'maxLiveOwnedProcesses':max(sum(p['state']!='Z' for p in x['processes']) for x in rows),'summedRSSBytes':spread(x['summedProcessRSSBytes'] for x in rows),'summedBrowserAndControlPlaneRSSBytes':spread(sum(x['summedProcessRSSBytesByRole'].get(role,0) for role in ('browser','control-plane')) for x in rows),'samplesWithMissingProcessIO':sum(any(p.get('io') is None for p in x['processes']) for x in rows),'finalLiveOwnedProcessCount':sum(p['state']!='Z' for p in rows[-1]['processes']),'peakRSSBytesByRole':{role:max(x['summedProcessRSSBytesByRole'].get(role,0) for x in rows) for role in roles},'hostAvailableMemoryBytes':spread(x['host'].get('memory',{}).get('MemAvailable') for x in rows if x['host'].get('memory')),'hostSwapFreeBytes':spread(x['host'].get('memory',{}).get('SwapFree') for x in rows if x['host'].get('memory')),'hostCPUUtilizationPercentAcrossVisibleLogicalCPUs':spread(host_cpu),'hostDiskAvailableBytes':{path:spread(x['host']['disks'].get(path,{}).get('availableBytes') for x in rows if x['host']['disks'].get(path)) for path in ('/','/workspace','/tmp')},'observedCounterDeltas':summary['observedCounterDeltas'],'containers':containers,'issues':{issue:sum(issue in x['containerIssues'] for x in rows) for issue in sorted({i for x in rows for i in x['containerIssues']})},'limitations':meta['limitations']}
(a.directory/'analysis.json').write_text(json.dumps(report,indent=2)+'\n')
print(json.dumps({'workflowPassed':report['workflowPassed'],'sampleCount':len(rows),'containerCount':len(containers),'maxLiveOwnedProcesses':report['maxLiveOwnedProcesses'],'summedRSSPeakGiB':report['summedRSSBytes']['max']/1024**3,'minimumHostAvailableGiB':report['hostAvailableMemoryBytes']['min']/1024**3},indent=2))
