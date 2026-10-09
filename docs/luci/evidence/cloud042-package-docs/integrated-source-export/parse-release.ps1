$ErrorActionPreference = 'Stop'
$target = '<source-checkout>/scripts/build-release.ps1'
$before = (Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash.ToLowerInvariant()
$tokens = $null
$parseErrors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile($target, [ref]$tokens, [ref]$parseErrors)
$after = (Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash.ToLowerInvariant()
$result = [ordered]@{ source = 'scripts/build-release.ps1'; sourceSHA256 = $before; sourceStable = ($before -eq $after); platform = $PSVersionTable.Platform.ToString(); version = $PSVersionTable.PSVersion.ToString(); errors = @($parseErrors | ForEach-Object { [ordered]@{ id = $_.ErrorId; line = $_.Extent.StartLineNumber; message = $_.Message } }); scope = 'PowerShell parser only on Linux; no packaging commands executed or Windows runtime proof' }
$result | ConvertTo-Json -Depth 4
if ($parseErrors.Count -ne 0 -or $before -ne $after) { exit 1 }
