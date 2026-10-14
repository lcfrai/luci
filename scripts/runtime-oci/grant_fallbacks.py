"""Exact source-controlled upstream grants for packages omitting notice files."""
import base64
from pathlib import Path

import archive
from common import read_json


def load(repo):
    root = Path(repo) / 'third-party/licenses/openai-codex-0.160.0'
    value = read_json(root / 'provenance.json')
    if value.get('version') != 1 or value.get('component') != '@openai/codex' or value.get('packageVersion') != '0.160.0' or value.get('grant') != 'Apache-2.0' or value.get('sourceCommit') != 'a956835d020762cb2b570053af06f643a11c0ecc' or value.get('sourceTag') != 'rust-v0.160.0' or value.get('sourceRepository') != 'https://github.com/openai/codex':
        raise ValueError('Codex upstream grant provenance changed; review the exact new version')
    files = []
    if {row['file'] for row in value['files']} != {'LICENSE', 'NOTICE'} or len(value['files']) != 2:
        raise ValueError('Codex fallback requires both original LICENSE and NOTICE')
    for row in value['files']:
        wanted = 'https://raw.githubusercontent.com/openai/codex/' + value['sourceCommit'] + '/' + row['file']
        actual = archive.file_record(root / row['file'])
        if row['url'] != wanted or actual['sha256'] != row['sha256'] or actual['bytes'] != row['bytes'] or not 0 < actual['bytes'] <= 1 << 20:
            raise ValueError('retained authoritative Codex grant bytes differ')
        files.append(dict(row, data=base64.b64encode((root / row['file']).read_bytes()).decode('ascii')))
    return [{'name': '@openai/codex', 'versions': ['0.160.0', '0.160.0-linux-x64'], 'grant': 'Apache-2.0', 'files': files}]
