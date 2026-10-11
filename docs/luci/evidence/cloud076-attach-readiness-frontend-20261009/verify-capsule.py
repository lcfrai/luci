"""Read-only verification of exact retained payloads; no product/test execution."""
from pathlib import Path
import gzip,hashlib,json
root=Path(__file__).resolve().parent
manifest=json.loads((root/'publication.json').read_text())
sha=lambda data:hashlib.sha256(data).hexdigest()
for item in manifest['files']:
 stored=(root/item['path']).read_bytes()
 assert len(stored)==item['storedBytes'] and sha(stored)==item['storedSHA256'],item['path']
 original=gzip.decompress(stored) if item['encoding']=='gzip' else stored
 assert len(original)==item['sourceBytes'] and sha(original)==item['sourceSHA256'],item['path']
print(json.dumps({'verifiedPayloadFiles':len(manifest['files']),'finalPatchSHA256':manifest['finalPatchSHA256'],'scope':'Read-only capsule integrity; not a test or acceptance run.'}))
