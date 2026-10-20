#!/usr/bin/env python3
"""Verify installed output against the server-derived document input closure.

This runs only inside the separately owned assembly sandbox, after dpkg/pip.
Input archive observations are not described as a final filesystem snapshot.
No native Python extension is imported and no inspected ELF is executed.
"""
import base64
import csv
import configparser
import email.parser
import email.policy
import hashlib
import io
import importlib.util
import json
import os
import posixpath
import re
import selectors
import stat
import struct
import subprocess
import sys
import time


MAX_LOCK = 2 << 20
MAX_INVENTORY = 16 << 20
MAX_FILES = 100000
MAX_FILE_BYTES = 2 << 30
MAX_TOTAL_BYTES = 2 << 30
GENERATED_PATHS = frozenset((
    "var/lib/dpkg/status", "var/lib/dpkg/status-old",
    "var/lib/dpkg/available", "var/lib/dpkg/available-old",
    "var/lib/dpkg/lock", "var/lib/dpkg/lock-frontend",
    "var/lib/dpkg/triggers/File", "var/lib/dpkg/triggers/Unincorp",
    "var/lib/dpkg/triggers/Lock", "etc/ld.so.cache",
    "etc/passwd", "etc/passwd-", "etc/group", "etc/group-",
    "etc/shadow", "etc/shadow-", "etc/gshadow", "etc/gshadow-",
    "var/log/dpkg.log", "var/log/alternatives.log",
))
SHA = re.compile(r"[0-9a-f]{64}\Z")
PACKAGE = re.compile(r"[a-z0-9][a-z0-9+.-]{0,199}\Z")


class Refused(Exception):
    pass


def refuse(condition):
    if not condition:
        raise Refused("installed document image does not match its closed inputs")


def unique_object(pairs):
    result = {}
    for name, value in pairs:
        refuse(name not in result)
        result[name] = value
    return result


def decode(data, maximum):
    refuse(0 < len(data) <= maximum)
    try:
        return json.loads(data.decode("utf-8"), object_pairs_hook=unique_object,
                          parse_constant=lambda _: (_ for _ in ()).throw(Refused()))
    except (ValueError, UnicodeError, RecursionError) as error:
        raise Refused("invalid closed input document") from error


def safe_path(value):
    return (isinstance(value, str) and 0 < len(value.encode("utf-8")) <= 1024
            and not value.startswith("/") and "\\" not in value and ":" not in value
            and all(ord(c) >= 32 and ord(c) != 127 for c in value)
            and all(part not in ("", ".", "..") for part in value.split("/")))


def parent_fd(root, name):
    refuse(safe_path(name))
    directory = os.dup(root)
    try:
        for part in name.split("/")[:-1]:
            child = os.open(part, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW,
                            dir_fd=directory)
            os.close(directory)
            directory = child
        return directory, name.split("/")[-1]
    except BaseException:
        os.close(directory)
        raise


def read_regular(root, name, maximum, expected=None):
    directory, leaf = parent_fd(root, name)
    try:
        fd = os.open(leaf, os.O_RDONLY | os.O_NOFOLLOW, dir_fd=directory)
        try:
            before = os.fstat(fd)
            refuse(stat.S_ISREG(before.st_mode) and 0 <= before.st_size <= maximum)
            result = bytearray()
            digest = hashlib.sha256()
            while True:
                piece = os.read(fd, min(65536, maximum + 1 - len(result)))
                if not piece:
                    break
                result.extend(piece)
                refuse(len(result) <= maximum)
                digest.update(piece)
            after = os.fstat(fd)
            refuse((before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns,
                    before.st_ctime_ns) == (after.st_dev, after.st_ino, after.st_size,
                                           after.st_mtime_ns, after.st_ctime_ns))
            if expected is not None:
                refuse(len(result) == expected["bytes"]
                       and digest.hexdigest() == expected["digest"])
            return bytes(result)
        finally:
            os.close(fd)
    finally:
        os.close(directory)


def generated_path(name, package_names):
    if name in GENERATED_PATHS:
        return True
    prefix = "var/lib/dpkg/info/"
    if name.startswith(prefix):
        stem, dot, suffix = name[len(prefix):].rpartition(".")
        if not dot or suffix not in ("list", "md5sums", "conffiles", "triggers",
                                     "preinst", "postinst", "prerm", "postrm",
                                     "shlibs", "symbols"):
            return False
        if stem.endswith(":amd64"):
            stem = stem[:-6]
        return stem in package_names
    return False


