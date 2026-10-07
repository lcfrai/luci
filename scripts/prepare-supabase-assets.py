#!/usr/bin/env python3
"""Refresh reviewed, pinned upstream Supabase inputs; never start a service.

Downloads configuration/licence bytes and resolves the exact image tags to OCI
digests. Runtime installation uses only the retained assets and immutable pins.
"""
import concurrent.futures
import hashlib
import json
from pathlib import Path
import re
import urllib.request
import io
import base64
import tarfile

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / 'control-plane/internal/platform/supabase_assets'
REV = '564eab8ad7840b13324f68b1bfac074ef8d51c21'
BASE = f'https://raw.githubusercontent.com/supabase/supabase/{REV}/'

def fetch(url):
    with urllib.request.urlopen(urllib.request.Request(url, headers={'User-Agent': 'Luci-source-preparation'}), timeout=60) as response:
        return response.read()

def main():
    paths = ['LICENSE', 'docker/docker-compose.yml', 'docker/.env.example',
        'docker/volumes/api/envoy/cds.yaml', 'docker/volumes/api/envoy/docker-entrypoint.sh',
        'docker/volumes/api/envoy/envoy.yaml', 'docker/volumes/api/envoy/lds.template.yaml',
        'docker/volumes/db/_supabase.sql', 'docker/volumes/db/jwt.sql',
        'docker/volumes/db/logs.sql', 'docker/volumes/db/pooler.sql',
        'docker/volumes/db/realtime.sql', 'docker/volumes/db/roles.sql',
        'docker/volumes/db/webhooks.sql', 'docker/volumes/db/init/data.sql',
        'docker/volumes/functions/deno.jsonc', 'docker/volumes/functions/hello/index.ts',
        'docker/volumes/functions/main/index.ts', 'docker/volumes/pooler/pooler.exs']
    records = []
    with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
        for path, data in zip(paths, pool.map(lambda p: fetch(BASE+p), paths)):
            name = path.removeprefix('docker/')
            if name == '.env.example': name = 'configuration-defaults.txt'
            target = OUT / name
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(data)
            records.append({'path': name, 'source': BASE+path, 'sha256': hashlib.sha256(data).hexdigest(), 'bytes':len(data)})
    compose = (OUT/'docker-compose.yml').read_text()
    images = {}
    for image in re.findall(r'^\s+image:\s+([^\s#]+)', compose, flags=re.M):
        repo, tag = image.rsplit(':', 1)
        info = json.loads(fetch(f'https://hub.docker.com/v2/repositories/{repo}/tags/{tag}'))
        digest = info.get('digest', '')
        if not re.fullmatch(r'sha256:[a-f0-9]{64}', digest): raise RuntimeError('Missing digest for '+image)
        images[image] = image+'@'+digest
    # The upstream main worker uses a floating jose@6 import. Vendor the exact
    # dependency (which itself has no runtime dependencies), retaining its grant.
    package = json.loads(fetch('https://registry.npmjs.org/jose/6.2.2'))
    if package.get('dependencies'): raise RuntimeError('Review newly introduced jose dependencies')
    archive = fetch(package['dist']['tarball'])
    integrity = 'sha512-'+base64.b64encode(hashlib.sha512(archive).digest()).decode()
    if integrity != package['dist']['integrity']: raise RuntimeError('jose integrity mismatch')
    with tarfile.open(fileobj=io.BytesIO(archive), mode='r:gz') as tar:
        for item in tar.getmembers():
            name = item.name.removeprefix('package/')
            if not item.isfile() or not (name.startswith('dist/') or name in ['LICENSE.md','package.json']): continue
            if '..' in Path(name).parts or name.startswith('/'): raise RuntimeError('Unsafe jose archive')
            data = tar.extractfile(item).read()
            retained = 'volumes/functions/_vendor/jose/'+name
            target = OUT/retained
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(data)
            records.append({'path':retained,'source':package['dist']['tarball'],'sha256':hashlib.sha256(data).hexdigest(),'bytes':len(data)})
    (OUT/'manifest.json').write_text(json.dumps({'upstream':'https://github.com/supabase/supabase', 'revision':REV, 'version':'self-hosted/v0.8.2', 'licence':'Apache-2.0', 'files':records, 'images':images, 'vendoredDependencies':[{'name':'jose','version':'6.2.2','license':'MIT','dependencies':{},'source':package['dist']['tarball'],'integrity':integrity}]}, indent=2)+'\n')
    print(f'Retained {len(records)} upstream files and {len(images)} immutable images.')

if __name__ == '__main__': main()
