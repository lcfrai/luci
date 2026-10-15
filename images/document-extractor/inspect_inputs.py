"""Contained stdlib data inspection only. Never import or execute an input member."""
import argparse
import base64
import bz2
import csv
from contextlib import contextmanager, nullcontext
import hashlib
import http.client
import io
import json
import lzma
import os
from pathlib import Path, PurePosixPath
import re
import ssl
import stat
import struct
import time
from urllib.parse import urlsplit
import zipfile
import zlib

MANIFEST_SHA256 = 'f839f2d1edaf0fbc1f039d20fcc4abd83b04accba2d0846e369174a0f2bad8b2'
LOCK_SHA256 = '91f2739b1ccfb8c5277e41ebbcaa45ba7ba16f1018abdf0122fc39c1ec0d0423'
BASE_DIFF_ID = '34467cc93912af891dfee8a3902adf9b4f355a9bca4eaa46f658d1181a21de0d'
CHUNK = 65536
MIB = 1 << 20
MAX_EXPANDED = 1024 * MIB
MAX_MEMBER = 128 * MIB
MAX_METADATA = MIB
MAX_RESULT = 2 * MIB
MAX_MEMBERS = 100000
HOSTS = {'files.pythonhosted.org', 'registry-1.docker.io', 'deb.debian.org', 'security.debian.org'}


class Refusal(ValueError):
    pass


class HeaderReader:
    def __init__(self, stream):
        self.stream, self.total = stream, 0

    def readline(self, maximum=-1):
        line = self.stream.readline(min(65537, maximum) if maximum >= 0 else 65537)
        self.total += len(line)
        if self.total > 65536:
            raise Refusal('HTTP header byte bound')
        return line

    def read(self, size):
        return self.stream.read(size)

    @property
    def closed(self):
        return self.stream.closed

    def flush(self):
        # HTTPResponse.close() flushes its read-only fp before retiring it.
        # This wrapper never buffers writes, so there is nothing to flush.
        # In particular, closing a refused header must retain that refusal.
        return None

    def close(self):
        self.stream.close()


class BoundedHTTPResponse(http.client.HTTPResponse):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.fp = HeaderReader(self.fp)

    def begin(self):
        try:
            return super().begin()
        except BaseException:
            self.close()  # Also retire the reader when getresponse never returns it.
            raise


def sha(data):
    return hashlib.sha256(data).hexdigest()


def regular(path, directory=False):
    path = Path(path).absolute()
    if '..' in path.parts:
        raise Refusal('parent traversal')
    for parent in (path, *path.parents):
        info = parent.lstat()
        if stat.S_ISLNK(info.st_mode) or getattr(info, 'st_file_attributes', 0) & 1024:
            raise Refusal('symlink/reparse path')
    if directory != path.is_dir() or not directory and not stat.S_ISREG(path.stat().st_mode):
        raise Refusal('path type')
    return path


@contextmanager
def locked_input(path):
    """One immutable read handle during hash+parse; Windows denies write/delete."""
    if os.name != 'nt':
        # Inert fixture support only; production controller requires Windows.
        with path.open('rb') as stream:
            yield stream
        return
    import ctypes
    from ctypes import wintypes
    import msvcrt
    kernel = ctypes.WinDLL('kernel32', use_last_error=True)
    kernel.CreateFileW.argtypes = [wintypes.LPCWSTR, wintypes.DWORD, wintypes.DWORD, wintypes.LPVOID, wintypes.DWORD, wintypes.DWORD, wintypes.HANDLE]
    kernel.CreateFileW.restype = wintypes.HANDLE
    kernel.CloseHandle.argtypes, kernel.CloseHandle.restype = [wintypes.HANDLE], wintypes.BOOL
    handle = kernel.CreateFileW(str(path), 0x80000000, 1, None, 3, 0x80 | 0x00200000, None)
    if handle in (None, ctypes.c_void_p(-1).value):
        raise Refusal('exclusive read identity unavailable')
    try:
        fd = msvcrt.open_osfhandle(handle, os.O_RDONLY | os.O_BINARY)
    except BaseException:
        kernel.CloseHandle(handle)
        raise
    with os.fdopen(fd, 'rb') as stream:
        if not stat.S_ISREG(os.fstat(stream.fileno()).st_mode):
            raise Refusal('opened input is not regular')
        yield stream


def json_read(path, maximum=MAX_RESULT):
    regular(path)
    if path.stat().st_size > maximum:
        raise Refusal('JSON byte bound')
    with path.open('rb') as stream:
        data = stream.read(maximum + 1)
    if len(data) > maximum:
        raise Refusal('JSON byte bound')
    def unique(pairs):
        result = {}
        for key, value in pairs:
            if key in result:
                raise Refusal('duplicate JSON key')
            result[key] = value
        return result
    return json.loads(data, object_pairs_hook=unique, parse_constant=lambda _: (_ for _ in ()).throw(Refusal('nonfinite JSON')))


