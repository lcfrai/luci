#!/usr/bin/env python3
"""Read-only exact selected-helper binding; permit only descendant docs/luci commits."""
from pathlib import Path, PurePosixPath
import argparse, hashlib, json, re, subprocess

def inspect_checkout(repo, helper, capture):
    repo=Path(repo)
    def git(*args): return subprocess.check_output(['git',*args],cwd=repo,timeout=15)
    assert re.fullmatch('[0-9a-f]{40}',helper), 'Exact helper revision required'
    observed=git('rev-parse','HEAD').decode().strip()
    assert re.fullmatch('[0-9a-f]{40}',observed), 'Exact observed checkout revision required'
    assert git('status','--porcelain')==b'', 'Clean observed checkout required'
    assert git('merge-base',helper,observed).decode().strip()==helper, 'Observed checkout must descend from declared helper'
    changed=[name.decode('utf-8') for name in git('diff','--name-only','-z',helper,observed).split(b'\0') if name]
    assert len(changed)<=4096 and all(name.startswith('docs/luci/') for name in changed), 'Only docs/luci differences from selected helper are permitted'
    sources=capture['sources'];assert isinstance(sources,dict) and 0<len(sources)<=64
    checked={}
    for name,expected in sorted(sources.items()):
        p=PurePosixPath(name)
        assert not p.is_absolute() and '..' not in p.parts and str(p)==name, 'Unsafe helper input path'
        path=repo/name;assert path.is_file() and not path.is_symlink(), 'Ordinary selected helper input required'
        for label,body in [('working',path.read_bytes()),('declared',git('show',helper+':'+name)),('observed',git('show',observed+':'+name))]:
            assert len(body)==expected['bytes'] and hashlib.sha256(body).hexdigest()==expected['sha256'], label+' selected helper input differs: '+name
        checked[name]={'bytes':expected['bytes'],'sha256':expected['sha256']}
    assert git('rev-parse','HEAD').decode().strip()==observed and git('status','--porcelain')==b'', 'Checkout changed during binding'
    return {'bindingMode':'exact-selected-helper-inputs-with-docs-only-descendant-checkout','helperSource':helper,'observedCheckoutCommit':observed,'observedCheckoutDirty':False,'docsOnlyChangedPaths':changed,'selectedSources':checked}

def main():
    p=argparse.ArgumentParser();p.add_argument('--repo',required=True);p.add_argument('--helper',required=True);p.add_argument('--capture',required=True);a=p.parse_args()
    print(json.dumps(inspect_checkout(a.repo,a.helper,json.loads(Path(a.capture).read_text())),sort_keys=True))
if __name__=='__main__':main()
