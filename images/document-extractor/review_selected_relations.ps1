# Data-only comparison of retained metadata; does not invoke dpkg or a resolver.
param([Parameter(Mandatory=$true)][string]$Output)
$ErrorActionPreference='Stop'
$lock=Get-Content (Join-Path $PSScriptRoot 'dependency-lock.json') -Raw|ConvertFrom-Json -AsHashtable
function CharacterOrder([string]$text,[int]$index){
 if($index -ge $text.Length){return 0}
 $c=[int][char]$text[$index]
 if($c -eq 126){return -1}
 if($c -ge 48 -and $c -le 57){return 0}
 if(($c -ge 65 -and $c -le 90) -or ($c -ge 97 -and $c -le 122)){return $c}
 return ($c+256)
}
function CompareVersionPart([string]$left,[string]$right){
 $i=0;$j=0
 while($i -lt $left.Length -or $j -lt $right.Length){
  while(($i -lt $left.Length -and $left[$i] -notmatch '[0-9]') -or ($j -lt $right.Length -and $right[$j] -notmatch '[0-9]')){
   $a=CharacterOrder $left $i;$b=CharacterOrder $right $j
   if($a -ne $b){return [Math]::Sign($a-$b)}
   if($i -lt $left.Length){$i++};if($j -lt $right.Length){$j++}
  }
  while($i -lt $left.Length -and $left[$i] -eq '0'){$i++}
  while($j -lt $right.Length -and $right[$j] -eq '0'){$j++}
  $startI=$i;$startJ=$j
  while($i -lt $left.Length -and $left[$i] -match '[0-9]'){$i++}
  while($j -lt $right.Length -and $right[$j] -match '[0-9]'){$j++}
  $sizeI=$i-$startI;$sizeJ=$j-$startJ
  if($sizeI -ne $sizeJ){return [Math]::Sign($sizeI-$sizeJ)}
  $cmp=[string]::CompareOrdinal($left.Substring($startI,$sizeI),$right.Substring($startJ,$sizeJ))
  if($cmp -ne 0){return [Math]::Sign($cmp)}
 }
 return 0
}
function SplitVersion([string]$value){
 if($value.Length -gt 256 -or $value -notmatch '^(?:(\d+):)?([0-9][A-Za-z0-9.+~:\-]*)$'){throw 'unsupported Debian version'}
 $epoch=if($Matches[1]){[long]$Matches[1]}else{0};$rest=$Matches[2];$dash=$rest.LastIndexOf('-')
 if($dash -ge 0){return @($epoch,$rest.Substring(0,$dash),$rest.Substring($dash+1))}
 return @($epoch,$rest,'0')
}
function CompareVersion([string]$left,[string]$right){
 $a=SplitVersion $left;$b=SplitVersion $right
 if($a[0] -ne $b[0]){return [Math]::Sign($a[0]-$b[0])}
 $cmp=CompareVersionPart $a[1] $b[1]
 if($cmp -ne 0){return $cmp};return (CompareVersionPart $a[2] $b[2])
}
$packages=@{};foreach($package in $lock.debianPackages){if($packages.ContainsKey($package.name)){throw 'duplicate package'};$packages[$package.name]=$package}
$edges=@();$failures=@()
foreach($package in $lock.debianPackages){foreach($choice in $package.dependencyChoices){
 $target=$packages[$choice.selected];if(!$target){throw 'missing selected target'}
 $constraint=$choice.constraint;$passed=$true
 if($constraint){
  if($constraint -notmatch '^(<<|<=|=|>=|>>) (\S+)$'){throw 'unsupported constraint'}
  $operator=$Matches[1];$required=$Matches[2];$cmp=CompareVersion $target.version $required
  $passed=switch($operator){'<<'{$cmp -lt 0};'<='{$cmp -le 0};'='{$cmp -eq 0};'>='{$cmp -ge 0};'>>'{$cmp -gt 0}}
 }
 $edge=@{from=$package.name;to=$target.name;version=$target.version;constraint=$constraint;passed=$passed}
 $edges+=$edge;if(!$passed){$failures+=$edge}
}}
$result=@{version=1;state='retained-unauthenticated-metadata-relations';packageCount=$packages.Count;edgeCount=$edges.Count;failures=$failures;edges=$edges;authenticated=$false;completeControlTransaction=$false;limitations=@('Depends/Pre-Depends choices are from retained package HTML; installed/authenticated binary control fields not yet observed.','Provides/Breaks/Conflicts/Replaces and maintainer-script effects still require bounded exact archive inspection.','The mawk virtual-provider choice is recorded separately; no unconstrained virtual package version is invented.');lockSha256=(Get-FileHash (Join-Path $PSScriptRoot 'dependency-lock.json') -Algorithm SHA256).Hash.ToLowerInvariant()}
if(Test-Path -LiteralPath $Output){throw 'receipt destination must be fresh'}
$result|ConvertTo-Json -Depth 12|Set-Content -LiteralPath $Output -Encoding utf8
Write-Output "packages=$($packages.Count) edges=$($edges.Count) failed=$($failures.Count) authenticated=false"
if($failures.Count){exit 1}
