"""Normal embedded source and owned Envoy preparation; no provider/builds."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
from types import SimpleNamespace
import unittest
from unittest import mock

HERE=Path(__file__).parent
def module(name):
    spec=importlib.util.spec_from_file_location(name,HERE/(name+'.py'))
    value=importlib.util.module_from_spec(spec);spec.loader.exec_module(value);return value
builder,guest,source_tests=module('builder'),module('builder_guest'),module('test_source_bundle')


class SourceJoinTests(unittest.TestCase):
    def test_normal_staged_embedded_bytes_restore_original_sparse_source(self):
        fixture=source_tests.SourceBundleTests();fixture.setUp();self.addCleanup(fixture.doCleanups)
        fixture.collect()
        root=fixture.root/'bundle'
        (root/'azure-setup').mkdir();(root/'azure-setup/source_bundle.py').write_bytes((HERE/'source_bundle.py').read_bytes())
        selection=fixture.root/'selection.json';selection.write_text('{}')
        revision=fixture.revision(fixture.luci)
        runtime=fixture.revision(fixture.e2b)
        canonical=lambda value:json.dumps(value,sort_keys=True,separators=(',',':')).encode()
        core=SimpleNamespace(bundle=lambda _: {'luciRevision':revision}, requested=lambda _: {},
            private_json=lambda path:json.loads(Path(path).read_text()), bounded_json=lambda path,**_:json.loads(Path(path).read_text()),
            canonical=canonical,atomic_json=lambda path,value:Path(path).write_bytes(canonical(value)))
        with mock.patch.object(builder,'RUNTIME',runtime):
            receipt=builder.source_payload(core,root,selection)
        self.assertFalse(receipt['connected'])
        extracted=fixture.root/'staged'
        guest.extract(Path(receipt['payloadPath']),extracted)
        self.assertEqual(set(receipt['inputs']['files']),{'builder-source/source.json','builder-source/luci.pack','builder-source/e2b.pack','source_bundle.py'})
        with mock.patch.object(source_tests.source_bundle,'RUNTIME_REVISION',runtime):
            restored=source_tests.source_bundle.restore(extracted,fixture.root/'joined')
        self.assertEqual(fixture.revision(restored['luci']),revision)
        self.assertFalse((restored['luci']/'outside').exists())
        with mock.patch.object(builder,'RUNTIME',runtime),self.assertRaisesRegex(ValueError,'retained'):
            builder.source_payload(core,root,selection)

    def envoy_fixture(self):
        temporary=tempfile.TemporaryDirectory();self.addCleanup(temporary.cleanup)
        target=Path(temporary.name)/'envoy'
        if os.name == 'nt':
            replacement=mock.patch.object(guest,'save',side_effect=lambda path,value:Path(path).write_text(json.dumps(value)))
            replacement.start();self.addCleanup(replacement.stop)
        calls=[];state={'present':False,'operation':'','foreign':False,'lost':False}
        def run(arguments,**_):
            calls.append(arguments)
            if arguments[:2]==['skopeo','inspect']:return 'sha256:'+'a'*64
            if arguments[:3]==['docker','image','inspect']:return json.dumps([{'Id':'sha256:'+'b'*64,'Architecture':'amd64','Os':'linux'}])
            if arguments[:3]==['docker','container','ls']:return 'owned' if state['present'] else ''
            if arguments[:2]==['docker','create']:
                state['present']=True;state['operation']=arguments[arguments.index('--label')+1].split('=',1)[1]
                if state['lost']:raise TimeoutError('lost create acknowledgement')
                return 'owned'
            if arguments[:3]==['docker','container','inspect']:
                return json.dumps([{'Id':'owned','Image':'foreign' if state['foreign'] else 'sha256:'+'b'*64,
                    'State':{'Running':False},'Config':{'Labels':{'luci.envoy-source':state['operation']}}}])
            if arguments[:3]==['docker','container','rm']:state['present']=False;return 'owned'
            if arguments[-1]=='--version':return 'envoy '+guest.ENVOY_SOURCE+'/'+guest.ENVOY_VERSION
            if arguments[:2]==['docker','pull']:return ''
            raise AssertionError(arguments)
        def fetch(url,sha,path,limit):
            self.assertIn(guest.ENVOY_SOURCE,url);self.assertIn(sha,(guest.ENVOY_LICENSE,guest.ENVOY_NOTICE));Path(path).write_bytes(b'original grant fixture')
        def copy(identifier,path):self.assertEqual(identifier,'owned');Path(path).write_bytes(b'bounded executable fixture')
        return target,calls,state,run,fetch,copy

    def test_official_envoy_source_never_runs_image_entrypoint_and_cleans_owner(self):
        target,calls,state,run,fetch,copy=self.envoy_fixture()
        receipt=guest.envoy_inputs(target,fetch,run,copy)
        self.assertEqual(receipt['sourceCommit'],guest.ENVOY_SOURCE)
        self.assertIn('@sha256:',receipt['image'])
        self.assertFalse(state['present'])
        self.assertFalse(any(row[:2] in (['docker','start'],['docker','run']) for row in calls))
        self.assertEqual(sum(row[:3]==['docker','container','rm'] for row in calls),1)
        with self.assertRaisesRegex(ValueError,'retained'):guest.envoy_inputs(target,fetch,run,copy)

    def test_lost_envoy_create_cleanup_is_owned_and_foreign_refused(self):
        target,calls,state,run,fetch,copy=self.envoy_fixture();state['lost']=True
        with self.assertRaises(TimeoutError):guest.envoy_inputs(target,fetch,run,copy)
        self.assertFalse(state['present']);self.assertFalse((target/'receipt.json').exists())
        target,calls,state,run,fetch,copy=self.envoy_fixture();state['foreign']=True
        with self.assertRaisesRegex(ValueError,'foreign'):guest.envoy_inputs(target,fetch,run,copy)
        self.assertTrue(state['present']);self.assertFalse(any(row[:3]==['docker','container','rm'] for row in calls))


if __name__=='__main__':unittest.main()
