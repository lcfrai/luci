#!/usr/bin/env python3
"""Offline pinned npm archive verification and fresh reconstruction; no scripts/network/browser."""
from pathlib import Path,PurePosixPath
import argparse,base64,hashlib,io,json,os,tarfile
HERE=Path(__file__).resolve().parent

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--archives',type=Path,required=True);p.add_argument('--output',type=Path,required=True);a=p.parse_args()
 assert not a.output.exists(),'Fresh output required; existing evidence must remain'
 pins=json.loads((HERE/'dependency-pins.json').read_text())['packages'];manifest=json.loads((HERE/'preparation/dependencies/manifest.json').read_text());expected=manifest['files'];found={};archive_hashes={};total=0
 for name,pin in pins.items():
  path=a.archives/(name+'-'+pin['version']+'.tgz');assert path.is_file() and not path.is_symlink() and path.stat().st_size<=32*1024**2
  data=path.read_bytes();algorithm,digest=pin['integrity'].split('-',1);assert algorithm=='sha512' and base64.b64encode(hashlib.sha512(data).digest()).decode()==digest,'Pinned npm integrity mismatch: '+name
  archive_hashes[name]=hashlib.sha256(data).hexdigest()
  with tarfile.open(fileobj=io.BytesIO(data),mode='r:gz') as archive:
   for entry in archive:
    path=PurePosixPath(entry.name);assert not path.is_absolute() and '..' not in path.parts and path.parts[0]=='package','Unsafe npm path'
    if entry.isdir():continue
    assert entry.isfile() and len(path.parts)>1,'Nonordinary npm member'
    key=name+'/'+str(PurePosixPath(*path.parts[1:]));assert key not in found and key in expected,'Unexpected or duplicate npm member: '+key
    row=expected[key];assert entry.size==row['bytes'];total+=entry.size;assert total<=128*1024**2 and len(found)<10000
    stream=archive.extractfile(entry);body=stream.read(row['bytes']+1);assert len(body)==row['bytes'] and hashlib.sha256(body).hexdigest()==row['sha256'],'Captured installed bytes differ from pinned npm archive: '+key
    found[key]=body
 assert set(found)==set(expected) and len(found)==176,'Captured inventory completeness mismatch'
 # Write only after every archive integrity, path, byte and completeness check.
 a.output.mkdir(mode=0o700,parents=False)
 for key,body in found.items():
  path=a.output/key;path.parent.mkdir(parents=True,exist_ok=True,mode=0o700);path.write_bytes(body);path.chmod(expected[key]['mode'])
 receipt={'scope':'Pinned npm archive identity and all176 captured file bytes/modes; local capture timestamps/inodes are not recreated. No package scripts, browser binaries or unrelated packages.','npmArchivesSHA256':archive_hashes,'files':len(found),'bytes':total,'inventorySHA256':manifest['inventorySHA256'],'reconstructed':True}
 (a.output/'reproduction-receipt.json').write_text(json.dumps(receipt,indent=2)+'\n');print(json.dumps(receipt))
if __name__=='__main__':main()
