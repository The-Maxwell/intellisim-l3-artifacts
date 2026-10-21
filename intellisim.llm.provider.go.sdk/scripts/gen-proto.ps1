# Regenerates proto/llmbridge/v1/*.pb.go from llmbridge.proto.
# Prerequisites (once): go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
#                       go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.1
# protoc resolution order: $env:PROTOC -> PATH -> auto-download pinned version into tools/.
$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
$protocVersion = '29.3'

$protoc = $env:PROTOC
if (-not $protoc) {
    $cmd = Get-Command protoc -ErrorAction SilentlyContinue
    if ($cmd) { $protoc = $cmd.Source }
}
if (-not $protoc) {
    $dir = Join-Path $root "tools\protoc-$protocVersion"
    $protoc = Join-Path $dir 'bin\protoc.exe'
    if (-not (Test-Path $protoc)) {
        Write-Host "protoc not found - downloading v$protocVersion to tools\ ..."
        New-Item -ItemType Directory -Force $dir | Out-Null
        $zip = Join-Path $root "tools\protoc-$protocVersion-win64.zip"
        Invoke-WebRequest "https://github.com/protocolbuffers/protobuf/releases/download/v$protocVersion/protoc-$protocVersion-win64.zip" -OutFile $zip
        Expand-Archive $zip -DestinationPath $dir -Force
        Remove-Item $zip
    }
}

& $protoc --version
& $protoc `
    --proto_path="$root\proto" `
    --go_out="$root\proto" --go_opt=paths=source_relative `
    --go-grpc_out="$root\proto" --go-grpc_opt=paths=source_relative `
    "llmbridge/v1/llmbridge.proto"
Write-Host 'OK: generated proto/llmbridge/v1/llmbridge.pb.go + llmbridge_grpc.pb.go'
