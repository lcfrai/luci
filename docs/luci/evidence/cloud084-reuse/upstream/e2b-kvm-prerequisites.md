# E2B Embed with Docker Compose

Runs the whole of Embed, the control plane and a real Firecracker
orchestrator alike, directly on one Linux host you own: no Terraform, no cloud
account, no wrapper scripts, and `docker compose up` as the whole interface.
It is a single-machine package rather than a deployment pattern;
the hub is [`../README.md`](../README.md).

## Requirements

- Linux x86-64 or arm64 with KVM and a 4 KiB-page kernel (Ubuntu's default
  on both), on a host you own and may mutate: bare metal, or a VM created with
  nested virtualization enabled (on GCE, `--enable-nested-virtualization`; on
  EC2, `--cpu-options NestedVirtualization=enabled` on an Intel type that
  offers it, such as `m8i`; on Apple silicon, a Lima or other
  Virtualization.framework VM with nested virtualization on, which needs an
  M3 or newer and macOS 15 or newer). x86-64
  is what the guides were written and tested on. arm64 is verified end to
  end on bare metal: on 2026-09-14 a bare-metal arm64 host (a GCE `c4a` metal
  instance, Ubuntu 26.04, kernel 7.0) reached `ready` in under two minutes
  and passed the smoke test, sandbox create, pause and resume, template
  builds from arm64 images and 64 concurrent sandboxes. arm64 needs kernel
  6.10 or newer: restoring a sandbox needs userfaultfd write-protect, which
  arm64 gained in Linux 6.10, so on Ubuntu 24.04's stock 6.8 kernel the stack
  is healthy but every sandbox start fails with `Failed to UFFD object` until
  `linux-generic-hwe-24.04` is installed. The nine pinned images are
  published for both architectures, and `fetch-artifacts` verifies the arm64
  orchestrator and envd against the `.sha256` their release writes beside
  the object. A pin with no such object still stops with a `FIX:` line
  naming it.
- Ubuntu 24.04 is the recommended host: kernel 6.8 or newer (6.10 or newer on
  arm64, above), glibc 2.34 or newer (the released orchestrator's floor),
  cgroup v2 (systemd's default),
  and `iptables`, `rsync`, `e2fsprogs` and `iproute2` installed. Ubuntu
  server has all four but not `python3-venv`, which [Try it](#try-it)
  needs. Refresh the index first, or a package that is genuinely missing
  fails to fetch:
  `sudo apt-get update && sudo apt-get install -y iptables rsync e2fsprogs iproute2 python3-venv`.
- `/dev/kvm` and `/dev/net/tun` present. On a VM, `/dev/kvm` means nested
  virtualization is enabled for it; on GCE, stop the VM, run
  `gcloud compute instances update <vm> --enable-nested-virtualization`, and
  start it again; on EC2, stop the instance, run
  `aws ec2 modify-instance-cpu-options --instance-id <id> --nested-virtualization enabled`,
  and start it again. Without either device, `up` stops at `preflight` with a
  `FIX:` line and nothing that needs KVM is started.
- Docker Engine 27 or newer with Compose 2.24 or newer, and your user in the
  `docker` group. Docker's own apt repository is the simplest way to get
  both: `curl -fsSL https://get.docker.com | sh && sudo usermod -aG docker
  $USER`, then log in again. Ubuntu 24.04's `docker-compose-v2` package also
  clears the Compose floor. `docker version` and `docker compose version`
  show what you have; an
  older Compose rejects the compose file with an error on `required:` before
  anything starts.
- 12 GiB RAM recommended and 20 GiB free disk. A pause the disk cannot hold is
  refused before it starts, keeping 1 GiB free
  (`PAUSE_ADMISSION_DISK_HEADROOM_MIB`), instead of failing midway; the
  estimate counts the sandbox's whole memory, so a large sandbox needs that
  much free disk to pause. `HUGEPAGES=2048` reserves
  4 GiB of that RAM for sandboxes, and `preflight` does not check RAM, so on
  a smaller host the first signal is `host-setup` failing with `FIX: give the
  host more memory (12 GiB recommended) or lower HUGEPAGES`. Lower
  `HUGEPAGES` rather than assuming 8 GiB is always enough. Size it by the
  sandboxes you will run at once: every sandbox takes its guest memory from
  this reservation as it touches pages, up to its full size (256 of the 2 MiB
  pages for a 512 MiB `base` sandbox, about 90 while idle), so the default
  carries eight sandboxes at full memory or about twenty idle ones. Past
  that the stack stays healthy and each create fails with `500: Failed to
  place sandbox` (the same 500 an exhausted NBD pool gives; the orchestrator
  log tells them apart) while `docker compose logs orchestrator` shows `uffd
  process exited: failed to wrap memfd: mmap memfd: cannot allocate memory`.
  Set a
  larger `HUGEPAGES` in `.env` and run `up` again: `host-setup` re-applies
  the sysctl on every start and the orchestrator needs no restart.
- Outbound HTTPS to Docker Hub, `us-docker.pkg.dev`, `storage.googleapis.com`,