def write_result(path, value):
    data = (json.dumps(value, separators=(',', ':'), sort_keys=True, ensure_ascii=True, allow_nan=False) + '\n').encode()
    if len(data) > MAX_RESULT:
        raise Refusal('result byte bound')
    with path.open('xb') as stream:
        stream.write(data)


def load_manifest(batch):
    path = regular(Path(__file__).parent / 'acquisition-inspection-manifest.json')
    if path.stat().st_size > MIB:
        raise Refusal('manifest bound')
    raw = path.read_bytes()
    if sha(raw) != MANIFEST_SHA256:
        raise Refusal('manifest pin')
    value = json_read(path, MIB)
    if batch in ('primary', 'wheels', 'base-layer', 'missing-notice'):
        items = value['binaryOnlyAndMissingNoticeInputs']
        expected = (4, 39013320)
        if batch != 'primary':
            kinds = {'wheels': ('wheel', 2, 10733989), 'base-layer': ('oci-layer', 1, 28238443), 'missing-notice': ('source-patch-metadata', 1, 40888)}
            kind, count, total = kinds[batch]
            items = [item for item in items if item['kind'] == kind]
            expected = (count, total)
    elif batch == 'debian-controls':
        items = [dict(item, kind='deb') for item in value['debianControlInputs']]
        expected = (174, 144257468)
    else:
        raise Refusal('enumerated batch required')
    if (len(items), sum(item['bytes'] for item in items)) != expected:
        raise Refusal('batch cardinality')
    if sum(item['bytes'] for item in items) > 256 * MIB or len({item['file'].casefold() for item in items}) != len(items):
        raise Refusal('batch bytes/duplicates')
    for item in items:
        if not re.fullmatch(r'[A-Za-z0-9_.+~%-]{1,240}', item['file']) or not re.fullmatch('[0-9a-f]{64}', item['sha256']):
            raise Refusal('input name/hash')
        url = urlsplit(item['url'])
        if url.scheme != 'https' or url.hostname not in HOSTS or url.port not in (None, 443) or url.username or url.password or url.fragment or url.query:
            raise Refusal('exact public URL required')
    return items


class Budget:
    def __init__(self, deadline):
        self.deadline = deadline
        self.expanded = self.members = 0
        self.retained = 0

    def check(self):
        if time.monotonic() >= self.deadline:
            raise Refusal('work deadline')

    def charge(self, size):
        self.check()
        self.expanded += size
        if self.expanded > MAX_EXPANDED:
            raise Refusal('aggregate actual expansion')

    def member(self, size):
        self.check()
        self.members += 1
        if self.members > MAX_MEMBERS or not 0 <= size <= MAX_MEMBER:
            raise Refusal('member count/size')

    def retain(self, row):
        self.retained += len(json.dumps(row, separators=(',', ':'), ensure_ascii=True))+1
        if self.retained > MAX_RESULT-8192:
            raise Refusal('aggregate retained metadata/member rows')


def acquire(item, output, budget, connection_factory=http.client.HTTPSConnection):
    """No proxy handler, ambient auth, redirects, cookies, retries or token flow."""
    budget.check()
    url = urlsplit(item['url'])
    context = ssl.create_default_context()
    client = connection_factory(url.hostname, port=443, timeout=min(30, max(.001, budget.deadline-time.monotonic())), context=context)
    client.response_class = BoundedHTTPResponse
    response = None
    partial = output / (item['file'] + '.partial')
    try:
        client.request('GET', url.path, headers={'Accept-Encoding': 'identity', 'Connection': 'close', 'User-Agent': 'Luci-extractor-static-input-review/1'})
        response = client.getresponse()
        lengths = response.headers.get_all('Content-Length', [])
        if response.status != 200 or len(lengths) != 1 or lengths[0] != str(item['bytes']) or response.getheader('Content-Encoding') or response.getheader('Transfer-Encoding') or response.getheader('Content-Range'):
            raise Refusal('unsupported HTTP status/framing; no redirect/token fallback')
        total, digest = 0, hashlib.sha256()
        with partial.open('xb') as stream:
            while True:
                budget.check()
                # Read the framed connection body through EOF ourselves, so the
                # library cannot hide bytes after its Content-Length boundary.
                block = response.fp.read(min(CHUNK, item['bytes']-total+1))
                if not block:
                    break
                total += len(block)
                if total > item['bytes']:
                    raise Refusal('transfer exceeds exact bytes')
                digest.update(block)
                stream.write(block)
        if total != item['bytes'] or digest.hexdigest() != item['sha256']:
            raise Refusal('incomplete/foreign transfer')
        os.link(partial, output / item['file'])  # exclusive publication, no overwrite
        partial.unlink()
        return {'file': item['file'], 'bytes': total, 'sha256': digest.hexdigest()}
    finally:
        if response is not None:
            response.close()
        client.close()


