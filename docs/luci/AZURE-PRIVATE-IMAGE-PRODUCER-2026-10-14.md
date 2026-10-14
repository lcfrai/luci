# Private Azure image producer reconciliation

This extends SETUP-02 and the private foundation/controller staging contract. Dylan authorized making local use work on constrained PCs and preparing easy Azure setup without live cloud execution. Root selects an explicitly owned temporary private image builder for offline implementation. This adds a resource class to the reviewed setup plan; it does not authorize provisioning now. Implementation begins after exact worker acknowledgments and the root ledger entry. Independent local runtime, foundation, recovery and retirement work continues.

## Selected route and boundaries

Azure Compute Gallery accepts a VM source, requiring write permission on that VM; VHD-source publication instead requires storage account key access. Keep the private state account's shared-key prohibition intact. Use the documented VM-source capture route. Generalization removes machine-specific state and is not a reusable running-builder checkpoint. [Gallery sources and permissions](https://learn.microsoft.com/en-us/azure/virtual-machines/shared-image-galleries), [generalization](https://learn.microsoft.com/en-us/azure/virtual-machines/generalize).

Prepare two sequential, independently owned clean builds, one runtime image and one controller image, with at most one builder VM active. Each uses an exact supported base-image version, selected region/SKU and the existing verified private VNet/route/egress. Its VM, NIC and managed OS disk have explicit IDs, source/role/operation tags and ownership receipts. No public IP, new NAT/gateway/VPN/Bastion, shared-key permission or implicit resource adoption is introduced. Do not re-use a generalized VM for the next role.

The generated plan includes builder compute duration, selected size, disk, image/gallery retention, permissions and a bounded operation deadline. Unknown prices and final charges stay explicit; estimates are not a hard provider billing cap. The user's eventual effectful setup command requires its exact cost/resource acknowledgment under existing authority. A lost acknowledgement retains the operation and possible charges; use authenticated exact readback rather than replaying create, install, generalize or capture.

Use existing authenticated private transfer and bounded commands to stage the source-bound role payload. Verify exact bytes, upstream versions and licence inventory, installed service/configuration and successful role preparation before capture. Do not bake credentials, personal projects, runtime journals, private keys, tokens or an active controller generation into an image. Bind account-specific private configuration later through the established initialization/fencing path. Builder access must not grant read access to runtime secrets merely for convenience.

Persist intent before each external effect: allocation, source transfer/install, deprovision, deallocate, generalize, gallery publication and retirement. Generalization and its uncertainty cannot silently restart an installation. Capture readback must bind the selected VM/OS disk, source/role, gallery definition and exact version. Preserve the published image/version and its retained costs separately from temporary VM/NIC/disk cleanup. Delete only positively owned temporary resources after the relevant terminal/readback conditions; incomplete cleanup remains visible and charge-bearing.

## Owner and acceptance map

| Requirement | Implementation owner | Required evidence |
| --- | --- | --- |
| SETUP-02; LC-06/12/14 easy BYO preparation | Local Sol `/root/main_branch_reconciliation`: generated commands, private builder plan, source staging and gallery join | Offline command composition, exact source/role validation, no secret-bearing image, bounded transfer and actionable missing-private-route/egress prerequisites |
| LC-11/13 authority and recovery | Same owner: durable intents, cost acknowledgment, refusal, restart and exact retirement | Fixtures for denied authority, drift, lost response at every effect boundary, no mutation replay, retained image/disk/VM charges and unrelated-resource preservation |
| LC-06/15/16 runtime and swarm isolation | Existing Azure runtime owner/path unchanged | A builder VM and an image are not task microVM evidence; actual per-agent isolation, fan-out, retirement and billing remain live acceptance gates |
| Integration and independent checks | Root integrates; existing cloud Sol performs targeted Linux checks when its runtime transfer checkpoint permits | Exact committed source, applicable Python/Go/Terraform checks, no duplicate unchanged suites and no cloud account/resource calls |

The scoped additional Azure actions are VM read/write/runCommand/deallocate/generalize/delete, disk read/write/delete, NIC read/write/delete, selected subnet join, and gallery/image/version read/write. The installer must show their exact resource scope. Any extra resource class or authority must return for reconciliation before implementation. Source and fixture success cannot mark Azure connected or provider acceptance complete.
