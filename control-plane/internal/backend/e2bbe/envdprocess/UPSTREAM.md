# E2B envd process wire bindings

Source: e2b-dev/runtime commit `cf536c284434fdd2f09bbab35ec807a5dacc87e4`, `packages/shared/pkg/grpc/envd/process/` and `packages/envd/spec/process/process.proto`. Apache-2.0 licence retained in LICENSE.

The protobuf and proto files are unmodified. The Connect generated file changes only its Go import path from the upstream process package to this local package. This copies wire bindings, not the runtime or a coding engine. Connect Go is pinned to the runtime's `v1.18.1`; Luci does not import the complete infrastructure module.

Official source URLs and SHA-256 values are retained in the CLOUD-084 guest transport evidence capsule.
