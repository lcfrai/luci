"""Create portable archives with safe modes and an exact runtime identity.

The identity distinguishes running packages, including different builds of the
same version. It is consistency metadata, not a signature or publisher proof.
"""
import hashlib
import pathlib
import stat
import sys
import tarfile
import zipfile


def package_identity(root, platform):
    root = pathlib.Path(root)
    binary = root / ('luci.exe' if platform == 'windows' else 'luci')
    web = root / 'web'
    if not web.is_dir() or not (web / 'index.html').is_file():
        raise ValueError('Package requires complete web assets')
    files = [binary]
    for entry in [web, *sorted(web.rglob('*'))]:
        mode = entry.lstat().st_mode
        if entry.is_symlink() or not (stat.S_ISDIR(mode) or stat.S_ISREG(mode)):
            raise ValueError('Package identity refuses linked or special web assets')
        if stat.S_ISREG(mode):
            files.append(entry)
    digest = hashlib.sha256(b'luci-portable-package-v1\n')
    for file in files:
        if file.is_symlink() or not stat.S_ISREG(file.lstat().st_mode):
            raise ValueError('Package identity requires an ordinary service binary')
        content = hashlib.sha256()
        with file.open('rb') as source:
            for block in iter(lambda: source.read(1024 * 1024), b''):
                content.update(block)
        digest.update(file.relative_to(root).as_posix().encode('utf-8') + b'\0' + content.digest())
    return digest.hexdigest()


def build_package(root, platform):
    root = pathlib.Path(root).resolve()
    identity = root / 'package-identity'
    if identity.is_symlink() or (identity.exists() and not identity.is_file()):
        raise ValueError('Package identity must be an ordinary file')
    identity.write_text(package_identity(root, platform) + '\n', encoding='ascii')
    if platform == 'windows':
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


if __name__ == '__main__':
    build_package(sys.argv[1], sys.argv[2])
