# SPDX-License-Identifier: MIT
# Copyright (C) 2026 WireHush. All Rights Reserved.
# Pinned generators: protoc 36.2, protoc-gen-go v1.36.12,
# protoc-gen-go-grpc v1.6.2, Grpc.Tools 2.84.0.
[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
$taskRepository = Split-Path -Parent $PSScriptRoot
Push-Location -LiteralPath $taskRepository
try {
    $taskGenerators = @('.deps/protoc/bin/protoc.exe', '.deps/bin/protoc-gen-go.exe', '.deps/bin/protoc-gen-go-grpc.exe', '.deps/grpc.tools/tools/windows_x64/grpc_csharp_plugin.exe')
    foreach ($taskGenerator in $taskGenerators) {
        if (-not (Test-Path -LiteralPath $taskGenerator)) { throw "Missing pinned protocol generator: $taskGenerator" }
    }
    & $taskGenerators[0] --plugin=protoc-gen-go=.deps/bin/protoc-gen-go.exe --plugin=protoc-gen-go-grpc=.deps/bin/protoc-gen-go-grpc.exe --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative --csharp_out=protocol/csharp --plugin=protoc-gen-grpc=.deps/grpc.tools/tools/windows_x64/grpc_csharp_plugin.exe --grpc_out=protocol/csharp --grpc_opt=no_server protocol/wirehush.proto
    if ($LASTEXITCODE -ne 0) { throw "Protocol generation failed: $LASTEXITCODE" }
} finally { Pop-Location }