def query_installed():
    # Fixed command only; bounded pipes and wall time, no shell or discovery.
    command = ["/usr/bin/dpkg-query", "-W", "-f=${Package}\t${Version}\t${Architecture}\t${db:Status-Abbrev}\n"]
    child = subprocess.Popen(command, stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
                             stderr=subprocess.PIPE, close_fds=True,
                             env={"PATH": "/usr/bin:/bin", "LC_ALL": "C"})
    selector = selectors.DefaultSelector()
    output, diagnostic = bytearray(), bytearray()
    deadline = time.monotonic() + 10
    try:
        for stream, target, maximum in ((child.stdout, output, 2 << 20),
                                        (child.stderr, diagnostic, 65536)):
            os.set_blocking(stream.fileno(), False)
            selector.register(stream, selectors.EVENT_READ, (target, maximum))
        while selector.get_map():
            refuse(time.monotonic() < deadline)
            for key, _ in selector.select(min(.1, max(0, deadline-time.monotonic()))):
                target, maximum = key.data
                piece = os.read(key.fd, 65536)
                if not piece:
                    selector.unregister(key.fileobj)
                else:
                    target.extend(piece)
                    refuse(len(target) <= maximum)
        refuse(child.wait(timeout=max(.01, deadline-time.monotonic())) == 0 and not diagnostic)
        rows = {}
        for line in output.decode("utf-8").splitlines():
            fields = line.split("\t")
            refuse(len(fields) == 4 and PACKAGE.fullmatch(fields[0])
                   and fields[0] not in rows and fields[3] == "ii ")
            rows[fields[0]] = (fields[1], fields[2])
        refuse(0 < len(rows) <= 512)
        return rows
    finally:
        selector.close()
        if child.poll() is None:
            child.kill()
        child.wait()
        child.stdout.close()
        child.stderr.close()


def verify_inventory(root, rows, packages, wheels=()):
    refuse(isinstance(rows, list) and 0 < len(rows) <= MAX_FILES)
    seen, total = set(), 0
    elf = []
    for row in rows:
        refuse(isinstance(row, dict) and set(row) == {
            "path", "owner", "kind", "bytes", "sha256", "target", "policy"})
        name = row["path"]
        refuse(safe_path(name) and name not in seen)
        seen.add(name)
        directory, leaf = parent_fd(root, name)
        try:
            info = os.stat(leaf, dir_fd=directory, follow_symlinks=False)
            if row["kind"] == "directory":
                refuse(stat.S_ISDIR(info.st_mode) and row["policy"] in ("", "immutable"))
                continue
            if row["kind"] == "symlink":
                refuse(stat.S_ISLNK(info.st_mode))
                if row["policy"] == "generated-alternative":
                    refuse(name.startswith("etc/alternatives/")
                           and "/" not in name[len("etc/alternatives/"):])
                else:
                    refuse(os.readlink(leaf, dir_fd=directory) == row["target"]
                           and row["policy"] in ("", "immutable"))
                continue
            refuse(row["kind"] == "regular" and stat.S_ISREG(info.st_mode)
                   and type(info.st_size) is int and 0 <= info.st_size <= MAX_FILE_BYTES)
        finally:
            os.close(directory)
        total += info.st_size
        refuse(total <= MAX_TOTAL_BYTES)
        policy = row["policy"]
        if policy == "wheel-installer-metadata":
            refuse(wheel_metadata_path(name, row["owner"], wheels) and info.st_size <= 2 << 20)
            continue
        if policy == "debian-conffile":
            refuse(row["owner"].startswith("debian/") and info.st_size <= 16 << 20)
            continue
        if policy == "generated-metadata":
            refuse(generated_path(name, packages) and info.st_size <= 16 << 20)
            continue
        refuse(policy in ("", "immutable") and SHA.fullmatch(row["sha256"]))
        # Hash streaming so a large native library is never copied wholesale.
        directory, leaf = parent_fd(root, name)
        try:
            fd = os.open(leaf, os.O_RDONLY | os.O_NOFOLLOW, dir_fd=directory)
            try:
                before = os.fstat(fd)
                digest, count, prefix = hashlib.sha256(), 0, b""
                while True:
                    piece = os.read(fd, 65536)
                    if not piece:
                        break
                    if not prefix:
                        prefix = piece[:4]
                    digest.update(piece)
                    count += len(piece)
                    refuse(count <= row["bytes"])
                after = os.fstat(fd)
                refuse(count == row["bytes"] and digest.hexdigest() == row["sha256"]
                       and (before.st_size, before.st_mtime_ns, before.st_ctime_ns) ==
                       (after.st_size, after.st_mtime_ns, after.st_ctime_ns))
                if prefix == b"\x7fELF":
                    elf.append((name, elf_dependencies(fd, count)))
            finally:
                os.close(fd)
        finally:
            os.close(directory)
    links = {row["path"]: row["target"] for row in rows if row["kind"] == "symlink"}
    for row in rows:
        if row["policy"] == "generated-alternative":
            directory, leaf = parent_fd(root, row["path"])
            try:
                links[row["path"]] = os.readlink(leaf, dir_fd=directory)
            finally:
                os.close(directory)
    providers = {name: details for name, details in elf}
    for name, (_, needed, search) in elf:
        origin = posixpath.dirname(name)
        directories = []
        for entry in search:
            entry = entry.replace("${ORIGIN}", origin).replace("$ORIGIN", origin)
            directory = posixpath.normpath(entry.lstrip("/"))
            refuse(safe_path(directory))
            directories.append(directory)
        directories.extend(("lib/x86_64-linux-gnu", "usr/lib/x86_64-linux-gnu",
                            "lib", "usr/lib", "lib64", "usr/lib64"))
        for dependency in needed:
            matched = False
            for directory in directories:
                candidate = resolve_inventory_link(directory+"/"+dependency, links)
                if candidate in providers:
                    soname = providers[candidate][0]
                    if not soname or soname == dependency:
                        matched = True
                        break
            refuse(matched)


