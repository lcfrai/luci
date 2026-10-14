"""Offline immutable guest graph and loopback/refusal tests; no Docker/Azure."""
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import socket
import tempfile
import unittest
from unittest import mock
import urllib.error

spec = importlib.util.spec_from_file_location('registry', Path(__file__).with_name('guest_registry.py'))
registry = importlib.util.module_from_spec(spec); spec.loader.exec_module(registry)


class RegistryTests(unittest.TestCase):
    def fixture(self):
        temp = tempfile.TemporaryDirectory(); self.addCleanup(temp.cleanup)
        root = Path(temp.name)
        license_bytes = b'original fixture license'
        replacement = mock.patch.object(registry, 'LICENSE_SHA', hashlib.sha256(license_bytes).hexdigest())
        replacement.start(); self.addCleanup(replacement.stop)
        def blob(data):
            checksum = hashlib.sha256(data).hexdigest()
            target = root / 'data/docker/registry/v2/blobs/sha256' / checksum[:2] / checksum / 'data'
            target.parent.mkdir(parents=True, exist_ok=True); target.write_bytes(data)
            return {'digest':'sha256:'+checksum, 'size':len(data)}
        config = blob(b'{"architecture":"amd64","os":"linux"}')
        layer = blob(b'exact original guest layer bytes')
        manifest = json.dumps({'schemaVersion':2, 'mediaType':registry.MEDIA.split(', ')[0], 'config':config, 'layers':[layer]}, separators=(',',':')).encode()
        digest = blob(manifest)['digest'][7:]
        (root / 'bin').mkdir(); (root / 'bin/registry').write_bytes(b'fixture executable')
        (root / 'LICENSE').write_bytes(license_bytes)
        (root / 'config.yaml').write_bytes(registry.configuration(root))
        files = {path.relative_to(root/'data').as_posix(): {'sha256':registry.sha_file(path), 'size':path.stat().st_size} for path in (root/'data').rglob('*') if path.is_file()}
        receipt = {'version':1, 'state':'immutable_guest_source_prepared', 'sourceCommit':'a'*40, 'imageDigest':digest,
            'fromImage':'127.0.0.1:5055/luci/guest@sha256:'+digest, 'registrySourceCommit':registry.SOURCE,
            'registryArchiveSHA256':registry.ARCHIVE_SHA, 'registryLicenseSHA256':registry.LICENSE_SHA,
            'registryBinarySHA256':registry.sha_file(root/'bin/registry'), 'files':files,
            'blobs':{config['digest'][7:]:config['size'],layer['digest'][7:]:layer['size']},
            'harnessVersions':{'codex':'0.160.0','claude':'2.1.274','opencode':'1.18.34','sandboxd':'fixture'},
            'readOnlyRegistryObserved':True,'microVMExecutionVerified':False,'taskStateRestored':False}
        (root/'receipt.json').write_text(json.dumps(receipt))
        for path in root.rglob('*'): path.chmod(0o555 if path.is_dir() else 0o444)
        root.chmod(0o555)
        return root,receipt,layer

    def test_full_source_graph_and_missing_layer_refusal(self):
        root,receipt,layer = self.fixture()
        self.assertEqual(registry.verify(root,'a'*40),receipt)
        path = root/'data/docker/registry/v2/blobs/sha256'/layer['digest'][7:9]/layer['digest'][7:]/'data'
        path.chmod(0o644); path.write_bytes(b'changed bytes'); path.chmod(0o444)
        with self.assertRaisesRegex(ValueError,'changed'): registry.verify(root,'a'*40)

    def test_wrong_source_extra_data_and_writable_config_refused(self):
        root,receipt,_ = self.fixture()
        with self.assertRaisesRegex(ValueError,'receipt changed'): registry.verify(root,'b'*40)
        (root/'config.yaml').chmod(0o644)
        with self.assertRaisesRegex(ValueError,'read-only'): registry.verify(root,'a'*40)
        (root/'config.yaml').write_text('http:\n  addr: 0.0.0.0:5055\n'); (root/'config.yaml').chmod(0o444)
        with self.assertRaisesRegex(ValueError,'loopback'): registry.verify(root,'a'*40)

    def test_runtime_configuration_disables_write_proxy_debug_and_uses_loopback(self):
        root,_,_ = self.fixture()
        config = registry.configuration(root).decode()
        self.assertIn('addr: 127.0.0.1:5055',config)
        self.assertIn('readonly:\n      enabled: true',config)
        self.assertIn('delete:\n    enabled: false',config)
        self.assertNotIn('proxy:',config); self.assertNotIn('debug:',config); self.assertNotIn('auth:',config)
        self.assertIn('ProtectSystem=strict',registry.UNIT)
        self.assertIn('IPAddressDeny=any',registry.UNIT)
        self.assertIn('IPAddressAllow=localhost',registry.UNIT)

    def test_local_byte_read_has_no_proxy_redirect_and_verifies_complete_payload(self):
        class Response(io.BytesIO): status=200
        opened=[]
        class Client:
            def open(self,request,timeout): opened.append(request.full_url); return Response(b'actual bytes')
        with mock.patch.object(registry.urllib.request,'build_opener',return_value=Client()):
            registry.local_read('/v2/luci/guest/blobs/sha256:x',hashlib.sha256(b'actual bytes').hexdigest(),12)
            with self.assertRaisesRegex(ValueError,'differ'):
                registry.local_read('/v2/luci/guest/blobs/sha256:x','a'*64,12)
        self.assertTrue(all(value.startswith('http://127.0.0.1:5055/v2/') for value in opened))
        with self.assertRaisesRegex(ValueError,'redirect'):
            registry.NoRedirect().redirect_request(None,None,None,None,None,'https://public.example')

    def test_readonly_probe_requires_all_exact_method_refusals(self):
        methods=[]
        class Client:
            def open(self,request,timeout):
                methods.append(request.method)
                raise urllib.error.HTTPError(request.full_url,405,'read-only',{},None)
        with mock.patch.object(registry.urllib.request,'build_opener',return_value=Client()): registry.readonly_probe()
        self.assertEqual(methods,['POST','PATCH','PUT','DELETE'])
        class Writable:
            def open(self,request,timeout): return io.BytesIO(b'accepted')
        with mock.patch.object(registry.urllib.request,'build_opener',return_value=Writable()):
            with self.assertRaisesRegex(ValueError,'accepted'): registry.readonly_probe()

    def test_port_conflict_refuses_without_adopting_listener(self):
        with socket.socket(socket.AF_INET,socket.SOCK_STREAM) as occupied:
            try: occupied.bind(('127.0.0.1',5055))
            except OSError: self.skipTest('5055 already in use by an unrelated process')
            occupied.listen()
            with self.assertRaises(OSError): registry.assert_port_free()

    def test_source_restore_never_claims_task_state(self):
        root,receipt,_ = self.fixture()
        result=registry.verify(root,'a'*40)
        self.assertFalse(result['taskStateRestored']); self.assertFalse(result['microVMExecutionVerified'])
        changed=dict(receipt,taskStateRestored=True)
        (root/'receipt.json').chmod(0o644); (root/'receipt.json').write_text(json.dumps(changed)); (root/'receipt.json').chmod(0o444)
        with self.assertRaisesRegex(ValueError,'receipt changed'): registry.verify(root,'a'*40)

    def test_lost_probe_ack_cleans_only_positive_owned_container(self):
        with tempfile.TemporaryDirectory() as directory:
            calls=[]; present=False
            def command(arguments,**kwargs):
                nonlocal present
                calls.append(arguments)
                if arguments[:3] == ['docker','container','ls']: return 'owned' if present else ''
                if arguments[:2] == ['docker','create']: present=True; return 'owned'
                if arguments[:3] == ['docker','container','inspect']:
                    return json.dumps([{'Id':'owned','Image':'image-id','Config':{'Labels':{'luci.azure-guest-source':'a'*40,'luci.azure-guest-probe':'operation'}}}])
                if arguments[:2] == ['docker','start']: raise TimeoutError('lost version acknowledgment')
                if arguments[:3] == ['docker','container','rm']: present=False; return 'owned'
                raise AssertionError(arguments)
            with self.assertRaises(TimeoutError): registry.owned_version_probe(command,'image-id','a'*40,'operation','codex',directory)
            self.assertFalse(present)
            self.assertFalse((Path(directory)/'probe-pending.json').exists())
            self.assertEqual(sum(row[:3] == ['docker','container','rm'] for row in calls),1)
            def foreign(arguments,**kwargs):
                if arguments[:3] == ['docker','container','inspect']: return json.dumps([{'Id':'foreign','Image':'other'}])
                return command(arguments,**kwargs)
            calls.clear()
            with self.assertRaisesRegex(ValueError,'foreign'): registry.owned_version_probe(foreign,'image-id','a'*40,'operation','codex',directory)
            self.assertFalse(any(row[:3] == ['docker','container','rm'] for row in calls))
            self.assertTrue((Path(directory)/'probe-pending.json').exists())


if __name__ == '__main__': unittest.main()
