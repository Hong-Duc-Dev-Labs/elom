<#
.SYNOPSIS
Generates or verifies SHA256SUMS for Elom Windows release artifacts.

.DESCRIPTION
Computes SHA-256 hashes for .exe and .msi release artifacts in the build output
directory and writes them in standard sha256sum format (`<hash>  <filename>`) to
SHA256SUMS. With -Verify, validates existing artifacts against SHA256SUMS.
#>
[CmdletBinding()]
param(
    [string]$Dir = "build\windows-amd64",
    [switch]$Verify
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path -LiteralPath $Dir)) {
    throw "Artifact directory not found: $Dir"
}

$sumsPath = Join-Path $Dir "SHA256SUMS"

if ($Verify) {
    if (-not (Test-Path -LiteralPath $sumsPath)) {
        throw "SHA256SUMS not found: $sumsPath"
    }
    $lines = Get-Content -LiteralPath $sumsPath | Where-Object { $_.Trim() -ne "" }
    $failed = 0
    foreach ($line in $lines) {
        if ($line -notmatch '^([0-9a-fA-F]{64})\s+\*?(.+)$') {
            Write-Error "Malformed SHA256SUMS line: $line"
            $failed++
            continue
        }
        $expected = $Matches[1].ToLowerInvariant()
        $name = $Matches[2].Trim()
        $target = Join-Path $Dir $name
        if (-not (Test-Path -LiteralPath $target)) {
            Write-Error "Missing artifact: $name"
            $failed++
            continue
        }
        $actual = (Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($actual -ne $expected) {
            Write-Error "Checksum mismatch for ${name}: expected $expected, got $actual"
            $failed++
        } else {
            Write-Output "OK  $name"
        }
    }
    if ($failed -gt 0) {
        exit 1
    }
    exit 0
}

$artifacts = Get-ChildItem -LiteralPath $Dir -File |
    Where-Object { $_.Extension -in @(".exe", ".msi") } |
    Sort-Object Name

if ($artifacts.Count -eq 0) {
    throw "No .exe or .msi release artifacts found in $Dir"
}

$entries = foreach ($file in $artifacts) {
    $hash = (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
    "$hash  $($file.Name)"
}

Set-Content -LiteralPath $sumsPath -Value $entries -Encoding ascii
Write-Output "Wrote $sumsPath ($($entries.Count) artifacts):"
$entries | ForEach-Object { Write-Output "  $_" }
