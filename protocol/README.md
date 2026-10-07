# WireHush manager protocol

`wirehush.proto` is the single source of truth for the Go manager and C# frontend.
Do not edit generated bindings or recreate IPC models independently.

Version 1.0 uses gRPC over the authenticated local Windows pipe only. Unknown
runtime values have explicit optional presence; zero traffic is distinct from
unknown traffic. Windows identity comes from the pipe, never a request. The
manager derives a private reference's OwnerSID from the authenticated caller.
Sharing transitions are not advertised or supported.

Generated files are committed so ordinary Go builds do not require protoc. C#
uses `Google.Protobuf` and `Grpc.Net.Client`/`Grpc.Core.Api`; no independent schema.

Regenerate with `scripts/generate-protocol.ps1` using these pinned tools:

- protoc 36.2 Windows x64, SHA256
  `f0c128dc0d8492eceece83bb459a4c0e316764b929ffbf1aa416357fd644edd3`
- protoc-gen-go v1.36.12
- protoc-gen-go-grpc v1.6.2
- Grpc.Tools 2.84.0 Windows x64 plugin

Compiler tools are development-only and are never packaged into the installer.
Protocol evolution must preserve field numbers and enum meanings. Reserve removed
numbers/names; major incompatibility requires an explicit version change.
