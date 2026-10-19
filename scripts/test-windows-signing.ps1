#requires -Version 7.0
# Offline behavioural checks only: never accesses Cert:, invokes SignTool,
# signs an executable, queries a service or builds a package.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
. (Join-Path $PSScriptRoot 'windows-signing.ps1')
$script:Checks = 0
$script:Thumbprint = 'A' * 40
function Assert-True([bool]$Condition, [string]$Name) {
    if (-not $Condition) { throw ('Failed: ' + $Name) }; $script:Checks++
}
function Assert-Refused([scriptblock]$Action, [string]$Name) {
    $luciRefused = $false
    try { & $Action | Out-Null } catch { $luciRefused = $true }
    Assert-True $luciRefused $Name
}
function Assert-SourceBefore([string]$Source, [string]$Before, [string]$After, [string]$Name) {
    $luciBefore = $Source.IndexOf($Before)
    $luciAfter = $Source.IndexOf($After)
    Assert-True ($luciBefore -ge 0 -and $luciAfter -gt $luciBefore) $Name
}
function New-TestCertificate([string]$Usage = '1.3.6.1.5.5.7.3.3') {
    [pscustomobject]@{Thumbprint=$script:Thumbprint;HasPrivateKey=$true;NotBefore=[DateTime]::Now.AddDays(-1);NotAfter=[DateTime]::Now.AddDays(1);Subject='CN=Offline fixture';Issuer='CN=Offline fixture issuer';Extensions=@([pscustomobject]@{Oid=[pscustomobject]@{Value='2.5.29.37'};EnhancedKeyUsages=@([pscustomobject]@{Value=$Usage})})}
}
$script:CertificateReads = 0
function Get-LuciCodeSigningCertificate([string]$Thumbprint) { $script:CertificateReads++; return $script:Certificate }
function Get-LuciSignTool([string]$SignToolPath = '') { return $(if ($SignToolPath) { $SignToolPath } else { 'C:\fixture\signtool.exe' }) }
$script:Certificate = New-TestCertificate
$luciContext = Get-LuciWindowsSigningContext $script:Thumbprint 'https://timestamp.example.test'
Assert-True ($luciContext.thumbprint -eq $script:Thumbprint) 'exact signer selected'
$luciExplicitTool = 'C:\Program Files (x86)\Windows Kits\signtool.exe'
$luciExplicitContext = Get-LuciWindowsSigningContext $script:Thumbprint 'https://timestamp.example.test' $luciExplicitTool
$luciCertificateReads = $script:CertificateReads
$luciVerifyContext = Get-LuciWindowsVerificationContext $script:Thumbprint.ToLowerInvariant() $luciExplicitTool
Assert-True ($luciVerifyContext.thumbprint -eq $script:Thumbprint -and $luciVerifyContext.tool -eq $luciExplicitTool -and $script:CertificateReads -eq $luciCertificateReads) 'public verification context never resolves a signing key'
Assert-Refused { Get-LuciWindowsVerificationContext 'not-a-thumbprint' $luciExplicitTool } 'malformed expected public signer refused'
$luciTauriCommand = Get-LuciTauriSigningCommand $luciExplicitContext 'C:\Program Files\PowerShell\pwsh.exe' 'C:\source with spaces\sign-windows-file.ps1' ('c' * 64)
Assert-True ($luciTauriCommand.args -contains $luciExplicitTool -and $luciTauriCommand.args[-1] -eq '%1') 'explicit tool and file placeholder survive spaces without shell parsing'
foreach ($luciUrl in @('http://timestamp.example.test','https://user:password@timestamp.example.test','https://timestamp.example.test/#fragment','https://timestamp.example.test/?token=secret','relative')) {
    Assert-Refused { Get-LuciWindowsSigningContext $script:Thumbprint $luciUrl } 'unsafe timestamp endpoint'
}
Assert-Refused { Get-LuciWindowsSigningContext 'not-a-thumbprint' 'https://timestamp.example.test' } 'malformed signer refused'
foreach ($luciFault in @('key','expired','future','usage','mismatch')) {
    $script:Certificate = New-TestCertificate
    switch ($luciFault) {
        key { $script:Certificate.HasPrivateKey = $false }
        expired { $script:Certificate.NotAfter = [DateTime]::Now.AddDays(-1) }
        future { $script:Certificate.NotBefore = [DateTime]::Now.AddDays(1) }
        usage { $script:Certificate = New-TestCertificate '1.3.6.1.5.5.7.3.1' }
        mismatch { $script:Certificate.Thumbprint = 'B' * 40 }
    }
    Assert-Refused { Get-LuciWindowsSigningContext $script:Thumbprint 'https://timestamp.example.test' } ('invalid certificate ' + $luciFault)
}