class Section:
    def __init__(self, stream, length):
        self.stream, self.remaining = stream, length

    def read(self, size=CHUNK):
        if size < 0 or size > CHUNK:
            raise Refusal('unbounded read')
        block = self.stream.read(min(size, self.remaining))
        if len(block) > self.remaining:
            raise Refusal('section overread')
        self.remaining -= len(block)
        return block


class Decoded:
    """Single compressed frame only, bounded output per call and decoder memory."""
    def __init__(self, stream, kind, budget):
        self.stream, self.budget, self.kind = stream, budget, kind
        if kind == 'gz':
            self.decoder = zlib.decompressobj(31)
        elif kind == 'xz':
            self.decoder = lzma.LZMADecompressor(format=lzma.FORMAT_XZ, memlimit=128*MIB)
        elif kind == 'bz2':
            self.decoder = bz2.BZ2Decompressor()
        else:
            raise Refusal('unsupported compression')
        self.pending, self.buffer, self.done = b'', bytearray(), False
        self.digest = hashlib.sha256()

    def read(self, size=CHUNK):
        if not 0 <= size <= CHUNK:
            raise Refusal('unbounded decoder read')
        while len(self.buffer) < size and not self.done:
            self.budget.check()
            if self.kind == 'gz':
                data = self.pending or self.stream.read(CHUNK)
            else:
                data = self.stream.read(CHUNK) if self.decoder.needs_input else b''
            if not data and (self.kind == 'gz' or self.decoder.needs_input):
                raise Refusal('incomplete compressed frame')
            block = self.decoder.decompress(data, CHUNK)
            self.budget.charge(len(block))
            self.digest.update(block)
            self.buffer.extend(block)
            if self.kind == 'gz':
                self.pending = self.decoder.unconsumed_tail
            if self.decoder.eof:
                if self.decoder.unused_data or self.stream.read(1):
                    raise Refusal('extra compressed frame/trailing bytes')
                self.done = True
        result = bytes(self.buffer[:size])
        del self.buffer[:size]
        return result


def exact(stream, size, maximum=MAX_METADATA):
    if not 0 <= size <= maximum:
        raise Refusal('bounded allocation')
    result = bytearray()
    while len(result) < size:
        block = stream.read(min(CHUNK, size-len(result)))
        if not block:
            raise Refusal('truncated framing')
        result.extend(block)
    return bytes(result)


def path_name(name):
    while name.startswith('./'):
        name = name[2:]
    if name in ('', '.'):
        return ''
    if len(name.encode('utf-8')) > 1024 or '\\' in name or '\0' in name or name.startswith('/') or any(p in ('', '..') or ':' in p for p in name.rstrip('/').split('/')):
        raise Refusal('unsafe archive path')
    return name.rstrip('/')


def link_target(name, target, hard=False):
    if len(target.encode()) > 1024 or '\\' in target or '\0' in target or ':' in target:
        raise Refusal('unsafe link target')
    parts = [] if hard or target.startswith('/') else list(PurePosixPath(name).parent.parts)
    for part in target.split('/'):
        if part in ('', '.'):
            continue
        if part == '..':
            if not parts:
                raise Refusal('link leaves immutable root')
            parts.pop()
        else:
            parts.append(part)
    return '/'.join(parts)


def octal(field):
    if field and field[0] & 128:
        raise Refusal('unsupported tar base256 integer')
    value = field.rstrip(b'\0 ').lstrip(b' ')
    if value and not re.fullmatch(b'[0-7]+', value):
        raise Refusal('invalid tar octal')
    return int(value or b'0', 8)


