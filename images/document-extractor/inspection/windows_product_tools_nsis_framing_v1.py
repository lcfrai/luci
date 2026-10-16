"""Finite current-NSIS firstheader shape only; no variant or decoder authority."""
import hashlib
import struct


def inspect_nsis_framing_v1(data, pe, budget):
    budget.check()
    if not isinstance(data, bytes) or len(data) != 5804864:
        budget.fail('nsis_fixed_original_size')
    start, size = pe['overlayFileOffset'], pe['overlayBytes']
    if not isinstance(start, int) or not isinstance(size, int) or start < 0 or size < 28 or start > len(data) - size or pe['trailingBytes'] != 0:
        budget.fail('nsis_overlay_extent')
    end = start + size
    certificate = pe['certificateFileOffset']
    if end != (certificate if certificate else len(data)) or start != pe['imageFileEnd']:
        budget.fail('nsis_overlay_boundary')
    first = ((start + 511) // 512) * 512
    sites = max(0, (end - 28 - first) // 512 + 1)
    if sites > 11338:
        budget.fail('nsis_site_bound')
    candidate = None
    for offset in range(first, end - 27, 512):
        budget.check()  # Finite in-memory sites, not additional original reads.
        header = data[offset:offset + 28]
        flags, signature, a, b, c, header_bytes, following_bytes = struct.unpack('<7I', header)
        if (signature, a, b, c) != (0xDEADBEEF, 0x6C6C754E, 0x74666F73, 0x74736E49):
            continue
        # No uninstall, unknown bits or contradictory no-CRC/forced-CRC arm.
        if flags & ~15 or flags & 1 or flags & 4 and flags & 8:
            budget.fail('nsis_unsupported_flags')
        crc_bytes = 0 if flags & 4 else 4
        if not 1 <= header_bytes <= 1048576 or not 28 + crc_bytes + 4 <= following_bytes <= 0x7fffffff:
            budget.fail('nsis_declared_length')
        # NSIS following length INCLUDES this28-byte header and possible CRC.
        if following_bytes > end - offset or offset + following_bytes != end:
            budget.fail('nsis_following_extent')
        if candidate is not None:
            budget.fail('nsis_ambiguous_firstheader')
        payload_start, payload_end = offset + 28, end - crc_bytes
        prefix = data[payload_start:min(payload_start + 32, payload_end)]
        word = struct.unpack('<I', prefix[:4])[0]
        low_length = word & 0x7fffffff
        candidate = {
            'firstheaderFileOffset': offset, 'firstheaderBytes': 28,
            'firstheaderSHA256': hashlib.sha256(header).hexdigest(),
            'flags': flags, 'declaredHeaderBytes': header_bytes,
            'followingBytesIncludingFirstheaderAndCRC': following_bytes,
            'dataFileOffset': payload_start, 'dataBytes': payload_end - payload_start,
            'possibleCRCFileOffset': payload_end if crc_bytes else 0,
            'possibleCRCBytes': crc_bytes, 'crcVerified': False,
            'prefixBytes': len(prefix), 'prefixHex': prefix.hex(),
            'prefixSHA256': hashlib.sha256(prefix).hexdigest(),
            'possibleLengthWord': word, 'possibleLengthHighBit': bool(word & 0x80000000),
            'possibleLengthLow31': low_length,
            'possibleRecordExtentFits': low_length <= payload_end - payload_start - 4,
        }
    if candidate is None:
        budget.fail('nsis_firstheader_absent')
    candidate.update(scanSites=sites, structuralFirstheaderIdentified=True,
                     variantIdentified=False, codecIdentified=False,
                     payloadDecoded=False, authenticated=False,
                     productProfileAvailable=False)
    budget.allocate(4096, 65536)
    return candidate
