"""Bind an extracted local package to its clean build manifest and every archive member."""
import datetime,hashlib,json,pathlib,sys,tarfile
root=pathlib.Path('/workspace/luci')
commit, evidence = sys.argv[1:]
out=root/'artifacts/releases/0.5.0'; ev=root/evidence; ev.mkdir(parents=True,exist_ok=True)
manifest=json.loads((out/'release-manifest.json').read_text())
assert manifest['sourceCommit']==commit and manifest['sourceDirty'] is False
sha=lambda f:hashlib.file_digest(f.open('rb'),'sha256').hexdigest()
subjects=[]
for item in manifest['artifacts']:
    file=out/item['name']; assert file.stat().st_size==item['bytes'] and sha(file)==item['sha256']
    subjects.append({'name':'linux-amd64/'+item['name'],'sha256':item['sha256'],'bytes':item['bytes']})
subjects.append({'name':'linux-amd64/release-manifest.json','sha256':sha(out/'release-manifest.json'),'bytes':(out/'release-manifest.json').stat().st_size})
archive=out/'luci-linux-amd64.tar.gz'; package=pathlib.Path('/tmp/luci-retained-tools')/('package-0.5.0-'+commit[:8])
assert not package.exists(),package
package.mkdir(mode=0o700)
with tarfile.open(archive) as tf:
    tf.extractall(package,filter='data')
    files=0
    for member in tf.getmembers():
        if member.isfile():
            actual=package/member.name
            assert actual.stat().st_size==member.size and sha(actual)==hashlib.file_digest(tf.extractfile(member),'sha256').hexdigest(),member.name
            files+=1
licenses=json.loads((package/'licenses/manifest.json').read_text()); assert not licenses['missingLicenses']
p={'sourceCommit':commit,'sourceDirty':False,'packageDir':str(package),'archive':str(archive),'archiveSHA256':sha(archive),'binarySHA256':sha(package/'luci'),'webIndexSHA256':sha(package/'web/index.html'),'subjects':subjects}
i={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'sourceCommit':commit,'sourceDirty':False,'verifiedExtractedFiles':files,'licenseComponents':len(licenses['components']),'missingLicenses':0,'subjects':subjects,'allMembersMatched':True}
(ev/'package-provenance.json').write_text(json.dumps(p,indent=2)+'\n'); (ev/'package-integrity.json').write_text(json.dumps(i,indent=2)+'\n')
print(json.dumps(i)); print(str(ev/'package-provenance.json'))
