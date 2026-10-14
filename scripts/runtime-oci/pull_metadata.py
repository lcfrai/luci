"""V2 receipt bytes match Go encoding/json.Marshal's strict representation.

Legacy v1 JSON keeps its original newline/hash contract unchanged.
"""
import json
from pathlib import Path

import archive
from common import plain_directory


def encoded(value):
    def strict(item):
        if isinstance(item, dict):
            if any(not isinstance(key, str) for key in item):
                raise ValueError('runtime pull JSON keys must be strings')
            for child in item.values():
                strict(child)
        elif isinstance(item, list):
            for child in item:
                strict(child)
        elif type(item) is int:
            # The Go verifier canonicalizes through map[string]any/float64.
            if abs(item) > (1 << 53) - 1:
                raise ValueError('runtime pull integer exceeds exact Go JSON range')
        elif not isinstance(item, (str, bool)):
            raise ValueError('runtime pull JSON requires strings, integers and booleans')
    strict(value)
    raw = json.dumps(value, ensure_ascii=False, allow_nan=False, sort_keys=True, separators=(',', ':'))
    for character, escape in (('<', '\\u003c'), ('>', '\\u003e'), ('&', '\\u0026'),
                              ('\u2028', '\\u2028'), ('\u2029', '\\u2029')):
        raw = raw.replace(character, escape)
    return raw.encode('utf-8', errors='strict')


def write(path, value):
    path = Path(path)
    plain_directory(path.parent)
    if path.exists() or path.is_symlink():
        archive.regular(path)
    raw = encoded(value)
    if len(raw) > 1 << 20:
        raise ValueError('runtime pull receipt exceeds 1 MiB')
    path.write_bytes(raw)