$luciFixture = Join-Path ([IO.Path]::GetTempPath()) ('luci-signing-offline-' + [guid]::NewGuid().ToString('N') + '.exe')
[IO.File]::WriteAllText($luciFixture, 'inert text fixture; never executed or signed')
$script:Calls = [Collections.Generic.List[object]]::new()
$script:ToolFailure = $false
function Invoke-LuciSignTool([string]$Tool, [string[]]$Arguments) {
    if ($script:ToolFailure) { throw 'Fixture SignTool trust/timestamp rejection.' }
    $script:Calls.Add(@{tool=$Tool;arguments=$Arguments})
}
$script:BadInstalledLeaf = ''
function Get-AuthenticodeSignature {
    param([string]$LiteralPath)
    if ($script:BadInstalledLeaf -and [IO.Path]::GetFileName($LiteralPath) -eq $script:BadInstalledLeaf) {
        $luciBad = New-TestSignature
        $luciBad.SignerCertificate.Thumbprint = 'B' * 40
        return $luciBad
    }
    return $script:Signature
}
function New-TestSignature {
    [pscustomobject]@{Status='Valid';SignerCertificate=(New-TestCertificate);TimeStamperCertificate=(New-TestCertificate '1.3.6.1.5.5.7.3.8')}
}
try {
    $script:Signature = New-TestSignature
    $luciRecord = Set-LuciWindowsSignature $luciFixture $luciContext 'fixture-core'
    Assert-True ($luciRecord.sha256 -eq (Get-FileHash -LiteralPath $luciFixture -Algorithm SHA256).Hash.ToLowerInvariant()) 'receipt binds final bytes'
    Confirm-LuciWindowsCoreIdentity $luciFixture $luciRecord.sha256
    Assert-Refused { Confirm-LuciWindowsCoreIdentity $luciFixture ('0' * 64) } 'post-bundle core mutation refused even when signature mock remains valid'
    $luciSign = $script:Calls[0].arguments
    Assert-True ($luciSign[0] -eq 'sign' -and $luciSign -contains '/sha1' -and $luciSign -contains '/tr' -and $luciSign -contains '/td' -and $luciSign -contains 'SHA256' -and $luciSign -notcontains '/p' -and $luciSign -notcontains '/f') 'explicit signer and RFC3161 without key export/password'
    Assert-True (($script:Calls[1].arguments -join ' ') -match '^verify /pa /all /tw ') 'trust and timestamp warning verification'
    foreach ($luciFault in @('mismatch','untrusted','missingTimestamp','wrongTimestampUsage')) {
        $script:Signature = New-TestSignature
        switch ($luciFault) {
            mismatch { $script:Signature.SignerCertificate.Thumbprint = 'B' * 40 }
            untrusted { $script:Signature.Status = 'NotTrusted' }
            missingTimestamp { $script:Signature.TimeStamperCertificate = $null }
            wrongTimestampUsage { $script:Signature.TimeStamperCertificate = New-TestCertificate }
        }
        Assert-Refused { Confirm-LuciWindowsSignature $luciFixture $luciContext 'fixture-core' } ('refuse signature ' + $luciFault)
    }
    $script:Signature = New-TestSignature; $script:ToolFailure = $true
    Assert-Refused { Confirm-LuciWindowsSignature $luciFixture $luciContext 'fixture-core' } 'invalid timestamp or trust-policy tool failure'
    $script:ToolFailure = $false
    Assert-Refused { Set-LuciWindowsSignature 'relative.exe' $luciContext 'fixture-core' } 'relative signing target refused'

    # Parse the builder; do not invoke it. Require-signed refusal must precede
    # signer lookup and all build work. Core signing must precede archive/identity
    # creation and native sidecar copy so those receipts cover the signed bytes.
    $luciBuilder = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot 'build-release.ps1')
    $luciEarlyRefusal = $null
    try { & (Join-Path $PSScriptRoot 'build-release.ps1') -Native -RequireSigned -RuntimePayloadDir 'does-not-exist' } catch { $luciEarlyRefusal = $_.Exception.Message }
    Assert-True ($luciEarlyRefusal -like 'RequireSigned requires*') 'actual builder refuses missing signer before touching absent build inputs'
    Assert-True ($luciBuilder.IndexOf('if ($RequireSigned -and') -lt $luciBuilder.IndexOf('Get-LuciWindowsSigningContext')) 'require-signed refuses before build'
    $luciSignOffset = $luciBuilder.IndexOf("'windows-portable-core'")
    Assert-True ($luciSignOffset -ge 0 -and $luciSignOffset -lt $luciBuilder.IndexOf("'package.py'")) 'core signed before package hash/archive'
    Assert-True ($luciSignOffset -lt $luciBuilder.IndexOf("'luci-api-x86_64-pc-windows-msvc.exe'")) 'signed core copied as sidecar'

    # Actual verification helper, inert files and mocked trust tools only.
    # The installed uninstaller must pass the same public signer boundary.
    $luciInstalled = Join-Path ([IO.Path]::GetTempPath()) ('luci-installed-signing-offline-' + [guid]::NewGuid().ToString('N'))
    [IO.Directory]::CreateDirectory($luciInstalled) | Out-Null
    $luciInstalledNames = @('luci.exe','luci-api.exe','uninstall.exe','extra-uninstall.exe')
    try {
        foreach ($luciName in $luciInstalledNames[0..2]) { [IO.File]::WriteAllText((Join-Path $luciInstalled $luciName), 'inert installed signature fixture; never executed') }
        $script:Signature = New-TestSignature
        $luciInstalledRecords = @(Confirm-LuciInstalledWindowsSignatures $luciInstalled $luciVerifyContext)
        Assert-True (($luciInstalledRecords.role -join ',') -eq 'installed-app,installed-api,installed-uninstaller') 'all three installed executables verified'
        foreach ($luciName in $luciInstalledNames[0..2]) {
            $script:BadInstalledLeaf = $luciName
            Assert-Refused { Confirm-LuciInstalledWindowsSignatures $luciInstalled $luciVerifyContext } ('installed signer mismatch refuses ' + $luciName)
        }
        $script:BadInstalledLeaf = ''
        [IO.File]::WriteAllText((Join-Path $luciInstalled 'extra-uninstall.exe'), 'inert duplicate fixture')
        Assert-Refused { Confirm-LuciInstalledWindowsSignatures $luciInstalled $luciVerifyContext } 'ambiguous installed uninstaller refused'
        [IO.File]::Delete((Join-Path $luciInstalled 'extra-uninstall.exe'))
        [IO.File]::Delete((Join-Path $luciInstalled 'uninstall.exe'))
        Assert-Refused { Confirm-LuciInstalledWindowsSignatures $luciInstalled $luciVerifyContext } 'missing installed uninstaller refused'
    } finally {
        foreach ($luciName in $luciInstalledNames) { [IO.File]::Delete((Join-Path $luciInstalled $luciName)) }
        [IO.Directory]::Delete($luciInstalled)
    }
    $luciInstalledSource = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot 'test-installed-windows.ps1')
    Assert-SourceBefore $luciInstalledSource "'candidate-installer'" 'New-Item -ItemType Directory' 'expected signer checked before installation setup'
    Assert-SourceBefore $luciInstalledSource 'Confirm-LuciInstalledWindowsSignatures $installDir' 'function Run-UI' 'installed verification is part of each install before UI'
    Assert-SourceBefore $luciInstalledSource "'uninstaller-before-execution'" 'Run-Installer $uninstallers[0].FullName' 'exact installed uninstaller reverified before execution'
    Assert-True ($luciInstalledSource.Contains('unsigned-developer-mode-not-signing-acceptance')) 'unsigned developer mode is explicitly not signed acceptance'
} finally { Remove-Item -LiteralPath $luciFixture -Force }
Write-Output ('Offline signing boundary checks passed: ' + $script:Checks + '; no cryptographic or package acceptance claimed.')
