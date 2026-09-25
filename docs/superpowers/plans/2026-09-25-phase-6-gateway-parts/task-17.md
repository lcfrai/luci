### Task 17: Local stack on Docker Desktop

**The whole gateway on one laptop, zero cloud spend, and the same shape as the cloud.** OD-20 keeps local mode, and every later task in this phase needs somewhere to run: task 18 probes the pinned LiteLLM here, task 23 runs unmodified Claude Code against it, and task 15's local control plane launches its sandboxes onto its network. This task writes that stack: Postgres, LiteLLM, keysmith, gatekeeper and two copies of task 16's fake upstream, on one Docker network named exactly `panacea-app-gateway`, with only the gatekeeper and keysmith published, and only on `127.0.0.1`.

**LiteLLM publishes nothing (P6-D1).** A sandbox, a developer's `curl` and a stray browser tab all reach LiteLLM only through the gatekeeper. Task 18 reaches LiteLLM directly by running its probes from a container on the network, which is the one door a probe needs and a sandbox never gets: a sandbox's token is bound to its own address and LiteLLM would still take it, which is exactly why LiteLLM must not be reachable. That residual (a container on the network can call LiteLLM with a session key and skip the source check) is local mode's and is stated below; in the cloud, task 19's NetworkPolicy closes it.

**No secret lives in a compose file or a container's environment.** `init.sh` creates the secrets once, as files in a gitignored `secrets/` directory. Compose mounts them at `/run/secrets/<name>` (file-based secrets, no swarm). Our three binaries read `*_FILE` variables. LiteLLM reads its master key, salt key and database URL from the **environment** (Panacea sets `LITELLM_MASTER_KEY` and `DATABASE_URL` as env vars, `Projects/Panacea/kubernetes/helm-values/litellm-values.yaml:111-122`; whether LiteLLM reads a file is **UNVERIFIED**), so its entrypoint is a three-line `sh -c` that exports them from the files and `exec`s LiteLLM. The values therefore never appear in `compose.yaml`, in `docker compose config`, or in `docker inspect`. Postgres uses its official image's `POSTGRES_PASSWORD_FILE`.

**Why the binaries are built on the host.** The corporate proxy intercepts TLS to `proxy.golang.org`, so `go mod download` fails inside a Docker build (`images/sandbox/Dockerfile:3-6`). `sandboxd` escapes that by vendoring; `control-plane` does not vendor. The gateway image therefore copies three binaries cross-compiled on the host, where the module cache already works, into a distroless base. `images/gateway/bin/` is gitignored.

**State the stack mounts is a directory, never a file.** The project route file (task 13's `File` source) is rewritten by task 21 with an atomic rename, and a bind-mounted **file** keeps its old inode, so a container would never see the change (the same reason the token directory is mounted, the skeleton's shared names). `init.sh` therefore seeds `state/projects.json` from the checked-in `projects.json` once, and compose mounts `state/` read-only. The checked-in seed is never written at run time, so the git tree stays clean.

**Two upstream modes.** `PANACEA_APP_GATEWAY_LOCAL_UPSTREAM=fake` (the default) points both LiteLLM groups at the fakes. `=anthropic` points the **Global** group at the real Anthropic API with a developer-supplied key and makes real spend; the **AU** group stays on `fakeanthropic-au`, because Bedrock is never reachable from local mode. `init.sh` implements the switch by writing compose's `.env` (non-secret: `COMPOSE_FILE` and its separator), so `docker compose up` does the right thing without flags. The anthropic mode needs a second rendered config, `litellm.anthropic.config.yaml`, which is **task 02's third target**, `gatewaygen -target local-anthropic` (task 02's decision (c); the skeleton's file map lists only `local` and `cloud`).

