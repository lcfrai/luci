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

# The runtime module is source-controlled tooling, never a package-supplied module.
sys.path.insert(0, str(pathlib.Path(__file__).parent / 'runtime-oci'))
import payload as runtime_payload

CORE_LIMIT = 512 << 20
PAYLOAD_LIMIT = 8 << 30


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
    runtime = root / 'runtime'
    bound_runtime = []
    if (runtime / runtime_payload.MANIFEST).exists():
        manifest = runtime_payload.verify(runtime, package=root, canonical=runtime)
        runtime_payload.source_receipt(runtime_payload.read_json(runtime / runtime_payload.SOURCE), manifest['sourceCommit'])
        bound_runtime = [runtime / name for name in (runtime_payload.MANIFEST, runtime_payload.RECEIPT, runtime_payload.SOURCE, runtime_payload.PAYLOAD)]
        if 'reuseEvidenceSHA256' in manifest['producer']:
            bound_runtime.extend(runtime / name for name in (runtime_payload.REUSE, runtime_payload.PRIOR_MANIFEST, runtime_payload.PRIOR_RECEIPT))
    elif (runtime / runtime_payload.COMPANION).exists():
        runtime_payload.verify_companion(runtime)
        bound_runtime = [runtime / runtime_payload.COMPANION, runtime / runtime_payload.SOURCE]
    files.extend(bound_runtime)
    digest = hashlib.sha256(b'luci-portable-package-v2\n' if bound_runtime else b'luci-portable-package-v1\n')
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
    root = runtime_payload.plain_directory(pathlib.Path(root).absolute())
    core_bytes, payload_bytes = 0, 0
    for file in root.rglob('*'):
        mode = file.lstat().st_mode
        if file.is_symlink() or getattr(file.lstat(), 'st_file_attributes', 0) & 1024 or not (stat.S_ISDIR(mode) or stat.S_ISREG(mode)):
            raise ValueError('Package refuses linked or special members')
        if stat.S_ISREG(mode):
            if file.relative_to(root).as_posix() == 'runtime/' + runtime_payload.PAYLOAD:
                payload_bytes += file.stat().st_size
            else:
                core_bytes += file.stat().st_size
    if payload_bytes > PAYLOAD_LIMIT or core_bytes > CORE_LIMIT:
        raise ValueError('Package exceeds role-specific core/runtime payload bounds')
    identity = root / 'package-identity'
    if identity.is_symlink() or (identity.exists() and not identity.is_file()):
        raise ValueError('Package identity must be an ordinary file')
    identity.write_bytes((package_identity(root, platform) + '\n').encode('ascii'))
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
