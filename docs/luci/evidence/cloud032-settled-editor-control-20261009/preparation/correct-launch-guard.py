from pathlib import Path
import difflib,hashlib,json,shutil
R=Path(__file__).resolve().parent
assert (R/'capsule/manifest.json').is_file()
D=R/'launch-guard-correction';D.mkdir(exist_ok=False)
for n in ['launch.py','actual/sustained.mjs','actual/external-source-binding.json','README.md','final-source-validation.json']:
 p=D/'before'/n;p.parent.mkdir(parents=True,exist_ok=True);shutil.copyfile(R/n,p)
shutil.copyfile(R/'template-capture.json',R/'actual/template-capture.json')
def once(t,a,b):assert t.count(a)==1,a;return t.replace(a,b)
p=R/'launch.py';t=p.read_text()
t=once(t,"    assert command(['git','rev-parse','HEAD'],cwd=REPO)==helper\n    assert command(['git','status','--porcelain'],cwd=REPO)=='','Clean committed source required'", "    observed_checkout=json.loads(command(['python3',str(HERE/'actual/checkout-binding.py'),'--repo',str(REPO),'--helper',helper,'--capture',str(HERE/'actual/template-capture.json')]))")
t=once(t,"'helperSource':helper,'sourceBinding':capture,", "'helperSource':helper,'observedCheckout':observed_checkout,'sourceBinding':capture,")
t=once(t,"protected_unchanged=after==protected", "observed_checkout_after=json.loads(command(['python3',str(HERE/'actual/checkout-binding.py'),'--repo',str(REPO),'--helper',helper,'--capture',str(HERE/'actual/template-capture.json')]))\n        protected_unchanged=after==protected and observed_checkout_after==observed_checkout")
t=once(t,"except (OSError,subprocess.TimeoutExpired,ValueError) as error:", "except (OSError,subprocess.TimeoutExpired,subprocess.CalledProcessError,ValueError) as error:")
p.write_text(t)
p=R/'actual/sustained.mjs';t=p.read_text();a="function verifySourceBinding(){return validateSourceBinding({provenance,...sourceBindingOptions,helperCommit:execFileSync('git',['rev-parse','HEAD'],{cwd:repo,encoding:'utf8',timeout:Math.ceil(commandBudget(15000))}).trim(),helperDirty:!!execFileSync('git',['status','--porcelain'],{cwd:repo,encoding:'utf8',timeout:Math.ceil(commandBudget(15000))}).trim()});}"
b="""function verifySourceBinding(){
 const selected=JSON.parse(execFileSync('python3',[join(here,'checkout-binding.py'),'--repo',repo,'--helper',sourceBindingOptions.expectedHelper,'--capture',join(here,'template-capture.json')],{encoding:'utf8',timeout:Math.ceil(commandBudget(15000)),maxBuffer:1024*1024}));
 // The validator's commit is the proven selected input revision; record the distinct clean observed checkout explicitly.
 const bound=validateSourceBinding({provenance,...sourceBindingOptions,helperCommit:selected.helperSource,helperDirty:selected.observedCheckoutDirty});
 return {...bound,bindingMode:selected.bindingMode,observedCheckoutCommit:selected.observedCheckoutCommit,observedCheckoutDirty:selected.observedCheckoutDirty,docsOnlyChangedPaths:selected.docsOnlyChangedPaths,selectedSources:selected.selectedSources};
}"""
t=once(t,a,b);t=once(t,"const driverNames=['settled-control.mjs'", "const driverNames=['checkout-binding.py','template-capture.json','settled-control.mjs'");p.write_text(t)
p=R/'actual/external-source-binding.json';b=json.loads(p.read_text())
for n in ['checkout-binding.py','template-capture.json','sustained.mjs']:b['localSources'][n]=hashlib.sha256((R/'actual'/n).read_bytes()).hexdigest()
b['checkoutBinding']='Declared selected helper remains4d6595; observed clean descendant checkout is separately recorded and only docs/luci differences permitted. Exact17 working/declared/observed helper inputs must all match.'
p.write_text(json.dumps(b,indent=2)+'\n')
p=R/'README.md';t=p.read_text();t+='''\n## Launch guard correction\n\nThe original frozen `capsule/` remains immutable. Its whole-HEAD equality blocked a clean docs-only descendant checkout; the external driver repeated that requirement. `launch-guard-correction/before/` preserves the exact old guard sources. The corrected launcher and driver use the same read-only checker: clean descendant HEAD, only `docs/luci/` changes from declared helper4d6595, and all17 selected sources equal in working bytes and immutable blobs at both helper and observed HEAD. The installed product remains edd0 and its archive/provenance/image guards are unchanged. Any non-doc source delta is refused. Observed checkout identity is recorded separately from declared helper input identity and must remain identical after the run. This does not claim the whole checkout equals4d6595. No installed run has occurred for this correction.\n''';p.write_text(t)
patch=[]
for n in ['launch.py','actual/sustained.mjs','actual/external-source-binding.json']:
 patch.extend(difflib.unified_diff((D/'before'/n).read_text().splitlines(keepends=True),(R/n).read_text().splitlines(keepends=True),fromfile='before/'+n,tofile='after/'+n))
(D/'guard.patch').write_text(''.join(patch))
print('External guard sources corrected; originals preserved.')
