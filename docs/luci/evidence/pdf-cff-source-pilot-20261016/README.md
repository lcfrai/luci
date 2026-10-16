# Fixed CJK CFF1 pilot: source-only receipt

The isolated source is `scripts/distribute-pdf-pilot/`; production `control-plane/go.mod`, export routing and release inventory were not changed. The selected font/program has not been acquired, compiled or rendered. The source includes an exact offline-input preflight, one face-0 CFF1 Go subset/PDF writer, and a separate bounded Python reader of its fixed PDF dialect. A separate standard-reader and actual Job/cgroup binding are still review gates.

Read-only upstream metadata checked before writing the source: Go 1.25.0 official Linux/Windows archive bytes and SHA-256 from `go.dev/dl/?mode=json&include=all`; seven selected module version ZIP URLs plus seven `.mod` and `.info` metadata SHA-256 values from `proxy.golang.org`; Debian CJK package URL and 56,674,044-byte HEAD from `deb.debian.org`. The font original bytes were only inspected read-only from the existing host installation and match the manifest's 19,484,784-byte SHA-256. No module ZIP, Debian package, toolchain archive or font was downloaded. The pinned upstream font subset source handles CFF1 and explicitly refuses CFF2; PDF model source exposes the CFF Type0/FontFile3 shape. Source shape is not compatibility proof.

Exact source check from repository root:

```sh
/dev/shm/luci-toolchain/go/bin/gofmt -w scripts/distribute-pdf-pilot/main.go
LUCI_PDF_PILOT_TMPDIR=/dev/shm python3 -m unittest discover -s scripts/distribute-pdf-pilot -p test_source.py -v
python3 - <<'PY'
import ast,json
from pathlib import Path
base=Path('scripts/distribute-pdf-pilot')
for name in ('preflight.py','verify_pdf.py'):
 ast.parse((base/name).read_text())
json.loads((base/'inputs-v1.json').read_text())
print('source parsers and manifest: valid')
PY
```

The three source/refusal tests passed and both Python parsers plus the JSON manifest parsed. An initial unchanged command used Python's default `/var/tmp`; it failed before the missing-input fixture because that mount had zero free bytes. The successful rerun set only this test's temporary directory to `/dev/shm`. Neither attempt downloaded a dependency, built Go, rendered a PDF, opened a provider or touched Docker.

The Go file was formatted and parsed by `gofmt`; it was **not** typechecked or compiled. The 512 MiB Job/cgroup cap, 30-second render limit, offline module `h1` closure, standard reader fidelity, positive cleanup and Windows/Linux package acceptance remain unverified. The pilot is not runnable under normal setup or a production API route.
