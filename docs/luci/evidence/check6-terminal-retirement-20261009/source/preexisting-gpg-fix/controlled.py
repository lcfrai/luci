#!/usr/bin/env python3
"""Fake process readers only. Never signals, probes Docker, or reads /proc."""
import copy
import hashlib
import importlib.util
import json
import os
from pathlib import Path
from types import SimpleNamespace

HERE = Path(__file__).resolve().parent
OUT = []


def module(path, name):
    spec = importlib.util.spec_from_file_location(name, path)
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


def config():
    roles = ['wrapper', 'driver', 'controller', 'browser', 'observer']
    known = [{'pid': 2124250 + i, 'startTicks': str(25466319 + i), 'role': role, 'evidenceKey': 'wrapperLifecycle'} for i, role in enumerate(roles)]
    return {'fixtureRoot': '/fixture/luci-check-recovery-fake', 'knownIdentities': known,
            'preexistingUnreadableExclusions': [{'pid': 260700, 'startTicks': '2331192', 'uid': 1000, 'comm': 'gpg-agent', 'parentPid': 1, 'evidenceKey': 'preexistingProcessProof'}],
            'inputs': {'wrapperLifecycle': {'sha256': 'ec1aed4cfd706ccf012c10b02e11e612a4a4858ce709aa50641ab49f9d71b84d'},
                       'preexistingProcessProof': {'sha256': '24648c97f3b4a5977283f7eceb7b8d1d49cdb11287762983d11f129f1cc85204'}}}


def facts():
    return {'pid': 260700, 'startTicks': '2331192', 'uid': 1000, 'comm': 'gpg-agent', 'parentPid': 1, 'state': 'S'}


def proof():
    return {'fixtureWrapperPID': 2124250, 'fixtureWrapperStartTicks': 25466319, 'observations': [facts(), facts()]}


def stub_readers(value, process_ids=(260700,), fact_reader=None):
    class FakePath:
        def __init__(self, *parts):
            self.raw = '/'.join(str(p) for p in parts)
            self.name = self.raw.rsplit('/', 1)[-1]
        def __str__(self):
            return self.raw
        def iterdir(self):
            assert self.raw == '/proc'
            return [FakePath('/proc', pid) for pid in process_ids]
        def joinpath(self, name):
            return FakePath(self.raw, name)
        def read_text(self):
            assert self.name == 'status'
            return 'Uid:\t1000 1000 1000 1000\n'
        def open(self, mode):
            assert self.name == 'environ' and mode == 'rb'
            raise PermissionError('fake unreadable environment')
        def glob(self, pattern):
            return []
        def __truediv__(self, item):
            return FakePath(self.raw, item)
    value.Path = FakePath
    value.os = SimpleNamespace(getuid=lambda: 1000, fsdecode=os.fsdecode, path=SimpleNamespace(lexists=lambda _: False))
    value.identity = lambda pid: {**facts(), 'pid': pid} if pid in process_ids else None
    if hasattr(value, 'minimal_facts'):
        value.minimal_facts = fact_reader or (lambda pid: {**facts(), 'pid': pid})


def expect(name, callback, rejection=False):
    try:
        detail = callback()
        assert not rejection, 'Expected refusal was not raised'
        OUT.append({'name': name, 'passed': True, 'result': detail})
    except RuntimeError as error:
        assert rejection, str(error)
        OUT.append({'name': name, 'passed': True, 'expectedRefusal': str(error)})


old = module(HERE / 'before-preflight.py', 'old_preflight_controlled')
new = module(HERE.parent / 'preflight.py', 'new_preflight_controlled')
stub_readers(old)
old_destination = {}
expect('original exact code refuses unreadable preexisting identity', lambda: old.closure(config(), old_destination), True)
assert old_destination['unreadableProcesses'] == [{'pid': 260700, 'result': 'unknown'}]

stub_readers(new)
def allowed():
    destination = {}
    new.closure(config(), destination, proof())
    assert len(destination['preexistingUnreadableExclusions']) == 1
    row = destination['preexistingUnreadableExclusions'][0]
    assert row['result'] == 'unreadable-preexisting-not-fixture-child'
    assert row['before'] == row['after'] == facts()
    assert destination['unreadableProcesses'][0]['result'] == row['result']
    assert 'absent' not in row['result']
    return {'exclusion': row['result'], 'remainsLiveAndUnreadable': True}
expect('corrected closure records only exact preexisting exclusion', allowed)

for field, changed in [('startTicks', '25466320'), ('startTicks', '25466319'), ('uid', 0), ('comm', 'other-process'), ('parentPid', 999)]:
    before = facts(); before[field] = changed
    expect('reject live identity drift ' + field + '=' + str(changed),
           lambda before=before: new.preexisting_exclusion(config(), 260700, before, facts(), proof()), True)

after = facts(); after['startTicks'] = '99999999'
expect('reject identity change during denied read', lambda: new.preexisting_exclusion(config(), 260700, facts(), after, proof()), True)
owned = config(); owned['knownIdentities'].append({'pid': 260700, 'startTicks': '2331192', 'role': 'controller', 'evidenceKey': 'wrapperLifecycle'})
expect('reject exact identity listed as fixture-owned', lambda: new.preexisting_exclusion(owned, 260700, facts(), facts(), proof()), True)
expect('reject missing proof', lambda: new.preexisting_exclusion(config(), 260700, facts(), facts(), None), True)
incomplete = proof(); incomplete['observations'].pop()
expect('reject single-observation proof', lambda: new.preexisting_exclusion(config(), 260700, facts(), facts(), incomplete), True)
wrong_wrapper = config(); wrong_wrapper['knownIdentities'][0]['startTicks'] = '2331192'
expect('reject equal-age wrapper or changed wrapper binding', lambda: new.preexisting_exclusion(wrong_wrapper, 260700, facts(), facts(), proof()), True)
unbound = config(); unbound['inputs']['preexistingProcessProof']['sha256'] = '0' * 64
expect('reject unbound proof', lambda: new.preexisting_exclusion(unbound, 260700, facts(), facts(), proof()), True)

stub_readers(new, process_ids=(260700, 260701))
expect('reject another unreadable process even after allowed old process', lambda: new.closure(config(), {}, proof()), True)
def unreadable(_):
    raise PermissionError('fake unreadable stat/status')
stub_readers(new, fact_reader=unreadable)
expect('reject unreadable stat/status before environment observation', lambda: new.closure(config(), {}, proof()), True)

receipt = {'mode': 'controlled fake readers only', 'passed': True, 'caseCount': len(OUT), 'cases': OUT,
           'beforeSourceSHA256': hashlib.sha256((HERE / 'before-preflight.py').read_bytes()).hexdigest(),
           'afterSourceSHA256': hashlib.sha256((HERE.parent / 'preflight.py').read_bytes()).hexdigest(),
           'noActualProcDockerFixtureReads': True, 'noSignalsOrMutations': True}
with (HERE / 'controlled-result.json').open('x') as output:
    json.dump(receipt, output, indent=2); output.write('\n')
print(json.dumps({'passed': True, 'cases': len(OUT), 'originalRefusalRetained': True}))