def tar_inventory(stream, budget, label):
    rows, seen, pending_pax = [], set(), None
    while True:
        header = exact(stream, 512)
        if header == bytes(512):
            if exact(stream, 512) != bytes(512):
                raise Refusal('tar missing double terminator')
            while True:
                trailing = stream.read(CHUNK)
                if not trailing:
                    break
                if any(trailing):
                    raise Refusal('tar trailing payload')
            if pending_pax is not None:
                raise Refusal('orphan PAX header')
            break
        expected = octal(header[148:156])
        if sum(header[:148]) + 8*32 + sum(header[156:]) != expected:
            raise Refusal('tar checksum')
        if header[257:265] != b'ustar\0' + b'00':
            raise Refusal('unsupported tar dialect')
        size, kind = octal(header[124:136]), header[156:157]
        budget.member(size)
        def text(field):
            return field.split(b'\0', 1)[0].decode('utf-8', 'strict')
        name = text(header[:100])
        prefix = text(header[345:500])
        if prefix:
            name = prefix + '/' + name
        target = text(header[157:257])
        if kind == b'x':
            if pending_pax is not None or size > MAX_METADATA:
                raise Refusal('PAX bound/order')
            body, cursor, attrs = exact(stream, size), 0, {}
            while cursor < len(body):
                space = body.find(b' ', cursor, cursor+16)
                if space < 0 or not body[cursor:space].isdigit():
                    raise Refusal('PAX record length')
                length = int(body[cursor:space]);end = cursor+length
                if length <= space-cursor+2 or end > len(body) or body[end-1:end] != b'\n':
                    raise Refusal('PAX framing')
                key, separator, value = body[space+1:end-1].partition(b'=')
                if not separator or key in attrs or key not in (b'path', b'linkpath', b'mtime', b'atime', b'ctime'):
                    raise Refusal('unsupported PAX feature')
                attrs[key] = value.decode('utf-8', 'strict');cursor = end
            pending_pax = attrs
            if any(exact(stream, (-size) % 512)):
                raise Refusal('tar padding')
            continue
        if pending_pax is not None:
            name = pending_pax.get(b'path', name);target = pending_pax.get(b'linkpath', target);pending_pax = None
        name = path_name(name)
        if PurePosixPath(name).name.startswith('.wh.'):
            raise Refusal('whiteout requires unavailable lower-layer evidence')
        if not name and kind != b'5':
            raise Refusal('empty regular tar path')
        if name in seen:
            raise Refusal('duplicate tar path')
        seen.add(name)
        if kind not in (b'0', b'\0', b'5', b'1', b'2') or kind in (b'5', b'1', b'2') and size:
            raise Refusal('special/sparse/unsupported tar member')
        digest, body, remaining, first_bytes = hashlib.sha256(), bytearray(), size, None
        metadata = (name.endswith('/copyright') or '/common-licenses/' in name or name in ('var/lib/dpkg/status', 'control', 'debian/copyright'))
        if metadata and size > MAX_METADATA:
            raise Refusal('metadata member bound')
        while remaining:
            block = exact(stream, min(CHUNK, remaining), CHUNK);remaining -= len(block);digest.update(block)
            if first_bytes is None:
                first_bytes = block[:4]
            if metadata or first_bytes == b'\x7fELF':
                if len(body)+len(block) > (MAX_METADATA if metadata else MAX_MEMBER):
                    raise Refusal('tar retained metadata/native bound')
                body.extend(block)
        if any(exact(stream, (-size) % 512)):
            raise Refusal('tar padding')
        row = {'path': name, 'type': kind.decode('ascii') if kind != b'\0' else '0', 'bytes': size}
        if kind in (b'0', b'\0'):
            row['sha256'] = digest.hexdigest()
            if first_bytes == b'\x7fELF':
                row['elf'] = elf(body)
        if kind in (b'1', b'2'):
            row.update(target=target, confinedTarget=link_target(name, target, kind == b'1'))
        if metadata and kind in (b'0', b'\0'):
            row['metadataBase64'] = base64.b64encode(body).decode('ascii')
        budget.retain(row)
        rows.append(row)
    indexed = {row['path']: row for row in rows}
    for row in rows:
        if row['type'] not in ('1', '2') or not (row['path'].endswith('/copyright') or '/common-licenses/' in row['path']):
            continue
        target, visited = row['path'], set()
        for _ in range(32):
            if target in visited:
                raise Refusal('archive link cycle')
            visited.add(target)
            parts, replacement = target.split('/'), None
            for end in range(1, len(parts)+1):
                prefix = '/'.join(parts[:end]);entry = indexed.get(prefix)
                if entry and entry['type'] in ('1', '2'):
                    replacement = entry['confinedTarget'] + ('/'+'/'.join(parts[end:]) if end < len(parts) else '')
                    break
            if replacement is None:
                entry = indexed.get(target)
                row['resolvedMetadata'] = {'available': bool(entry and entry.get('metadataBase64') is not None), 'path': target, 'sha256': entry.get('sha256') if entry else None}
                budget.retain(row['resolvedMetadata'])
                break
            target = replacement
        else:
            raise Refusal('archive link depth')
    return {'label': label, 'members': rows}


