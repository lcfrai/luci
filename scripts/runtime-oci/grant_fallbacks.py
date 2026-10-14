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
    result = [{'name': '@openai/codex', 'versions': ['0.160.0', '0.160.0-linux-x64'], 'grant': 'Apache-2.0', 'files': files}]
    # Reviewed exact upstream declarations plus their original grant bodies.
    # These hashes bind attribution/provenance too, not just an SPDX string.
    pinned = {
        'npmcli-agent-4.0.2': '8847978132cb812e5544ed4c7762b315351151a20fe570677340a3ba1cd8956b',
        'sigstore-verify-3.1.1': '2c85f40dd02fbb65617248eac339f7a7e1b3c4b30dbf007854936d37b5d43465',
        'spdx-exceptions-2.5.0': 'ea26d7ae17dfa8e0e6a91a1e0039535bc5a4001f2c51daa43a0fc3806163434d',
        'spdx-license-ids-3.0.23': 'ba8d945937b0beafe01186c17846c3c0fc7d27603479ee57e8ec04aaab0b2614',
    }
    for directory, digest in pinned.items():
        root = Path(repo) / 'third-party/licenses' / directory
        if archive.file_record(root / 'provenance.json')['sha256'] != digest:
            raise ValueError('exact npm grant provenance changed; review the new declaration')
        value = read_json(root / 'provenance.json')
        files = []
        for row in value['files']:
            actual = archive.file_record(root / row['file'])
            if actual['sha256'] != row['sha256'] or actual['bytes'] != row['bytes'] or not 0 < actual['bytes'] <= 1 << 20:
                raise ValueError('retained authoritative npm grant bytes differ')
            files.append(dict(row, data=base64.b64encode((root / row['file']).read_bytes()).decode('ascii')))
        metadata = archive.file_record(root / 'provenance.json')
        files.append({'file': 'provenance.json', 'url': value['registryMetadataURL'], **metadata,
                      'data': base64.b64encode((root / 'provenance.json').read_bytes()).decode('ascii')})
        result.append({'name': value['component'], 'versions': [value['packageVersion']], 'grant': value['grant'], 'files': files})
    note = Path(repo) / 'third-party/licenses/claude-code-2.1.274/DISTRIBUTION.md'
    actual = archive.file_record(note)
    result.append({'name': '@anthropic-ai/claude-code', 'versions': ['2.1.274'], 'grant': 'SEE LICENSE IN README.md', 'supplemental': True,
                   'files': [{'file': 'DISTRIBUTION.md', 'url': 'https://code.claude.com/docs/en/legal-and-compliance', **actual,
                              'data': base64.b64encode(note.read_bytes()).decode('ascii')}]})
    return result
