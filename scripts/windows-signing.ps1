#requires -Version 7.0
# Maintainer-only helpers. Dot-sourcing does not inspect certificates or sign files.
function Get-LuciCodeSigningCertificate([string]$Thumbprint) {
    Get-Item -LiteralPath ('Cert:\CurrentUser\My\' + $Thumbprint) -ErrorAction Stop
}

function Get-LuciSignTool([string]$SignToolPath = '') {
    if ($SignToolPath) {
        if ([IO.Path]::GetExtension($SignToolPath) -ne '.exe') { throw 'SignToolPath must name an executable.' }
        return (Get-LuciOrdinarySigningFile $SignToolPath).FullName
    }
    (Get-Command signtool.exe -CommandType Application -ErrorAction Stop).Source
}

function Get-LuciWindowsSigningContext([string]$Thumbprint, [string]$TimestampUrl, [string]$SignToolPath = '') {
    if (-not $IsWindows -or $Thumbprint -notmatch '^[0-9a-fA-F]{40}$') { throw 'Windows signing requires an exact CurrentUser certificate-store SHA1 thumbprint.' }
    $luciTimestamp = $null
    if (-not [Uri]::TryCreate($TimestampUrl, [UriKind]::Absolute, [ref]$luciTimestamp) -or $luciTimestamp.Scheme -ne 'https' -or -not $luciTimestamp.Host -or $luciTimestamp.UserInfo -or $luciTimestamp.Fragment -or $luciTimestamp.Query) { throw 'Use an absolute HTTPS RFC3161 timestamp endpoint without credentials, query or fragment.' }
    $luciTool = Get-LuciSignTool $SignToolPath
    $luciCertificate = Get-LuciCodeSigningCertificate $Thumbprint
    $luciUsages = @($luciCertificate.Extensions | Where-Object { $_.Oid.Value -eq '2.5.29.37' } | ForEach-Object { $_.EnhancedKeyUsages } | ForEach-Object { $_.Value })
    if ($luciCertificate.Thumbprint -ne $Thumbprint -or -not $luciCertificate.HasPrivateKey -or $luciCertificate.NotAfter -le [DateTime]::Now -or $luciCertificate.NotBefore -gt [DateTime]::Now -or $luciUsages -notcontains '1.3.6.1.5.5.7.3.3') { throw 'The provisioned certificate must match, be current, contain a private key, and permit code signing.' }
    return @{thumbprint=$Thumbprint.ToUpperInvariant(); timestampUrl=$TimestampUrl; tool=$luciTool; subject=$luciCertificate.Subject; issuer=$luciCertificate.Issuer}
}

function Get-LuciWindowsVerificationContext([string]$Thumbprint, [string]$SignToolPath = '') {
    # Consumer verification needs only the independently supplied public signer.
    # Never resolve a certificate store entry or require possession of its key.
    if (-not $IsWindows -or $Thumbprint -notmatch '^[0-9a-fA-F]{40}$') { throw 'Windows verification requires an exact expected public signer SHA1 thumbprint.' }
    return @{thumbprint=$Thumbprint.ToUpperInvariant(); tool=(Get-LuciSignTool $SignToolPath)}
}

function Get-LuciTauriSigningCommand([hashtable]$Context, [string]$PowerShellPath, [string]$ScriptPath, [string]$CoreSHA256) {
    # Tauri's object form preserves spaces in paths and substitutes only %1.
    # Reuse the bounded signer for the shell, installer and generated uninstaller.
    return @{cmd=$PowerShellPath; args=@('-NoProfile','-NonInteractive','-File',$ScriptPath,'-CertificateThumbprint',$Context.thumbprint,'-TimestampUrl',$Context.timestampUrl,'-SignToolPath',$Context.tool,'-CoreSHA256',$CoreSHA256,'-Path','%1')}
}

function Invoke-LuciSignTool([string]$Tool, [string[]]$Arguments) {
    # No shell, password argument or exported key. Bound timestamp/trust-service
    # outages; a killed/unknown operation cannot produce a verified receipt.
    $luciInfo = [Diagnostics.ProcessStartInfo]::new()
    $luciInfo.FileName = $Tool; $luciInfo.UseShellExecute = $false; $luciInfo.CreateNoWindow = $true
    $luciInfo.RedirectStandardOutput = $true; $luciInfo.RedirectStandardError = $true
    foreach ($luciArgument in $Arguments) { $luciInfo.ArgumentList.Add($luciArgument) }
    $luciProcess = [Diagnostics.Process]::new(); $luciProcess.StartInfo = $luciInfo; $luciStarted = $false
    try {
        if (-not $luciProcess.Start()) { throw 'SignTool did not start.' }
        $luciStarted = $true
        # Discard tool output while draining both pipes; no raw provider output
        # or certificate-device prompts are placed in the release manifest.
        $luciOut = $luciProcess.StandardOutput.BaseStream.CopyToAsync([IO.Stream]::Null)
        $luciErr = $luciProcess.StandardError.BaseStream.CopyToAsync([IO.Stream]::Null)
        if (-not $luciProcess.WaitForExit(120000)) { throw 'SignTool exceeded its 120-second deadline.' }
        if (-not [Threading.Tasks.Task]::WaitAll([Threading.Tasks.Task[]]@($luciOut,$luciErr), 5000)) { throw 'SignTool output readers did not settle.' }
        if ($luciProcess.ExitCode -ne 0) { throw ('SignTool failed or warned (exit ' + $luciProcess.ExitCode + ').') }
    } finally {
        try {
            if ($luciStarted) {
                if (-not $luciProcess.HasExited) {
                    $luciProcess.Kill($true)
                    if (-not $luciProcess.WaitForExit(5000)) { throw 'SignTool cleanup remains unknown; stop release preparation.' }
                }
                if (-not [Threading.Tasks.Task]::WaitAll([Threading.Tasks.Task[]]@($luciOut,$luciErr), 5000)) { throw 'SignTool output cleanup remains unknown; stop release preparation.' }
            }
        } finally { $luciProcess.Dispose() }
    }
}

function Get-LuciOrdinarySigningFile([string]$Path) {
    if (-not [IO.Path]::IsPathFullyQualified($Path) -or [IO.Path]::GetExtension($Path) -notin @('.exe','.dll')) { throw 'Signing requires an absolute Windows executable or library path.' }
    $luciFile = Get-Item -LiteralPath $Path -Force -ErrorAction Stop
    if ($luciFile.PSIsContainer -or $luciFile.Length -le 0) { throw 'Signing input must be a nonempty file.' }
    $luciAncestor = $luciFile
    while ($null -ne $luciAncestor) {
        if ($luciAncestor.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Linked signing inputs or ancestors are refused.' }
        $luciAncestor = if ($luciAncestor -is [IO.FileInfo]) { $luciAncestor.Directory } else { $luciAncestor.Parent }
    }
    return $luciFile
}

function Confirm-LuciWindowsCoreIdentity([string]$Path, [string]$ExpectedSHA256) {
    $luciFile = Get-LuciOrdinarySigningFile $Path
    if ($ExpectedSHA256 -notmatch '^[0-9a-f]{64}$' -or (Get-FileHash -LiteralPath $luciFile.FullName -Algorithm SHA256).Hash -ne $ExpectedSHA256) { throw 'Windows core changed after signing; refuse mismatched archive/sidecar identity.' }
}

function Confirm-LuciWindowsSignature([string]$Path, [hashtable]$Context, [string]$Role) {
    $luciFile = Get-LuciOrdinarySigningFile $Path
    Invoke-LuciSignTool $Context.tool @('verify','/pa','/all','/tw',$luciFile.FullName)
    $luciSignature = Get-AuthenticodeSignature -LiteralPath $luciFile.FullName
    if ($luciSignature.Status -ne 'Valid' -or -not $luciSignature.SignerCertificate -or $luciSignature.SignerCertificate.Thumbprint -ne $Context.thumbprint -or -not $luciSignature.TimeStamperCertificate) { throw ('Trusted publisher signature/timestamp verification failed: ' + $Role) }
    $luciTimestampUsages = @($luciSignature.TimeStamperCertificate.Extensions | Where-Object { $_.Oid.Value -eq '2.5.29.37' } | ForEach-Object { $_.EnhancedKeyUsages } | ForEach-Object { $_.Value })
    if ($luciTimestampUsages -notcontains '1.3.6.1.5.5.7.3.8') { throw ('Timestamp signer does not permit timestamping: ' + $Role) }
    return @{role=$Role; name=$luciFile.Name; bytes=$luciFile.Length; sha256=(Get-FileHash -Algorithm SHA256 -LiteralPath $luciFile.FullName).Hash.ToLowerInvariant(); status='verified-authenticode'; signerThumbprint=$luciSignature.SignerCertificate.Thumbprint; signerSubject=$luciSignature.SignerCertificate.Subject; timestampSignerThumbprint=$luciSignature.TimeStamperCertificate.Thumbprint; timestampProtocol='RFC3161-requested'}
}

function Set-LuciWindowsSignature([string]$Path, [hashtable]$Context, [string]$Role) {
    $luciFile = Get-LuciOrdinarySigningFile $Path
    Invoke-LuciSignTool $Context.tool @('sign','/s','My','/sha1',$Context.thumbprint,'/fd','SHA256','/tr',$Context.timestampUrl,'/td','SHA256','/d','Luci',$luciFile.FullName)
    return Confirm-LuciWindowsSignature $luciFile.FullName $Context $Role
}

function Confirm-LuciInstalledWindowsSignatures([string]$InstallDirectory, [hashtable]$Context) {
    $luciUninstallers = @(Get-ChildItem -LiteralPath $InstallDirectory -File -Filter '*uninstall*.exe')
    if ($luciUninstallers.Count -ne 1) { throw 'Expected one installed NSIS uninstaller for signature verification.' }
    Confirm-LuciWindowsSignature (Join-Path $InstallDirectory 'luci.exe') $Context 'installed-app'
    Confirm-LuciWindowsSignature (Join-Path $InstallDirectory 'luci-api.exe') $Context 'installed-api'
    Confirm-LuciWindowsSignature $luciUninstallers[0].FullName $Context 'installed-uninstaller'
}
