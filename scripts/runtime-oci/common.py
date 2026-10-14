"""Small standard-library validators for the committed runtime schema subset."""
import json
from pathlib import Path
import re

from archive import encoded, json_bytes, regular


def validate(value, schema, root=None):
    root = schema if root is None else root
    if "$ref" in schema:
        reference = schema["$ref"]
        if not reference.startswith("#/$defs/"):
            raise ValueError("unsupported schema reference")
        return validate(value, root["$defs"][reference.split("/")[-1]], root)
    if "const" in schema and encoded(value) != encoded(schema["const"]) or "enum" in schema and not any(encoded(value) == encoded(item) for item in schema["enum"]):
        raise ValueError("runtime metadata constant mismatch")
    kind = schema.get("type")
    types = {"object": dict, "array": list, "string": str, "integer": int, "boolean": bool}
    if kind and (not isinstance(value, types[kind]) or kind == "integer" and isinstance(value, bool)):
        raise ValueError("runtime metadata type mismatch")
    if isinstance(value, dict):
        properties = schema.get("properties", {})
        if any(key not in value for key in schema.get("required", [])) or schema.get("additionalProperties") is False and set(value) - set(properties):
            raise ValueError("runtime metadata missing/unknown field")
        for key, item in value.items():
            if key in properties:
                validate(item, properties[key], root)
    if isinstance(value, list):
        if not schema.get("minItems", 0) <= len(value) <= schema.get("maxItems", 1 << 30):
            raise ValueError("runtime metadata array bound")
        if "items" in schema:
            for item in value:
                validate(item, schema["items"], root)
    if isinstance(value, str):
        if not schema.get("minLength", 0) <= len(value) <= schema.get("maxLength", 1 << 30) or "pattern" in schema and re.search(schema["pattern"], value) is None:
            raise ValueError("runtime metadata string bound/pattern")
    if isinstance(value, int) and not isinstance(value, bool):
        if value < schema.get("minimum", value) or value > schema.get("maximum", value):
            raise ValueError("runtime metadata integer bound")
    for child in schema.get("allOf", []):
        if "if" in child:
            try:
                validate(value, child["if"], root)
            except ValueError:
                continue
            validate(value, child.get("then", {}), root)
        else:
            validate(value, child, root)
    return value


def schema_validate(value, name):
    path = Path(__file__).resolve().parents[2] / "docs/luci/schemas" / name
    return validate(value, json.loads(path.read_text(encoding="utf-8")))


def read_json(path):
    regular(path)
    with Path(path).open("rb") as stream:
        return json_bytes(stream.read((1 << 20) + 1))


def write_json(path, value):
    path = Path(path)
    plain_directory(path.parent)
    if path.exists() or path.is_symlink():
        regular(path)
    raw = encoded(value)
    if len(raw) > 1 << 20:
        raise ValueError("runtime metadata exceeds 1 MiB")
    path.write_bytes(raw)


def plain_directory(path):
    path = Path(path).absolute()
    for entry in [path, *path.parents]:
        info = entry.lstat()
        if entry.is_symlink() or getattr(info, "st_file_attributes", 0) & 1024 or not entry.is_dir():
            raise ValueError("runtime directory is linked/special")
    return path
