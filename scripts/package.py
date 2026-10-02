"""Create portable archives with safe, explicit executable modes."""
import pathlib, sys, tarfile, zipfile
root = pathlib.Path(sys.argv[1]).resolve()
if sys.argv[2] == 'windows':
    with zipfile.ZipFile(str(root) + '.zip', 'w', zipfile.ZIP_DEFLATED) as archive:
        for file in sorted(root.rglob('*')):
            if file.is_file():
                archive.write(file, file.relative_to(root).as_posix())
else:
    def modes(info):
        info.uid = info.gid = 0
        info.uname = info.gname = ''
        info.mode = 0o755 if info.isdir() or info.name in ('luci', 'start-luci.sh') or info.name.endswith('.sh') else 0o644
        return info
    with tarfile.open(str(root) + '.tar.gz', 'w:gz') as archive:
        for file in sorted(root.iterdir()):
            archive.add(file, arcname=file.name, filter=modes)