def elf(data):
    """Read x86_64 LE shared-object dependency tables; never load native code."""
    if len(data) < 64 or data[:7] != b'\x7fELF\x02\x01\x01':
        raise Refusal('unsupported ELF format')
    fields = struct.unpack_from('<HHIQQQIHHHHHH', data, 16)
    e_type, machine, _, _, phoff, _, _, ehsize, phsize, phnum, _, _, _ = fields
    if e_type not in (2, 3) or machine != 62 or ehsize != 64 or phsize != 56 or not 0 < phnum <= 1024 or phoff+phsize*phnum > len(data):
        raise Refusal('ELF header bounds')
    loads, dynamic = [], None
    for index in range(phnum):
        kind, _, offset, vaddr, _, filesz, _, _ = struct.unpack_from('<IIQQQQQQ', data, phoff+index*56)
        if offset+filesz > len(data):
            raise Refusal('ELF segment bounds')
        if kind == 1:
            loads.append((vaddr, offset, filesz))
        if kind == 2:
            if dynamic is not None or filesz > 65536 or filesz % 16:
                raise Refusal('ELF dynamic bounds')
            dynamic = (offset, filesz)
    if dynamic is None:
        raise Refusal('missing ELF dynamic table')
    tags, ended = {}, False
    for offset in range(dynamic[0], dynamic[0]+dynamic[1], 16):
        tag, value = struct.unpack_from('<qQ', data, offset)
        if ended:
            if tag or value:
                raise Refusal('ELF trailing dynamic entry')
            continue
        if tag == 0:
            ended = True;continue
        tags.setdefault(tag, []).append(value)
    if not ended or len(tags.get(5, [])) != 1 or len(tags.get(10, [])) != 1 or tags[10][0] > MAX_METADATA:
        raise Refusal('ELF string table')
    address, size = tags[5][0], tags[10][0]
    matches = [offset+address-vaddr for vaddr, offset, filesz in loads if vaddr <= address and address+size <= vaddr+filesz]
    if len(matches) != 1:
        raise Refusal('ambiguous ELF address mapping')
    strings = data[matches[0]:matches[0]+size]
    def string_at(index):
        end = strings.find(b'\0', index, min(len(strings), index+4097)) if index < len(strings) else -1
        if end < 0:
            raise Refusal('ELF dynamic string bound')
        return strings[index:end].decode('ascii', 'strict')
    return {name: [string_at(value) for value in tags.get(tag, [])] for tag, name in ((1, 'needed'), (14, 'soname'), (15, 'rpath'), (29, 'runpath'))}


