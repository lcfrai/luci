# Private assembly catalog source check — 2026-10-16

Reviewed source input: `58459c093ffec926b01e5a1a6eb3c6f842a2fb95` (catalog `aea45f76d`, fence corrections `240f483ba` and `58459c093`). Merged with the repair-budget/PDF source branch at `c7bca7eac92a81753a21f05b97faecd1cfea3950`. `git diff --check` passed.

One serial source batch ran from `control-plane` using Go 1.25.0, `GOWORK=off`, `GOMAXPROCS=1`, `GOMEMLIMIT=512MiB`, `GOGC=20`, existing `/dev/shm/luci-*` task caches, and a 300-second command timeout:

```
go test -p 1 -parallel 1 -count=1 -run '^TestAssemblyCatalog' ./internal/api ./internal/backend/dockerbe ./internal/backend
```

Initial cgroup charge/max are in [precheck.txt](precheck.txt). Exit 0: six `TestAssemblyCatalog*` API tests passed; both affected backend packages compiled, with no matching tests. Raw [go-test.log](go-test.log) SHA-256 `9481289a370294aa8e32aad99ea34a00b400736a23e88c5cd6b7d0038f038a07`.

The previously passed repair-budget and unchanged desktop tests were not rerun. Linux compilation does not verify the Windows handle/fence cases. No assembler module, payload, image, Docker, native runtime, or provider effect was executed.
