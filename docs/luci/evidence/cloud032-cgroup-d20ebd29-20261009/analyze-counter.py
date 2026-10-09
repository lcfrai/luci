"""Offline validation of retained counter rows; never samples or starts a workload."""
import argparse
import datetime
import gzip
import hashlib
import json
from pathlib import Path

MAX_BYTES=2*1024*1024
def digest(raw):return hashlib.sha256(raw).hexdigest()
def read(path):
    if str(path).endswith('.gz'):
        with gzip.open(path,'rb') as stream:raw=stream.read(MAX_BYTES+1)
    else:raw=Path(path).read_bytes()
    assert len(raw)<=MAX_BYTES
    return raw
def timestamp(value):
    date=datetime.datetime.fromisoformat(value.replace('Z','+00:00'))
    delta=date-datetime.datetime(1970,1,1,tzinfo=datetime.timezone.utc)
    return (delta.days*86400+delta.seconds)*1_000_000_000+delta.microseconds*1000

def analyze(cpu_path,phase_path,supervision_path):
    raw=read(cpu_path);lines=raw.splitlines(keepends=True)
    assert raw.endswith(b'\n') and 2<=len(lines)<=4000
    rows=[json.loads(line) for line in lines]
    header,footer=rows[0],rows[-1];samples=rows[1:-1]
    assert header['type']=='header' and footer['type']=='footer'
    assert [r['index'] for r in samples]==list(range(len(samples)))
    assert all(r['type']=='sample' for r in samples)
    assert footer['sampleRows']==len(samples) and footer['rowsBeforeFooter']==len(lines)-1
    assert footer['bytesBeforeFooter']==sum(map(len,lines[:-1]))
    assert header['limits']['intervalNS']==100_000_000 and header['limits']['totalRows']==4000
    assert header['limits']['totalBytes']==MAX_BYTES and header['limits']['durationNS']<=400_000_000_000
    assert footer['elapsedNS']<=header['limits']['durationNS'] and footer['cooperativeDeadlineOverrunNS']==0
    before,after=header['metadataBefore'],footer['metadataAfter']
    metadata_equal=before==after
    unavailable=[]
    def unknown(value,path=''):
        if isinstance(value,dict):
            if value.get('status')=='unavailable':unavailable.append({'path':path,**value})
            for key,child in value.items():unknown(child,path+'/'+key)
    unknown(before)
    previous=None;totals={};positive=[];max_late=max_read=0;skipped=0;raw_matches=0
    for row in samples:
        bracket=row['readBracket'];observation=row['observation'];delta=row['delta']
        assert all(type(bracket[key]) is int for key in bracket)
        assert bracket['startMonotonicNS']<=bracket['endMonotonicNS']
        assert row['readDurationNS']==bracket['endMonotonicNS']-bracket['startMonotonicNS']
        assert row['latenessNS']==max(0,bracket['startMonotonicNS']-row['plannedMonotonicNS'])
        max_late=max(max_late,row['latenessNS']);max_read=max(max_read,row['readDurationNS']);skipped+=row['missedDeadlinesBeforeNext']
        assert observation['status']=='observed' and observation['missing']==[] and observation['unknownKeys']==[]
        values=observation['values'];assert all(type(v) is int and v>=0 for v in values.values())
        reconstructed=''.join(f'{key} {value}\n' for key,value in values.items()).encode()
        assert len(reconstructed)==observation['rawBytes'] and digest(reconstructed)==observation['rawSHA256'];raw_matches+=1
        if previous is None:
            assert delta['status']=='baseline' and delta['values'] is None and delta['previousValidReadEndMonotonicNS'] is None
        else:
            assert row['plannedMonotonicNS']-previous['plannedMonotonicNS']==100_000_000
            prior=previous['observation']['values'];assert values.keys()==prior.keys()
            actual={key:values[key]-prior[key] for key in values};assert all(value>=0 for value in actual.values())
            assert delta['status']=='observed' and delta['values']==actual
            assert delta['resetKeys']==delta['changedFields']==[] and delta['interveningUnknownSamples']==0
            assert delta['previousValidReadEndMonotonicNS']==previous['readBracket']['endMonotonicNS']
            for key,value in actual.items():totals[key]=totals.get(key,0)+value
            if actual['nr_throttled']>0 or actual['throttled_usec']>0:
                positive.append({'sampleIndex':row['index'],'previousSampleIndex':previous['index'],
                                 'nrThrottledDelta':actual['nr_throttled'],'throttledUsecDelta':actual['throttled_usec'],
                                 'possibleCounterIntervalStartWallNS':previous['readBracket']['startWallNS'],
                                 'possibleCounterIntervalEndWallNS':bracket['endWallNS'],
                                 'previousReadBracket':previous['readBracket'],'currentReadBracket':bracket})
        previous=row
    assert totals=={key:samples[-1]['observation']['values'][key]-samples[0]['observation']['values'][key] for key in samples[0]['observation']['values']}
    assert footer['maxLatenessNS']==max_late and footer['maxReadDurationNS']==max_read
    assert footer['missedSamplingDeadlines']==skipped==0
    assert footer['unknownObservations']==footer['counterResetOrShapeChanges']==footer['overrunReads']==0
    assert footer['anyPositiveObservedThrottleDelta']==bool(positive)
    assert footer['metadataStable'] is False and footer['noPositiveThrottleWithinCompleteObservedPairs'] is False
    assert metadata_equal and unavailable==[{'path':'/namespaces/time','status':'unavailable','errno':2}]
    phase_binding=json.loads(Path(phase_path).read_text());phases=phase_binding['phases']
    phase_buckets={p['name']:{'positiveIntervalsWhollyWithinWallLabels':0,'nrThrottledDelta':0,'throttledUsecDelta':0} for p in phases}
    unassigned=[]
    for interval in positive:
        start,end=interval['possibleCounterIntervalStartWallNS'],interval['possibleCounterIntervalEndWallNS']
        contained=[p['name'] for p in phases if timestamp(p['startedAt'])<=start and end<=timestamp(p['finishedAt'])]
        overlaps=[p['name'] for p in phases if timestamp(p['startedAt'])<=end and start<=timestamp(p['finishedAt'])]
        interval['nominalWallContainedPhase']=contained[0] if len(contained)==1 else None
        interval['nominalWallOverlappingPhases']=overlaps
        if len(contained)==1:
            bucket=phase_buckets[contained[0]];bucket['positiveIntervalsWhollyWithinWallLabels']+=1
            bucket['nrThrottledDelta']+=interval['nrThrottledDelta'];bucket['throttledUsecDelta']+=interval['throttledUsecDelta']
        else:unassigned.append(interval['sampleIndex'])
    supervision=json.loads(Path(supervision_path).read_text())
    assert supervision['pid']==header['observerPID'] and supervision['observerStartedAcknowledgedByHeader'] is True
    assert supervision['settled'] is True and supervision['exitCode']==0 and supervision['forced'] is False
    assert supervision['sourceStable'] is True and supervision['sourcesBefore']==supervision['sourcesAfter']
    assert supervision['observationComplete'] is True and supervision['processOutcome']=='clean_exit'
    assert supervision['observation']['rows']==len(rows) and supervision['observation']['bytes']==len(raw)
    assert supervision['observation']['sampleRows']==len(samples)
    first_wall=samples[0]['readBracket']['startWallNS'];last_wall=samples[-1]['readBracket']['endWallNS']
    phase_coverage=[{'name':p['name'],'phaseStartWallNS':timestamp(p['startedAt']),'phaseEndWallNS':timestamp(p['finishedAt']),
                     'firstToLastCounterWindowEnclosesPhaseWallLabels':first_wall<=timestamp(p['startedAt']) and timestamp(p['finishedAt'])<=last_wall} for p in phases]
    return {'scope':'Offline validation and nominal wall-phase grouping of retained shared-cgroup counters; no new sampling, exact-action/process attribution or timing adjustment.',
            'cpuOriginalSHA256':digest(raw),'cpuOriginalBytes':len(raw),'totalRows':len(rows),'sampleRows':len(samples),
            'rawCounterReadHashesReconstructed':raw_matches,'adjacentDeltaComparisons':len(samples)-1,
            'totalObservedDeltas':totals,'positiveCounterIntervals':len(positive),'positiveIntervals':positive,
            'metadataBeforeAfterEqual':metadata_equal,'metadataStableOriginal':footer['metadataStable'],'metadataUnavailable':unavailable,
            'observerSelfCost':{'observeEntryThroughFooterCPUNS':footer['observerSelfCPUNS'],
                                'overlappingProcessCPUThroughFooterNS':footer['processCPUAtFooterNS'],
                                'watchdogThreadCPUNS':supervision['watchdogThreadCPU']['elapsedNS'],
                                'elapsedNS':footer['elapsedNS'],'readCount':footer['readCount'],
                                'maxLatenessNS':max_late,'maxReadDurationNS':max_read,'missedDeadlines':skipped,
                                'scope':'Process CPU contains observe() CPU; do not add them. Watchdog is separate. All exclude declared finalization/other work and do not measure full causal overhead.'},
            'counterWindow':{'firstRead':samples[0]['readBracket'],'lastRead':samples[-1]['readBracket']},
            'phaseBinding':phase_binding,'phaseCoverageByWallLabels':phase_coverage,'nominalWallPhaseBuckets':phase_buckets,
            'positiveIntervalIndicesOutsideOrAcrossPhaseBoundaries':unassigned,
            'closure':{'observerPID':header['observerPID'],'pidfdSignals':supervision['events'],
                       'exitCode':supervision['exitCode'],'forced':supervision['forced'],'sourceStable':supervision['sourceStable'],
                       'scope':'Exact observer child settlement/serialized capture only. App/browser closure is separate.'},
            'limits':['Shared visible cgroup may contain unrelated tasks; hidden ancestors are not observed.',
                      'Positive adjacent deltas show observed bandwidth throttling, not which process/action it affected.',
                      'MetadataStable remains false: missing time-namespace evidence is not repaired or treated as observed.',
                      'Wall-phase labels are coarse association only; no driver/kernel absolute monotonic clock calibration was added.',
                      'Fourteen positive counter intervals do not mean fourteen independent throttle events.',
                      'No previous-run backfill, timing subtraction, causal gain or acceptance pass.']}

if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--cpu',required=True);parser.add_argument('--phases',required=True)
    parser.add_argument('--supervision',required=True);parser.add_argument('--report',required=True)
    args=parser.parse_args();report=Path(args.report);assert not report.exists()
    result=analyze(args.cpu,args.phases,args.supervision)
    report.write_text(json.dumps(result,indent=2)+'\n')
    print(json.dumps({'report':str(report),'sha256':digest(report.read_bytes()),'samples':result['sampleRows'],
                      'positiveIntervals':result['positiveCounterIntervals'],'deltas':result['totalObservedDeltas'],
                      'phaseBuckets':result['nominalWallPhaseBuckets']}))