def resolve_inventory_link(name, links):
    seen = set()
    for _ in range(64):
        refuse(safe_path(name) and name not in seen)
        seen.add(name)
        pieces = name.split("/")
        for index in range(1, len(pieces)+1):
            prefix = "/".join(pieces[:index])
            if prefix in links:
                target = links[prefix]
                if target.startswith("/"):
                    base = target.lstrip("/")
                else:
                    base = posixpath.join(posixpath.dirname(prefix), target)
                name = posixpath.normpath(posixpath.join(base, *pieces[index:]))
                break
        else:
            return name
    raise Refused("cyclic installed library link")


def verify_alternatives(root, inventory):
    # dpkg's own finite administrative records select the active link. No file
    # in this directory is accepted as executable input or a new source grant.
    directory, leaf = parent_fd(root, "var/lib/dpkg/alternatives")
    try:
        fd = os.open(leaf, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=directory)
    finally:
        os.close(directory)
    accepted, total = set(), 0
    links = {r["path"]: r["target"] for r in inventory.values() if r["kind"] == "symlink"}
    try:
        names = sorted(os.listdir(fd))
        refuse(len(names) <= 512)
        for name in names:
            refuse(re.fullmatch(r"[A-Za-z0-9+._-]{1,200}", name))
            data = read_regular(root, "var/lib/dpkg/alternatives/"+name, 65536)
            total += len(data)
            refuse(total <= 2 << 20)
            lines = data.decode("utf-8").splitlines()
            refuse(len(lines) >= 2 and lines[0] in ("auto", "manual")
                   and lines[1].startswith("/") and safe_path(lines[1][1:]))
            targets, identifiers = set(), {name}
            for line in lines[2:]:
                if not line or re.fullmatch(r"-?[0-9]{1,10}", line):
                    continue
                if line.startswith("/"):
                    refuse(safe_path(line[1:]))
                    targets.add(line[1:])
                else:
                    refuse(re.fullmatch(r"[A-Za-z0-9+._-]{1,200}", line))
                    identifiers.add(line)
            for identifier in identifiers:
                candidate = "etc/alternatives/"+identifier
                parent, component = parent_fd(root, candidate)
                try:
                    try:
                        actual = os.readlink(component, dir_fd=parent)
                    except FileNotFoundError:
                        continue  # absent optional manual-page slave grants nothing
                finally:
                    os.close(parent)
                refuse(actual.startswith("/") and actual[1:] in targets)
                links[candidate] = actual
                resolved = resolve_inventory_link(candidate, links)
                refuse(resolved in inventory and inventory[resolved]["kind"] == "regular")
                accepted.add(candidate)
            accepted.add("var/lib/dpkg/alternatives/"+name)
        return accepted
    finally:
        os.close(fd)


