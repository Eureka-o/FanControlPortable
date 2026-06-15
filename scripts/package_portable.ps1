param(
    [string]$Version = "",
    [switch]$CopyToRoot
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Resolve-Path (Join-Path $scriptDir "..")
$buildBin = Join-Path $repoRoot "build\bin"
$outputDir = Join-Path $repoRoot "build\output"

function Resolve-InRepoPath {
    param([string]$Path)

    $fullPath = [System.IO.Path]::GetFullPath($Path)
    $rootPath = [System.IO.Path]::GetFullPath($repoRoot.Path)
    if (-not $fullPath.StartsWith($rootPath, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "Refusing to operate outside repo: $fullPath"
    }
    return $fullPath
}

function Require-File {
    param([string]$Path)

    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        throw "Required file missing: $Path"
    }
}

function Require-Directory {
    param([string]$Path)

    if (-not (Test-Path -LiteralPath $Path -PathType Container)) {
        throw "Required directory missing: $Path"
    }
}

if ([string]::IsNullOrWhiteSpace($Version)) {
    $wailsConfigPath = Join-Path $repoRoot "wails.json"
    Require-File $wailsConfigPath
    $wailsConfig = Get-Content -LiteralPath $wailsConfigPath -Raw | ConvertFrom-Json
    $Version = [string]$wailsConfig.info.productVersion
}

if ([string]::IsNullOrWhiteSpace($Version)) {
    throw "Version is empty. Pass -Version or set info.productVersion in wails.json."
}

Require-Directory $buildBin

$requiredFiles = @(
    "FanControl.exe",
    "FanControl Core.exe",
    "PawnIO_setup.exe"
)

foreach ($file in $requiredFiles) {
    Require-File (Join-Path $buildBin $file)
}

Require-Directory (Join-Path $buildBin "bridge")

$safeOutputDir = Resolve-InRepoPath $outputDir
if (Test-Path -LiteralPath $safeOutputDir) {
    Remove-Item -LiteralPath $safeOutputDir -Recurse -Force
}
New-Item -ItemType Directory -Path $safeOutputDir | Out-Null

Copy-Item -LiteralPath (Join-Path $buildBin "FanControl.exe") -Destination (Join-Path $safeOutputDir "FanControl.exe")
Copy-Item -LiteralPath (Join-Path $buildBin "FanControl Core.exe") -Destination (Join-Path $safeOutputDir "FanControl Core.exe")
Copy-Item -LiteralPath (Join-Path $buildBin "PawnIO_setup.exe") -Destination (Join-Path $safeOutputDir "PawnIO_setup.exe")
Copy-Item -LiteralPath (Join-Path $buildBin "bridge") -Destination (Join-Path $safeOutputDir "bridge") -Recurse

foreach ($dirName in @("themes")) {
    $sourceDir = Join-Path $buildBin $dirName
    if (Test-Path -LiteralPath $sourceDir -PathType Container) {
        Copy-Item -LiteralPath $sourceDir -Destination (Join-Path $safeOutputDir $dirName) -Recurse
    }
}

$zipPath = Join-Path $buildBin "FanControl-$Version-portable.zip"
if (Test-Path -LiteralPath $zipPath) {
    Remove-Item -LiteralPath $zipPath -Force
}

Compress-Archive -Path (Join-Path $safeOutputDir "*") -DestinationPath $zipPath -Force
Write-Host "Portable package created: $zipPath"

if ($CopyToRoot) {
    $rootArtifact = Join-Path (Split-Path -Parent $repoRoot.Path) "FanControl-$Version-portable.zip"
    Copy-Item -LiteralPath $zipPath -Destination $rootArtifact -Force
    Write-Host "Portable package copied to: $rootArtifact"
}