def zip_inventory(path, budget, raw_stream=None):
    file_size = path.stat().st_size
    with (nullcontext(raw_stream) if raw_stream is not None else path.open('rb')) as raw:
        tail_size = min(file_size, 65557);raw.seek(file_size-tail_size);tail = raw.read(tail_size)
        position = tail.rfind(b'PK\x05\x06')
        if position < 0 or position+22 != len(tail):
            raise Refusal('ZIP EOCD/comment/trailing bytes')
        _, disk, cd_disk, local_count, count, cd_size, cd_offset, comment = struct.unpack_from('<4sHHHHIIH', tail, position)
        if disk or cd_disk or local_count != count or not 0 < count <= MAX_MEMBERS or cd_size > 16*MIB or count == 65535 or cd_offset+cd_size != file_size-tail_size+position or comment:
            raise Refusal('ZIP central directory bound/ZIP64/multidisk')
        rows, names, intervals, metadata = [], set(), [], {}
        raw.seek(cd_offset)
        scanned = 0
        for _ in range(count):
            header = exact(raw, 46)
            if header[:4] != b'PK\x01\x02':
                raise Refusal('ZIP central record signature')
            name_len, extra_len, comment_len = struct.unpack_from('<HHH', header, 28)
            if name_len > 1024 or extra_len > 4096 or comment_len:
                raise Refusal('ZIP central variable field bound')
            exact(raw, name_len+extra_len)
            scanned += 46+name_len+extra_len
            if scanned > cd_size:
                raise Refusal('ZIP central extent')
        if scanned != cd_size:
            raise Refusal('ZIP hidden central records')
        with zipfile.ZipFile(raw) as archive:
            infos = archive.infolist()
            if len(infos) != count:
                raise Refusal('ZIP entry count')
            for info in infos:
                budget.member(info.file_size)
                name = path_name(info.filename)
                if name.casefold() in names or info.flag_bits & ~0x808 or info.compress_type not in (0, 8):
                    raise Refusal('ZIP duplicate/unsupported flags/method')
                names.add(name.casefold())
                mode = info.external_attr >> 16
                if stat.S_IFMT(mode) not in (0, stat.S_IFREG, stat.S_IFDIR) or info.is_dir() and info.file_size:
                    raise Refusal('ZIP link/special member')
                cursor = 0
                while cursor < len(info.extra):
                    if cursor+4 > len(info.extra):
                        raise Refusal('ZIP extra framing')
                    tag, length = struct.unpack_from('<HH', info.extra, cursor);cursor += 4
                    if tag not in (0x5455, 0x7875) or length > 1024 or cursor+length > len(info.extra):
                        raise Refusal('unsupported ZIP extra/ZIP64')
                    cursor += length
                raw.seek(info.header_offset);header = exact(raw, 30)
                sig, _, flags, method, _, _, crc, compressed, expanded, name_len, extra_len = struct.unpack('<4sHHHHHIIIHH', header)
                encoded = info.orig_filename.encode('utf-8' if flags & 0x800 else 'cp437')
                if sig != b'PK\x03\x04' or flags != info.flag_bits or method != info.compress_type or name_len > 1024 or extra_len > 4096 or exact(raw, name_len) != encoded:
                    raise Refusal('ZIP local/central mismatch')
                if exact(raw, extra_len) != info.extra:
                    raise Refusal('unsupported differing local/central ZIP extras')
                start = info.header_offset+30+name_len+extra_len;end = start+info.compress_size
                if flags & 8:
                    raw.seek(end);descriptor = exact(raw, 12)
                    if descriptor[:4] == b'PK\x07\x08':
                        descriptor = descriptor[4:] + exact(raw, 4);end += 4
                    if struct.unpack('<III', descriptor) != (info.CRC, info.compress_size, info.file_size):
                        raise Refusal('ZIP data descriptor')
                    end += 12
                elif (crc, compressed, expanded) != (info.CRC, info.compress_size, info.file_size):
                    raise Refusal('ZIP local size/hash mismatch')
                if end > cd_offset or start > end:
                    raise Refusal('ZIP member extent')
                intervals.append((info.header_offset, end))
                digest, total, first, body = hashlib.sha256(), 0, b'', bytearray()
                keep_metadata = '.dist-info/' in name or re.search(r'(^|/)(LICENSE[^/]*|COPYING[^/]*|NOTICE[^/]*|METADATA|WHEEL|RECORD)$', name, re.I)
                with archive.open(info) as stream:
                    while True:
                        block = stream.read(CHUNK)
                        if not block:
                            break
                        budget.charge(len(block));total += len(block)
                        if total > info.file_size or total > MAX_MEMBER:
                            raise Refusal('ZIP actual expansion')
                        if not first:
                            first = block[:4]
                        digest.update(block)
                        if keep_metadata or first == b'\x7fELF':
                            if len(body)+len(block) > (MAX_METADATA if keep_metadata else MAX_MEMBER):
                                raise Refusal('ZIP retained metadata/native bound')
                            body.extend(block)
                if total != info.file_size:
                    raise Refusal('ZIP incomplete member')
                row = {'path': name, 'bytes': total, 'sha256': digest.hexdigest()}
                if first == b'\x7fELF':
                    row['elf'] = elf(body)
                if keep_metadata:
                    metadata[name] = bytes(body)
                    row['metadataBase64'] = base64.b64encode(body).decode('ascii')
                if not info.is_dir():
                    budget.retain(row)
                    rows.append(row)
            intervals.sort()
            if intervals[0][0] != 0 or any(a[1] != b[0] for a, b in zip(intervals, intervals[1:])) or intervals[-1][1] != cd_offset:
                raise Refusal('ZIP hidden prefix/gaps/overlap')
    records = [name for name in metadata if name.endswith('.dist-info/RECORD')]
    if len(records) != 1:
        raise Refusal('one wheel RECORD required')
    record_path, observed = records[0], {row['path']: row for row in rows}
    seen = set()
    for record in csv.reader(io.StringIO(metadata[record_path].decode('utf-8', 'strict'))):
        if len(record) != 3 or record[0] in seen or record[0] not in observed:
            raise Refusal('wheel RECORD scope/duplicates')
        name, digest, length = record;seen.add(name)
        if name == record_path:
            if digest or length:
                raise Refusal('wheel RECORD self entry')
            continue
        expected = 'sha256='+base64.urlsafe_b64encode(bytes.fromhex(observed[name]['sha256'])).rstrip(b'=').decode()
        if digest != expected or length != str(observed[name]['bytes']):
            raise Refusal('wheel RECORD hash/size')
    if seen != set(observed):
        raise Refusal('wheel RECORD incomplete')
    return {'kind': 'wheel', 'members': rows, 'compiledFeaturesVerified': False, 'nativeVersionAttestationsVerified': False}


