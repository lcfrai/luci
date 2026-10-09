from pathlib import Path
import hashlib,importlib.util,json,sys
sys.dont_write_bytecode=True
root=Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location('guard_tests',root/'scripts/acceptance/test_verify_publication.py')
mod=importlib.util.module_from_spec(spec);spec.loader.exec_module(mod)
fixture=mod.PublicationTests();fixture.setUp();cases=[]
def record(name,args,expected):
 code,result=fixture.verify(*args);assert code==expected,(name,code,result)
 cases.append({'case':name,'arguments':list(args),'exitCode':code,'result':result})
try:
 record('original-ignored-logs-omitted',('--index','--require-complete'),1)
 old=fixture.commit()
 fixture.add_logs();record('exact-originals-force-added',('--index','--require-complete'),0)
 new=fixture.commit()
 record('old-immutable-publication-still-missing',('--ref',old),1)
 record('new-immutable-publication-complete',('--ref',new,'--require-complete'),0)
 fixture.write(mod.CAPSULE+'/stdout.log',b'Changed staged bytes\n');fixture.git('add','--force','--',mod.CAPSULE+'/stdout.log');fixture.write(mod.CAPSULE+'/stdout.log',b'Original output\n')
 record('working-copy-correct-but-staged-content-wrong',('--index',),1)
finally:
 fixture.doCleanups()
report={'scope':'Real disposable Git demonstration; no writes to Luci repository/index. Expected negative findings retained.','cases':cases,'ownedFixtureRemoved':not fixture.root.exists(),'inputs':{p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in [root/'scripts/acceptance/verify-publication.py',root/'scripts/acceptance/test_verify_publication.py',Path(__file__)]}}
(root/'controlled-demonstration.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({'cases':len(cases),'originalRef':old,'correctedRef':new,'fixtureRemoved':report['ownedFixtureRemoved']},indent=2))
