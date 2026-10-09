"""Stage distribution guides without shipping the validation archive in clean builds.

This is a build-time tool. It never runs in Luci startup. Source documents and
evidence remain untouched; unknown/dirty source keeps the complete docs tree.
Only recognized Markdown link destinations are rewritten, not document prose.
"""
import argparse
from dataclasses import dataclass
import html
import os
from pathlib import Path, PurePosixPath
import posixpath
import re
import shutil
import stat
import subprocess
from urllib.parse import quote, unquote
from typing import Optional


EVIDENCE = 'docs/luci/evidence'
FIXED = ('LICENSE', 'NOTICE', 'README.md', 'DESIGN.md', 'docs/UAT.md', 'docs/RUNBOOK.md')
PUNCTUATION = r'!"#$%&\'()*+,-./:;<=>?@[\]^_`{|}~'


def escaped(text, offset):
    count = 0
    while offset > 0 and text[offset - 1] == '\\':
        count += 1
        offset -= 1
    return count % 2 == 1


def unescape(text):
    return html.unescape(re.sub(r'\\([' + re.escape(PUNCTUATION) + r'])', r'\1', text))


def label(text):
    return ' '.join(unescape(text).split()).casefold()


def prose_mask(text):
    """Keep offsets, excluding fenced/indented code, comments and code spans."""
    masked = list(text)
    fence = None
    offset = 0
    for line in text.splitlines(keepends=True):
        # Block quote fences are also Markdown code, not guide links.
        body = re.sub(r'^(?: {0,3}> ?)+', '', line)
        marker = re.match(r'^ {0,3}(`{3,}|~{3,})(.*)$', body.rstrip('\r\n'))
        is_code = bool(fence) or bool(re.match(r'^(?: {4}|\t)', body))
        if fence:
            if marker and marker[1][0] == fence[0] and len(marker[1]) >= fence[1] and not marker[2].strip():
                fence = None
        elif marker and not (marker[1][0] == '`' and '`' in marker[2]):
            fence = (marker[1][0], len(marker[1]))
            is_code = True
        if is_code:
            masked[offset:offset + len(line)] = ['\n' if c == '\n' else ' ' for c in line]
        offset += len(line)
    view = ''.join(masked)
    cursor = 0
    while cursor < len(view):
        # Comments and code spans are consumed in document order: a literal
        # comment opener in a code span must not hide later real links.
        if view.startswith('<!--', cursor):
            closing = view.find('-->', cursor + 4)
            stop = closing + 3 if closing != -1 else len(view)
            masked[cursor:stop] = ['\n' if c == '\n' else ' ' for c in view[cursor:stop]]
            cursor = stop
            continue
        if view[cursor] != '`' or escaped(view, cursor):
            cursor += 1
            continue
        end = cursor
        while end < len(view) and view[end] == '`':
            end += 1
        close = re.search(r'(?<!`)`{' + str(end - cursor) + r'}(?!`)', view[end:])
        if close:
            stop = end + close.end()
            masked[cursor:stop] = ['\n' if c == '\n' else ' ' for c in view[cursor:stop]]
            cursor = stop
        else:
            cursor = end
    return ''.join(masked)


def destination(text, start):
    """Return destination byte offsets and following position; titles stay intact."""
    i = start
    while i < len(text) and text[i].isspace():
        i += 1
    if i == len(text):
        return None
    if text[i] == '<':
        begin = i + 1
        i = begin
        while i < len(text) and text[i] not in '\r\n':
            if text[i] == '>' and not escaped(text, i):
                return begin, i, i + 1
            if text[i] == '<' and not escaped(text, i):
                return None
            i += 1
        return None
    begin, depth = i, 0
    while i < len(text):
        char = text[i]
        if char == '\\' and i + 1 < len(text) and text[i + 1] in PUNCTUATION:
            i += 2
            continue
        if char.isspace() or (char == ')' and depth == 0):
            break
        if char == '(':
            depth += 1
        elif char == ')':
            depth -= 1
        elif char in '<>':
            return None
        i += 1
    return (begin, i, i) if depth == 0 else None


def link_end(text, after):
    i = after
    while i < len(text) and text[i].isspace():
        i += 1
    if i < len(text) and text[i] == ')':
        return i + 1
    if i == after or i == len(text) or text[i] not in '\'"(':
        return None
    closing = ')' if text[i] == '(' else text[i]
    i += 1
    while i < len(text) and (text[i] != closing or escaped(text, i)):
        i += 1
    i += 1
    while i < len(text) and text[i].isspace():
        i += 1
    return i + 1 if i < len(text) and text[i] == ')' else None


@dataclass
class Link:
    start: int
    end: int
    image: bool = False
    reference: Optional[str] = None


