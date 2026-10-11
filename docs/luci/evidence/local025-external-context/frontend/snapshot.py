import hashlib,json,pathlib,sys
root=pathlib.Path('/workspace/luci')
paths=sorted(p for p in (root/'desktop/src').rglob('*') if p.is_file())
paths += [root/'desktop'/name for name in ['package.json','package-lock.json','tsconfig.json','tsconfig.node.json','vite.config.ts','vitest.config.ts'] if (root/'desktop'/name).exists()]
rows={str(p.relative_to(root)): {'sha256': hashlib.sha256(p.read_bytes()).hexdigest(), 'bytes': p.stat().st_size} for p in paths}
pathlib.Path(sys.argv[1]).write_text(json.dumps(rows,sort_keys=True,indent=2)+'\n')
print(len(rows))
