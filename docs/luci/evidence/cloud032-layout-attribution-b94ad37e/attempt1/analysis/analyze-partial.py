"""Read-only partial diagnostic; never supplies the required eight-round acceptance receipt."""
from pathlib import Path
import json,hashlib,collections,re
R=Path(__file__).resolve().parent;D=Path('/workspace/luci/artifacts/quality-swarm/layout-b94ad37e-attempt1/driver');P=Path('/tmp/luci-retained-tools/package-0.5.0-b94ad37e/web')
raw=(D/'layout-trace.raw.json').read_bytes();assert len(raw)==32234319 and hashlib.sha256(raw).hexdigest()=='45463172fffdaa9503bfb6bddb78e8dde13446305c60327362e7e6151974d763'
c=json.loads((D/'layout-trace-capture.json').read_text());trace=json.loads(raw);events=trace['traceEvents'];assert len(events)==28758
bundles=json.loads(Path('/workspace/.luci-tools/cloud032-layout-b94/actual/long-frame-bundles.json').read_text());byPath={x['path']:x['sha256'] for x in bundles}
selected=set(c['categories']['selected']);cats=collections.Counter();names=collections.Counter();categoryCounts=collections.Counter();nameCounts=collections.Counter();nameCategories=collections.defaultdict(set)
# Measure exact raw event JSON UTF8 byte lengths, excluding array separators/outer metadata.
text=raw.decode();cursor=re.search(r'"traceEvents"\s*:\s*\[',text).end();decoder=json.JSONDecoder()
for expected in events:
 while text[cursor] in ' \r\n\t,':cursor+=1
 event,end=decoder.raw_decode(text,cursor);assert event==expected;size=len(text[cursor:end].encode());cursor=end
 cats[event['cat']]+=size;categoryCounts[event['cat']]+=1;names[event['name']]+=size;nameCounts[event['name']]+=1;nameCategories[event['name']].add(event['cat'])
anchors=[]
for m in c['markers']:
 found=[x for x in events if x['name']==m['name']];assert len(found)==1;v=found[0];anchors.append({**m,'pid':v['pid'],'tid':v['tid'],'traceTsUs':v['ts'],'offsetUs':v['ts']-m['startTime']*1000})
assert len({(x['pid'],x['tid'],x['timeOrigin']) for x in anchors})==1
spread=max(x['offsetUs'] for x in anchors)-min(x['offsetUs'] for x in anchors);assert spread<=1000
pid,tid=anchors[0]['pid'],anchors[0]['tid'];lo=anchors[0]['traceTsUs'];hi=anchors[1]['traceTsUs'];partialStart=anchors[2]['traceTsUs']
layouts=[];sourceCache={};stackSummary=collections.Counter();forcedLayouts=[]
def safeFrame(f):
 from urllib.parse import urlparse
 u=urlparse(f.get('url',''));path=u.path;digest=byPath.get(path)
 if not digest or u.hostname!='127.0.0.1' or u.scheme!='http':return {'attribution':'unattributed'}
 line=f.get('lineNumber');col=f.get('columnNumber');assert isinstance(line,int) and isinstance(col,int)
 key=(path,line,col);stackSummary[key]+=1
 return {'attribution':'bundle','bundleSHA256':digest,'assetPath':path,'lineNumber':line,'columnNumber':col}
for index,e in enumerate(events):
 if e['name'] not in ['Layout','UpdateLayoutTree','RecalculateStyles']:continue
 stacks=[]
 for location,v in [('args',e.get('args',{})),('beginData',e.get('args',{}).get('beginData',{})),('data',e.get('args',{}).get('data',{})),('endData',e.get('args',{}).get('endData',{}))]:
  if isinstance(v,dict) and 'stackTrace' in v:
   frames=v['stackTrace'];assert isinstance(frames,list);stacks.append({'location':location,'frames':[safeFrame(f) for f in frames]})
 same=e['pid']==pid and e['tid']==tid;end=e['ts']+e.get('dur',0)
 layouts.append({'rawEventIndex':index,'name':e['name'],'ph':e['ph'],'pid':e['pid'],'tid':e['tid'],'tsUs':e['ts'],'durUs':e.get('dur'),'sameRenderer':same,'overlapsCompleteRound0':same and e['ts']<=hi and end>=lo,'afterUnfinishedRound1Start':same and end>=partialStart,'stacks':stacks})
# Names in these fixed browser categories are reviewed static Chrome instrumentation names.
nameRows=[{'name':n,'events':nameCounts[n],'rawEventJSONBytes':b,'categories':sorted(nameCategories[n])} for n,b in names.most_common(20)]
report={'scope':'Partial failed diagnostic only; one completed/two attempted rounds. No eight-round capture pass or latency acceptance. Raw private source preserved; this report does not invoke or weaken the strict projector.','rawTraceSHA256':hashlib.sha256(raw).hexdigest(),'rawBytes':len(raw),'totalEvents':len(events),'eventJSONBytes':sum(cats.values()),'categoryBytes':[{'category':cat,'events':categoryCounts[cat],'rawEventJSONBytes':b} for cat,b in cats.most_common()],'topEventByteCosts':nameRows,'unselectedOnlyNonMetadataEvents':sum(1 for e in events if e['cat']!='__metadata' and not selected.intersection(e['cat'].split(','))),'anchors':anchors,'anchorSpreadUs':spread,'allLayoutAndStyleEvents':layouts,'mappedStackPositions':[{'assetPath':p,'bundleSHA256':byPath[p],'lineNumber':line,'columnNumber':col,'occurrences':count} for (p,line,col),count in stackSummary.most_common()],'limits':'Raw event-byte attribution excludes array separators/outer metadata. Stack wall durations are not CPU attribution. Exact source positions may identify React/observer/framework wrappers, not a product component. Incomplete round1 has no closing marker; its suffix cannot be called a completed action cohort.'}
(R/'partial-layout-analysis.json').write_text(json.dumps(report,indent=2)+'\n')
print(json.dumps({'categoryBytes':report['categoryBytes'],'topEventByteCosts':nameRows,'layoutStyleEvents':len(layouts),'withStacks':sum(bool(x['stacks']) for x in layouts),'mappedPositions':len(stackSummary),'anchorSpreadUs':spread,'topLayouts':sorted([x for x in layouts if x['name']=='Layout'],key=lambda x:x['durUs'] or 0,reverse=True)[:5]},indent=2))