**Files:**
- `deploy/local/gateway/compose.yaml` — six services, one network, file secrets, `127.0.0.1` ports (new)
- `deploy/local/gateway/compose.anthropic.yaml` — the override for `LOCAL_UPSTREAM=anthropic`: the second config and the upstream key (new)
- `deploy/local/gateway/compose.testdb.yaml` — an opt-in override that publishes Postgres on `127.0.0.1:55432` for task 05's `pgstore` tests only; never part of the default stack (new, a file the skeleton does not list)
- `deploy/local/gateway/init.sh` — secrets, state seed, `.env`; never overwrites a secret (new)
- `deploy/local/gateway/compose_test.sh` — static checks over `docker compose config`, and `init.sh`'s idempotence (new)
- `deploy/local/gateway/projects.json` — the seed route file: two fixture projects, one Global, one AU (new)
- `images/gateway/Dockerfile` — distroless, three host-built binaries (new)
- `.gitignore` — `deploy/local/gateway/secrets/`, `deploy/local/gateway/state/`, `images/gateway/bin/` (modified)
- Read, not written: `deploy/local/gateway/litellm.config.yaml` and `litellm.anthropic.config.yaml` (task 02's generator)

**Interfaces:**
- Produces:
  - the Docker network **`panacea-app-gateway`** (compose `networks.gateway.name`, so no project prefix), which task 15's `Spec.Network` names
  - DNS names on it: `gatekeeper:4000`, `keysmith:4100`, `litellm:4001`, `postgres:5432`, `fakeanthropic-global:4900`, `fakeanthropic-au:4901` (the skeleton's shared names; task 02's local config points at the last two)
  - host ports `127.0.0.1:4000` (gatekeeper) and `127.0.0.1:4100` (keysmith), and no other
  - `secrets/controlplane_token`: the local control plane's keysmith bearer, the file task 15's `PANACEA_APP_GATEWAY_CALLER_TOKEN_FILE` names
  - `secrets/callers`: keysmith's `bearer-file`, in task 08's `name token routes` line format
  - `state/projects.json`: the live route file, which keysmith and the gatekeeper read as `file:/etc/panacea/state/projects.json` and task 21's file store writes on the host
  - the image `panacea-app/gateway:local` carrying `/usr/local/bin/{keysmith,gatekeeper,fakeanthropic}`
- Consumes: task 02's `litellm.config.yaml` (and the anthropic variant, gap above); task 08's `keysmith` binary and its variables; task 12's `gatekeeper`; task 16's `fakeanthropic` and its flags; task 13's `projects.json` format; task 03's mint JSON (acceptance only).
- Consumed by: task 05 (`PANACEA_APP_GATEWAY_TEST_DATABASE_URL` against this Postgres, step 7), task 15 (network, bearer, keysmith URL), task 18 (probes from a container on the network), task 21 (writes `state/projects.json`), task 23 (the whole stack).

**What each service is given, complete.** Every variable name is from the skeleton's shared names; a variable a binary does not know fails its start-up with the name (global constraint 5), which is how a drift between this table and tasks 08 and 12 shows itself.

| Service | Variables | Secrets mounted | Other mounts |
|---|---|---|---|
| `postgres` | `POSTGRES_DB=gateway`, `POSTGRES_USER=gateway`, `POSTGRES_PASSWORD_FILE=/run/secrets/postgres_password` | `postgres_password` | volume `pgdata` |
| `litellm` | none in compose; exported by its entrypoint from files: `LITELLM_MASTER_KEY`, `LITELLM_SALT_KEY`, `DATABASE_URL` (plus `ANTHROPIC_API_KEY` in anthropic mode) | `master_key`, `salt_key`, `database_url` (+ `anthropic_api_key`) | `./litellm.config.yaml:/app/config.yaml:ro` (Panacea's path, `litellm-values.yaml:100-103`) |
| `keysmith` | `PANACEA_APP_GATEWAY_LITELLM_URL=http://litellm:4001`, `_MASTER_KEY_FILE=/run/secrets/master_key`, `_DATABASE_URL_FILE=/run/secrets/database_url`, `_CALLER_AUTH=bearer-file:/run/secrets/callers`, `_ROUTE_SOURCE=file:/etc/panacea/state/projects.json`, `_BASE_URL=http://gatekeeper:4000` | `master_key`, `database_url`, `callers` | `./state:/etc/panacea/state:ro` |
| `gatekeeper` | `PANACEA_APP_GATEWAY_LITELLM_URL=http://litellm:4001`, `_KEYSMITH_URL=http://keysmith:4100`, `_ROUTE_SOURCE=file:/etc/panacea/state/projects.json`, `_CALLER_TOKEN_FILE=/run/secrets/gatekeeper_token` | `gatekeeper_token` | `./state:/etc/panacea/state:ro` |
| `fakeanthropic-global` | flags `-addr :4900 -label global` | none | none |
| `fakeanthropic-au` | flags `-addr :4901 -label au` | none | none |

`PANACEA_APP_GATEWAY_BASE_URL` is what keysmith returns in `MintResponse.BaseURL`: the address a **sandbox** uses, `http://gatekeeper:4000`, with no path (the skeleton's docs table, row one). The host's `127.0.0.1:4000` is for a developer's checks, and a token used from the host is refused by design (its source is not the sandbox's).

`_CALLER_TOKEN_FILE` for the gatekeeper is the same variable task 15 gives the local control plane. The skeleton's shared-names list omits it; this part uses it for both callers and raises the omission as a gap.

**The bearer file, complete.** Task 08 owns the format (`name token routes`); `init.sh` writes exactly these two lines, with the route names as task 08's scope lists them, plus `usage` for task 21's read:

```text
controlplane <64 hex> mint,refresh,revoke,usage
gatekeeper <64 hex> bindings
```

If task 08's part names a route differently, `init.sh` follows task 08, and an existing `secrets/callers` is regenerated by deleting it by hand (`init.sh` never overwrites).

**The seed route file, complete** (`deploy/local/gateway/projects.json`, task 13's `File` format, task 01's class keys):

```json
{
  "local-global": {
    "dataClass": { "level": "internal", "residency": "any" },
    "inferenceRoute": "global"
  },
  "local-au": {
    "dataClass": { "level": "confidential", "residency": "au-only" },
    "inferenceRoute": "au"
  }
}
```

The two ids are fixtures for this task's acceptance, task 18 and task 23. Real projects of the local control plane are added to `state/projects.json` by task 21, never to the seed.

- [ ] **Step 1: Write the failing test**

`deploy/local/gateway/compose_test.sh` works on a **copy** of the directory in a temporary tree, so it never touches a developer's real `secrets/`, `state/` or `.env`. It needs Docker Compose v2 and `python3` (standard library only; `jq` is not assumed). Each check names the mutant it kills in its comment.

```bash
#!/usr/bin/env bash
# compose_test.sh: static checks over `docker compose config` for phase 6's local
# stack (task 17). Runs on a copy; never reads or writes the real secrets/.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
repo="$(cd "$here/../../.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fx="$tmp/deploy/local/gateway"
mkdir -p "$fx"
for f in compose.yaml compose.anthropic.yaml litellm.config.yaml \
         litellm.anthropic.config.yaml projects.json init.sh; do
  cp "$here/$f" "$fx/$f"
done
fail=0
bad() { echo "FAIL: $*"; fail=1; }

# init.sh creates everything; a second run changes nothing.
# Kills: regenerating secrets on every run (a new salt key makes LiteLLM's stored
# credentials unreadable; a new Postgres password locks out the existing volume).
PANACEA_APP_GATEWAY_LOCAL_UPSTREAM=fake bash "$fx/init.sh" "$fx" >/dev/null
before="$(cd "$fx" && sha256sum secrets/* state/*)"
PANACEA_APP_GATEWAY_LOCAL_UPSTREAM=fake bash "$fx/init.sh" "$fx" >/dev/null
after="$(cd "$fx" && sha256sum secrets/* state/*)"
[ "$before" = "$after" ] || bad "init.sh rewrote a secret or the state file"
for s in master_key salt_key postgres_password database_url callers \
         controlplane_token gatekeeper_token; do
  [ -s "$fx/secrets/$s" ] || bad "init.sh did not create secrets/$s"
done
grep -q '^sk-[0-9a-f]\{64\}$' "$fx/secrets/master_key" \
  || bad "master_key lacks the sk- prefix task 04 requires"

# A dummy upstream key so the anthropic override renders. Never a real key.
printf 'test-not-a-key' > "$fx/secrets/anthropic_api_key"

render() { docker compose --project-directory "$fx" "$@" config --format json; }
render -f "$fx/compose.yaml" > "$tmp/fake.json"
render -f "$fx/compose.yaml" -f "$fx/compose.anthropic.yaml" > "$tmp/anthropic.json"

for mode in fake anthropic; do
  python3 - "$tmp/$mode.json" "$mode" <<'PY' || fail=1
import json, re, sys
cfg, mode = json.load(open(sys.argv[1])), sys.argv[2]
svcs, errs = cfg["services"], []
# Kills "4000:4000" (binds 0.0.0.0) and any extra published port, LiteLLM's first.
seen = set()
for name, s in svcs.items():
    for p in s.get("ports") or []:
        if p.get("host_ip") != "127.0.0.1":
            errs.append(f"{name}: port {p.get('target')} binds {p.get('host_ip') or '0.0.0.0'}")
        seen.add((name, int(p["target"])))
if seen != {("gatekeeper", 4000), ("keysmith", 4100)}:
    errs.append(f"published ports are {sorted(seen)}, want gatekeeper:4000 and keysmith:4100 only")
if svcs["litellm"].get("ports"):
    errs.append("litellm publishes a port: the gatekeeper can be bypassed")
# Kills a tag-only pin (a tag moves; a digest does not).
for name, s in svcs.items():
    if "build" not in s and not re.search(r"@sha256:[0-9a-f]{64}$", s.get("image", "")):
        errs.append(f"{name}: image {s.get('image')!r} is not pinned by digest")
# Kills the compose project prefix ("gateway_panacea-app-gateway"): task 15 joins by name.
if cfg.get("networks", {}).get("gateway", {}).get("name") != "panacea-app-gateway":
    errs.append("network gateway is not named panacea-app-gateway")
for name, s in svcs.items():
    if sorted((s.get("networks") or {}).keys()) != ["gateway"]:
        errs.append(f"{name}: not on the gateway network alone")
# Kills a secret in the environment: only *_FILE names may mention one.
for name, s in svcs.items():
    for k in (s.get("environment") or {}):
        if re.search(r"(KEY|TOKEN|PASSWORD|SECRET|DATABASE_URL)$", k):
            errs.append(f"{name}: {k} is set in the environment, not read from a file")
# Kills a debug flag that makes LiteLLM log request bodies (P6-D9).
ll = " ".join((svcs["litellm"].get("entrypoint") or []) + (svcs["litellm"].get("command") or []))
if re.search(r"--(detailed_)?debug", ll):
    errs.append("litellm runs with a debug flag")
if "exec litellm" not in ll or "--port 4001" not in ll:
    errs.append("litellm entrypoint does not exec litellm on :4001")
# Kills the anthropic override losing the fake AU group (Bedrock is never local).
want = "litellm.anthropic.config.yaml" if mode == "anthropic" else "litellm.config.yaml"
mounts = [v.get("source", "") for v in svcs["litellm"].get("volumes") or []
          if v.get("target") == "/app/config.yaml"]
if len(mounts) != 1 or not mounts[0].endswith(want):
    errs.append(f"litellm mounts {mounts} at /app/config.yaml, want {want}")
if "fakeanthropic-au" not in svcs:
    errs.append("fakeanthropic-au is missing")
for e in errs:
    print(f"FAIL ({mode}): {e}")
sys.exit(1 if errs else 0)
PY
done

# Kills a literal secret in a compose file or in what compose renders.
for s in master_key salt_key postgres_password controlplane_token \
         gatekeeper_token anthropic_api_key; do
  v="$(cat "$fx/secrets/$s")"
  for f in "$fx/compose.yaml" "$fx/compose.anthropic.yaml" "$tmp/fake.json" "$tmp/anthropic.json"; do
    if grep -qF -- "$v" "$f"; then bad "the value of secrets/$s appears in $(basename "$f")"; fi
  done
done

# Kills a tag-only base image in the gateway Dockerfile.
if grep -E '^FROM ' "$repo/images/gateway/Dockerfile" | grep -vqE '@sha256:[0-9a-f]{64}'; then
  bad "images/gateway/Dockerfile has a FROM without a digest"
fi

[ "$fail" -eq 0 ] && echo "PASS: compose_test (8 checks, fake and anthropic modes)"
exit "$fail"
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
~/.claude/tools/claudezsh/claudezsh 'bash deploy/local/gateway/compose_test.sh'
```

Expected: exit non-zero with `cp: cannot stat '.../compose.yaml'`. If task 02 has not yet rendered `litellm.anthropic.config.yaml`, the `cp` of that file fails the same way, and the test stays red until task 02's `local-anthropic` target renders it.

- [ ] **Step 3: Write `deploy/local/gateway/compose.yaml`**

The two `sha256:` values marked `PIN` are filled in step 5 from a real pull; the test stays red until they are, which is the point.

```yaml
# Phase 6 local mode (OD-20): the inference gateway on Docker Desktop, zero cloud
# spend. Task 17 of docs/superpowers/plans/2026-09-25-phase-6-gateway.md.
#   - Only gatekeeper (4000) and keysmith (4100) are published, on 127.0.0.1.
#   - LiteLLM publishes nothing: nothing reaches it but gatekeeper and keysmith.
#   - No secret value appears here: init.sh writes secrets/, compose mounts files.
#   - litellm.config.yaml is generated by cmd/gatewaygen (task 02). Never edit it.
# Run ./init.sh once, then: docker compose up -d --wait
name: panacea-app-gateway

networks:
  gateway:
    name: panacea-app-gateway # task 15's sandboxes join this exact name

volumes:
  pgdata: {}

secrets:
  master_key: { file: ./secrets/master_key }
  salt_key: { file: ./secrets/salt_key }
  postgres_password: { file: ./secrets/postgres_password }
  database_url: { file: ./secrets/database_url }
  callers: { file: ./secrets/callers }
  gatekeeper_token: { file: ./secrets/gatekeeper_token }

x-gateway: &gateway
  image: panacea-app/gateway:local
  build:
    context: ../../..
    dockerfile: images/gateway/Dockerfile
  user: "65532:65532"
  read_only: true
  networks: [gateway]
  restart: unless-stopped

services:
  postgres:
    image: postgres:16-alpine@sha256:PIN
    environment:
      POSTGRES_DB: gateway
      POSTGRES_USER: gateway
      POSTGRES_PASSWORD_FILE: /run/secrets/postgres_password
    secrets: [postgres_password]
    volumes: ["pgdata:/var/lib/postgresql/data"]
    networks: [gateway]
    restart: unless-stopped
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U gateway -d gateway"]
      interval: 2s
      timeout: 3s
      retries: 30

  litellm:
    image: ghcr.io/berriai/litellm:TAG@sha256:PIN
    # LiteLLM reads these from the environment, so they are exported from the
    # mounted files here and never written into this file. Never add
    # --detailed_debug or --debug: they log request bodies (P6-D9).
    entrypoint: ["/bin/sh", "-c"]
    command:
      - >-
        export LITELLM_MASTER_KEY="$$(cat /run/secrets/master_key)";
        export LITELLM_SALT_KEY="$$(cat /run/secrets/salt_key)";
        export DATABASE_URL="$$(cat /run/secrets/database_url)";
        exec litellm --config /app/config.yaml --port 4001
    secrets: [master_key, salt_key, database_url]
    volumes: ["./litellm.config.yaml:/app/config.yaml:ro"]
    networks: [gateway]
    restart: unless-stopped
    depends_on:
      postgres: { condition: service_healthy }
    healthcheck:
      test:
        - CMD
        - python3
        - -c
        - "import urllib.request; urllib.request.urlopen('http://127.0.0.1:4001/health/readiness', timeout=3)"
      interval: 3s
      timeout: 5s
      retries: 60
```

The same file, continued (the four services built from this repo):

```yaml
  keysmith:
    <<: *gateway
    command: ["/usr/local/bin/keysmith"]
    environment:
      PANACEA_APP_GATEWAY_LITELLM_URL: http://litellm:4001
      PANACEA_APP_GATEWAY_MASTER_KEY_FILE: /run/secrets/master_key
      PANACEA_APP_GATEWAY_DATABASE_URL_FILE: /run/secrets/database_url
      PANACEA_APP_GATEWAY_CALLER_AUTH: bearer-file:/run/secrets/callers
      PANACEA_APP_GATEWAY_ROUTE_SOURCE: file:/etc/panacea/state/projects.json
      PANACEA_APP_GATEWAY_BASE_URL: http://gatekeeper:4000
    secrets: [master_key, database_url, callers]
    volumes: ["./state:/etc/panacea/state:ro"]
    ports: ["127.0.0.1:4100:4100"]
    depends_on:
      postgres: { condition: service_healthy }
      litellm: { condition: service_healthy }

  gatekeeper:
    <<: *gateway
    command: ["/usr/local/bin/gatekeeper"]
    environment:
      PANACEA_APP_GATEWAY_LITELLM_URL: http://litellm:4001
      PANACEA_APP_GATEWAY_KEYSMITH_URL: http://keysmith:4100
      PANACEA_APP_GATEWAY_ROUTE_SOURCE: file:/etc/panacea/state/projects.json
      PANACEA_APP_GATEWAY_CALLER_TOKEN_FILE: /run/secrets/gatekeeper_token
    secrets: [gatekeeper_token]
    volumes: ["./state:/etc/panacea/state:ro"]
    ports: ["127.0.0.1:4000:4000"]
    depends_on:
      keysmith: { condition: service_started }
      litellm: { condition: service_healthy }

  fakeanthropic-global:
    <<: *gateway
    command: ["/usr/local/bin/fakeanthropic", "-addr", ":4900", "-label", "global"]

  fakeanthropic-au:
    <<: *gateway
    command: ["/usr/local/bin/fakeanthropic", "-addr", ":4901", "-label", "au"]
```

The distroless services have no shell, so they carry no `healthcheck`; `--wait` waits for them to be running, and step 7's acceptance is what proves they answer. All four share one `build`, so compose may build the image once per service; the layer cache makes the repeats instant.

**`deploy/local/gateway/compose.anthropic.yaml`, complete.** It changes the LiteLLM service only; `init.sh` adds it to `COMPOSE_FILE` when `PANACEA_APP_GATEWAY_LOCAL_UPSTREAM=anthropic`.

```yaml
# Phase 6 local mode, real Global upstream. Makes REAL SPEND on the developer's
# own Anthropic key in secrets/anthropic_api_key (never committed; init.sh
# refuses this mode without it). The AU group stays on fakeanthropic-au:
# Bedrock is never reachable from local mode.
secrets:
  anthropic_api_key: { file: ./secrets/anthropic_api_key }

services:
  litellm:
    command:
      - >-
        export LITELLM_MASTER_KEY="$$(cat /run/secrets/master_key)";
        export LITELLM_SALT_KEY="$$(cat /run/secrets/salt_key)";
        export DATABASE_URL="$$(cat /run/secrets/database_url)";
        export ANTHROPIC_API_KEY="$$(cat /run/secrets/anthropic_api_key)";
        exec litellm --config /app/config.yaml --port 4001
    secrets: [master_key, salt_key, database_url, anthropic_api_key]
    volumes: ["./litellm.anthropic.config.yaml:/app/config.yaml:ro"]
```

Compose merges an override's `volumes` by target path, so `/app/config.yaml` is replaced rather than mounted twice; the test's mount check proves it for the pinned Compose.

**`images/gateway/Dockerfile`, complete.**

```dockerfile
# Phase 6's three binaries in one distroless image (task 17). The binaries are
# cross-compiled on the host and copied in: the corporate proxy breaks
# `go mod download` inside a build (see images/sandbox/Dockerfile), and
# control-plane does not vendor. Build context: the repository root.
FROM gcr.io/distroless/static-debian12:nonroot@sha256:PIN
COPY images/gateway/bin/keysmith images/gateway/bin/gatekeeper images/gateway/bin/fakeanthropic /usr/local/bin/
USER 65532:65532
```

**`.gitignore`, appended:**

```text
# Phase 6 local mode: generated secrets and live state, and host-built binaries.
deploy/local/gateway/secrets/
deploy/local/gateway/state/
images/gateway/bin/
```

Compose's `.env` is already ignored by the root pattern `.env` (`.gitignore:6`), and it holds no secret.

- [ ] **Step 4: Write `init.sh`, `compose.testdb.yaml`, `projects.json`, the override, the Dockerfile and `.gitignore`**

The last four are complete above. `deploy/local/gateway/init.sh`:

```bash
#!/usr/bin/env bash
# init.sh [dir]: phase 6 local mode's secrets and state, created once (task 17).
# Never overwrites a secret: a new salt key or Postgres password would lock
# LiteLLM out of the data it already holds. To rotate one, delete it by hand
# and `docker compose down -v`. Prints file NAMES only, never a value.
#   PANACEA_APP_GATEWAY_LOCAL_UPSTREAM=fake (default) | anthropic
set -euo pipefail
dir="${1:-$(cd "$(dirname "$0")" && pwd)}"
mode="${PANACEA_APP_GATEWAY_LOCAL_UPSTREAM:-fake}"
case "$mode" in
  fake | anthropic) ;;
  *) echo "init.sh: PANACEA_APP_GATEWAY_LOCAL_UPSTREAM must be fake or anthropic" >&2; exit 2 ;;
esac

umask 077
mkdir -p "$dir/secrets" "$dir/state"
chmod 700 "$dir/secrets"
chmod 755 "$dir/state"

hex() { head -c "$1" /dev/urandom | od -An -tx1 | tr -d ' \n'; }

# once NAME VALUE: write secrets/NAME atomically, only if it does not exist.
# 0644 inside a 0700 directory: the directory keeps other host users out, and a
# bind-mounted 0600 file owned by the developer is unreadable by the containers'
# non-root users (65532, postgres) on a Linux host.
once() {
  local f="$dir/secrets/$1"
  if [ -e "$f" ]; then echo "kept    secrets/$1"; return 0; fi
  printf '%s\n' "$2" > "$f.tmp"
  chmod 644 "$f.tmp"
  mv "$f.tmp" "$f"
  echo "created secrets/$1"
}

once master_key "sk-$(hex 32)" # task 04 refuses a key without sk-
once salt_key "$(hex 32)"
once postgres_password "$(hex 24)" # hex: no URL escaping in database_url
pw="$(cat "$dir/secrets/postgres_password")"
once database_url "postgresql://gateway:${pw}@postgres:5432/gateway?sslmode=disable"
once controlplane_token "$(hex 32)"
once gatekeeper_token "$(hex 32)"
cp_tok="$(cat "$dir/secrets/controlplane_token")"
gk_tok="$(cat "$dir/secrets/gatekeeper_token")"
once callers "$(printf 'controlplane %s mint,refresh,revoke,usage\ngatekeeper %s bindings' "$cp_tok" "$gk_tok")"

if [ ! -e "$dir/state/projects.json" ]; then
  cp "$dir/projects.json" "$dir/state/projects.json"
  chmod 644 "$dir/state/projects.json"
  echo "created state/projects.json from the seed"
fi

files="compose.yaml"
if [ "$mode" = anthropic ]; then
  if [ ! -s "$dir/secrets/anthropic_api_key" ]; then
    echo "init.sh: anthropic mode makes real spend and needs your own key in secrets/anthropic_api_key" >&2
    exit 1
  fi
  files="compose.yaml:compose.anthropic.yaml"
fi
# Compose reads .env from the project directory; the separator is ; on Windows
# by default, so it is set explicitly. Non-secret; rewritten on every run.
printf 'COMPOSE_PATH_SEPARATOR=:\nCOMPOSE_FILE=%s\n' "$files" > "$dir/.env"
echo "mode $mode: docker compose up -d --wait"
```

On Windows, NTFS does not honour these modes (Git Bash's `chmod` sets little beyond the read-only bit); the protection there is the user profile's ACL, and that is stated, not tested. That `COMPOSE_FILE` and `COMPOSE_PATH_SEPARATOR` are honoured from `.env` is from Docker's documentation of its pre-defined variables, read from training knowledge and **UNVERIFIED** against the installed Compose; step 7 observes it (`docker compose ps` lists six services in the default mode).

`deploy/local/gateway/compose.testdb.yaml`, complete:

```yaml
# Opt-in, for task 05's pgstore tests only: publishes Postgres on loopback.
#   docker compose -f compose.yaml -f compose.testdb.yaml up -d --wait postgres
# Never add this file to COMPOSE_FILE; the default stack publishes 4000 and 4100 only.
services:
  postgres:
    ports: ["127.0.0.1:55432:5432"]
```

- [ ] **Step 5: Pin the three digests from real pulls**

No digest is written in this plan: each is read from the registry on the day and recorded in the file and in task 18's results.

```bash
~/.claude/tools/claudezsh/claudezsh 'docker pull postgres:16-alpine && docker inspect --format "{{index .RepoDigests 0}}" postgres:16-alpine'
~/.claude/tools/claudezsh/claudezsh 'docker pull gcr.io/distroless/static-debian12:nonroot && docker inspect --format "{{index .RepoDigests 0}}" gcr.io/distroless/static-debian12:nonroot'
```

For LiteLLM, pick the newest release tag marked stable on its container page (`https://github.com/BerriAI/litellm/pkgs/container/litellm`), not Panacea's `v1.63.11` (`litellm-values.yaml:29-31`), which is a year of fixes behind. Replace `TAG` and pull it:

```bash
~/.claude/tools/claudezsh/claudezsh 'docker pull ghcr.io/berriai/litellm:TAG && docker inspect --format "{{index .RepoDigests 0}}" ghcr.io/berriai/litellm:TAG'
~/.claude/tools/claudezsh/claudezsh 'docker run --rm --entrypoint /bin/sh ghcr.io/berriai/litellm:TAG -c "command -v litellm python3"'
```

Expected: a `repo@sha256:<64 hex>` line each, and two paths from the last command. Write each `repo@sha256:…` into `compose.yaml` and the Dockerfile in place of `PIN`, keeping the tag before the `@` for the reader. If `python3` is not on the image's `PATH`, the LiteLLM healthcheck names whichever interpreter `command -v python` finds; if the image has no `/bin/sh`, the entrypoint cannot export the secrets and this task stops and says so, because the only other route is secrets in the environment. That `ghcr.io` is reachable from the corporate network is **UNVERIFIED**; if the pull fails on TLS, that is an owner blocker, not a reason to use an unpinned mirror.

- [ ] **Step 6: Run the test to verify it passes**

```bash
~/.claude/tools/claudezsh/claudezsh 'bash deploy/local/gateway/compose_test.sh'
```

Expected: `PASS: compose_test (8 checks, fake and anthropic modes)` and exit 0.

**Prove two checks are not vacuous.** Change keysmith's port to `"4100:4100"` and re-run: it must fail with `keysmith: port 4100 binds 0.0.0.0`. Restore it. Add `ports: ["127.0.0.1:4001:4001"]` to `litellm` and re-run: it must fail with `litellm publishes a port`. Restore it. If either still passes, `docker compose config --format json` is rendering ports in a shape the checker does not read, and the checker is fixed before anything else.

- [ ] **Step 7: Build, start, and the acceptance run**

```bash
~/.claude/tools/claudezsh/claudezsh 'cd control-plane && for b in keysmith gatekeeper fakeanthropic; do CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o ../images/gateway/bin/$b ./cmd/$b || exit 1; done && ls ../images/gateway/bin'
~/.claude/tools/claudezsh/claudezsh 'bash deploy/local/gateway/init.sh'
~/.claude/tools/claudezsh/claudezsh 'cd deploy/local/gateway && docker compose up -d --build --wait && docker compose ps --format "{{.Service}} {{.State}} {{.Ports}}"'
```

Expected: `fakeanthropic gatekeeper keysmith`; seven `created secrets/…` lines, `created state/projects.json from the seed`, and `mode fake: docker compose up -d --wait`; then six services `running`, with ports shown only on `gatekeeper` (`127.0.0.1:4000->4000/tcp`) and `keysmith` (`127.0.0.1:4100->4100/tcp`). `GOARCH=amd64` is for this device's Docker Desktop; an Apple-silicon developer uses `arm64`.

The acceptance itself runs from a container **on the network**, because a token is bound to the address it was minted for and the host is not a sandbox. Save this as a scratch script (it has both quote kinds, so it is not passed inline) and run it with `claudezsh 'bash <path>'`:

```bash
set -euo pipefail
cd deploy/local/gateway
docker run -d --rm --name gw-accept --network panacea-app-gateway \
  curlimages/curl:8.10.1@sha256:PIN sleep 300 >/dev/null
ip="$(docker inspect -f '{{(index .NetworkSettings.Networks "panacea-app-gateway").IPAddress}}' gw-accept)"
bearer="$(cat secrets/controlplane_token)"
mint="$(curl -sS -X POST http://127.0.0.1:4100/v1/mint \
  -H "Authorization: Bearer $bearer" -H 'content-type: application/json' \
  -d "{\"sessionId\":\"accept-17\",\"generation\":1,\"owner\":{\"tid\":\"local\",\"oid\":\"dev\"},\"teamId\":\"local-team\",\"projectId\":\"local-global\",\"agentId\":\"accept\",\"sessionClass\":\"solo\",\"podAddr\":\"$ip\"}")"
token="$(printf '%s' "$mint" | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')"
[ -n "$token" ] || { echo "mint failed: $mint"; docker rm -f gw-accept >/dev/null; exit 1; }
body='{"model":"claude-opus-4-8","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}'
hdr=(-H 'anthropic-version: 2023-06-01' -H 'content-type: application/json')
docker exec gw-accept curl -sS http://gatekeeper:4000/v1/messages \
  -H "authorization: Bearer $token" "${hdr[@]}" -d "$body" | grep -o 'fake global reply'
echo "host replay: $(curl -sS -o /dev/null -w '%{http_code}' http://127.0.0.1:4000/v1/messages \
  -H "authorization: Bearer $token" "${hdr[@]}" -d "$body")"
echo "litellm from host: $(curl -sS -o /dev/null -w '%{http_code}' --max-time 2 http://127.0.0.1:4001/ || true)"
curl -sS -o /dev/null -w 'revoke: %{http_code}\n' -X POST http://127.0.0.1:4100/v1/revoke \
  -H "Authorization: Bearer $bearer" -H 'content-type: application/json' -d '{"sessionId":"accept-17"}'
docker rm -f gw-accept >/dev/null
```

Expected, four lines: `fake global reply`, `host replay: 403`, `litellm from host: 000`, `revoke: 204`. The `curlimages/curl` digest is pinned the way step 5 pins the others. The mint JSON's keys are task 03's golden (camelCase, as task 13's `projects.json` is); if task 03 fixed different keys, this script follows task 03. The `403` is the source binding at work from the host side; task 23 repeats it from a sandbox.

- [ ] **Step 8: Give task 05 its test database, and check the canary in the logs**

```bash
~/.claude/tools/claudezsh/claudezsh 'cd deploy/local/gateway && docker compose -f compose.yaml -f compose.testdb.yaml up -d --wait postgres && docker compose exec -T postgres createdb -U gateway keysmith_test && echo created'
```

Expected: `created` (or `database "keysmith_test" already exists` on a second run). Task 05's suite then runs with `PANACEA_APP_GATEWAY_TEST_DATABASE_URL=postgresql://gateway:<secrets/postgres_password>@127.0.0.1:55432/keysmith_test?sslmode=disable`, a database of its own, so its tables never meet the live `keysmith` schema. Put the stack back to the default ports afterwards with `docker compose up -d --wait`.

```bash
~/.claude/tools/claudezsh/claudezsh 'cd deploy/local/gateway && docker compose logs --no-color 2>&1 | grep -c -F -e "hello" -e "$(cat secrets/master_key)" -e "$(cat secrets/controlplane_token)" || true'
```

Expected: `0`. The acceptance prompt `hello` and both secrets appear in no container's log. This is a smoke check, not task 18's P06, which uses a unique canary and reads LiteLLM's spend-log table as well.

- [ ] **Step 9: Commit**

```bash
~/.claude/tools/claudezsh/claudezsh 'git add deploy/local/gateway/compose.yaml deploy/local/gateway/compose.anthropic.yaml deploy/local/gateway/compose.testdb.yaml deploy/local/gateway/init.sh deploy/local/gateway/compose_test.sh deploy/local/gateway/projects.json images/gateway/Dockerfile'
~/.claude/tools/claudezsh/claudezsh 'git commit -F /tmp/msg-task17.txt -- deploy/local/gateway/compose.yaml deploy/local/gateway/compose.anthropic.yaml deploy/local/gateway/compose.testdb.yaml deploy/local/gateway/init.sh deploy/local/gateway/compose_test.sh deploy/local/gateway/projects.json images/gateway/Dockerfile .gitignore'
```

with `/tmp/msg-task17.txt` holding:

```text
feat(gateway): local mode on Docker Desktop

Postgres, LiteLLM, keysmith, gatekeeper and two fake upstreams on one
network, panacea-app-gateway, which the local control plane's sandboxes
join. Only the gatekeeper (4000) and keysmith (4100) are published, on
127.0.0.1; LiteLLM publishes nothing, so nothing reaches it around the
gatekeeper.

init.sh writes the secrets once, as files, and never overwrites one. Our
binaries read *_FILE variables; LiteLLM's entrypoint exports its three from
the mounted files, so no value is in compose.yaml, the rendered config or
docker inspect. Images are pinned by digest; ours is distroless with
host-built binaries, because the corporate proxy breaks go mod download in
a build.

compose_test.sh checks the rendered config in both upstream modes: loopback
ports only, no LiteLLM port, digests, the exact network name, no secret in
the environment, no debug flag, and init.sh's idempotence.

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
```

**What this task does NOT claim:**

- **Anything about the cloud.** The chart, the NetworkPolicies and Key Vault are task 19; this compose file is not a model of them beyond the shared names.
- **That a container on the network cannot reach LiteLLM.** It can: compose has no NetworkPolicy. A local sandbox holding its own session key could call `litellm:4001` directly and skip the source binding and the live route. Local mode is for development (owner question Q4), and task 19's policy closes this in the cloud.
- **That LiteLLM honours the rendered config.** It starts and answers readiness; which keys it obeys is task 18's.
- **That the file modes protect anything on Windows.** NTFS ignores them; the user profile's ACL is the protection.
- **That the chosen LiteLLM, Postgres, distroless and curl digests are safe or current.** They are what the registry served on the day, recorded for task 18.
- **The `anthropic` mode's correctness.** It needs task 02's `local-anthropic` target to render `litellm.anthropic.config.yaml`, and it makes real spend on a developer's key; this task's acceptance runs only the `fake` mode.
- **The route names in `secrets/callers`.** They follow the skeleton's scope for task 08; task 08's part is authoritative.
