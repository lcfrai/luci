"""Fixed offline entrypoint for Luci's contained document evidence profile.

Only the positively owned Docker adapter may invoke this entrypoint. The host
controller enforces the cgroup and whole deadline, then verifies retirement.
No network, model, user command, JavaScript or form environment is used.
"""
import argparse
import csv
import hashlib
import io
import json
import math
import os
from pathlib import Path
import re
import subprocess
import struct
import sys
import time

PROFILE = "pdfium-tesseract-v1"
VERSION = "luci-document-v1"
MAX_ORIGINAL = 10 * 1024 * 1024
MAX_RESULT = 1024 * 1024
MAX_SEGMENTS = 10000
MAX_PAGE_PIXELS = 4 * 1024 * 1024
MAX_TOTAL_PIXELS = 64 * 1024 * 1024
PAGE_SECONDS = 30
WHOLE_SECONDS = 120


class EvidenceFailure(Exception):
    def __init__(self, status, message):
        self.status, self.message = status, message


def digest(data):
    return hashlib.sha256(data).hexdigest()


def encoded(value):
    return json.dumps(value, ensure_ascii=False, allow_nan=False, separators=(",", ":")).encode("utf-8")


def result_frame(value):
    raw = encoded(value)
    if not 1 <= len(raw) <= MAX_RESULT:
        raise EvidenceFailure("over-limit", "The complete attributed stream exceeds 1 MiB.")
    return b"LUCI-DOCUMENT-RESULT/1\n" + struct.pack(">I", len(raw)) + hashlib.sha256(raw).digest() + raw + b"\nLUCI-DOCUMENT-END/1\n"


def read_regular(path, maximum):
    descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    with os.fdopen(descriptor, "rb") as stream:
        import stat
        size = os.fstat(stream.fileno()).st_size
        if not stat.S_ISREG(os.fstat(stream.fileno()).st_mode) or size > maximum:
            raise EvidenceFailure("over-limit", "The complete private file exceeds its permitted bound.")
        data = stream.read(maximum + 1)
        if len(data) != size or len(data) > maximum:
            raise EvidenceFailure("corrupt", "The complete private file could not be verified.")
        return data


def original_bytes(args):
    data = read_regular(args.original, MAX_ORIGINAL)
    if len(data) != args.bytes or digest(data) != args.digest:
        raise EvidenceFailure("corrupt", "The original does not match its immutable private digest.")
    expected = {"application/pdf": b"%PDF-", "image/png": b"\x89PNG\r\n\x1a\n", "image/jpeg": b"\xff\xd8\xff"}
    if args.mime not in expected or not data.startswith(expected[args.mime]):
        raise EvidenceFailure("unsupported", "The fixed extractor does not support this original format.")
    return data


def dependencies():
    try:
        import pypdfium2 as pdfium
        from PIL import Image, ImageOps
        if pdfium.PYPDFIUM_INFO.tag != "5.14.0" or pdfium.PDFIUM_INFO.flags:
            raise EvidenceFailure("dependency-unavailable", "The pinned non-V8/non-XFA PDFium build is unavailable.")
        if Image.__version__ != "12.3.0":
            raise EvidenceFailure("dependency-unavailable", "The pinned raster decoder is unavailable.")
        Image.MAX_IMAGE_PIXELS = MAX_PAGE_PIXELS
        return pdfium, Image, ImageOps
    except ImportError:
        raise EvidenceFailure("dependency-unavailable", "The pinned offline PDF/raster dependencies are unavailable.") from None


def page_count(args, data, pdfium, Image):
    if args.mime == "application/pdf":
        try:
            with pdfium.PdfDocument(data) as document:
                count = len(document)
        except pdfium.PdfiumError as error:
            if error.err_code == 4:
                raise EvidenceFailure("encrypted", "The document requires a password. No password was requested or read.") from None
            raise EvidenceFailure("corrupt", "PDFium could not read the complete document.") from None
        if count < 1 or count > 50:
            raise EvidenceFailure("over-limit", "Use a PDF with 1 to 50 pages; no truncated page set is accepted.")
        return count
    with Image.open(io.BytesIO(data)) as image:
        if image.format not in ("PNG", "JPEG") or getattr(image, "n_frames", 1) != 1:
            raise EvidenceFailure("unsupported", "Only single-frame PNG and JPEG originals are supported.")
        checked_pixels(*image.size)
    return 1


def checked_pixels(width, height):
    if width < 1 or height < 1 or width > 4096 or height > 4096 or width * height > MAX_PAGE_PIXELS:
        raise EvidenceFailure("over-limit", "A decoded page exceeds 4 MP or the 4096-pixel side limit.")


