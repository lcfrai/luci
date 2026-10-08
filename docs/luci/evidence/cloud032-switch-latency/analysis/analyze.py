#!/usr/bin/env python3
"""Read-only, content-free decomposition of the preserved CLOUD-032 witness."""
from pathlib import Path
from collections import defaultdict
import hashlib,json,math,statistics,subprocess

REPO=Path('/workspace/luci');HERE=Path(__file__).resolve().parent
BASE=REPO/'artifacts/quality-swarm/sustained-4df94ab9-helper-5d306b6d-attempt3'
path=BASE/'result/result.json';raw=path.read_bytes();data=json.loads(raw)
sha=lambda b:hashlib.sha256(b).hexdigest()
def stats(values):
    values=sorted(values)
    if not values:return {'count':0}
    return {'count':len(values),'p50Ms':values[math.ceil(len(values)*.5)-1],'p95Ms':values[math.ceil(len(values)*.95)-1],'p99Ms':values[math.ceil(len(values)*.99)-1],'maxMs':values[-1],'meanMs':statistics.mean(values)}

groups=defaultdict(list);documents=defaultdict(dict);last={}
for window in data['browserEventWindows']:
    latency=window['latency'];origin=latency['timeOrigin']
    for row in latency['rows']:
        assert row['sequence'] not in documents[origin]
        documents[origin][row['sequence']]=row
    last[origin]=latency
for item in data['interactions']:groups[item['phase']+':'+item['kind']].append(item)
report={'version':1,'source':{'path':str(path.relative_to(REPO)),'sha256':sha(raw),'product':data['sourceCommit'],'helper':data['harnessCheckout']['commit']},
 'scope':'Failed original sustained attempt. Loaded phase has only74 switches/148 inputs and is incomplete. All times are end-to-end automation, not physical input/display latency. No threshold change or observer-cost subtraction.',
 'methods':['Nearest-rank quantiles match original helper. Separate span quantiles are not additive. Nested click spans are contained in action.','Only subtract values within the same Node or browser clock. Last event references are temporally associated, not explicit causal IDs.','Resource rows are known-route completed browser observations only; not full in-flight/aborted inventory or backend spans.','Callbacks use final cumulative snapshot per document; never sum cumulative per-window counters.'],
 'groups':{},'resourceClasses':{},'browserCollector':{'documents':len(last),'callbacks':sum(v['callbacks'] for v in last.values()),'callbackMs':sum(v['callbackMs'] for v in last.values()),'costScope':'Partial callback body self-time only. Excludes other EventTiming collector, IPC/serialization, browser/Node clocks and observer process cost.'}}
for key,items in groups.items():
    spans=defaultdict(list);tails=[]
    for index,item in enumerate(items):
        marks={v['stage']:v['offsetMs'] for v in item['diagnostics']['nodeStages']}
        parts={}
        for name,end,start in [('action','action-end','action-start'),('verify','verify-end','verify-start'),('framesRPC','frames-end','frames-start'),('allProjectsClick','all-projects-click-end','action-start'),('overviewHeading','overview-heading-ready','all-projects-click-end'),('openProjectClick','open-project-click-end','overview-heading-ready')]:
            if start in marks and end in marks:parts[name]=marks[end]-marks[start];spans[name].append(parts[name])
        frame=item['diagnostics']['browserFrames']
        if frame:
            spans['browserFinalFrameInterval'].append(frame['secondFrameAt']-frame['requestedAt'])
            refs=frame.get('references') or {};ref=refs.get('expectedDOM')
            if ref and ref.get('actionKind')==item['kind']:
                previous=refs.get('input' if item['kind']=='input' else 'pointer')
                if previous:spans['browserLatestDispatchToExpectedDOM'].append(ref['at']-previous['at'])
        tails.append({'index':index,'at':item['at'],'elapsedMs':item['elapsedMs'],'nodeSpansMs':parts})
    threshold=100 if items[0]['kind']=='input' else 300 if items[0]['kind']=='project-switch' else None
    report['groups'][key]={'original':data['interactionSummary'][key],'aboveP95TargetCount':sum(v['elapsedMs']>threshold for v in items) if threshold else None,'spanDistributions':{name:stats(rows) for name,rows in spans.items()},'fiveLargest':sorted(tails,key=lambda v:v['elapsedMs'],reverse=True)[:5]}
routes=defaultdict(list)
for rows in documents.values():
    for row in rows.values():
        if row['kind']=='resource':routes[row['route']].append(row['duration'])
report['resourceClasses']={name:stats(rows) for name,rows in routes.items()}
costpath=BASE/'observer-observer-self-cost.json';cost=json.loads(costpath.read_text())
report['observer']={key:cost.get(key) for key in ['scope','wallSeconds','userCPUSeconds','systemCPUSeconds','cpuSecondsPerWallSecond','maxRSSBytes']}
report['observer']['sha256']=sha(costpath.read_bytes())
report['observer']['causality']='No matched no-observer replay. Full interpreter cost is not attributable solely to the additive cgroup reader.'
report['originalActionabilityTrace']={'available':False,'basis':'Retained artifact file inventory has no trace/HAR/Playwright API action logs; successful interactions only retain aggregate await-boundary spans and passive browser rows.','unresolved':['Element stability/actionability retries','Target bounding-box movement','Event-loop scheduling/transport gaps','React render/commit and storage spans','Backend function/request admission spans']}
paths=['desktop/src/components/AppShell.tsx','desktop/src/components/projects/ProjectsHome.tsx','desktop/src/components/projects/ProjectOverviewBoard.tsx','desktop/src/components/projects/HostCapacity.tsx','desktop/src/components/projects/project-overview.css','desktop/src/components/definition/DefinitionWorkspace.tsx','desktop/src/hooks/useProjectDraft.ts','desktop/src/hooks/useUnsavedProjectDraft.ts','desktop/src/store/slices/uiSlice.ts','desktop/src/store/store.ts','desktop/src/components/rail/ProjectRow.tsx','desktop/src/components/rail/Rail.tsx','desktop/src/components/orchestrator/OrchestratorPanel.tsx','desktop/src/components/common/DraftNavigationDialog.tsx','desktop/src/components/common/WorkflowDialog.tsx','desktop/src/styles/shadcn.css','desktop/src/styles/tokens.css']
report['sourceBindings']={}
for name in paths:
    pinned=subprocess.check_output(['git','show',data['sourceCommit']+':'+name],cwd=REPO,timeout=10)
    live=(REPO/name).read_bytes();report['sourceBindings'][name]={'productSHA256':sha(pinned),'currentSHA256':sha(live),'matchesProduct':pinned==live}
    target=HERE/'product-source'/name;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(pinned)
(HERE/'analysis.json').write_text(json.dumps(report,indent=2)+'\n')
print(json.dumps({'analysisOnly':True,'report':str(HERE/'analysis.json'),'groups':len(groups),'productSourceFiles':len(paths),'allSourceFilesMatchProduct':all(v['matchesProduct'] for v in report['sourceBindings'].values())}))
