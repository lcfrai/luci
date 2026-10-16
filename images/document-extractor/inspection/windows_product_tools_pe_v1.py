"""Finite PE metadata observations only; never load or execute an image."""
import struct


def inspect_pe_v1(data, budget):
    budget.check()
    if not isinstance(data, bytes) or not 64 <= len(data) <= 16 << 20:
        budget.fail('pe_object_bound')

    def read(offset, size):
        budget.check()
        if offset < 0 or size < 0 or size > 65536 or offset > len(data) - size:
            budget.fail('pe_file_extent')
        budget.allocate(size, 65536)
        return data[offset:offset + size]

    def number(offset, fmt):
        return struct.unpack(fmt, read(offset, struct.calcsize(fmt)))[0]

    if read(0, 2) != b'MZ':
        budget.fail('pe_dos_signature')
    pe = number(60, '<I')
    if not 64 <= pe <= (1 << 20) or read(pe, 4) != b'PE\0\0':
        budget.fail('pe_signature')
    machine, count = struct.unpack('<HH', read(pe + 4, 4))
    optional_size = number(pe + 20, '<H')
    if machine not in (0x14c, 0x8664) or not 1 <= count <= 96 or optional_size > 4096:
        budget.fail('pe_coff')
    optional, table = pe + 24, pe + 24 + optional_size
    magic = number(optional, '<H')
    if (machine, magic) not in ((0x14c, 0x10b), (0x8664, 0x20b)):
        budget.fail('pe_machine_magic')
    fixed = 96 if magic == 0x10b else 112
    if optional_size < fixed or table + 40 * count > (1 << 20):
        budget.fail('pe_header_bound')
    read(optional, optional_size)
    directory_count = number(optional + fixed - 4, '<I')
    if directory_count > 16 or fixed + 8 * directory_count > optional_size:
        budget.fail('pe_directory_count')
    headers = number(optional + 60, '<I')
    if not table + 40 * count <= headers <= len(data):
        budget.fail('pe_headers_extent')
    sections, raw_spans, virtual_spans = [], [], []
    for index in range(count):
        header = read(table + 40 * index, 40)
        virtual_size, address, raw_size, raw = struct.unpack_from('<IIII', header, 8)
        if raw_size and (raw < headers or raw > len(data) - raw_size):
            budget.fail('pe_section_file_extent')
        mapped_size = max(virtual_size, raw_size)
        if address + mapped_size > 0x100000000:
            budget.fail('pe_section_rva_overflow')
        if raw_size:
            raw_spans.append((raw, raw + raw_size))
        if mapped_size:
            virtual_spans.append((address, address + mapped_size))
        sections.append((address, raw_size, raw))
    for spans in (raw_spans, virtual_spans):
        spans.sort()
        if any(a[1] > b[0] for a, b in zip(spans, spans[1:])):
            budget.fail('pe_overlapping_sections')

    def rva(address, size):
        if address <= 0 or size <= 0 or address + size > 0x100000000:
            budget.fail('pe_rva_extent')
        matches = [(raw + address - base, raw_size - (address - base))
                   for base, raw_size, raw in sections
                   if base <= address and address + size <= base + raw_size]
        if len(matches) != 1:
            budget.fail('pe_rva_not_unique_file_backed')
        return matches[0]

    def dll_name(address):
        offset, remaining = rva(address, 1)
        raw = read(offset, min(129, remaining))
        end = raw.find(b'\0')
        if not 1 <= end <= 128:
            budget.fail('pe_import_name_bound')
        value = raw[:end]
        if any(c < 33 or c > 126 for c in value) or any(c in value for c in b'/\\:') or not value.lower().endswith(b'.dll'):
            budget.fail('pe_import_name')
        return value.decode('ascii')

    def directory(index):
        if index >= directory_count:
            return 0, 0
        address, size = struct.unpack('<II', read(optional + fixed + 8 * index, 8))
        if bool(address) != bool(size):
            budget.fail('pe_directory_pair')
        return address, size

    imports, delay_imports = [], []
    for directory_index, width, names in ((1, 20, imports), (13, 32, delay_imports)):
        address, size = directory(directory_index)
        if not address:
            continue
        if size < width or size > 1 << 20:
            budget.fail('pe_import_table_bound')
        offset, _ = rva(address, size)
        terminated = False
        for index in range(129):
            if (index + 1) * width > size:
                break
            descriptor = read(offset + index * width, width)
            if descriptor == bytes(width):
                terminated = True
                break
            if len(imports) + len(delay_imports) >= 128:
                budget.fail('pe_import_count')
            fields = struct.unpack('<' + 'I' * (width // 4), descriptor)
            if directory_index == 1:
                name_address, thunk = fields[3], fields[4]
            else:
                # VA-based delay descriptors require a separate qualified arm.
                if fields[0] != 1:
                    budget.fail('pe_delay_va_unsupported')
                name_address, thunk = fields[1], fields[3]
            rva(thunk, 8 if magic == 0x20b else 4)
            name = dll_name(name_address)
            if name.casefold() in {v.casefold() for v in imports + delay_imports}:
                budget.fail('pe_duplicate_import')
            names.append(name)
        if not terminated:
            budget.fail('pe_import_terminator')
    # CLR/mixed managed images are outside this native-tool profile.
    if directory(14) != (0, 0):
        budget.fail('pe_managed_unsupported')
    certificate, certificate_size = directory(4)  # FILE OFFSET, never an RVA.
    if certificate and (certificate % 8 or certificate_size < 8 or certificate > len(data) - certificate_size):
        budget.fail('pe_certificate_file_extent')
    end = max([headers] + [stop for _, stop in raw_spans])
    if certificate:
        if certificate < end:
            budget.fail('pe_certificate_overlaps_image')
        # Certificate bytes are opaque; no Authenticode/catalog/trust service.
        occupied_end = certificate + certificate_size
    else:
        occupied_end = end
    return {'machine': machine, 'peMagic': magic, 'sections': count,
            'imports': imports, 'delayImports': delay_imports,
            'certificateFileOffset': certificate, 'certificateBytes': certificate_size,
            'imageFileEnd': end, 'overlayFileOffset': end,
            'overlayBytes': (certificate if certificate else len(data)) - end,
            'trailingFileOffset': occupied_end,
            'trailingBytes': len(data) - occupied_end,
            'requiredFunctionExportsProven': False, 'authenticated': False}
