from pathlib import Path
import json,hashlib
r=Path(__file__).resolve().parent
h=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
a=json.loads((r/'browser-before-final/report.json').read_text());b=json.loads((r/'browser-after-final/report.json').read_text());c=json.loads((r/'comparison.json').read_text())
assert a['passed'] and b['passed'] and len(a['checks'])==len(b['checks'])==42
assert not a['changedInputs'] and not b['changedInputs']
assert a['sourceBefore']==a['sourceAfter']==b['sourceBefore']==b['sourceAfter']
assert a['helperSHA256']==b['helperSHA256']==h(r/'browser.mjs')
assert a['fixtureSHA256']==b['fixtureSHA256']==h(r/'fixture.tsx')
assert c['beforeReportSHA256']==h(r/'browser-before-final/report.json') and c['afterReportSHA256']==h(r/'browser-after-final/report.json')
for old in a['checks']:
 if old['label'].startswith('closed-'):
  new=next(v for v in b['checks'] if v['label']==old['label'])
  assert old['editorNodes']==6124 and new['editorNodes']==867
  assert sum(g['controls'] for g in old['groups'])==2307 and sum(g['controls'] for g in new['groups'])==0
for run in [a,b]:
 assert len([v for v in run['checks'] if 'browserProcessObservations' in v])==26
 for row in run['checks']:
  if row['label'].startswith('four-drafts-'):assert len(row['drafts'])==4 and sum(x['bytes'] for x in row['drafts'])==2445044
v=json.loads((r/'after-affected.json').read_text());assert v['numPassedTests']==91 and v['numFailedTests']==0
v=json.loads((r/'before-fixed-harness.json').read_text());assert v['numFailedTests']==4 and v['numPassedTests']==0
assert json.loads((r/'typecheck-exit.json').read_text())['exitCode']==0
print(json.dumps({'passed':True,'pairedDOMObservations':8,'recordedCheckpointsEach':42,'affectedAssertions':91,'originalIntendedFailures':4,'scope':'Read-only artifact/hash/arithmetic verification; no application or runtime launched.'}))
