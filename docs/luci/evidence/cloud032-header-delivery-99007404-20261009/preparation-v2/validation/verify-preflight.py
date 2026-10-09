"""Inert refusal checks: no app, browser, Docker, package hashing or output fixture."""
import importlib.util,json,sys,tempfile
from pathlib import Path

root=Path(__file__).resolve().parents[1]
spec=importlib.util.spec_from_file_location('reviewed_launcher',root/'launch.py')
module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
helper='67a5b7c75ca6354d43085f5bb81ec53ee4291f34'
product='99007404a5ddd8592dee376ea00d99c6180d26fe'
original=Path('/workspace/luci/artifacts/quality-swarm/candidate-99007404/package/package-provenance.json')
calls=[]
def inert_command(args,**kwargs):
 calls.append(args)
 if args==['git','rev-parse','HEAD']:return helper
 if args==['git','status','--porcelain']:return ''
 raise AssertionError('Forbidden non-Git-preflight command: '+str(args))
module.command=inert_command
def refuse(provenance,image,message):
 sys.argv=['launch.py','--source',product,'--helper-source',helper,'--package','/nonexistent-must-not-be-opened','--provenance',str(provenance),'--archive','/nonexistent-must-not-be-opened','--image','controlled-not-started','--expected-image-id',image,'--image-provenance','/nonexistent-must-not-be-opened','--output','/nonexistent-must-not-be-created']
 try:module.main()
 except AssertionError as error:assert str(error)==message,(str(error),message)
 else:raise AssertionError('Pinned input refusal was bypassed')
with tempfile.TemporaryDirectory(prefix='inert-pin-',dir=root/'validation') as temporary:
 altered=Path(temporary)/'provenance.json';value=json.loads(original.read_text());value['binarySHA256']='a'*64;altered.write_text(json.dumps(value))
 refuse(altered,'sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98','Only exact original990 provenance is eligible')
 print('PASS changed internally-declared product990 provenance refused before package/Docker access')
 refuse(original,'sha256:'+'a'*64,'Only original diagnostic image is eligible')
 print('PASS changed original image identity refused before package/Docker access')
assert all(args[0]=='git' for args in calls)
print('Only bounded committed Git source reads executed; no installed fixture or workload')
