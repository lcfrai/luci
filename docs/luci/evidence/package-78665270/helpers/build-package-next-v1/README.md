# Next exact-source package wrapper

Preparation only: no actual build, release rename, source change or runtime occurred. Root supplies the new immutable clean/retrievable source after its combined commit. The current prior source is exactly `5f4a966f6d70264c92ee2176dde912e1b43fc3c7`.

The original wrapper fixed its prior identity and preservation label to edd0. This small change requires `--expected-previous-source`, validates distinct complete40-character lowercase SHA values, verifies the actual prior manifest's exact source/clean flag, hashes every declared artifact plus manifest/checksum file before rename, preserves the prior directory under `0.5.0-<PRIOR8>-preserved-before-<NEW8>`, and rechecks the preserved byte identities before building. Prior receipt/archive/package bytes remain intact; no old fixture/ledger is touched. Destination collision refuses. No stale104-input runtime assumption is made; this helper builds only the control-plane/web package. Runtime760 reuse still requires the root's independent current106-input parity check.

The repository build command, inherited supported toolchain, GOMAXPROCS2, GOFLAGS `-mod=readonly -p=1`,600-second build budget,10-second TERM and10-second KILL settlement, and existing180-second package-verifier invocation are unchanged. A clean new HEAD must be present under an origin remote-tracking ref before output creation. No resource/admission limit or package verifier is weakened.

Root command after its exact clean commit and coordinated window:

```sh
python3 -B /workspace/.luci-tools/build-package-next-v1/build-package.py \
  --expected-source NEW_FULL_40_HEX_COMMIT \
  --expected-previous-source 5f4a966f6d70264c92ee2176dde912e1b43fc3c7 \
  --window-granted
```

`NEW_FULL_40_HEX_COMMIT` is an intentionally invalid placeholder; replace with the actual published candidate. Script SHA256 `8771e5c661745b4cb4bfefc9aea0839f947dec534d3a438459bda0ecc409e418`. The parameterization diff and exact original `150b7e999d7238eb765f3025e9e42f904001828e37defdde39e780a937d463f1` remain beside it.

Ten controlled cases pass against tiny disposable files/fake subprocess responses: original prior-source refusal; correct transition preserving complete selected package/source bytes; wrong or dirty prior; artifact tampering; occupied destination; invalid/same source; linked release; dirty current source refusing before build/output. These are guard/transition controls, not a new package build or Windows acceptance. Current verifier/build-source bindings are recorded separately; no toolchain installation or network changes were made.
