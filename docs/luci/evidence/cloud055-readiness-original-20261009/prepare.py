from pathlib import Path
import hashlib,json,shutil,difflib,subprocess
HERE=Path(__file__).resolve().parent
BASE=Path('/workspace/.luci-tools/cloud032-idle-67a5b7c7')
REPO=Path('/workspace/luci')
sha=lambda data:hashlib.sha256(data).hexdigest()
for directory in ['original','actual']:
 for path in (BASE/'actual').iterdir():
  if path.is_file():shutil.copyfile(path,HERE/directory/path.name)
source=(HERE/'original/sustained.mjs').read_text();changes=[]
def replace(old,new,label):
 global source
 assert source.count(old)==1,(label,source.count(old))
 source=source.replace(old,new);changes.append({'label':label,'oldSHA256':sha(old.encode()),'newSHA256':sha(new.encode())})
replace("import { installBrowserTiming, timingSnapshot, interactionRunner, summarizeInteractions, createTimingLedger } from './interaction-metrics.mjs';", "import { installBrowserTiming, timingSnapshot, interactionRunner, summarizeInteractions, createTimingLedger } from './interaction-metrics.mjs';\nimport {beginReadiness,drainReadiness,endReadiness} from './autonomous-readiness.mjs';\nimport {createReadinessLedger} from './readiness-ledger.mjs';",'Add only passive diagnostic modules')
replace("const timingLedger=createTimingLedger(receipt);", "const timingLedger=createTimingLedger(receipt);const readinessLedger=createReadinessLedger(receipt);let readinessAttempted=false,readinessStarted=false,readinessClosed=false;",'Independent diagnostic ledger')
replace("const driverNames=['sustained.mjs','external-source-binding.json',", "const driverNames=['sustained.mjs','external-source-binding.json','autonomous-readiness.mjs','readiness-ledger.mjs',",'Capture every executed diagnostic module')
replace("if(value)timingLedger.append(value,reason);}", "if(value)timingLedger.append(value,reason);if(readinessStarted&&!readinessClosed)readinessLedger.append(await timed(drainReadiness(page,reason),3000,'Autonomous DOM witness drain'));}",'Drain after original round span, never inside an action')
replace("await quietPhase('quiet-app-open-idle');const idle=", "await quietPhase('quiet-app-open-idle');readinessAttempted=true;readinessLedger.attempt();readinessLedger.begin(await timed(beginReadiness(page,projectStates),3000,'Autonomous DOM witness setup'));readinessStarted=true;const idle=",'Install after quiet and before primary phase start')
replace("await passiveTiming('end-idle-calibration');", "await passiveTiming('end-idle-calibration');readinessLedger.append(await timed(endReadiness(page),3000,'Autonomous DOM witness close'));readinessClosed=true;",'Close outside primary phase and before separate trace')
replace("  receipt.cleanupPhases=await settleSustainedResources();", "  if(page&&readinessAttempted&&!readinessClosed){try{const closed=await boundedAttempt(()=>endReadiness(page),cleanupBudget(3000),'readiness-close-timeout');if(readinessStarted)readinessLedger.append(closed);else readinessLedger.installationCleanup(closed);readinessClosed=closed?.closed===true;}catch{readinessLedger.unavailable();}}\n  receipt.cleanupPhases=await settleSustainedResources();",'Owned bounded failure cleanup before browser closure')
replace("mode:'installed-four-project-idle-diagnostic',", "mode:'installed-four-project-autonomous-readiness-diagnostic',",'Separate diagnostic identity')
(HERE/'actual/sustained.mjs').write_text(source)
for name in ['autonomous-readiness.mjs','readiness-ledger.mjs']:shutil.copyfile(HERE/'observer'/name,HERE/'actual'/name)
binding=json.loads((BASE/'actual/external-source-binding.json').read_text())
binding.update(executionDirectory=str(HERE/'actual'),scope='External autonomous DOM witness diagnostic derived from original idle helper; no app changes or modified primary actions/thresholds.',originalIdleDriverSHA256=sha((HERE/'original/sustained.mjs').read_bytes()),observerChanges=changes,prospectiveProduct='d20ebd292f3f1de04d6726bab0998c210987bcb5')
for name in list(binding['localSources'])+['autonomous-readiness.mjs','readiness-ledger.mjs']:binding['localSources'][name]=sha((HERE/'actual'/name).read_bytes())
(HERE/'actual/external-source-binding.json').write_text(json.dumps(binding,indent=2)+'\n')
capture=json.loads((BASE/'template-capture.json').read_text());capture['observerPreparedAtHelper']=subprocess.check_output(['git','rev-parse','HEAD'],cwd=REPO,text=True).strip()
for name,expected in capture['sources'].items():assert sha((REPO/name).read_bytes())==expected['sha256'],name+' changed; explicit new source review required'
(HERE/'template-capture.json').write_text(json.dumps(capture,indent=2)+'\n')
launch=(BASE/'launch.py').read_text().replace("args=parser.parse_args();helper=args.helper_source or args.source", "args=parser.parse_args();helper=args.helper_source or args.source\n    assert args.source=='d20ebd292f3f1de04d6726bab0998c210987bcb5','Frozen prospective product is d20; no silent package rebase'")
(HERE/'launch.py').write_text(launch)
(HERE/'integration.patch').write_text(''.join(difflib.unified_diff((HERE/'original/sustained.mjs').read_text().splitlines(keepends=True),source.splitlines(keepends=True),fromfile='original-idle/sustained.mjs',tofile='autonomous-readiness/sustained.mjs')))
for name in ['interaction-metrics.mjs','latency-diagnostics.mjs']:assert (HERE/'actual'/name).read_bytes()==(HERE/'original'/name).read_bytes()==(REPO/'scripts/acceptance/sustained-four-project'/name).read_bytes()
# The original complete installRoutes hook, all target comparisons and full
# measurement helper bytes remain unchanged; no optional interception branch.
def line_with(text,prefix):return next(row for row in text.splitlines() if row.startswith(prefix))
old=(HERE/'original/sustained.mjs').read_text()
for prefix in ['  installRoutes:',' receipt.latencyTargets=',' receipt.targetComparison=',' assert.equal(receipt.interactions.length,400);']:
 assert line_with(old,prefix)==line_with(source,prefix),prefix
(HERE/'preparation.json').write_text(json.dumps({'preparedOnly':True,'installedRuntimeExecuted':False,'originalPrimaryModulesExact':True,'originalRouteGuardExact':True,'originalTargetsExact':True,'changes':changes,'prospectiveProduct':binding['prospectiveProduct'],'helperAtPreparation':capture['observerPreparedAtHelper']},indent=2)+'\n')
print(json.dumps({'prepared':True,'runtimeExecuted':False,'changes':len(changes)}))
