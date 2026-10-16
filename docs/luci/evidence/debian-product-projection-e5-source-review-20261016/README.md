# Debian product projection resources: frozen source review

Reviewed immutable commit `e5b11e80c9c54ec7f9192af0ba08ee7a2303d792` from `codex/product-projection-resources-v1`. Seven exact Git blob pins:

| Resource under `control-plane/internal/product/resources/debian_signature/v1/` | Git blob | Bytes |
| --- | --- | ---: |
| `LICENSE.txt` | `8e9ef1a09913e815836529bfe21b0ddeb9108574` | 1,077 |
| `producer_config.json` | `03fe10f51cfb433d892819d54e4e69f5b9fbd7c7` | 12,245 |
| `producer_source_receipt.json` | `2855634156201caa8684dbeaa5d7bb2f98cf0101` | 12,073 |
| `projection_entry.py` | `32ebcdebf9beffaf9612502087ab704ba09798d9` | 14,156 |
| `projection_protocol.json` | `475fe32a46a57d70e3a3be0cf0b3780cb34e5a38` | 8,035 |
| `status_projection.py` | `841a7b31436a0b120f01c54cd7303ed690efbf4c` | 15,522 |
| `test_projection_entry.py` | `176f01aaec9e8e3db18627cad7332e0314a1e536` | 14,397 |

Source finding: no additional blocking defect found in this seven-resource review. `projection_entry.py:168–206,300–318` closes the ordered 13-role request, exact config/source hashes and fixed names. `:47–79,128–165` charges reads before opening and latches refusal; `:255–290,321–334` bounds canonical output, opens exclusively and emits a fixed small error. `test_projection_entry.py:74–199,234–289` uses synthetic shape, budget and output fixtures. The Go raw output DTO at the fetched source head has matching JSON names; this was a narrow shape comparison only. The unchanged 8401 pure copy received no second semantic audit.

Qualification: `projection_entry.py:144–145` opens before its own regular-file check; a blocking/symlinked path remains the missing Go owner's original-file and namespace/lifetime responsibility. `producer_source_receipt.json:3–5,253–260` is explicitly incomplete. Current-host interpreter/tool ownership, copied originals and actual native results remain unavailable. No imports, parser, compiler, tests, GPG, acquisition or native action ran for this review. Source review is not product eligibility or signature proof.
