from pathlib import Path,PurePosixPath
import importlib.util,hashlib,json,tarfile,subprocess,sys
repo=Path('/workspace/luci');out=Path(__file__).parent/'verified';out.mkdir();release=repo/'artifacts/releases/0.5.0';source='577f7a0fdb039b7f875901a6a567d62fdfee88c8';package=Path('/workspace/.luci-tools/package-577f7a0f/installed');assert not package.exists();package.mkdir(mode=0o700)
sha=lambda p:hashlib.file_digest(p.open('rb'),'sha256').hexdigest()
man=json.loads((release/'release-manifest.json').read_text());assert man['sourceCommit']==source and man['sourceDirty'] is False
for row in man['artifacts']:assert (release/row['name']).stat().st_size==row['bytes'] and sha(release/row['name'])==row['sha256']
archive=release/'luci-linux-amd64.tar.gz';members=[]
with tarfile.open(archive) as tf:
 for m in tf:
  n=PurePosixPath(m.name);assert not n.is_absolute() and '..' not in n.parts and (m.isfile() or m.isdir())
  if m.isfile():members.append({'path':m.name,'bytes':m.size,'sha256':hashlib.file_digest(tf.extractfile(m),'sha256').hexdigest()})
 tf.extractall(package,filter='data')
for row in members:
 p=package/row['path'];assert p.is_file() and not p.is_symlink() and p.stat().st_size==row['bytes'] and sha(p)==row['sha256']
actual={str(p.relative_to(package)) for p in package.rglob('*') if p.is_file()};assert actual=={r['path'] for r in members}
spec=importlib.util.spec_from_file_location('pack',repo/'scripts/package.py');m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m);identity=m.package_identity(package,'linux');assert (package/'package-identity').read_text().strip()==identity
licenses=json.loads((package/'licenses/manifest.json').read_text());assert not licenses['missingLicenses']
# Bind every unchanged packaged runtime source to this exact Git revision.
runtime=[]
for p in sorted((package/'runtime').rglob('*')):
 if p.is_file():
  rel=str(p.relative_to(package/'runtime'));b=subprocess.check_output(['git','show',source+':'+rel],cwd=repo);assert hashlib.sha256(b).hexdigest()==sha(p);runtime.append({'path':rel,'sha256':sha(p),'bytes':len(b)})
for name in ['release-manifest.json','SHA256SUMS']:(out/name).write_bytes((release/name).read_bytes())
receipt={'sourceCommit':source,'sourceDirty':False,'packageDir':str(package),'archive':str(archive),'archiveSHA256':sha(archive),'binarySHA256':sha(package/'luci'),'webIndexSHA256':sha(package/'web/index.html'),'packageIdentity':identity,'verifiedMembers':len(members),'runtimeSources':len(runtime),'licenseComponents':len(licenses['components']),'missingLicenses':0,'allMembersMatch':True,'scope':'Actual local archives and extracted files read and hashed. Does not establish Windows, runtime image, installed workflow or performance acceptance.'}
(out/'integrity.json').write_text(json.dumps(receipt,indent=2));(out/'archive-members.json').write_text(json.dumps(members,indent=2));(out/'runtime-inputs.json').write_text(json.dumps(runtime,indent=2));print(json.dumps(receipt))
