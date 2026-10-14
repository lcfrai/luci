#requires -Version 7.0
# Maintainer entry point for Tauri's supported signCommand; never used at startup.
param(
    [Parameter(Mandatory=$true)][string]$Path,
    [Parameter(Mandatory=$true)][string]$CertificateThumbprint,
    [Parameter(Mandatory=$true)][string]$TimestampUrl,
    [Parameter(Mandatory=$true)][string]$SignToolPath,
    [Parameter(Mandatory=$true)][string]$CoreSHA256
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
. (Join-Path $PSScriptRoot 'windows-signing.ps1')
$luciContext = Get-LuciWindowsSigningContext $CertificateThumbprint $TimestampUrl $SignToolPath
if ([IO.Path]::GetFileName($Path) -match '^luci-api(?:-x86_64-pc-windows-msvc)?\.exe$') {
    # Never timestamp/sign the already archived core again: that changes bytes.
    Confirm-LuciWindowsCoreIdentity $Path $CoreSHA256
    Confirm-LuciWindowsSignature $Path $luciContext 'tauri-existing-core' | Out-Null
} else {
    # Tauri also signs owned NSIS plugin DLL copies; leave cached originals alone.
    Set-LuciWindowsSignature $Path $luciContext 'tauri-signing-input' | Out-Null
}