def deb_inventory(stream, budget):
    if exact(stream, 8) != b'!<arch>\n':
        raise Refusal('Debian ar magic')
    rows, seen = [], set()
    while True:
        first = stream.read(1)
        if not first:
            break
        header = first+exact(stream, 59)
        name = header[:16].rstrip(b' ').rstrip(b'/').decode('ascii', 'strict')
        length = header[48:58].strip()
        if header[58:] != b'`\n' or not length.isdigit() or name in seen or name not in ('debian-binary', 'control.tar.gz', 'control.tar.xz', 'control.tar.bz2', 'data.tar.gz', 'data.tar.xz', 'data.tar.bz2'):
            raise Refusal('unsupported/duplicate ar member')
        seen.add(name);size = int(length)
        if size > 256*MIB:
            raise Refusal('ar compressed member bound')
        section = Section(stream, size)
        if name == 'debian-binary':
            if size != 4 or exact(section, 4) != b'2.0\n':
                raise Refusal('Debian format version')
        else:
            rows.append(tar_inventory(Decoded(section, name.rsplit('.', 1)[1], budget), budget, name))
        if section.remaining:
            raise Refusal('ar section incomplete')
        if size % 2 and exact(stream, 1) != b'\n':
            raise Refusal('ar alignment')
    if 'debian-binary' not in seen or sum(name.startswith('control.') for name in seen) != 1 or sum(name.startswith('data.') for name in seen) != 1:
        raise Refusal('Debian incomplete archive')
    return {'kind': 'deb', 'archives': rows, 'controlRelationsAuthenticated': False}


def inspect(item, path, budget):
    regular(path)
    if path.stat().st_size != item['bytes']:
        raise Refusal('input byte identity')
    with locked_input(path) as stream:
        return inspect_locked(item, path, stream, budget)


def inspect_locked(item, path, stream, budget):
    digest = hashlib.sha256()
    if os.fstat(stream.fileno()).st_size != item['bytes']:
        raise Refusal('opened byte identity')
    while True:
        budget.check();block = stream.read(CHUNK)
        if not block:
            break
        digest.update(block)
    if digest.hexdigest() != item['sha256']:
        raise Refusal('input hash identity')
    stream.seek(0)
    if item['kind'] == 'wheel':
        result = zip_inventory(path, budget, stream)
        metadata = [row for row in result['members'] if row['path'].endswith('.dist-info/METADATA') and row.get('metadataBase64')]
        # Wheel METADATA permits repeated classifiers; only identity headers are
        # parsed here. Everything else remains preserved untrusted metadata.
        if len(metadata) != 1:
            raise Refusal('one wheel metadata identity required')
        text = base64.b64decode(metadata[0]['metadataBase64']).decode('utf-8', 'strict').split('\n\n', 1)[0]
        names = re.findall(r'^Name: ([^\r\n]+)', text, re.M)
        versions = re.findall(r'^Version: ([^\r\n]+)', text, re.M)
        expected_name, expected_version = ('pypdfium2', '5.14.0') if item['file'].startswith('pypdfium2-') else ('pillow', '12.3.0')
        if len(names) != 1 or names[0].lower() != expected_name or versions != [expected_version]:
            raise Refusal('wheel exact metadata identity')
        result['versionMetadataClaims'] = {'name': names[0], 'version': versions[0]}
    else:
        result = inspect_tar_or_deb(item, stream, budget)
    result.update(file=item['file'], bytes=item['bytes'], sha256=item['sha256'])
    if item['kind'] == 'deb':
        controls = [row for archive in result['archives'] if archive['label'].startswith('control.') for row in archive['members'] if row['path'] == 'control' and row.get('metadataBase64')]
        if len(controls) != 1:
            raise Refusal('one Debian control required')
        fields = control_records(base64.b64decode(controls[0]['metadataBase64']))
        if len(fields) != 1 or any(fields[0].get(key) != item[expected] for key, expected in (('Package', 'name'), ('Version', 'version'), ('Architecture', 'architecture'))):
            raise Refusal('Debian exact control identity')
        result['controlClaims'] = {key: fields[0].get(key) for key in ('Package', 'Version', 'Architecture', 'Depends', 'Pre-Depends', 'Provides', 'Breaks', 'Conflicts', 'Replaces')}
    return result


def inspect_tar_or_deb(item, stream, budget):
    if item['kind'] == 'deb':
        return deb_inventory(stream, budget)
    if item['kind'] not in ('oci-layer', 'source-patch-metadata'):
        raise Refusal('unsupported fixed input kind')
    compression = 'gz' if item['kind'] == 'oci-layer' else 'xz'
    decoded = Decoded(stream, compression, budget)
    result = tar_inventory(decoded, budget, item['file'])
    if item['kind'] == 'oci-layer':
        if decoded.digest.hexdigest() != BASE_DIFF_ID:
            raise Refusal('OCI uncompressed diff_id mismatch')
        lock_path = regular(Path(__file__).parent/'dependency-lock.json')
        if lock_path.stat().st_size > MIB or sha(lock_path.read_bytes()) != LOCK_SHA256:
            raise Refusal('base inventory lock pin')
        lock = json_read(lock_path, MIB)
        statuses = [row for row in result['members'] if row['path'] == 'var/lib/dpkg/status' and row.get('metadataBase64')]
        if len(statuses) != 1:
            raise Refusal('one actual dpkg status required')
        actual = {}
        for control in control_records(base64.b64decode(statuses[0]['metadataBase64'])):
            if control.get('Status') != 'install ok installed' or control['Package'] in actual:
                raise Refusal('base package status')
            actual[control['Package']] = control['Version']
        if actual != {row['name']: row['version'] for row in lock['base']['packages']}:
            raise Refusal('actual base inventory differs')
        result['basePackageInventoryVerified'] = True
    elif not any(row['path'] == 'debian/copyright' and row.get('metadataBase64') for row in result['members']):
        raise Refusal('exact missing source notice absent')
    return result