def verify_font_caches(root):
    # These caches are generated acceleration data, not inspected input bytes.
    # Only this one fixed cache directory and the selected amd64/cache8 family
    # are allowed; other /var/cache entries receive no exception.
    parent, leaf = parent_fd(root, "var/cache/fontconfig")
    try:
        fd = os.open(leaf, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=parent)
    finally:
        os.close(parent)
    accepted, total = set(), 0
    try:
        names = os.listdir(fd)
        refuse(len(names) <= 4096)
        for name in names:
            refuse(name == "CACHEDIR.TAG" or re.fullmatch(r"[0-9a-f]{32}-le64\.cache-8", name))
            info = os.stat(name, dir_fd=fd, follow_symlinks=False)
            refuse(stat.S_ISREG(info.st_mode) and 0 < info.st_size <= 16 << 20)
            total += info.st_size
            refuse(total <= 64 << 20)
            if name == "CACHEDIR.TAG":
                refuse(read_regular(root, "var/cache/fontconfig/"+name, 4096).startswith(
                    b"Signature: 8a477f597d28d172789f06886806bc55"))
            accepted.add("var/cache/fontconfig/"+name)
        return accepted
    finally:
        os.close(fd)


def pyc_source(name, inventory):
    match = re.fullmatch(r"(.+)/__pycache__/([^/]+)\.cpython-311(?:\.opt-[12])?\.pyc", name)
    if not match:
        return None
    source = match[1]+"/"+match[2]+".py"
    row = inventory.get(source)
    return source if row and row["kind"] == "regular" and row["policy"] in ("", "immutable") else None


def verify_namespace(root, inventory, package_names, extra):
    allowed = set(inventory) | set(extra)
    ancestors = {""}
    for name in allowed:
        parent = posixpath.dirname(name)
        while parent:
            ancestors.add(parent)
            parent = posixpath.dirname(parent)
    seen, metadata, total = 0, 0, 0

    def visit(fd, prefix, depth):
        nonlocal seen, metadata, total
        refuse(depth <= 64)
        with os.scandir(fd) as entries:
            for entry in entries:
                name = prefix+entry.name
                seen += 1
                metadata += len(name.encode("utf-8"))
                refuse(seen <= MAX_FILES+20000 and metadata <= 32 << 20 and safe_path(name))
                info = entry.stat(follow_symlinks=False)
                if name == "opt/luci/build-input":
                    refuse(stat.S_ISDIR(info.st_mode))
                    continue  # exact original assembly owner retires this tree after verification
                if stat.S_ISDIR(info.st_mode):
                    generated_pycache = name.endswith("/__pycache__") and any(
                        p.startswith(name[:-len("__pycache__")]) and p.endswith(".py") for p in inventory)
                    child = os.open(entry.name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=fd)
                    try:
                        # An empty structural directory carries no extra
                        # executable/content claim. Its names/depth still debit
                        # the same finite namespace budget.
                        refuse(name in ancestors or name in allowed or generated_pycache or not os.listdir(child))
                        visit(child, name+"/", depth+1)
                    finally:
                        os.close(child)
                    continue
                if name == "dev/null":
                    refuse(stat.S_ISCHR(info.st_mode) and os.major(info.st_rdev) == 1 and os.minor(info.st_rdev) == 3)
                    continue
                if name in allowed:
                    continue
                if generated_path(name, package_names):
                    refuse(stat.S_ISREG(info.st_mode) and info.st_size <= 16 << 20)
                    total += info.st_size
                    refuse(total <= 64 << 20)
                    continue
                source = pyc_source(name, inventory)
                if source:
                    # A pyc header is not proof of its marshalled code body.
                    # Drop only derived, non-inventoried caches for an exact
                    # immutable source. Python can load that retained source;
                    # inspected bytecode is never imported or unmarshalled.
                    refuse(stat.S_ISREG(info.st_mode) and info.st_size <= 4 << 20)
                    os.unlink(entry.name, dir_fd=fd)
                    continue
                raise Refused("unexpected installed path")
    visit(root, "", 0)