def links(text):
    view = prose_mask(text)
    result, image_refs, stack = [], set(), []
    definitions = []
    for match in re.finditer(r'^ {0,3}\[((?:\\.|[^\]\\\n])+)\]:[ \t]*', view, re.M):
        token = destination(view, match.end())
        if token:
            start, end, _ = token
            definitions.append((match.start(), end))
            result.append(Link(start, end, reference=label(match[1])))
    cursor = 0
    while cursor < len(view):
        if any(start <= cursor < end for start, end in definitions):
            cursor += 1
            continue
        char = view[cursor]
        if char == '[' and not escaped(view, cursor):
            stack.append((cursor, cursor > 0 and view[cursor - 1] == '!' and not escaped(view, cursor - 1)))
        elif char == ']' and not escaped(view, cursor) and stack:
            begin, image = stack.pop()
            after = cursor + 1
            if after < len(view) and view[after] == '(':
                token = destination(view, after + 1)
                if token and link_end(view, token[2]):
                    result.append(Link(token[0], token[1], image=image))
                    cursor = link_end(view, token[2]) - 1
            elif image:
                name = view[begin + 1:cursor]
                if after < len(view) and view[after] == '[':
                    stop = view.find(']', after + 1)
                    if stop != -1:
                        name = view[after + 1:stop] or name
                image_refs.add(label(name))
        cursor += 1
    for item in result:
        if item.reference in image_refs:
            item.image = True
    return sorted(result, key=lambda item: item.start)


def ordinary(path):
    info = path.lstat()
    if stat.S_ISLNK(info.st_mode) or getattr(info, 'st_file_attributes', 0) & getattr(stat, 'FILE_ATTRIBUTE_REPARSE_POINT', 0):
        raise ValueError(f'Linked documentation input refused: {path}')
    if not (stat.S_ISDIR(info.st_mode) or stat.S_ISREG(info.st_mode)):
        raise ValueError(f'Special documentation input refused: {path}')
    return stat.S_ISDIR(info.st_mode)


def source_files(root, compact):
    selected = []
    for relative in FIXED:
        path = root / relative
        ordinary(path.parent)
        if ordinary(path):
            raise ValueError(f'Expected documentation file: {relative}')
        selected.append(relative)
    base = root / 'docs/luci'
    ordinary(base)
    for parent, directories, files in os.walk(base, followlinks=False):
        for name in list(directories):
            child = Path(parent) / name
            ordinary(child)
            if compact and child.relative_to(root).as_posix() == EVIDENCE:
                directories.remove(name)
        for name in files:
            child = Path(parent) / name
            ordinary(child)
            selected.append(child.relative_to(root).as_posix())
    return sorted(selected)


def local_target(document, raw):
    value = unescape(raw)
    if not value or value.startswith(('#', '?', '//')) or re.match(r'^[A-Za-z][A-Za-z0-9+.-]*:', value):
        return None
    pieces = re.split(r'([?#].*)', value, maxsplit=1)
    path = unquote(pieces[0], errors='strict')
    if path.startswith('/') or '\\' in path or any(ord(c) < 32 for c in path):
        raise ValueError(f'Unsafe guide link in {document}: {raw}')
    resolved = posixpath.normpath(posixpath.join(posixpath.dirname(document), path))
    if resolved == '..' or resolved.startswith('../'):
        raise ValueError(f'Guide link escapes source root in {document}: {raw}')
    # Find the same first delimiter in the written Markdown. Decode an entity
    # used as the delimiter itself, preserving the rest of the written suffix.
    suffix = ''
    tokens = r'\\[' + re.escape(PUNCTUATION) + r']|&(?:#[xX][0-9a-fA-F]+;?|#[0-9]+;?|[A-Za-z][A-Za-z0-9]*;)|.'
    for token in re.finditer(tokens, raw):
        decoded = unescape(token[0])
        if decoded in ('?', '#'):
            suffix = decoded + raw[token.end():]
            break
    return resolved, suffix


def git(root, *args, input=None):
    environment = {key: value for key, value in os.environ.items() if not key.startswith('GIT_')}
    return subprocess.run(['git', '-C', str(root), *args], input=input, check=True,
                          stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=30,
                          env=environment).stdout


def exact_objects(root, commit, targets):
    if not (root / '.git').exists() or git(root, 'rev-parse', '--verify', 'HEAD').decode().strip() != commit:
        raise ValueError('Clean documentation staging requires this exact checkout revision')
    if git(root, 'status', '--porcelain=v1', '--untracked-files=all').strip():
        raise ValueError('Clean documentation staging refuses a changed checkout')
    ordered = sorted(targets)
    if not ordered:
        return {}
    for relative in ordered:
        path = root
        for part in relative.split('/'):
            path /= part
            ordinary(path)
    data = ''.join(f'{commit}:{relative}\n' for relative in ordered).encode()
    kinds = git(root, 'cat-file', '--batch-check=%(objecttype)', input=data).decode().splitlines()
    if len(kinds) != len(ordered) or any(kind not in ('blob', 'tree') for kind in kinds):
        raise ValueError('A source-only guide link is absent from the exact source revision')
    return dict(zip(ordered, kinds))


