#!/usr/bin/env python3
"""Offline selected-route overlap counts; never infer complete connection occupancy."""
from pathlib import Path
import gzip,json,collections
B=Path(__file__).resolve().parent;p=json.loads(gzip.decompress((B/'timings.all400.projection.json.gz').read_bytes()));resources=[r for r in p['browserLatencyRows'] if r['kind']=='resource'];switches=json.loads((B/'switches.all100.json').read_text());rows=[]
for ordinal in [49,58,87,96,99]:
 s=switches[ordinal-1];origin=s['browserReferences']['pointer']['timeOrigin'];home=s['homePointerAssociation'].get('sourceConsistentHomePointer');start=home['at'] if home else s['browserReferences']['pointer']['at'];end=s['browserReferences']['expectedDOM']['at'];scope=[r for r in resources if r['timeOrigin']==origin and r['at']<=end and r['responseEnd']>=start]
 targets=[r for r in scope if r['requestStart'] and r['requestStart']-r['at']>60]
 for r in targets:
  earlier=[o for o in resources if o['timeOrigin']==origin and o['sequence']!=r['sequence'] and o['at']<r['at']<o['responseEnd']]
  active_at_request=[o for o in resources if o['timeOrigin']==origin and o['sequence']!=r['sequence'] and o['requestStart'] and o['requestStart']<=r['requestStart']<o['responseEnd']]
  interval=[o for o in resources if o['timeOrigin']==origin and o['at']<=r['requestStart'] and o['responseEnd']>=r['at']]
  points=sorted({r['at'],r['requestStart'],*[o['at'] for o in interval if r['at']<=o['at']<=r['requestStart']]})
  occupancy=max(sum(o['at']<=t<o['responseEnd'] for o in interval) for t in points)
  prior_ends=[o['responseEnd'] for o in resources if o['timeOrigin']==origin and o['sequence']!=r['sequence'] and o['responseEnd']<=r['requestStart']]
  last=max(prior_ends) if prior_ends else None
  rows.append({'switchOrdinal':ordinal,'sequence':r['sequence'],'route':r['route'],'beforeRequestStartMs':r['requestStart']-r['at'],'earlierCapturedResourcesUnfinishedAtStart':len(earlier),'earlierCapturedRouteCounts':dict(collections.Counter(o['route'] for o in earlier)),'maxCapturedStartedButNotEndedDuringPreRequestInterval':occupancy,'otherCapturedRequestsInRequestToResponseIntervalAtRequestStart':len(active_at_request),'nearestEarlierCapturedResponseEndToRequestStartMs':r['requestStart']-last if last is not None else None})
result={'originalResultSHA256':p['originalResultSHA256'],'rows':rows,'retainedResourceFields':sorted(set().union(*(set(r) for r in resources))),'missingForConnectionAttribution':['nextHopProtocol','connectStart','connectEnd','domainLookupStart','domainLookupEnd','fetchStart','workerStart','connectionId','unclassified same-origin resource records','in-flight resources never completed before final capture'],'conclusion':'No retained delayed49/58/87 resource has six captured outstanding route-class resources during its pre-request interval. This does not rule out an HTTP/1.1 slot queue: the collector retains only six route classes, no protocol/connection fields and no exhaustive outstanding-request census. Request routing/scheduling/connection queue cannot be separated from these retained fields.'}
(B/'connection-queue-coverage.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps({'rows':rows,'missing':result['missingForConnectionAttribution']},indent=2))