def elf_dependencies(fd, size):
    def at(offset, count):
        refuse(0 <= offset <= size and 0 <= count <= min(1 << 20, size-offset))
        data = os.pread(fd, count, offset)
        refuse(len(data) == count)
        return data
    header = at(0, 64)
    refuse(header[:6] == b"\x7fELF\x02\x01" and struct.unpack_from("<H", header, 18)[0] == 62)
    if struct.unpack_from("<H", header, 16)[0] == 1:  # relocatable data, not a loadable object
        return "", [], []
    offset = struct.unpack_from("<Q", header, 32)[0]
    width, count = struct.unpack_from("<HH", header, 54)
    refuse(width == 56 and 0 < count <= 256)
    table = at(offset, width*count)
    loads, dynamic = [], None
    for i in range(count):
        kind, _, file_offset, address, _, file_size, _, _ = struct.unpack_from("<IIQQQQQQ", table, i*width)
        refuse(file_offset <= size and file_size <= size-file_offset)
        if kind == 1:
            loads.append((address, file_offset, file_size))
        if kind == 2:
            refuse(dynamic is None and file_size <= 65536 and file_size % 16 == 0)
            dynamic = at(file_offset, file_size)
    if dynamic is None:
        return "", [], []
    tags, strings_address, strings_size, terminated = [], None, None, False
    for i in range(0, len(dynamic), 16):
        tag, value = struct.unpack_from("<QQ", dynamic, i)
        if tag == 0:
            terminated = True
            break
        if tag == 5:
            refuse(strings_address is None)
            strings_address = value
        elif tag == 10:
            refuse(strings_size is None and 0 < value <= 1 << 20)
            strings_size = value
        elif tag in (1, 14, 15, 29):
            tags.append((tag, value))
    refuse(terminated)
    if not tags:
        return "", [], []
    refuse(strings_address is not None and strings_size is not None)
    choices = [(base, position, length) for base, position, length in loads
               if base <= strings_address and strings_address-base <= length
               and strings_size <= length-(strings_address-base)]
    refuse(len(choices) == 1)
    base, position, _ = choices[0]
    strings = at(position+strings_address-base, strings_size)
    needed, paths, soname = [], [], ""
    for tag, index in tags:
        refuse(index < len(strings))
        end = strings.find(b"\0", index)
        refuse(index < end <= index+4096)
        value = strings[index:end].decode("utf-8", "strict")
        if tag in (1, 14):
            refuse("/" not in value and "\\" not in value and len(value) <= 255)
            if tag == 1:
                needed.append(value)
            else:
                refuse(not soname)
                soname = value
        else:
            for entry in value.split(":"):
                refuse(entry and (entry.startswith("$ORIGIN/") or entry == "$ORIGIN"
                                 or entry.startswith("${ORIGIN}/") or entry == "${ORIGIN}"
                                 or entry in ("/lib", "/lib64", "/usr/lib", "/usr/lib64",
                                              "/lib/x86_64-linux-gnu", "/usr/lib/x86_64-linux-gnu")))
                paths.append(entry)
    return soname, needed, paths


