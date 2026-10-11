# CLOUD-080/082 live preview cancellation — product scope

Final product correction passes **87 race outcomes across 36 top-level tests**, zero failures/skips. Every selected Go/module input remained unchanged within each run. No Docker build, installed cancellation, browser workload or abrupt-controller-death acceptance was run here.

## Behavior

POST `/v1/product/projects/{id}/releases/{releaseId}/cancel` accepts `{expectedRevision}`. The public local Release adds `revision`, `canCancel` and `cancelRequested`. Only this controller’s live exact operation can be signalled. Cancellation saves intent before cancelling the owned context, returns HTTP202/cancelling and does not wait for the runtime lifecycle lock or invoke Docker. Retrying the original accepted revision while the same operation is still live returns HTTP200; stale/nonlive/reopened requests refuse. Both reads of current Deploy authority are required: the second occurs after waiting for the product mutex, independently of request-disconnect propagation. Execute is not required.

The producer merges only the exact cancellation-intent revision into its next receipt; unrelated newer results and changed runtime identities remain refused. Confirmed no-demand cancellation becomes cancelled. Unknown build/pull or an acknowledged serving allocation remains unknown/charged, preserving exact Stop capability where supported. A cancellation winning the final ready receipt returns422, not201. Server.Close signals owned previews, uses one five-second join budget, then seals remaining publishers and retains uncertainty. Reopen never recreates a live cancellation handle or replays work.

## Retained runs

- `before`: original product overlay + two new regressions; exit1, two expected failures (cancel404 and Close returned without cancelling). 796 unchanged inputs. This original run used default Go testing without race.
- `fixed-1`: first correction, exit0, 84 race outcomes/33 top-level tests. 798 unchanged inputs. This predates the fresh-authority follow-up and does not replace its later counterexample.
- `authority-before`: an authoritative grant changed while the request context remained live and the product mutex was held; cancellation incorrectly returned202 with only one authority read. Exit1, one expected failing race test, 798 unchanged inputs.
- `fixed-2`: final correction, exit0, 87 race outcomes/36 top-level tests, 798 unchanged inputs. It includes the original counterexamples, both authority timing cases, initially empty allocation forgery, immutable identity advancement, exclusive source-guard control, pre-allocation cancellation, exact intent CAS, settled/unknown/serving outcomes, late-ready422, bounded sealing/reopen and existing installer Close behavior.

Earlier/final counts overlap and must not be summed. Management, frontend and actual API/ledger integration are separate capsules. Unknown Docker build recovery remains an existing limitation; cancellation alone does not establish daemon settlement or retire a charge.

## Source and invocation bindings

Each run retains its exact invocation, underlying exit, complete JSONL output and before/after selected-input inventories. The checkout had concurrent authorized documentation/frontend work; no whole-checkout-clean claim is made. `selected-product-sources.json` maps each touched product file in each run to exact content-addressed bytes. Intermediate test files were reconstructed from final bytes and verified against their original input SHA256 before inclusion. `product.patch` is the complete product-only change from the frozen original files. The final runner includes all modes; each original invocation is authoritative for the executed arguments. Tests used activated Go1.26.3, GOMAXPROCS2, -p1, default vet, readonly modules, canonical shared caches and a 150-second wrapper bound (100-second package timeout).

No private pre-existing fixture, execution ledger or unrelated state was changed. Test projects are owned disposable fixtures. This capsule contains code and synthetic test receipts, not fixture contents or credentials.

Public capsule storage: the eight broad before/after input inventories are losslessly gzip-compressed; `compressed-input-bindings.json` preserves their original byte lengths and SHA256. Decompression reproduces the exact original inventory bytes. [Focused Windows cases](WINDOWS-FOCUSED-CASES.md) are proposed, unexecuted follow-up checks.
