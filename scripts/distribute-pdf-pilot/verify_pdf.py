#!/usr/bin/env python3
"""Independent, bounded reader for the pilot's fixed PDF dialect only.

This is deliberately separate from the Go writer. It checks xref/object and
font structure, follows the embedded ToUnicode map and extracts the original
text from the content stream. A standard-reader/visual check is still required
before any production PDF route can be enabled.
"""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re

MAX_PDF = 16 << 20
TEXT = "日本語"
STAGE = re.compile(r"pilot-[0-9a-f]{32}\Z")


def refuse(condition, reason):
    if not condition:
        raise ValueError(reason)


def parse_objects(pdf):
    marker = re.search(rb"startxref\n([0-9]+)\n%%EOF\n\Z", pdf)
    refuse(marker is not None, "missing exact PDF trailer")
    xref = int(marker[1])
    refuse(0 < xref < len(pdf), "xref offset outside PDF")
    lines = pdf[xref:].splitlines()
    refuse(len(lines) >= 13 and lines[:3] == [b"xref", b"0 10", b"0000000000 65535 f "], "invalid exact xref")
    offsets = []
    for row in lines[3:12]:
        refuse(re.fullmatch(rb"[0-9]{10} 00000 n ", row) is not None, "invalid xref entry")
        offsets.append(int(row[:10]))
    refuse(len(offsets) == 9 and offsets == sorted(offsets) and len(set(offsets)) == 9, "xref object set changed")
    objects = {}
    for index, start in enumerate(offsets, 1):
        end = offsets[index] if index < 9 else xref
        header = f"{index} 0 obj\n".encode()
        refuse(start < end and pdf[start:end].startswith(header) and pdf[start:end].endswith(b"endobj\n"), "xref points outside exact object")
        objects[index] = pdf[start + len(header):end]
    return objects


def stream(body, opentype=False):
    prefix = rb"<< /Length ([0-9]+)" + (rb" /Subtype /OpenType" if opentype else rb"") + rb" >>\nstream\n"
    match = re.match(prefix, body)
    refuse(match is not None, "stream dictionary differs")
    count = int(match[1])
    refuse(0 < count <= MAX_PDF, "stream length outside bound")
    content = body[match.end():match.end() + count]
    refuse(len(content) == count and body[match.end() + count:] == b"\nendstream\nendobj\n", "stream boundary differs")
    return content


def font_tables(font):
    refuse(len(font) >= 12 and font[:4] == b"OTTO", "embedded font is not OpenType CFF")
    n = int.from_bytes(font[4:6], "big")
    refuse(1 <= n <= 128 and 12 + 16*n <= len(font), "font table directory is invalid")
    tags = set()
    for i in range(n):
        row = font[12 + 16*i:28 + 16*i]
        tag, offset, size = row[:4], int.from_bytes(row[8:12], "big"), int.from_bytes(row[12:16], "big")
        refuse(tag not in tags and offset <= len(font) and 0 < size <= len(font) - offset, "font table is duplicate or outside program")
        tags.add(tag)
    refuse(b"CFF " in tags and b"CFF2" not in tags and b"glyf" not in tags, "embedded subset has wrong outline format")
    return len(tags)


def verify(pdf):
    refuse(0 < len(pdf) <= MAX_PDF and pdf.startswith(b"%PDF-1.7\n"), "PDF version or byte bound differs")
    objects = parse_objects(pdf)
    refuse(b"/Subtype /Type0" in objects[4] and b"/Encoding /Identity-H" in objects[4] and b"/ToUnicode 7 0 R" in objects[4], "Type0/ToUnicode route missing")
    refuse(b"/Subtype /CIDFontType0" in objects[5] and b"/FontDescriptor 6 0 R" in objects[5] and b"/CIDToGIDMap" not in objects[5], "CFF descendant differs")
    refuse(b"/FontFile3 9 0 R" in objects[6] and b"/FontFile2" not in objects[6], "CFF font descriptor differs")
    cmap, page, font = stream(objects[7]), stream(objects[8]), stream(objects[9], opentype=True)
    count = font_tables(font)
    block = re.search(rb"3 beginbfchar\n(.*?)endbfchar\n", cmap, re.DOTALL)
    refuse(block is not None, "ToUnicode map lacks three fixed entries")
    pairs = re.findall(rb"<([0-9A-F]{4})> <([0-9A-F]{4,8})>\n", block[1])
    refuse(len(pairs) == 3 and b"".join(b"<" + a + b"> <" + b + b">\n" for a, b in pairs) == block[1], "ToUnicode entries are malformed")
    mapping = {int(a, 16): bytes.fromhex(b.decode()).decode("utf-16-be") for a, b in pairs}
    match = re.search(rb"<([0-9A-F]+)> Tj\n", page)
    refuse(match is not None and len(match[1]) == 12 and page.count(b" Tj") == 1, "page text is not three exact CIDs")
    codes = bytes.fromhex(match[1].decode())
    extracted = "".join(mapping[int.from_bytes(codes[i:i+2], "big")] for i in range(0, len(codes), 2))
    refuse(extracted == TEXT, "independent ToUnicode extraction changed original text")
    return {
        "version": 1,
        "source": "fixed-dialect-independent-python-reader",
        "pdfSHA256": hashlib.sha256(pdf).hexdigest(),
        "pdfBytes": len(pdf),
        "embeddedOpenTypeSHA256": hashlib.sha256(font).hexdigest(),
        "embeddedOpenTypeBytes": len(font),
        "embeddedCFFTableCount": count,
        "extractedTextSHA256": hashlib.sha256(TEXT.encode()).hexdigest(),
        "extractedCodepoints": len(TEXT),
        "xrefObjects": len(objects),
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--out-dir", type=Path, required=True)
    args = parser.parse_args()
    stage = args.out_dir
    refuse(stage.is_absolute() and STAGE.fullmatch(stage.name) is not None, "stage identity differs")
    refuse(stage.is_dir() and not stage.is_symlink(), "owned stage missing or linked")
    target = stage / "pilot.pdf"
    refuse(target.is_file() and not target.is_symlink() and target.stat().st_size <= MAX_PDF, "pilot PDF missing or oversized")
    result = verify(target.read_bytes())
    payload = json.dumps(result, sort_keys=True, separators=(",", ":")).encode() + b"\n"
    receipt = stage / "verification.json"
    flags = os.O_WRONLY | os.O_CREAT | os.O_EXCL | getattr(os, "O_NOFOLLOW", 0)
    fd = os.open(receipt, flags, 0o600)
    with os.fdopen(fd, "wb") as handle:
        handle.write(payload)
        handle.flush()
        os.fsync(handle.fileno())


if __name__ == "__main__":
    main()