def verify_wheels(root, wheels, inventory):
    refuse(isinstance(wheels, list) and len(wheels) == 2)
    site = "usr/local/lib/python3.11/dist-packages/"
    seen, installed_paths = set(), set()
    for wheel in wheels:
        identity, version = wheel["id"], wheel["version"]
        refuse(identity in ("wheel/pypdfium2", "wheel/pillow") and identity not in seen)
        seen.add(identity)
        name = identity.split("/")[1]
        refuse(isinstance(version, str) and re.fullmatch(r"[A-Za-z0-9.+_-]{1,128}", version))
        owner = wheel["installedOwner"]
        refuse(safe_path(owner) and "/" not in owner and owner.endswith(".dist-info"))
        directory = site + owner + "/"
        metadata_bytes = read_regular(root, directory+"METADATA", 1 << 20)
        metadata = email.parser.BytesParser(policy=email.policy.compat32).parsebytes(metadata_bytes, headersonly=True)
        refuse(not metadata.defects)
        headers = {}
        for key in ("Name", "Version", "Requires-Python"):
            values = metadata.get_all(key, [])
            refuse(len(values) <= 1)
            headers[key] = " ".join(str(values[0]).splitlines()).strip() if values else ""
        requires = [" ".join(str(value).splitlines()).strip() for value in metadata.get_all("Requires-Dist", [])]
        refuse(re.sub(r"[-_.]+", "-", headers["Name"]).lower() == name
               and headers["Version"] == version
               and headers["Requires-Python"] == wheel["requiresPython"]
               and requires == (wheel["requiresDist"] or []))
        wheel_metadata = email.parser.BytesParser(policy=email.policy.compat32).parsebytes(
            read_regular(root, directory+"WHEEL", 65536), headersonly=True)
        pure = wheel_metadata.get_all("Root-Is-Purelib", [])
        tags = wheel_metadata.get_all("Tag", [])
        refuse(not wheel_metadata.defects and len(pure) == 1 and pure[0] in ("true", "false")
               and (pure[0] == "true") == wheel["rootIsPurelib"]
               and list(tags) == wheel["tags"] and 0 < len(tags) <= 16)
        refuse(os.uname().machine == "x86_64" and os.confstr("CS_GNU_LIBC_VERSION") == "glibc 2.36")
        refuse(compatible_wheel_tags(list(tags), sys.version_info.minor, 36))
        verify_python_requirements(headers["Requires-Python"], requires, wheels)
        scripts = set()
        observed_entries = []
        try:
            entry_bytes = read_regular(root, directory+"entry_points.txt", 65536)
        except FileNotFoundError:
            entry_bytes = b""
        if entry_bytes:
            parser = configparser.RawConfigParser(interpolation=None, strict=True)
            parser.optionxform = str
            parser.read_string(entry_bytes.decode("utf-8"))
            for group in parser.sections():
                refuse(group in ("console_scripts", "gui_scripts"))
                for entry, value in parser.items(group):
                    refuse(re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]{0,127}", entry))
                    observed_entries.append({"group": group, "name": entry, "value": value.strip()})
                    scripts.add(site+"bin/"+entry)
        refuse(observed_entries == (wheel["entryPoints"] or []))
        record_name = directory+"RECORD"
        refuse(read_regular(root, directory+"INSTALLER", 64) == b"pip\n")
        for optional, maximum in (("REQUESTED", 0), ("direct_url.json", 65536)):
            try:
                value = read_regular(root, directory+optional, maximum)
            except FileNotFoundError:
                continue
            if optional == "REQUESTED":
                refuse(value == b"")
            else:
                direct = decode(value, maximum)
                refuse(direct.get("url") == "file:///opt/luci/build-input/"+wheel["archive"]["path"])
                archive = direct.get("archive_info", {})
                refuse(archive.get("hashes", {}).get("sha256") == wheel["archive"]["digest"]
                       or archive.get("hash") == "sha256="+wheel["archive"]["digest"])
        record = read_regular(root, record_name, 2 << 20).decode("utf-8")
        paths, total = set(), 0
        for row in csv.reader(io.StringIO(record, newline="")):
            refuse(len(row) == 3 and len(paths) < 20000)
            relative, digest, length = row
            refuse(relative and len(relative) <= 1024 and not relative.startswith("/")
                   and "\\" not in relative and "\x00" not in relative)
            target = installed_wheel_record_path(relative)
            refuse(target not in paths)
            refuse(target in inventory or wheel_metadata_path(target, identity, wheels) or target in scripts)
            paths.add(target)
            if not digest and not length:
                refuse(target == record_name)
                continue
            refuse(digest.startswith("sha256=") and re.fullmatch(r"0|[1-9][0-9]{0,9}", length))
            raw = read_regular(root, target, 32 << 20)
            total += len(raw)
            refuse(total <= 128 << 20 and len(raw) == int(length)
                   and base64.urlsafe_b64encode(hashlib.sha256(raw).digest()).decode().rstrip("=") == digest[7:])
        refuse(record_name in paths and directory+"METADATA" in paths and directory+"WHEEL" in paths)
        installed_paths.update(paths)
    return installed_paths


def installed_wheel_record_path(relative):
    # The fixed Linux pip --target arm installs via its posix_home temporary
    # scheme (lib/python, bin, share), then moves both libraries and data under
    # target. RECORD is relative to that temporary lib/python directory. Apply
    # this ONE recipe mapping; do not search alternate installation locations.
    refuse(isinstance(relative, str) and 0 < len(relative) <= 1024
           and not relative.startswith("/") and "\\" not in relative
           and all(ord(c) >= 32 and ord(c) != 127 for c in relative))
    item = posixpath.normpath("lib/python/"+relative)
    target = "usr/local/lib/python3.11/dist-packages/"
    if item.startswith("lib/python/"):
        item = item[len("lib/python/"):]
    else:
        refuse(item.startswith(("bin/", "share/")))
    refuse(safe_path(item))
    return target+item


