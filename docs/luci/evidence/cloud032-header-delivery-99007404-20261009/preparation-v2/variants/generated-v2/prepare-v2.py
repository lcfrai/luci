from pathlib import Path
import difflib,hashlib,json,shutil,subprocess

HERE=Path(__file__).resolve().parent
REFERENCE=Path('/workspace/.luci-tools/cloud032-idle-integrated-candidate-20261009')
REPO=Path('/workspace/luci')
PRODUCT='99007404a5ddd8592dee376ea00d99c6180d26fe'
sha=lambda b:hashlib.sha256(b).hexdigest()
original=(REFERENCE/'actual/sustained.mjs').read_text()
assert sha(original.encode())=='18429d2a6497cfec582c8565b572b3ca567b756f2f9d403efd79b6b405719dc2'
source=original
changes=[]
def replace(before,after,label):
 global source
 assert source.count(before)==1,(label,source.count(before))
 source=source.replace(before,after);changes.append(label)

replace("import { mixedTaskCommand } from './task-command.mjs';","import {createHeaderDelivery,installLongTaskCollector} from './header-delivery.mjs';\nimport { mixedTaskCommand } from './task-command.mjs';",'external headers-only diagnostic module')
replace("const controllerIdentities=[];let measuredPhase=","const controllerIdentities=[];let headerDelivery;let measuredPhase=",'owned intervention lifecycle')
replace("const driverNames=['sustained.mjs',","const driverNames=['sustained.mjs','header-delivery.mjs','schedule.json',",'retain exact new executed sources')
replace("await installBrowserTiming(context);assertWorkflowActive();await context.route", "await installBrowserTiming(context);await context.addInitScript(installLongTaskCollector);assertWorkflowActive();await context.route",'same additive long-task observer in both arms; original route guard unchanged')
replace("  {name:'controllers',timeoutMs:30000,", "  {name:'header-delivery',timeoutMs:8000,run:async()=>{if(!headerDelivery)return {notInstalled:true};assert(await headerDelivery.close(),'Header interception did not drain');return {closed:true};}},\n  {name:'controllers',timeoutMs:30000,",'bounded intervention cleanup before original controller/browser cleanup')
replace("maxMeasuredActions:432,primaryMeasuredActions:400,optionalTraceMeasuredActions:32", "maxMeasuredActions:320,primaryMeasuredActions:320,optionalTraceMeasuredActions:0",'fixed forty-switch-per-condition count')
replace("Prepared idle subset:60s quiet,100 switches/200 inputs/100 resets;210s workflow,60s preparation and120s cleanup. No extension on failure.","Headers-only diagnostic:60s quiet,40 switches/80 inputs/40 resets per condition;210s workflow,60s preparation and120s cleanup. No extension on failure.",'accurate fixed workload')
replace("mode:'installed-four-project-idle-diagnostic'","mode:'installed-header-delivery-diagnostic-v2'",'explicit diagnostic identity')
replace("CLOUD-032 diagnostic only: frozen installed prefix through idle100 switches/200 inputs, zero Create/Start hard guards. Original package/image/source/provenance arguments remain mandatory. No full sustained pass. Explicit controlled runtime window required.","CLOUD-032 v2 diagnostic only: exact installed990,40 switches/80 inputs/40 resets per condition. Headers availability/backpressure intervention, no body capture, zero Create/Start guards. Explicit retained archive/package/image/source/provenance remain mandatory. No acceptance or frontend-only causal claim. Exclusive root-coordinated window required.",'accurate diagnostic help')
replace('receipt.idleDiagnosticCompleted=true;','receipt.headerDeliveryDiagnosticCompleted=true;','specific completion flag')
replace("scope:'Fresh installed package, exact original four saved contexts and paused queues, original draft/warm setup and idle100 switches/200 inputs. No task execution or sustained completion claim.'","scope:'Exact installed990 package and original four saved contexts/paused queues. Forty switches/eighty inputs/forty resets per condition. Response HEADERS availability/backpressure intervention, not a natural performance, frontend-commit or upstream-completion claim.'",'expanded intervention interpretation')
replace("provenance.archive, provenance.archiveSHA256, dirname(binary)","resolve(option('--archive')), provenance.archiveSHA256, dirname(binary)",'explicit retained archive path; original provenance bytes/hash unchanged')
replace("const provenancePath = option('--provenance');", "assert(option('--archive'),'Exact retained package archive path is required');\nassert.equal(option('--trace-after','no'),'no','No actionability trace in matched headers-only comparison');\nconst provenancePath = option('--provenance');",'reject undeclared archive/trace before runtime')
start=source.index(" setStage('idle-baseline');await quietPhase('quiet-app-open-idle');")
end=source.index(" assert.deepEqual(effects(),{create:0,start:0,destroy:0});",start)
source=source[:start]+''' setStage('header-delivery-quiet');await quietPhase('quiet-app-open-idle');
 const schedule=JSON.parse(readFileSync(join(here,'schedule.json'),'utf8'));
 assert.equal(schedule.rounds.length,80);
 for(const condition of ['baseline','defer'])for(let projectIndex=0;projectIndex<4;projectIndex++)assert.equal(schedule.rounds.filter(row=>row.condition===condition&&row.projectIndex===projectIndex).length,10);
 headerDelivery=await createHeaderDelivery({page,base,projectIDs:projectStates.map(owner=>owner.owner.id),receipt,onFailure:stopOnFailure});
 const mutationBaseline=receipt.browserMutations.length;
 receipt.headerDeliverySchedule=schedule;
 for(const row of schedule.rounds){
  assert.equal(row.projectIndex,row.round%4);health();setStage('header-delivery-'+row.condition+'-'+row.round);
  const phase=phaseStart('diagnostic-header-'+row.condition);headerDelivery.enterRound(row);
  try{await interactions.round(row.round);}
  finally{phaseEnd(phase);await headerDelivery.endRound();await page.evaluate(()=>new Promise(done=>requestAnimationFrame(()=>requestAnimationFrame(done))));await headerDelivery.longTasks(page,'after-round-'+row.round);}
 }
 assert(await headerDelivery.close(),'Header interception must drain before any diagnostic completion claim');
 assert.equal(receipt.browserMutations.length,mutationBaseline,'Measurement must issue no browser mutation');
 receipt.interactionSummary=summarizeInteractions(receipt.interactions);
 receipt.latencyTargets={measurement:'Unchanged original All projects→saved Define value→two frames and input witnesses, under matched diagnostic intervention',projectSwitchP95Ms:300,inputP95Ms:100,p99StrictlyBelowMs:1000};
 receipt.targetComparison={};
 for(const condition of ['baseline','defer']){
  receipt.targetComparison[condition]={};
  for(const [kind,count]of [['project-switch',40],['input',80],['draft-reset',40]]){
   const value=receipt.interactionSummary['diagnostic-header-'+condition+':'+kind];assert.equal(value.count,count);assert.equal(value.failed,0);
   receipt.targetComparison[condition][kind]={...value,...(kind==='draft-reset'?{}:{withinReference:value.p95Ms<=(kind==='input'?100:300)&&value.p99Ms<1000})};
  }
 }
 assert.equal(receipt.interactions.length,320);assert.equal(receipt.headerDelivery.valid,true);
 assert(receipt.headerDelivery.responses.some(row=>row.held===true),'No actual response hold means no intervention evidence');
''' +source[end:]
changes.append('replace old idle/trace tail with frozen balanced schedule; preserve each original round/span and thresholds')
replace("check('Fresh installed idle diagnostic completed; no full sustained or latency-target success implied');","check('Headers-only conditional diagnostic completed; no natural performance, full sustained, frontend-only cause or backend completion claim');",'completion is conditional diagnostic only')

