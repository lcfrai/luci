"""Bind maintainer-selected document proof inputs into an ordinary release.

This never discovers trust by hashing a descriptor and accepting its own claim.
The expected profile and tool-domain hashes are independent build inputs. The
prebuilt extraction image is a separate companion contract, not this profile.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import stat

PROFILE_ROOT = Path('runtime/windows/debian-signature')
PIN_FILE = 'document-runtime-pins'
IMAGE_ROOT = Path('runtime/document-image')
IMAGE_PIN_FILE = 'document-image-manifest-pin'
IMAGE_MANIFEST = 'document-runtime-manifest-v1.json'
IMAGE_ARCHIVE = 'document-image-linux-amd64.tar'
IMAGE_NOTICES = 'document-runtime-notices-v1.json'
IMAGE_LIMIT = 8 << 30
TOOL_PATH = 'manifests/windows-input-proof-tool-closure-v1.json'
TOOL_ROLE = 'support:' + TOOL_PATH
SIGNATURE_ROLE = 'signature-profile'
SIGNATURE_PATH = 'profiles/signature-profile.json'
PROFILE_LIMIT = 64 << 10
TOTAL_LIMIT = 256 << 20
FILE_LIMIT = 128 << 20
HEX = re.compile(r'[0-9a-f]{64}\Z')


def plain_directory(path):
    path = Path(path).absolute()
    for item in (path, *path.parents):
        info = item.lstat()
        if not stat.S_ISDIR(info.st_mode) or item.is_symlink() or getattr(info, 'st_file_attributes', 0) & 1024:
            raise ValueError('Document proof inputs require ordinary directories')
    return path


def ordinary_file(path, limit):
    path = Path(path)
    plain_directory(path.parent)
    info = path.lstat()
    if path.is_symlink() or getattr(info, 'st_file_attributes', 0) & 1024 or not stat.S_ISREG(info.st_mode) or not 0 < info.st_size <= limit:
        raise ValueError('Document proof requires a bounded ordinary file')
    return info


def make_directory(path, boundary):
    boundary = plain_directory(boundary)
    path = Path(path).absolute()
    relative = path.relative_to(boundary)
    current = boundary
    for part in relative.parts:
        current = current / part
        if not current.exists() and not current.is_symlink():
            current.mkdir()
        plain_directory(current)
    return path


def checked_bytes(path, limit, expected=None):
    before = ordinary_file(path, limit)
    with Path(path).open('rb') as source:
        opened = os.fstat(source.fileno())
        if not os.path.samestat(before, opened):
            raise ValueError('Document proof input changed before open')
        data = source.read(limit + 1)
        after = os.fstat(source.fileno())
    if len(data) != before.st_size or not os.path.samestat(opened, after) or (opened.st_size, opened.st_mtime_ns) != (after.st_size, after.st_mtime_ns):
        raise ValueError('Document proof input changed during read')
    if expected is not None and hashlib.sha256(data).hexdigest() != expected:
        raise ValueError('Document proof input does not match its independent pin')
    return data


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError('Duplicate document profile field')
        result[key] = value
    return result


def relative_name(name):
    if not isinstance(name, str) or not name or '\\' in name or ':' in name or any(ord(c) < 32 or ord(c) == 127 for c in name):
        raise ValueError('Invalid document proof relative path')
    path = PurePosixPath(name)
    if path.is_absolute() or path.as_posix() != name or any(part in ('', '.', '..') or part.endswith((' ', '.')) for part in path.parts):
        raise ValueError('Invalid document proof relative path')
    devices = {'CON', 'PRN', 'AUX', 'NUL', *(f'COM{i}' for i in range(1, 10)), *(f'LPT{i}' for i in range(1, 10))}
    if any(part.split('.')[0].upper() in devices for part in path.parts):
        raise ValueError('Document proof path aliases a Windows device')
    return path


def pins(profile, tool):
    if not isinstance(profile, str) or not isinstance(tool, str) or not HEX.fullmatch(profile) or not HEX.fullmatch(tool):
        raise ValueError('Supply both independent lowercase document proof SHA256 pins')
    return (profile + '\n' + tool + '\n').encode('ascii')


def read_pins(package):
    file = Path(package) / PIN_FILE
    if not file.exists() and not file.is_symlink():
        if (Path(package) / PROFILE_ROOT).exists():
            raise ValueError('Document proof files have no independently selected package pins')
        return None
    raw = checked_bytes(file, 130)
    rows = raw.decode('ascii').splitlines()
    if len(rows) != 2 or raw != pins(*rows):
        raise ValueError('Invalid document proof package pins')
    return rows


def _profile_records(directory, profile_sha256, tool_sha256, descriptor='profile.json', child=False):
    pins(profile_sha256, tool_sha256)
    directory = plain_directory(directory)
    raw = checked_bytes(directory / descriptor, PROFILE_LIMIT, profile_sha256)
    profile = json.loads(raw.decode('utf-8'), object_pairs_hook=unique_object)
    required = ('schema', 'id', 'protocol', 'sourceSHA256', 'files')
    if not isinstance(profile, dict) or set(profile) not in (set(required), set(required) | {'purpose'}):
        raise ValueError('Invalid document proof descriptor fields')
    if type(profile['schema']) is not int or profile['schema'] != 1 or not isinstance(profile['id'], str) or not re.fullmatch(r'[A-Za-z0-9._-]{1,128}', profile['id']) or profile['protocol'] != 'luci-debian-signature-go-owner-v1' or not isinstance(profile['sourceSHA256'], str) or not HEX.fullmatch(profile['sourceSHA256']):
        raise ValueError('Invalid document proof descriptor identity')
    if 'purpose' in profile and profile['purpose'] != 'binary-bindings':
        raise ValueError('Unsupported document proof purpose')
    if child and 'purpose' in profile:
        raise ValueError('The fixed signature descriptor must have the original empty purpose')
    if not isinstance(profile['files'], list) or not 1 <= len(profile['files']) <= 212:
        raise ValueError('Invalid document proof input count')
    records = []
    seen_paths, seen_roles = {'profile.json'}, set()
    total, tool_found = len(raw), False
    for record in profile['files']:
        if not isinstance(record, dict) or set(record) != {'role', 'relativePath', 'bytes', 'sha256'}:
            raise ValueError('Invalid document proof file fields')
        relative = relative_name(record['relativePath']).as_posix()
        folded = relative.casefold()
        if folded in seen_paths or not isinstance(record['role'], str) or not record['role'] or len(record['role']) > 256 or record['role'] in seen_roles or type(record['bytes']) is not int or not 0 < record['bytes'] <= FILE_LIMIT or not isinstance(record['sha256'], str) or not HEX.fullmatch(record['sha256']):
            raise ValueError('Invalid or duplicate document proof file')
        seen_paths.add(folded)
        seen_roles.add(record['role'])
        total += record['bytes']
        if total > TOTAL_LIMIT:
            raise ValueError('Document proof package exceeds its finite source budget')
        if record['role'] == TOOL_ROLE:
            tool_found = relative == TOOL_PATH and record['sha256'] == tool_sha256 and record['bytes'] <= PROFILE_LIMIT
        if record['role'] == SIGNATURE_ROLE and (child or profile.get('purpose') != 'binary-bindings' or relative != SIGNATURE_PATH or record['bytes'] > PROFILE_LIMIT):
            raise ValueError('Only a binary profile may bind the one fixed signature descriptor')
        records.append({key: record[key] for key in ('role', 'relativePath', 'bytes', 'sha256')})
    if not tool_found:
        raise ValueError('Profile does not bind the independently selected tool domain')
    canonical = {key: profile[key] for key in required}
    canonical['files'] = records
    if 'purpose' in profile:
        canonical['purpose'] = profile['purpose']
    # Match encoding/json's canonical descriptor bytes, including HTML escaping.
    encoded = json.dumps(canonical, ensure_ascii=False, separators=(',', ':')).replace('&', '\\u0026').replace('<', '\\u003c').replace('>', '\\u003e').replace('\u2028', '\\u2028').replace('\u2029', '\\u2029').encode('utf-8')
    if raw != encoded:
        raise ValueError('Document proof descriptor is not the exact canonical schema')
    return raw, records


def profile_records(directory, profile_sha256, tool_sha256):
    raw, records = _profile_records(directory, profile_sha256, tool_sha256)
    child = next((record for record in records if record['role'] == SIGNATURE_ROLE), None)
    if child is None:
        return raw, records
    # One explicitly primary-pinned descriptor is permitted, never recursive
    # profile discovery. Its independent trust is the verified primary record.
    child_raw, child_records = _profile_records(directory, child['sha256'], tool_sha256, SIGNATURE_PATH, True)
    if len(child_raw) != child['bytes']:
        raise ValueError('Signature descriptor size changed')
    primary_identity, child_identity = json.loads(raw), json.loads(child_raw)
    if any(primary_identity[key] != child_identity[key] for key in ('id', 'protocol', 'sourceSHA256')):
        raise ValueError('Signature descriptor differs from its selected profile identity')
    union = {record['relativePath'].casefold(): record for record in records}
    for record in child_records:
        key = record['relativePath'].casefold()
        if key in union:
            previous = union[key]
            if any(previous[field] != record[field] for field in ('relativePath', 'bytes', 'sha256')):
                raise ValueError('Document profiles disagree about a shared input')
        else:
            union[key] = record
    combined = list(union.values())
    if len(combined) > 424 or len(raw) + sum(record['bytes'] for record in combined) > TOTAL_LIMIT:
        raise ValueError('Combined document proof input budget exceeded')
    return raw, combined


def verify_file(path, size, digest, destination=None, limit=FILE_LIMIT):
    before = ordinary_file(path, limit)
    if before.st_size != size:
        raise ValueError('Document proof file size changed')
    current, count = hashlib.sha256(), 0
    with Path(path).open('rb') as source:
        opened = os.fstat(source.fileno())
        if not os.path.samestat(before, opened):
            raise ValueError('Document proof file identity changed')
        sink = None
        try:
            if destination is not None:
                plain_directory(destination.parent)
                sink = destination.open('xb')
            while True:
                block = source.read(min(1 << 20, size + 1 - count))
                if not block:
                    break
                count += len(block)
                if count > size:
                    raise ValueError('Document proof file grew during copy')
                current.update(block)
                if sink is not None:
                    if sink.write(block) != len(block):
                        raise ValueError('Document proof copy did not acknowledge all bytes')
            after = os.fstat(source.fileno())
            if count != size or current.hexdigest() != digest or not os.path.samestat(opened, after) or (opened.st_size, opened.st_mtime_ns) != (after.st_size, after.st_mtime_ns):
                raise ValueError('Document proof file does not match its selected record')
            if sink is not None:
                sink.flush()
                os.fsync(sink.fileno())
        finally:
            if sink is not None:
                sink.close()


def verify_package(package, expected_profile=None, expected_tool=None):
    package = plain_directory(package)
    selected = read_pins(package)
    if expected_profile is not None or expected_tool is not None:
        pins(expected_profile, expected_tool)
        if selected != [expected_profile, expected_tool]:
            raise ValueError('Copied document proof pins differ from the selected release inputs')
    if selected is None:
        return None
    root = package / PROFILE_ROOT
    _, records = profile_records(root, *selected)
    files = [package / PIN_FILE, root / 'profile.json']
    for record in records:
        file = root / record['relativePath']
        verify_file(file, record['bytes'], record['sha256'])
        files.append(file)
    expected = {file.relative_to(root).as_posix() for file in files[1:]}
    expected_directories = set()
    for name in expected:
        expected_directories.update(parent.as_posix() for parent in PurePosixPath(name).parents if parent.as_posix() != '.')
    actual = set()
    for file in root.rglob('*'):
        info = file.lstat()
        if file.is_symlink() or getattr(info, 'st_file_attributes', 0) & 1024 or not (stat.S_ISDIR(info.st_mode) or stat.S_ISREG(info.st_mode)):
            raise ValueError('Linked or special document proof member')
        relative = file.relative_to(root).as_posix()
        if stat.S_ISDIR(info.st_mode):
            if relative not in expected_directories:
                raise ValueError('Document proof package contains an undeclared directory')
        else:
            if relative not in expected:
                raise ValueError('Document proof package contains an undeclared file')
            actual.add(relative)
    if actual != expected:
        raise ValueError('Document proof package contains undeclared files')
    return {'profileSHA256': selected[0], 'toolDomainSHA256': selected[1], 'files': files}


def stage(directory, package, profile_sha256, tool_sha256):
    raw, records = profile_records(directory, profile_sha256, tool_sha256)
    package = plain_directory(package)
    root = package / PROFILE_ROOT
    if root.exists() or (package / PIN_FILE).exists():
        raise ValueError('Document proof staging requires fresh destinations')
    make_directory(root, package)
    for record in records:
        target = root / record['relativePath']
        make_directory(target.parent, package)
        verify_file(Path(directory) / record['relativePath'], record['bytes'], record['sha256'], target)
    # Publish the immutable configuration only after all actual copies match.
    with (root / 'profile.json').open('xb') as output:
        output.write(raw)
    with (package / PIN_FILE).open('xb') as output:
        output.write(pins(profile_sha256, tool_sha256))
    return verify_package(package)


def verify_source(directory, profile_sha256, tool_sha256):
    _, records = profile_records(directory, profile_sha256, tool_sha256)
    for record in records:
        verify_file(Path(directory) / record['relativePath'], record['bytes'], record['sha256'])
    return {'profileSHA256': profile_sha256, 'toolDomainSHA256': tool_sha256, 'fileCount': len(records)}


def image_member_path(name):
    if not isinstance(name, str) or not re.fullmatch(r'[A-Za-z0-9._/-]{1,256}', name):
        raise ValueError('Invalid document image member name')
    path = PurePosixPath(name)
    if path.is_absolute() or path.as_posix() != name or any(part in ('', '.', '..') for part in name.split('/')):
        raise ValueError('Invalid document image member name')
    return name


def image_manifest(directory, expected):
    if not isinstance(expected, str) or not HEX.fullmatch(expected):
        raise ValueError('Supply the independently selected document image manifest SHA256')
    directory = plain_directory(directory)
    raw = checked_bytes(directory / IMAGE_MANIFEST, 1 << 20, expected)
    manifest = json.loads(raw.decode('utf-8'), object_pairs_hook=unique_object)
    if not isinstance(manifest, dict) or set(manifest) != {'version', 'kind', 'toolProtocol', 'provenance', 'archive', 'image', 'extractorSha256'} or type(manifest['version']) is not int or manifest['version'] != 1 or manifest['kind'] != 'luci-document-prebuilt-docker-save' or type(manifest['toolProtocol']) is not int or manifest['toolProtocol'] != 1:
        raise ValueError('Not the dedicated document prebuilt manifest')
    provenance = manifest['provenance']
    provenance_fields = {'assemblyReceiptSha256', 'inputClosureSha256', 'captureSha256', 'captureBytes', 'noticeClosureSha256', 'noticeClosureFileName', 'noticeClosureBytes', 'sourceCommit', 'sourceDigest', 'sourceRetentionDigest'}
    if not isinstance(provenance, dict) or set(provenance) != provenance_fields:
        raise ValueError('Invalid document image provenance fields')
    for key in ('assemblyReceiptSha256', 'inputClosureSha256', 'captureSha256', 'noticeClosureSha256', 'sourceDigest', 'sourceRetentionDigest'):
        if not isinstance(provenance[key], str) or not HEX.fullmatch(provenance[key]):
            raise ValueError('Invalid document image provenance digest')
    if not isinstance(provenance['sourceCommit'], str) or not re.fullmatch(r'[0-9a-f]{40}', provenance['sourceCommit']) or type(provenance['captureBytes']) is not int or not 1024 <= provenance['captureBytes'] <= 2 << 30 or provenance['noticeClosureFileName'] != IMAGE_NOTICES or type(provenance['noticeClosureBytes']) is not int or not 1 <= provenance['noticeClosureBytes'] <= 1 << 20:
        raise ValueError('Invalid document image provenance identity')
    archive, image = manifest['archive'], manifest['image']
    if not isinstance(archive, dict) or set(archive) != {'fileName', 'sha256', 'bytes', 'members'} or not isinstance(image, dict) or set(image) != {'id', 'configPath', 'layerPath', 'diffId'}:
        raise ValueError('Invalid document image manifest fields')
    if archive['fileName'] != IMAGE_ARCHIVE or type(archive['bytes']) is not int or not 1024 <= archive['bytes'] <= IMAGE_LIMIT or not isinstance(archive['sha256'], str) or not HEX.fullmatch(archive['sha256']) or not isinstance(manifest['extractorSha256'], str) or not HEX.fullmatch(manifest['extractorSha256']):
        raise ValueError('Invalid document image archive identity')
    for key in ('id', 'diffId'):
        if not isinstance(image[key], str) or not re.fullmatch(r'sha256:[0-9a-f]{64}', image[key]):
            raise ValueError('Invalid document image or layer identity')
    if image['configPath'] != image['id'][7:] + '.json' or not image_member_path(image['layerPath']).endswith('/layer.tar'):
        raise ValueError('Invalid document image config or layer path')
    required = {'manifest.json', image['configPath'], image['layerPath']}
    members = archive['members']
    if len(required) != 3 or not isinstance(members, list) or len(members) != 3:
        raise ValueError('Document image requires its exact three outer members')
    previous, found, member_bytes = '', set(), 0
    for member in members:
        if not isinstance(member, dict) or set(member) != {'path', 'sha256', 'bytes'}:
            raise ValueError('Invalid document image member record')
        path = image_member_path(member['path'])
        if path <= previous or path not in required or type(member['bytes']) is not int or not 1 <= member['bytes'] <= archive['bytes'] or not isinstance(member['sha256'], str) or not HEX.fullmatch(member['sha256']):
            raise ValueError('Invalid or duplicate document image member')
        if path == image['configPath'] and member['sha256'] != image['id'][7:] or path == image['layerPath'] and member['sha256'] != image['diffId'][7:]:
            raise ValueError('Document image config or layer digest disagrees')
        member_bytes += member['bytes']
        previous = path
        found.add(path)
    if found != required or member_bytes > archive['bytes']:
        raise ValueError('Invalid document image member coverage')
    return raw, manifest


def image_notices(directory, manifest):
    directory = plain_directory(directory)
    provenance = manifest['provenance']
    raw = checked_bytes(directory / IMAGE_NOTICES, 1 << 20, provenance['noticeClosureSha256'])
    if len(raw) != provenance['noticeClosureBytes']:
        raise ValueError('Document notice closure size changed')
    closure = json.loads(raw.decode('utf-8'), object_pairs_hook=unique_object)
    if not isinstance(closure, dict) or set(closure) != {'schema', 'kind', 'inputClosureSha256', 'sourceRetentionDigest', 'notices'} or type(closure['schema']) is not int or closure['schema'] != 1 or closure['kind'] != 'luci-document-notice-source-closure-v1' or closure['inputClosureSha256'] != provenance['inputClosureSha256'] or closure['sourceRetentionDigest'] != provenance['sourceRetentionDigest'] or not isinstance(closure['notices'], list) or not 1 <= len(closure['notices']) <= 512:
        raise ValueError('Document notice closure differs from the pinned provenance')
    seen, records, total = set(), [], 0
    for notice in closure['notices']:
        if not isinstance(notice, dict) or set(notice) != {'path', 'sha256', 'bytes', 'component', 'version', 'grant', 'provenance'}:
            raise ValueError('Invalid document notice fields')
        name = relative_name(notice['path']).as_posix()
        if not name.startswith('notices/') or name.casefold() in seen or type(notice['bytes']) is not int or not 1 <= notice['bytes'] <= 1 << 20 or not isinstance(notice['sha256'], str) or not HEX.fullmatch(notice['sha256']):
            raise ValueError('Invalid or duplicate document notice identity')
        if any(not isinstance(notice[key], str) or not 0 < len(notice[key]) <= 512 or any(ord(c) < 32 or ord(c) == 127 for c in notice[key]) for key in ('component', 'version', 'grant', 'provenance')):
            raise ValueError('Invalid document notice declaration')
        seen.add(name.casefold())
        total += notice['bytes']
        if total > 512 << 20:
            raise ValueError('Document notice aggregate exceeds its declared envelope')
        verify_file(directory / name, notice['bytes'], notice['sha256'])
        records.append(notice)
    return raw, records


def verify_image_source(directory, expected):
    _, manifest = image_manifest(directory, expected)
    _, notices = image_notices(directory, manifest)
    archive = manifest['archive']
    # Packaging checks whole-file integrity only. The runtime owner rechecks
    # actual Docker-save members, extractor bytes and image semantics before use.
    verify_file(Path(directory) / IMAGE_ARCHIVE, archive['bytes'], archive['sha256'], limit=IMAGE_LIMIT)
    return {'manifestSHA256': expected, 'imageID': manifest['image']['id'], 'archiveSHA256': archive['sha256'], 'archiveBytes': archive['bytes'], 'provenance': manifest['provenance'], 'noticeCount': len(notices)}


def verify_image_package(package, selected_manifest=None):
    package = plain_directory(package)
    pin = package / IMAGE_PIN_FILE
    if not pin.exists() and not pin.is_symlink():
        if selected_manifest is not None:
            raise ValueError('Selected document image package pin is missing')
        if (package / IMAGE_ROOT).exists():
            raise ValueError('Document image has no independently selected package pin')
        return None
    raw_pin = checked_bytes(pin, 65)
    expected = raw_pin.decode('ascii').removesuffix('\n')
    if not HEX.fullmatch(expected) or raw_pin != (expected + '\n').encode('ascii'):
        raise ValueError('Invalid document image package pin')
    if selected_manifest is not None and expected != selected_manifest:
        raise ValueError('Copied document image pin differs from the selected release input')
    root = plain_directory(package / IMAGE_ROOT)
    result = verify_image_source(root, expected)
    _, manifest = image_manifest(root, expected)
    _, notices = image_notices(root, manifest)
    required = {IMAGE_MANIFEST, IMAGE_ARCHIVE, IMAGE_NOTICES, *(record['path'] for record in notices)}
    directories = {parent.as_posix() for name in required for parent in PurePosixPath(name).parents if parent.as_posix() != '.'}
    found = set()
    for entry in root.rglob('*'):
        info = entry.lstat()
        relative = entry.relative_to(root).as_posix()
        if entry.is_symlink() or getattr(info, 'st_file_attributes', 0) & 1024 or not (stat.S_ISDIR(info.st_mode) or stat.S_ISREG(info.st_mode)):
            raise ValueError('Document image package has undeclared members')
        if stat.S_ISDIR(info.st_mode):
            if relative not in directories:
                raise ValueError('Document image package has an undeclared directory')
        elif relative not in required or relative in found:
            raise ValueError('Document image package has an undeclared file')
        else:
            found.add(relative)
    if found != required:
        raise ValueError('Document image package has undeclared members')
    result['files'] = [pin, *(root / name for name in sorted(required))]
    return result


def stage_image(directory, package, expected):
    raw, manifest = image_manifest(directory, expected)
    notice_raw, notices = image_notices(directory, manifest)
    package = plain_directory(package)
    root = package / IMAGE_ROOT
    if root.exists() or (package / IMAGE_PIN_FILE).exists():
        raise ValueError('Document image staging requires fresh destinations')
    make_directory(root, package)
    archive = manifest['archive']
    verify_file(Path(directory) / IMAGE_ARCHIVE, archive['bytes'], archive['sha256'], root / IMAGE_ARCHIVE, IMAGE_LIMIT)
    for notice in notices:
        destination = root / notice['path']
        make_directory(destination.parent, package)
        verify_file(Path(directory) / notice['path'], notice['bytes'], notice['sha256'], destination)
    with (root / IMAGE_NOTICES).open('xb') as output:
        output.write(notice_raw)
    with (root / IMAGE_MANIFEST).open('xb') as output:
        output.write(raw)
    with (package / IMAGE_PIN_FILE).open('xb') as output:
        output.write((expected + '\n').encode('ascii'))
    return verify_image_package(package)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('operation', choices=('stage', 'verify', 'verify-source', 'stage-image', 'verify-image', 'verify-image-source'))
    parser.add_argument('--package')
    parser.add_argument('--profile-dir')
    parser.add_argument('--profile-sha256')
    parser.add_argument('--tool-domain-sha256')
    parser.add_argument('--image-dir')
    parser.add_argument('--image-manifest-sha256')
    args = parser.parse_args()
    if args.operation == 'stage-image':
        if not args.package:
            parser.error('stage-image requires --package')
        result = stage_image(args.image_dir, args.package, args.image_manifest_sha256)
    elif args.operation == 'verify-image-source':
        result = verify_image_source(args.image_dir, args.image_manifest_sha256)
    elif args.operation == 'verify-image':
        if not args.package:
            parser.error('verify-image requires --package')
        result = verify_image_package(args.package, args.image_manifest_sha256)
    elif args.operation == 'stage':
        if not args.package:
            parser.error('stage requires --package')
        result = stage(args.profile_dir, args.package, args.profile_sha256, args.tool_domain_sha256)
    elif args.operation == 'verify-source':
        result = verify_source(args.profile_dir, args.profile_sha256, args.tool_domain_sha256)
    else:
        if not args.package:
            parser.error('verify requires --package')
        result = verify_package(args.package, args.profile_sha256, args.tool_domain_sha256)
    print(json.dumps(None if result is None else {key: value for key, value in result.items() if key != 'files'}, separators=(',', ':')))