def compatible_wheel_tags(tags, python_minor, glibc_minor):
    refuse(isinstance(tags, list) and 0 < len(tags) <= 16 and len(set(tags)) == len(tags))
    compatible = False
    for tag in tags:
        parts = tag.split("-")
        refuse(len(tag) <= 512 and len(parts) == 3)
        interpreters, abis, platforms = (part.split(".") for part in parts)
        refuse(max(len(interpreters), len(abis), len(platforms)) <= 4)
        for platform in platforms:
            if platform == "manylinux2014_x86_64":
                minor = 17
            else:
                match = re.fullmatch(r"manylinux_2_([0-9]{1,2})_x86_64", platform)
                refuse(match is not None)
                minor = int(match[1])
                refuse(17 <= minor <= 36)
            for interpreter in interpreters:
                for abi in abis:
                    if interpreter == "py3":
                        refuse(abi == "none")
                        compatible |= minor <= glibc_minor
                        continue
                    match = re.fullmatch(r"cp3([0-9]{1,2})", interpreter)
                    refuse(match is not None and 6 <= int(match[1]) <= 13)
                    required = int(match[1])
                    if abi == "abi3":
                        compatible |= required <= python_minor and minor <= glibc_minor
                    else:
                        refuse(abi == interpreter)
                        compatible |= required == python_minor and minor <= glibc_minor
    return compatible


def wheel_metadata_path(name, owner, wheels):
    for wheel in wheels:
        if owner == wheel["id"]:
            prefix = "usr/local/lib/python3.11/dist-packages/"+wheel["installedOwner"]+"/"
            if name.startswith(prefix):
                return name[len(prefix):] in ("RECORD", "INSTALLER", "REQUESTED", "direct_url.json")
    return False


def verify_python_requirements(requires_python, requirements, wheels):
    refuse(sys.version_info[:2] == (3, 11) and sys.platform == "linux" and struct.calcsize("P") == 8)
    if not requires_python and not requirements:
        return
    # Load the already installed Debian/pip parser, never a wheel's own module
    # from the target tree. There is no import of pypdfium2, PIL or native ELF.
    original_path = sys.path[:]
    try:
        sys.path[:] = [value for value in original_path if value in (
                       "/usr/lib/python311.zip", "/usr/lib/python3.11",
                       "/usr/lib/python3.11/lib-dynload", "/usr/lib/python3/dist-packages")
                       or value.startswith("/usr/share/python-wheels/")]
        specifiers = importlib.import_module("pip._vendor.packaging.specifiers")
        requirement_parser = importlib.import_module("pip._vendor.packaging.requirements")
        markers = importlib.import_module("pip._vendor.packaging.markers")
        for module in (specifiers, requirement_parser, markers):
            refuse(module.__file__.startswith(("/usr/lib/python3", "/usr/share/python-wheels/")))
        version = ".".join(str(part) for part in sys.version_info[:3])
        refuse(not requires_python or specifiers.SpecifierSet(requires_python).contains(version, prereleases=True))
        environment = markers.default_environment()
        environment["extra"] = ""
        available = {wheel["id"].split("/")[1]: wheel["version"] for wheel in wheels}
        for text in requirements:
            requirement = requirement_parser.Requirement(text)
            if requirement.marker is not None and not requirement.marker.evaluate(environment):
                continue
            key = re.sub(r"[-_.]+", "-", requirement.name).lower()
            refuse(requirement.url is None and not requirement.extras and key in available
                   and requirement.specifier.contains(available[key], prereleases=True))
    finally:
        sys.path[:] = original_path