def guard(s):
 start=s.index("await context.route('**/*',async route=>");stop=s.index('await route.continue();});',start)+len('await route.continue();});');return s[start:stop]
assert guard(source)==guard(original)
OUT=HERE/'actual';assert not OUT.exists();shutil.copytree(REFERENCE/'actual',OUT)
shutil.copytree(REFERENCE/'original',HERE/'original')
shutil.copy2(REFERENCE/'template-capture.json',HERE/'template-capture.json')
(OUT/'sustained.mjs').write_text(source)
shutil.copy2(HERE/'header-delivery.mjs',OUT/'header-delivery.mjs')
shutil.copy2(Path('/workspace/.luci-tools/cloud032-summary-delivery-diagnostic-20261009/schedule-40-per-condition.json'),OUT/'schedule.json')
binding=json.loads((OUT/'external-source-binding.json').read_text());binding['executionDirectory']=str(OUT);binding['scope']='Generated v2 headers-only external diagnostic. Frozen original template/product990; exact clean current dependency checkout separately declared. No body capture/replay/guard replacement.'
binding['templateProductCommit']=PRODUCT;binding['parentIdleDriverSHA256']=sha(original.encode());binding['originalRouteGuardSHA256']=sha(guard(original).encode());binding['v2Changes']=changes
for name in [*binding['localSources'],'header-delivery.mjs','schedule.json']:binding['localSources'][name]=sha((OUT/name).read_bytes())
(OUT/'external-source-binding.json').write_text(json.dumps(binding,indent=2)+'\n')

