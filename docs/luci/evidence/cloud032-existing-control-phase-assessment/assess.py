#!/usr/bin/env python3
"""Bounded offline assessment of existing full-action controls and Node regions."""
import argparse, collections, gzip, hashlib, io, json, math
from pathlib import Path

PINS = {
 'cloud032-switch-latency/static/result.json.gz': '05099d76c0cf53b1921d7f33f2b49879d6adadd71ad709932633e294ff55f440',
 'cloud032-switch-latency/static/accepted-harness/control.mjs': 'b9581cc3460ca4034025906f9b0afa7fe4f2852ed5346263395846f3d04a1f98',
 'cloud032-switch-latency/static/accepted-harness/control.html': 'c5fa7ffb694ed0a76cc6446745c7848249d242994a21da9727857568b138e1af',
 'cloud032-switch-latency/static/accepted-harness/interaction-metrics.mjs': 'ec0bc9d06fda6636cbba6975e05b9c66979ca189b59aa38c373b37a318b79502',
 'cloud032-long-animation-frame-29931afc/analysis/switch-callback-joins.json': '0662c43f23c3e2071c1ad63038a3dc0f947b72d507927f0d7949a91c29daa057',
 'cloud032-navigation-callback-5f4a966f/switch-callback-joins.json': '6e52bbc509b2be68ccfd6284732cadc82279941f3af7b767afe7e7f9359c5f70',
 'cloud032-layout-attribution-b94ad37e/attempt2/all-interactions.json': 'd378b4fdfcb3ab7fd132633319de00b7b1d1db1842fcf8dfb668c023ddd042cf',
 'cloud032-settled-editor-control-20261009/analysis/analysis.json': '975df15fdbd3749da56e61914eae984932f788da386a75ff07fa9fab823eedeb',
}

def quantiles(values):
 values=sorted(values)
 assert values and all(math.isfinite(v) for v in values)
 return {'n':len(values),'min':values[0],'p50':values[math.ceil(len(values)*.5)-1],
         'p95':values[math.ceil(len(values)*.95)-1],'p99':values[math.ceil(len(values)*.99)-1],'max':values[-1]}

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--evidence-root',required=True,type=Path);p.add_argument('--output',required=True,type=Path);a=p.parse_args()
 assert not a.output.exists(),'Fresh output required'
 blobs={};bindings=[]
 for name,pin in PINS.items():
  f=a.evidence_root/name;assert f.stat().st_size<1024**2
  b=f.read_bytes();assert hashlib.sha256(b).hexdigest()==pin
  blobs[name]=b;bindings.append({'path':name,'bytes':len(b),'sha256':pin})
 with gzip.GzipFile(fileobj=io.BytesIO(blobs['cloud032-switch-latency/static/result.json.gz'])) as stream: raw=stream.read(8*1024**2+1)
 assert len(raw)<=8*1024**2
 assert hashlib.sha256(raw).hexdigest()=='ed5cd4d8c2dc75a13f085642537e43aa8d326909ca56bac6c798a56fb9e8127b'
 static=json.loads(raw);assert len(static['interactions'])==432
 static_groups={}
 for phase in sorted({r['phase'] for r in static['interactions']}):
  static_groups[phase]={}
  for kind in sorted({r['kind'] for r in static['interactions']}):
   rows=[r for r in static['interactions'] if r['phase']==phase and r['kind']==kind]
   if rows:static_groups[phase][kind]={**quantiles([r['elapsedMs'] for r in rows]),'successful':sum(r['success'] for r in rows),'over300':sum(r['elapsedMs']>300 for r in rows) if kind=='project-switch' else None}
 static_rows=[{'index':i,**{k:r[k] for k in ('phase','kind','success','elapsedMs')}} for i,r in enumerate(static['interactions'])]
 cohorts={};all_rows={}
 for name,key in [('299','cloud032-long-animation-frame-29931afc/analysis/switch-callback-joins.json'),('5f4','cloud032-navigation-callback-5f4a966f/switch-callback-joins.json')]:
  rows=json.loads(blobs[key]);assert len(rows)==108
  all_rows[name]=[{k:r[k] for k in ('index','phase','projectOrdinal','fullActionMs','nodeRegionsMs','largestNodeRegion')} for r in rows]
  cohorts[name]={}
  for phase in sorted({r['phase'] for r in rows}):
   phase_rows=[r for r in rows if r['phase']==phase];slow=[r for r in phase_rows if r['fullActionMs']>300]
   assert all(r['largestNodeRegion']==max(r['nodeRegionsMs'],key=r['nodeRegionsMs'].get) for r in phase_rows)
   cohorts[name][phase]={'fullAction':quantiles([r['fullActionMs'] for r in phase_rows]),'over300':len(slow),'largestRegion':dict(collections.Counter(r['largestNodeRegion'] for r in phase_rows)),
                       'largestRegionOver300':dict(collections.Counter(r['largestNodeRegion'] for r in slow)),
                       'regions':{k:quantiles([r['nodeRegionsMs'][k] for r in phase_rows]) for k in phase_rows[0]['nodeRegionsMs']}}
 b94=json.loads(blobs['cloud032-layout-attribution-b94ad37e/attempt2/all-interactions.json'])['rows']
 assert len(b94)==32
 b94_switch=[r for r in b94 if r['kind']=='project-switch'];assert len(b94_switch)==8
 output={'scope':'Historical full-action static control and separately measured product phases; no cross-clock or baseline subtraction, no cause or speedup inference.',
         'inputBindings':bindings,'staticDecodedSHA256':hashlib.sha256(raw).hexdigest(),'staticAll432ScalarRows':static_rows,'staticCohorts':static_groups,
         'all216HistoricalSwitchNodeRows':all_rows,'productCohorts':cohorts,'b94SeparateEightSwitches':quantiles([r['elapsedMs'] for r in b94_switch]),
         'settledEditorControlScope':'32 verifier evaluations on settled real DOM, not full navigation; excluded from any two-click floor claim.',
         'metricPolicy':'Original 300/100 targets unchanged; no summed percentiles, driver/browser subtraction, static-cost subtraction or acceptance replacement.'}
 a.output.write_text(json.dumps(output,indent=2,allow_nan=False)+'\n')
 print(json.dumps({'staticCohorts':static_groups,'primary299':cohorts['299']['idle-interactive-calibration'],'primary5f4':cohorts['5f4']['idle-interactive-calibration']},allow_nan=False))

if __name__=='__main__':main()