def verify(root, lock_name):
    lock = decode(read_regular(root, lock_name, MAX_LOCK), MAX_LOCK)
    refuse(isinstance(lock, dict) and lock.get("state") == "closed-reviewed"
           and lock.get("unresolved") == [])
    observed = lock.get("luciObserved")
    refuse(isinstance(observed, dict) and observed.get("schema") == 1
           and SHA.fullmatch(observed.get("sourceLockSha256", "")))
    source_text = lock.get("sourceMetadataBase64")
    refuse(isinstance(source_text, str) and len(source_text) <= (4 << 20)//3+8)
    try:
        original = base64.b64decode(source_text, validate=True)
    except ValueError as error:
        raise Refused("invalid retained source metadata") from error
    refuse(base64.b64encode(original).decode() == source_text
           and hashlib.sha256(original).hexdigest() == observed["sourceLockSha256"])
    decode(original, 1 << 20)
    packages = observed["packages"]
    refuse(isinstance(packages, list) and 0 < len(packages) <= 512)
    expected = {}
    for row in packages:
        refuse(PACKAGE.fullmatch(row["name"]) and row["name"] not in expected
               and row["architecture"] in ("amd64", "all"))
        expected[row["name"]] = (row["version"], row["architecture"])
    refuse(query_installed() == expected)
    inventories = observed["installedInputs"]
    refuse(isinstance(inventories, list) and 0 < len(inventories) <= 64)
    rows, total = [], 0
    for index, inventory in enumerate(inventories):
        refuse(set(inventory) == {"path", "bytes", "digest"}
               and inventory["path"] == "verification/installed-%04d.json" % index
               and SHA.fullmatch(inventory["digest"]) and type(inventory["bytes"]) is int
               and 0 < inventory["bytes"] <= 1 << 20)
        total += inventory["bytes"]
        refuse(total <= 32 << 20)
        inventory_name = "opt/luci/build-input/"+inventory["path"]
        page = decode(read_regular(root, inventory_name, 1 << 20, inventory), 1 << 20)
        refuse(isinstance(page, list) and len(rows)+len(page) <= MAX_FILES)
        rows.extend(page)
    verify_inventory(root, rows, expected, observed["wheels"])
    wheel_paths = verify_wheels(root, observed["wheels"], {row["path"]: row for row in rows})
    inventory_map = {row["path"]: row for row in rows}
    alternatives = verify_alternatives(root, inventory_map)
    caches = verify_font_caches(root)
    extra = set(wheel_paths) | alternatives | caches | {
        "opt/luci/document_extractor.py", "opt/luci/dependency-lock.json",
        "opt/luci/assembly-manifest.json", "opt/luci/original",
        "opt/luci/tessdata/eng.traineddata", "opt/luci/tessdata/osd.traineddata",
        "dev/null", "opt/luci/build-input",
    }
    for notice in observed["notices"]:
        refuse(safe_path(notice["path"]) and notice["path"].startswith("notices/"))
        read_regular(root, "opt/luci/build-input/"+notice["path"], 1 << 20, notice)
        read_regular(root, "opt/luci/licenses/"+notice["path"][len("notices/"):], 1 << 20, notice)
        extra.add("opt/luci/licenses/"+notice["path"][len("notices/"):])
    helper = observed["helper"]
    read_regular(root, "opt/luci/document_extractor.py", 2 << 20, helper["extractor"])
    original = read_regular(root, "opt/luci/original", 0)
    refuse(original == b"")
    files = {row["path"]: row for row in rows}
    for language in ("eng", "osd"):
        source = "usr/share/tesseract-ocr/5/tessdata/"+language+".traineddata"
        refuse(source in files and files[source]["kind"] == "regular"
               and files[source]["policy"] in ("", "immutable"))
        language_pin = {"bytes": files[source]["bytes"], "digest": files[source]["sha256"]}
        read_regular(root, "opt/luci/tessdata/"+language+".traineddata", 128 << 20, language_pin)
    manifest = decode(read_regular(root, "opt/luci/assembly-manifest.json", 256 << 10), 256 << 10)
    source = observed["source"]
    for key in ("sourceCommit", "sourceDigest", "helperDigest", "recipeDigest", "sourceRetentionDigest"):
        refuse(manifest[key] == source[key])
    refuse(manifest["verificationInputs"] == inventories
           and manifest["notices"] == observed["notices"]
           and manifest["dependencyLock"] == {"path": "dependency-lock.json",
                "bytes": len(read_regular(root, lock_name, MAX_LOCK)),
                "digest": hashlib.sha256(read_regular(root, lock_name, MAX_LOCK)).hexdigest()})
    verify_namespace(root, inventory_map, expected, extra)


def main():
    refuse(len(sys.argv) == 2 and sys.argv[1] == "/opt/luci/dependency-lock.json"
           and os.getuid() == 0 and os.getgid() == 0)
    root = os.open("/", os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        verify(root, "opt/luci/dependency-lock.json")
    finally:
        os.close(root)


if __name__ == "__main__":
    try:
        main()
    except (Refused, OSError, ValueError, KeyError, TypeError, UnicodeError, ImportError, configparser.Error,
            struct.error, subprocess.SubprocessError):
        print("Document image installed-output verification refused.", file=sys.stderr)
        sys.exit(1)