launcher=(REFERENCE/'launch.py').read_text()
launcher=launcher.replace("    parser.add_argument('--provenance',type=Path,required=True)","    parser.add_argument('--provenance',type=Path,required=True)\n    parser.add_argument('--archive',type=Path,required=True)")
before="""        path=REPO/name;assert path.is_file() and sha(path)==expected['sha256'] and path.stat().st_size==expected['bytes'],name+' changed after preparation'
        committed=subprocess.check_output(['git','show',helper+':'+name],cwd=REPO,timeout=30)
        assert hashlib.sha256(committed).hexdigest()==expected['sha256'],name+' not bound to exact helper revision'"""
after="""        committed=subprocess.check_output(['git','show','99007404a5ddd8592dee376ea00d99c6180d26fe:'+name],cwd=REPO,timeout=30)
        frozenName=name.rsplit('/',1)[1] if '/sustained-four-project/' in name else name.split('/')[-2]+'_'+name.split('/')[-1]
        path=HERE/'original'/frozenName
        assert path.is_file() and sha(path)==expected['sha256'] and path.stat().st_size==expected['bytes'],name+' frozen template changed'
        assert hashlib.sha256(committed).hexdigest()==expected['sha256'],name+' not bound to frozen template990'"""
assert launcher.count(before)==1;launcher=launcher.replace(before,after)
before="    for name,expected in binding['repositoryDependencies'].items():assert sha(REPO/name)==expected,name"
after="""    for name,expected in binding['repositoryDependencies'].items():
        assert sha(REPO/name)==expected,name
        committed=subprocess.check_output(['git','show',helper+':'+name],cwd=REPO,timeout=30)
        assert hashlib.sha256(committed).hexdigest()==expected,name+' differs in current clean helper commit'
    assert args.source=='99007404a5ddd8592dee376ea00d99c6180d26fe','This reviewed experiment requires exact original product990'
    assert args.trace_after=='no','No trace arm in this experiment'
    assert args.archive.is_file(),'Explicit retained990 archive required'"""
assert launcher.count(before)==1;launcher=launcher.replace(before,after)
launcher=launcher.replace("'--provenance',str(args.provenance.resolve()),","'--provenance',str(args.provenance.resolve()),'--archive',str(args.archive.resolve()),")
launcher=launcher.replace("choices=['yes','no'],default='yes'","choices=['yes','no'],default='no'")
launcher=launcher.replace("'primarySwitches':100,'primaryInputs':200,'primaryResets':100,'extraTraceRounds':8 if args.trace_after=='yes' else 0","'perConditionSwitches':40,'perConditionInputs':80,'perConditionResets':40,'conditions':2,'totalMeasuredRows':320,'extraTraceRounds':0")
launcher=launcher.replace("'scope':'Fresh installed idle diagnostic only; no runtime allocation, no full sustained pass, no model quality or Windows acceptance.'","'scope':'Headers-only response availability/backpressure diagnostic on exact installed990. No frontend-only causality, upstream completion, natural performance, runtime allocation, sustained, model quality or Windows acceptance.'")
(HERE/'launch.py').write_text(launcher)
(HERE/'candidate.patch').write_text(''.join(difflib.unified_diff(original.splitlines(True),source.splitlines(True),fromfile='frozen-idle-reference/actual/sustained.mjs',tofile='actual/sustained.mjs')))
(HERE/'generation.json').write_text(json.dumps({'templateProductCommit':PRODUCT,'parentIdleDriverSHA256':sha(original.encode()),'generatedDriverSHA256':sha(source.encode()),'originalGuardUnchanged':True,'routeGuardSHA256':sha(guard(source).encode()),'measurementFilesUnchanged':{name:sha((OUT/name).read_bytes()) for name in ['interaction-metrics.mjs','latency-diagnostics.mjs']},'changes':changes,'installedRun':False},indent=2)+'\n')
print(json.dumps({'generatedDriverSHA256':sha(source.encode()),'routeGuardSHA256':sha(guard(source).encode()),'externalSourceBindingSHA256':sha((OUT/'external-source-binding.json').read_bytes())}))