def segment(page, kind, index, text, rectangle, confidence=None):
    if "\0" in text or len(text.encode("utf-8")) > 64 * 1024:
        raise EvidenceFailure("over-limit", "A complete source segment exceeds its text bound.")
    return {"id": f"p{page}-{kind}-{index}", "kind": kind, "text": text, "textDigest": digest(text.encode("utf-8")),
            "location": {**rectangle, "firstLine": 0, "lastLine": 0}, "confidence": confidence,
            "uncertainty": "OCR is an attributed recognition claim, including possible orientation and duplicate-text errors; inspect the original." if kind == "ocr" else ""}


def ocr(path, page, width, height):
    output = "/tmp/ocr-page"
    process = subprocess.Popen(["/usr/bin/tesseract", path, output, "-l", "eng", "--psm", "1", "tsv"],
                               stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    try:
        process.wait(timeout=PAGE_SECONDS)
    except subprocess.TimeoutExpired:
        process.kill()
        process.wait(timeout=1)
        raise EvidenceFailure("over-limit", "The owned OCR process exceeded its page deadline.") from None
    if process.returncode != 0:
        raise EvidenceFailure("failed", "The offline OCR process did not produce a complete receipt.")
    raw = read_regular(output + ".tsv", MAX_RESULT)
    rows = csv.DictReader(io.StringIO(raw.decode("utf-8", errors="strict")), delimiter="\t")
    values = []
    for row in rows:
        if row.get("level") != "5" or not row.get("text", "").strip():
            continue
        if len(values) >= MAX_SEGMENTS:
            raise EvidenceFailure("over-limit", "OCR exceeds the complete source segment budget.")
        left, top, box_width, box_height = (int(row[key]) for key in ("left", "top", "width", "height"))
        confidence = float(row["conf"])
        if not math.isfinite(confidence) or confidence < 0 or confidence > 100 or min(left, top, box_width, box_height) < 0 or left + box_width > width or top + box_height > height:
            raise EvidenceFailure("corrupt", "OCR returned invalid confidence or source geometry.")
        x, y = round(left * 10000 / width), round(top * 10000 / height)
        right, bottom = round((left + box_width) * 10000 / width), round((top + box_height) * 10000 / height)
        values.append(segment(page, "ocr", len(values) + 1, row["text"], {"x": x, "y": y, "width": right - x, "height": bottom - y}, round(confidence)))
    return values


def extract_page(args):
    data = original_bytes(args)
    pdfium, Image, ImageOps = dependencies()
    text_segments = []
    bitmap = image = None
    document = page = textpage = None
    try:
        if args.mime == "application/pdf":
            document = pdfium.PdfDocument(data)
            if args.page < 1 or args.page > len(document) or len(document) > 50:
                raise EvidenceFailure("corrupt", "The exact source page is unavailable.")
            page = document[args.page - 1]
            width, height = page.get_size()
            if not math.isfinite(width) or not math.isfinite(height) or min(width, height) <= 0:
                raise EvidenceFailure("corrupt", "The PDF page dimensions are invalid.")
            textpage = page.get_textpage()
            if textpage.count_chars() > 64 * 1024:
                raise EvidenceFailure("over-limit", "The complete PDF page text exceeds its character bound.")
            text = textpage.get_text_bounded(errors="strict")
            if len(text.encode("utf-8")) > 64 * 1024:
                raise EvidenceFailure("over-limit", "The complete PDF page text exceeds 64 KiB.")
            for line in text.replace("\r\n", "\n").split("\n"):
                if line.strip():
                    text_segments.append(segment(args.page, "text", len(text_segments) + 1, line, {"x": 0, "y": 0, "width": 10000, "height": 10000}))
            scale = min(2.0, 4096 / max(width, height), math.sqrt(MAX_PAGE_PIXELS / (width * height)))
            while math.ceil(width * scale) * math.ceil(height * scale) > MAX_PAGE_PIXELS:
                scale *= 0.999
            # No init_forms(), V8/XFA build, annotation execution or JavaScript.
            bitmap = page.render(scale=scale, draw_annots=False, limit_image_cache=True)
            image = bitmap.to_pil()
        else:
            with Image.open(io.BytesIO(data)) as source:
                checked_pixels(*source.size)
                image = ImageOps.exif_transpose(source).convert("RGB")
        checked_pixels(*image.size)
        raster = "/tmp/ocr-page.png"
        image.save(raster, format="PNG")
        values = text_segments + ocr(raster, args.page, *image.size)
        if len(values) > MAX_SEGMENTS:
            raise EvidenceFailure("over-limit", "The complete page exceeds its source segment budget.")
        return {"page": args.page, "width": image.width, "height": image.height, "segments": values}
    finally:
        # PDFium is used serially in this owned page process. Explicit child
        # closure is required; garbage collection is never the resource bound.
        for owned in (image, bitmap, textpage, page, document):
            if owned is not None:
                owned.close()


def publish(path, value):
    raw = encoded(value)
    if len(raw) > MAX_RESULT:
        raise EvidenceFailure("over-limit", "The complete result exceeds 1 MiB; no truncated evidence is ready.")
    temporary = path + ".tmp"
    descriptor = os.open(temporary, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
    with os.fdopen(descriptor, "wb") as stream:
        stream.write(raw)
        stream.flush()
        os.fsync(stream.fileno())
    os.replace(temporary, path)


def run(args):
    started = time.monotonic()
    result = {"protocol": 1, "profile": PROFILE, "attemptId": args.attempt, "originalDigest": args.digest,
              "extractorVersion": VERSION, "status": "needs-review", "pages": [], "issues": []}
    try:
        data = original_bytes(args)
        pdfium, Image, _ = dependencies()
        count = page_count(args, data, pdfium, Image)
        total_pixels = total_segments = 0
        for page in range(1, count + 1):
            remaining = WHOLE_SECONDS - (time.monotonic() - started)
            if remaining <= 0:
                raise EvidenceFailure("over-limit", "The extraction compute deadline expired.")
            output = "/tmp/page-result.json"
            Path(output).unlink(missing_ok=True)
            command = ["/usr/bin/python3", "-I", __file__, "--original", args.original, "--digest", args.digest,
                       "--mime", args.mime, "--bytes", str(args.bytes), "--attempt", args.attempt, "--page", str(page)]
            process = subprocess.Popen(command, stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            try:
                process.wait(timeout=min(PAGE_SECONDS, remaining))
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait(timeout=1)
                # No next page is launched. Container retirement closes any
                # descendant still owned by this exact extraction namespace.
                raise EvidenceFailure("over-limit", "The exact page process exceeded its deadline.") from None
            if process.returncode != 0:
                raise EvidenceFailure("failed", "The owned page process exited without a complete result.")
            page_result = json.loads(read_regular(output, MAX_RESULT))
            if "failure" in page_result:
                raise EvidenceFailure(page_result["failure"], page_result["message"])
            if page_result.get("page") != page:
                raise EvidenceFailure("corrupt", "The page receipt does not match the requested source.")
            total_pixels += page_result["width"] * page_result["height"]
            total_segments += len(page_result["segments"])
            if total_pixels > MAX_TOTAL_PIXELS or total_segments > MAX_SEGMENTS:
                raise EvidenceFailure("over-limit", "The complete document exceeds its pixel or segment budget.")
            result["pages"].append(page_result)
            if len(encoded(result)) > MAX_RESULT:
                raise EvidenceFailure("over-limit", "The complete source-linked document exceeds 1 MiB.")
        result["issues"] = ["OCR confidence and text are evidence claims. Compare originals, orientation, digital-text duplicates and uncertain words before accepting context."]
        if len(encoded(result)) > MAX_RESULT:
            raise EvidenceFailure("over-limit", "The complete attributed result exceeds 1 MiB.")
    except EvidenceFailure as failure:
        result.update(status=failure.status, pages=[], issues=[failure.message])
    except Exception:
        result.update(status="corrupt", pages=[], issues=["The complete document could not be decoded safely. Preserve the original before restarting."])
    # Docker tmpfs disappears when the container exits. The exact host SDK
    # stream is attached BEFORE Start; emit the complete bounded result while
    # alive, then let the controller independently prove exit and retirement.
    sys.stdout.buffer.write(result_frame(result))
    sys.stdout.buffer.flush()


def main():
    parser = argparse.ArgumentParser(allow_abbrev=False)
    parser.add_argument("--original", choices=["/input/original"], required=True)
    parser.add_argument("--digest", required=True)
    parser.add_argument("--mime", choices=["application/pdf", "image/png", "image/jpeg"], required=True)
    parser.add_argument("--bytes", type=int, required=True)
    parser.add_argument("--attempt", required=True)
    parser.add_argument("--page", type=int)
    args = parser.parse_args()
    if not re.fullmatch(r"[a-f0-9]{64}", args.digest) or not re.fullmatch(r"extract-[a-z0-9_-]{1,190}", args.attempt) or not 1 <= args.bytes <= MAX_ORIGINAL or args.page is not None and not 1 <= args.page <= 50:
        raise SystemExit(2)
    if args.page is None:
        run(args)
    else:
        try:
            page = extract_page(args)
        except EvidenceFailure as failure:
            page = {"failure": failure.status, "message": failure.message}
        except Exception:
            page = {"failure": "corrupt", "message": "The complete page could not be decoded safely."}
        publish("/tmp/page-result.json", page)


if __name__ == "__main__":
    main()