def control_records(data):
    if len(data) > MAX_METADATA:
        raise Refusal('control metadata bound')
    records, fields, previous = [], {}, None
    for line in data.decode('utf-8', 'strict').splitlines()+['']:
        if not line:
            if fields:
                records.append(fields);fields = {};previous = None
        elif line[:1] in (' ', '\t'):
            if previous is None:
                raise Refusal('orphan control continuation')
            fields[previous] += '\n'+line
        else:
            key, separator, value = line.partition(':')
            if not separator or not re.fullmatch('[A-Za-z0-9-]{1,64}', key) or key in fields:
                raise Refusal('control duplicate/field syntax')
            fields[key] = value.lstrip();previous = key
    return records


def prior_acquisition(source, batch):
    settled = json_read(source/'controller-receipt.json')
    prior_path = regular(source/'worker-result.json')
    prior = json_read(prior_path)
    if settled.get('state') != 'completed_owned_worker_retired' or settled.get('phase') != 'acquire' or settled.get('batch') != batch or settled.get('manifestSha256') != MANIFEST_SHA256 or not settled.get('treeSettled') or not settled.get('pdhClosed') or prior.get('state') != 'all_transfers_exactly_verified' or prior.get('manifestSha256') != MANIFEST_SHA256 or prior.get('batch') != batch or settled.get('workerResultSha256') != sha(prior_path.read_bytes()):
        raise Refusal('prior acquisition/cleanup unavailable')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--phase', required=True, choices=('acquire', 'inspect'))
    parser.add_argument('--batch', required=True, choices=('primary', 'wheels', 'base-layer', 'missing-notice', 'debian-controls'))
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--input', type=Path)
    parser.add_argument('--deadline', type=float, required=True)
    args = parser.parse_args()
    output = regular(args.output, directory=True)
    intent = json_read(output / 'intent.json')
    if intent.get('phase') != args.phase or intent.get('batch') != args.batch or intent.get('manifestSha256') != MANIFEST_SHA256 or intent.get('workerSha256') != sha(Path(__file__).read_bytes()):
        raise Refusal('owned intent mismatch')
    if not time.monotonic() < args.deadline <= time.monotonic()+300:
        raise Refusal('finite work deadline')
    items, budget = load_manifest(args.batch), Budget(args.deadline)
    receipt = {'version': 1, 'phase': args.phase, 'batch': args.batch, 'manifestSha256': MANIFEST_SHA256, 'state': 'incomplete', 'inputs': [], 'sbomClosed': False, 'authenticated': False}
    try:
        if args.phase == 'inspect':
            if args.input is None:
                raise Refusal('prior positively settled acquisition required')
            source = regular(args.input, directory=True)
            prior_acquisition(source, args.batch)
        elif args.input is not None:
            raise Refusal('acquisition cannot accept arbitrary input')
        for item in items:
            budget.check()
            receipt['inputs'].append(acquire(item, output, budget) if args.phase == 'acquire' else inspect(item, source/item['file'], budget))
            # Bound accumulated material before the next input, not only final write.
            if len(json.dumps(receipt, separators=(',', ':'), ensure_ascii=True)) > MAX_RESULT-4096:
                raise Refusal('aggregate result byte bound')
        receipt['state'] = 'all_transfers_exactly_verified' if args.phase == 'acquire' else 'static_inspection_complete_with_explicit_evidence_gaps'
        receipt['expandedBytes'], receipt['memberCount'] = budget.expanded, budget.members
    except BaseException as error:
        receipt['state'], receipt['errorType'] = 'incomplete_no_replay', type(error).__name__
        # Never reflect URLs, auth headers, server text, member text or secrets in errors.
        receipt['inputs'] = [{'file': row['file'], 'sha256': row['sha256'], 'bytes': row['bytes']} for row in receipt['inputs']]
        write_result(output/'worker-result.json', receipt)
        raise SystemExit(2) from None
    write_result(output/'worker-result.json', receipt)


if __name__ == '__main__':
    main()