def revision_files(root, commit, selected):
    """Bind every compact staged byte to an ordinary file at the named commit."""
    kinds = exact_objects(root, commit, selected)
    if any(kind != 'blob' for kind in kinds.values()):
        raise ValueError('A staged guide is not a file at the exact source revision')
    entries = git(root, 'ls-tree', '-z', commit, '--', *selected).split(b'\0')
    modes = {}
    for entry in entries:
        if entry:
            metadata, name = entry.split(b'\t', 1)
            modes[name.decode()] = metadata.split()[0]
    if set(modes) != set(selected) or any(mode not in (b'100644', b'100755') for mode in modes.values()):
        raise ValueError('Documentation must be ordinary tracked source files')
    data = git(root, 'cat-file', '--batch', input=''.join(f'{commit}:{path}\n' for path in selected).encode())
    cursor, result = 0, {}
    for path in selected:
        stop = data.index(b'\n', cursor)
        header = data[cursor:stop].split()
        if len(header) != 3 or header[1] != b'blob':
            raise ValueError('Unexpected committed documentation object')
        size = int(header[2])
        cursor = stop + 1
        content = data[cursor:cursor + size]
        cursor += size
        if len(content) != size or data[cursor:cursor + 1] != b'\n':
            raise ValueError('Incomplete committed documentation object')
        cursor += 1
        if content != (root / path).read_bytes():
            raise ValueError('A documentation input differs from the exact source revision')
        result[path] = content
    if cursor != len(data):
        raise ValueError('Unexpected data after committed documentation objects')
    return result


def stage(root, output, commit='', dirty='unknown'):
    root, output = Path(root).absolute(), Path(output).absolute()
    if output.exists() or output.is_symlink():
        raise ValueError('Documentation destination must be a new staging directory')
    for ancestor in output.parents:
        if ancestor.exists() or ancestor.is_symlink():
            if not ordinary(ancestor):
                raise ValueError('Documentation destination has a non-directory ancestor')
    if commit and not re.fullmatch(r'(?:[0-9a-f]{40}|[0-9a-f]{64})', commit):
        raise ValueError('Source revision must be a full immutable Git object ID')
    compact = bool(commit) and dirty == 'false'
    selected = source_files(root, compact)
    shipped = set(selected)
    shipped_dirs = {posixpath.dirname(p) for p in selected}
    shipped_dirs |= {parent.as_posix() for p in selected for parent in PurePosixPath(p).parents}
    texts, replacements, targets = {}, {}, set()
    if compact:
        committed = revision_files(root, commit, selected)
        for relative in selected:
            if not relative.endswith('.md'):
                continue
            text = committed[relative].decode('utf-8')
            texts[relative] = text
            replacements[relative] = []
            for item in links(text):
                target = local_target(relative, text[item.start:item.end])
                if not target or target[0] in shipped or target[0] in shipped_dirs:
                    continue
                targets.add(target[0])
                replacements[relative].append((item, *target))
        kinds = exact_objects(root, commit, targets)
    output.mkdir(parents=True)
    for relative in selected:
        destination_path = output / relative
        destination_path.parent.mkdir(parents=True, exist_ok=True)
        if relative not in texts:
            if compact:
                destination_path.write_bytes(committed[relative])
            else:
                shutil.copy2(root / relative, destination_path)
            continue
        text = texts[relative]
        for item, target, suffix in reversed(replacements[relative]):
            encoded = quote(target, safe='/')
            if item.image and kinds[target] == 'blob':
                address = f'https://raw.githubusercontent.com/lcfrai/luci/{commit}/{encoded}'
            else:
                kind = 'tree' if kinds[target] == 'tree' else 'blob'
                address = f'https://github.com/lcfrai/luci/{kind}/{commit}/{encoded}'
            text = text[:item.start] + address + suffix + text[item.end:]
        destination_path.write_bytes(text.encode('utf-8'))
    index = output / 'docs/PACKAGE-VALIDATION.md'
    if index.exists():
        raise ValueError('Generated validation index would replace source documentation')
    index.write_text('# Validation and source\n\n' + (
        f'This package was built from `{commit}`. Full validation reports, failed attempts, '
        f'screenshots and reproduction helpers remain in the [exact source revision]'
        f'(https://github.com/lcfrai/luci/tree/{commit}/docs/luci/evidence). '
        'Source-only guide links open that revision; clone it to run source test procedures. '
        'Runtime packages omit the validation archive to limit download and installation size.\n'
        if compact else
        'Exact clean source provenance is unavailable for this development/export build. '
        'The complete documentation and validation tree is retained; no hosted revision is '
        'asserted for changed files. Source-only procedures require the matching source tree.\n'
    ) + '\nA packaged build or validation report is not a claim that every release gate has passed.\n', encoding='utf-8')
    readme = output / 'README.md'
    with readme.open('ab') as stream:
        stream.write(b'\n\nPackaged validation and source access: [exact provenance and scope](docs/PACKAGE-VALIDATION.md).\n')
    return {'compact': compact, 'sourceCommit': commit or None, 'files': len(selected) + 1,
            'rewrittenLinks': sum(map(len, replacements.values()))}


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('root')
    parser.add_argument('output')
    parser.add_argument('--source-commit', default='')
    parser.add_argument('--source-dirty', choices=('true', 'false', 'unknown'), default='unknown')
    args = parser.parse_args()
    print(stage(args.root, args.output, args.source_commit, args.source_dirty))
