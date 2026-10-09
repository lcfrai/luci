from pathlib import Path
import hashlib,json,shutil
root=Path(__file__).resolve().parent;out=root/'capsule';out.mkdir(exist_ok=False)
sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
originals=[]
def copy(src,rel):
 dst=out/rel;dst.parent.mkdir(parents=True,exist_ok=True);shutil.copyfile(src,dst);assert src.read_bytes()==dst.read_bytes();originals.append({'member':rel,'originalPath':str(src),'sha256':sha(src),'bytes':src.stat().st_size})
for name in ['verify-publication.py','test_verify_publication.py']:copy(root/'scripts/acceptance'/name,'sources/'+name)
for name in ['README.md','tests-result.json','tests.stdout.log','tests.stderr.log','controlled-demonstration.json','demonstrate.py','lazy-fetch-before.json','lazy-fetch-before.stdout.log','lazy-fetch-before.stderr.log']:copy(root/name,name)
for p in sorted((root/'initial').iterdir()):copy(p,'initial/'+p.name)
for p in sorted(Path('/workspace/.luci-tools/cloud053-root-publication-checks').iterdir()):
 if p.name in ['original-result.json','idle-original.stdout.log','idle-original.stderr.log','counter-original.stdout.log','counter-original.stderr.log']:copy(p,'root-reproduction/'+p.name)
copy(Path(__file__),'publish.py')
(out/'original-bindings.json').write_text(json.dumps({'copies':originals,'transformations':'none'},indent=2)+'\n')
files={p.relative_to(out).as_posix():{'sha256':sha(p),'bytes':p.stat().st_size} for p in sorted(out.rglob('*')) if p.is_file()}
manifest={'scope':'CLOUD-053 publication guard and controlled/immutable original reproductions; no product or runtime acceptance claim. Actual omitted-log repair belongs to root follow-up.','finalTests':11,'finalPass':True,'historicalInitialTests':10,'historicalTestsOverlap':True,'partialCloneBeforeRegressionFailed':True,'sameRegressionSHA256':'4f98ba1fab5429f9ee74c915e451ad075ad12d94b1921364de1a69e69781dcc0','members':files,'manifestConvention':'Members exclude publication.json and SHA256SUMS; sums include publication.json.'}
(out/'publication.json').write_text(json.dumps(manifest,indent=2)+'\n')
(out/'SHA256SUMS').write_text(''.join(sha(p)+'  '+p.relative_to(out).as_posix()+'\n' for p in sorted(out.rglob('*')) if p.is_file() and p.name!='SHA256SUMS'))
result={'capsule':str(out),'members':len(files)+2,'publicationSHA256':sha(out/'publication.json'),'sumsSHA256':sha(out/'SHA256SUMS'),'sourceSHA256':sha(root/'scripts/acceptance/verify-publication.py'),'testSHA256':sha(root/'scripts/acceptance/test_verify_publication.py')};(root/'publication-result.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result,indent=2))
