import pathlib,subprocess,hashlib,json,time,sys
root=pathlib.Path('/workspace/luci');stage=pathlib.Path(__file__).parent;mode=sys.argv[1];phase=sys.argv[2]
overlay='unsafe-overlay.json' if mode.startswith('recovery-before') else ('original-overlay.json' if mode.startswith('before') else 'overlay.json')
replacements=json.loads((stage/overlay).read_text())['Replace']
tracked=subprocess.check_output(['git','ls-files','-z'],cwd=root).decode().split('\0')
sources={}
for rel in tracked:
 p=root/rel
 if not rel or not (rel.startswith('control-plane/') or rel.startswith('sandboxd/')):continue
 if p.suffix not in ['.go','.s','.S','.h','.c'] and p.name not in ['go.mod','go.sum'] and not rel.startswith('control-plane/internal/platform/supabase_assets/'):continue
 src=pathlib.Path(replacements.get(str(p),str(p)))
 sources[rel]={'sha256':hashlib.sha256(src.read_bytes()).hexdigest(),'overlay':str(p) in replacements}
for logical,real in replacements.items():
 rel=str(pathlib.Path(logical).relative_to(root))
 if rel not in sources:sources[rel]={'sha256':hashlib.sha256(pathlib.Path(real).read_bytes()).hexdigest(),'overlay':True}
work=pathlib.Path('/workspace/.luci-tools/go.work')
record={'sourceBaseCommit':'786652704fa8bebd1ba1af41cfa1d414d7def100','sourceCommit':subprocess.check_output(['git','rev-parse','HEAD'],cwd=root,text=True).strip(),'workingTreeStatus':subprocess.check_output(['git','status','--porcelain'],cwd=root,text=True),'scope':'All tracked Go/native sources and module manifests in both active go.work modules, all tracked go:embed supabase_assets, and overlay additions. External dependencies are bound through module manifests; individual external cache files are not included.','goWorkSHA256':hashlib.sha256(work.read_bytes()).hexdigest(),'goToolchainSHA256':hashlib.sha256(pathlib.Path('/workspace/.luci-tools/go1.26.3/bin/go').read_bytes()).hexdigest(),'recordedAt':time.time(),'files':sources}
out=stage/'evidence'/mode/('effective-compiled-repository-sources-'+phase+'.json');out.write_text(json.dumps(record,indent=2)+'\n');print(json.dumps({'mode':mode,'effectiveModuleSources':len(sources),'manifestSHA256':hashlib.sha256(out.read_bytes()).hexdigest(),'sourceClean':not record['workingTreeStatus']}))
