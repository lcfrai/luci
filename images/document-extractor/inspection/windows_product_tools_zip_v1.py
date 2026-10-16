"""Non-wheel outer ZIP inventory. No extraction, recursive ZIP or input imports."""
import base64
import hashlib
import stat
import struct
import unicodedata
import zlib


def safe_name(raw, flags, budget):
    if not 1 <= len(raw) <= 1024 or any(c < 32 or c == 127 for c in raw):
        budget.fail('zip_raw_name')
    try:
        name = raw.decode('utf-8' if flags & 0x800 else 'cp437', 'strict')
    except UnicodeError:
        budget.fail('zip_name_encoding')
    if name.startswith('/') or '\\' in name or ':' in name or '\x00' in name:
        budget.fail('zip_path')
    parts = name.rstrip('/').split('/')
    reserved = {'con', 'prn', 'aux', 'nul'} | {'com' + str(i) for i in range(1, 10)} | {'lpt' + str(i) for i in range(1, 10)}
    if any(p in ('', '.', '..') or p.endswith(('.', ' ')) or p.split('.')[0].casefold() in reserved for p in parts) or any(c in name for c in '<>"|?*') or any(unicodedata.category(c).startswith('C') for c in name):
        budget.fail('zip_path')
    return name


def extras(raw, budget):
    cursor = 0
    while cursor < len(raw):
        if cursor + 4 > len(raw):
            budget.fail('zip_extra_frame')
        tag, size = struct.unpack_from('<HH', raw, cursor)
        cursor += 4
        if tag not in (0x5455, 0x7875) or size > 1024 or cursor + size > len(raw):
            budget.fail('zip_extra_unsupported')
        cursor += size


