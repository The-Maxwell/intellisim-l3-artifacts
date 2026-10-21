# Regenerates the llmbridge proto in the sibling SDK checkout and rebuilds
# both repos. Use after manually editing
# ../intellisim.llm.provider.go.sdk/proto/llmbridge/v1/llmbridge.proto.
# Prerequisites: see the SDK's scripts/gen-proto.ps1 header.
$ErrorActionPreference = 'Stop'
$core = Split-Path $PSScriptRoot -Parent
$sdk = Join-Path (Split-Path $core -Parent) 'intellisim.llm.provider.go.sdk'
$gen = Join-Path $sdk 'scripts\gen-proto.ps1'
if (-not (Test-Path $gen)) { throw "SDK checkout not found at $sdk" }

& powershell -ExecutionPolicy Bypass -File $gen

Push-Location $sdk
try { go build ./... } finally { Pop-Location }

Push-Location $core
try { go build ./cmd/... ./api/... ./modules/llm/... ./data/... ./pkg/... } finally { Pop-Location }

Write-Host 'OK: proto regenerated, SDK and core both rebuilt'