def inspect_cpython_embed_v1(original, wrapper, budget, pe_inspector):
    size = wrapper['bytes']
    tail_size = min(size, 65557)
    tail = original.exact_at(size - tail_size, tail_size)
    position = tail.rfind(b'PK\x05\x06')
    if position < 0 or position + 22 != len(tail):
        budget.fail('zip_eocd')
    _, disk, cd_disk, local_count, count, cd_size, cd_offset, comment = struct.unpack_from('<4sHHHHIIH', tail, position)
    if disk or cd_disk or local_count != count or not 1 <= count <= 211 or comment or cd_size > 1 << 20 or cd_offset + cd_size != size - tail_size + position:
        budget.fail('zip_directory_bound')
    central = original.exact_at(cd_offset, cd_size)
    cursor, header_bytes, members, spans, names, native_count, notice_total = 0, 0, [], [], set(), 0, 0
    for _ in range(count):
        budget.check()
        if cursor + 46 > len(central) or central[cursor:cursor + 4] != b'PK\x01\x02':
            budget.fail('zip_central_frame')
        fields = struct.unpack_from('<4s6H3I5H2I', central, cursor)
        flags, method, crc, compressed, expanded = fields[3], fields[4], fields[7], fields[8], fields[9]
        name_size, extra_size, comment_size, start_disk = fields[10:14]
        attributes, local = fields[15], fields[16]
        stop = cursor + 46 + name_size + extra_size + comment_size
        if flags & ~0x808 or method not in (0, 8) or extra_size > 4096 or comment_size or start_disk or stop > len(central):
            budget.fail('zip_central_fields')
        raw_name = central[cursor + 46:cursor + 46 + name_size]
        raw_extra = central[cursor + 46 + name_size:stop]
        name = safe_name(raw_name, flags, budget)
        extras(raw_extra, budget)
        if name.casefold() in names:
            budget.fail('zip_case_alias')
        names.add(name.casefold())
        mode = attributes >> 16
        directory = name.endswith('/')
        if stat.S_IFMT(mode) not in (0, stat.S_IFREG, stat.S_IFDIR) or directory and expanded or attributes & 0x400 or attributes & 0x10 and not directory:
            budget.fail('zip_special_member')
        if expanded > 16 << 20 or compressed > size or local >= cd_offset:
            budget.fail('zip_member_bound')
        local_header = original.exact_at(local, 30)
        sig, _, local_flags, local_method, _, _, local_crc, local_compressed, local_expanded, ln, le = struct.unpack('<4s5H3I2H', local_header)
        if sig != b'PK\x03\x04' or (local_flags, local_method, ln, le) != (flags, method, name_size, extra_size):
            budget.fail('zip_local_fields')
        local_variable = original.exact_at(local + 30, ln + le)
        # Raw local names are compared before any ZipInfo-style normalization.
        if local_variable[:ln] != raw_name or local_variable[ln:] != raw_extra:
            budget.fail('zip_raw_local_central_name')
        header_bytes += 76 + 2 * (ln + le)
        if header_bytes > 2 << 20:
            budget.fail('zip_header_aggregate')
        data_start, data_end = local + 30 + ln + le, local + 30 + ln + le + compressed
        if data_end > cd_offset:
            budget.fail('zip_data_extent')
        end = data_end
        if flags & 8:
            descriptor = original.exact_at(end, 12)
            if descriptor[:4] == b'PK\x07\x08':
                descriptor = descriptor[4:] + original.exact_at(end + 12, 4)
                end += 4
            if struct.unpack('<III', descriptor) != (crc, compressed, expanded):
                budget.fail('zip_data_descriptor')
            end += 12
        elif (local_crc, local_compressed, local_expanded) != (crc, compressed, expanded):
            budget.fail('zip_local_size_crc')
        if end > cd_offset:
            budget.fail('zip_descriptor_extent')
        spans.append((local, end))
        native = name.lower().endswith(('.exe', '.dll', '.pyd'))
        notice = name.rsplit('/', 1)[-1].lower().startswith(('license', 'copying', 'notice'))
        if native:
            native_count += 1
            if native_count > 128:
                budget.fail('zip_native_count')
        if notice and (expanded > 262144 or notice_total + expanded > 1 << 20):
            budget.fail('zip_notice_bound')
        keep = native or notice
        body, total, checksum, digest, first = bytearray(), 0, 0, hashlib.sha256(), b''
        decoder = zlib.decompressobj(-15) if method == 8 else None

        def emit(block):
            nonlocal total, checksum, first
            total += len(block)
            if total > expanded:
                budget.fail('zip_expanded_identity')
            if len(first) < 2:
                first += block[:2 - len(first)]
            checksum = zlib.crc32(block, checksum)
            digest.update(block)
            if keep:
                budget.allocate(len(body) + len(block), 16 << 20)
                body.extend(block)

        consumed, pending = 0, b''
        while consumed < compressed or pending:
            budget.check()
            if not pending:
                amount = min(65536, compressed - consumed)
                pending = original.exact_at(data_start + consumed, amount)
                consumed += amount
            allowance = min(65536, expanded - total + 1)
            ticket = budget.reserve_expansion(allowance)
            try:
                if decoder is None:
                    block, pending = pending, b''
                else:
                    block = decoder.decompress(pending, allowance)
                    pending = decoder.unconsumed_tail
                    if decoder.unused_data:
                        budget.fail('zip_deflate_trailing')
                budget.finish_expansion(ticket, len(block))
            except BaseException:
                budget.failed = True
                raise
            emit(block)
        if total != expanded or checksum != crc or decoder is not None and not decoder.eof:
            budget.fail('zip_member_eof_crc')
        if first == b'MZ' and not native:
            budget.fail('zip_unclassified_native')
        row = {'path': name, 'directory': directory, 'bytes': total, 'sha256': digest.hexdigest(), 'crc32': checksum}
        if native:
            row['pe'] = pe_inspector(bytes(body), budget)
        if notice:
            notice_total += total
            row['noticeBase64'] = base64.b64encode(body).decode('ascii')
        if name.lower().endswith('.zip'):
            row['nestedArchive'] = 'opaque_not_decoded'
        budget.retain(row)
        members.append(row)
        cursor = stop
    if cursor != len(central):
        budget.fail('zip_hidden_central_records')
    spans.sort()
    if not spans or spans[0][0] != 0 or any(a[1] != b[0] for a, b in zip(spans, spans[1:])) or spans[-1][1] != cd_offset:
        budget.fail('zip_hidden_gap_overlap')
    if not any(v['path'] == 'python.exe' and 'pe' in v for v in members):
        budget.fail('zip_python_entry_absent')
    return {'members': members, 'memberCount': count,
            'nestedStdlibDecoded': False, 'authenticated': False}
