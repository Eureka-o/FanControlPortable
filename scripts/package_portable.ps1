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
    "PawnIO_setup.exe",
    "libusb-1.0.dll"
)

foreach ($file in $requiredFiles) {
    Require-File (Join-Path $buildBin $file)
}

Require-Directory (Join-Path $buildBin "bridge")
Require-Directory (Join-Path $buildBin "themes")

function Invoke-WithFileLockRetry {
    param(
        [scriptblock]$Action,
        [int]$MaxAttempts = 6,
        [int]$BaseDelayMs = 400
    )

    for ($attempt = 1; $attempt -le $MaxAttempts; $attempt++) {
        try {
            & $Action
            return
        } catch {
            $ex = $_.Exception
            $isTransientLock = ($ex -is [System.IO.IOException]) -or
                ($ex -is [System.UnauthorizedAccessException]) -or
                ($ex.InnerException -is [System.IO.IOException]) -or
                ($ex.InnerException -is [System.UnauthorizedAccessException])
            if (-not $isTransientLock -or $attempt -ge $MaxAttempts) {
                throw
            }
            Start-Sleep -Milliseconds ($BaseDelayMs * $attempt)
        }
    }
}

function New-PortableZipWithReadWriteShare {
    param(
        [string]$SourceDir,
        [string]$ZipPath
    )

    Add-Type -AssemblyName System.IO.Compression
    Add-Type -AssemblyName System.IO.Compression.FileSystem

    $rootFull = [System.IO.Path]::GetFullPath($SourceDir).TrimEnd([System.IO.Path]::DirectorySeparatorChar, [System.IO.Path]::AltDirectorySeparatorChar)
    if (Test-Path -LiteralPath $ZipPath) {
        Remove-Item -LiteralPath $ZipPath -Force
    }

    $zipStream = [System.IO.File]::Open($ZipPath, [System.IO.FileMode]::CreateNew, [System.IO.FileAccess]::ReadWrite, [System.IO.FileShare]::None)
    try {
        $archive = New-Object System.IO.Compression.ZipArchive($zipStream, [System.IO.Compression.ZipArchiveMode]::Create, $false)
        try {
            foreach ($file in Get-ChildItem -LiteralPath $rootFull -Recurse -File) {
                $relativePath = $file.FullName.Substring($rootFull.Length + 1).Replace('\', '/')
                $entry = $archive.CreateEntry($relativePath, [System.IO.Compression.CompressionLevel]::Optimal)
                $entry.LastWriteTime = $file.LastWriteTime
                $entryStream = $entry.Open()
                try {
                    $fileStream = [System.IO.File]::Open($file.FullName, [System.IO.FileMode]::Open, [System.IO.FileAccess]::Read, [System.IO.FileShare]::ReadWrite)
                    try {
                        $fileStream.CopyTo($entryStream)
                    } finally {
                        $fileStream.Dispose()
                    }
                } finally {
                    $entryStream.Dispose()
                }
            }
        } finally {
            $archive.Dispose()
        }
    } finally {
        $zipStream.Dispose()
    }
}

$safeOutputDir = Resolve-InRepoPath $outputDir
if (Test-Path -LiteralPath $safeOutputDir) {
    Invoke-WithFileLockRetry {
        if (Test-Path -LiteralPath $safeOutputDir) {
            Remove-Item -LiteralPath $safeOutputDir -Recurse -Force
        }
    }
}
New-Item -ItemType Directory -Path $safeOutputDir | Out-Null

Copy-Item -LiteralPath (Join-Path $buildBin "FanControl.exe") -Destination (Join-Path $safeOutputDir "FanControl.exe")
Copy-Item -LiteralPath (Join-Path $buildBin "FanControl Core.exe") -Destination (Join-Path $safeOutputDir "FanControl Core.exe")
Copy-Item -LiteralPath (Join-Path $buildBin "PawnIO_setup.exe") -Destination (Join-Path $safeOutputDir "PawnIO_setup.exe")
Copy-Item -LiteralPath (Join-Path $buildBin "libusb-1.0.dll") -Destination (Join-Path $safeOutputDir "libusb-1.0.dll")
Copy-Item -LiteralPath (Join-Path $buildBin "bridge") -Destination (Join-Path $safeOutputDir "bridge") -Recurse
Copy-Item -LiteralPath (Join-Path $buildBin "themes") -Destination (Join-Path $safeOutputDir "themes") -Recurse

$zipPath = Join-Path $buildBin "FanControl-$Version-portable.zip"
Invoke-WithFileLockRetry {
    New-PortableZipWithReadWriteShare -SourceDir $safeOutputDir -ZipPath $zipPath
}
Write-Host "Portable package created: $zipPath"

if ($CopyToRoot) {
    $rootArtifact = Join-Path (Split-Path -Parent $repoRoot.Path) "FanControl-$Version-portable.zip"
    Copy-Item -LiteralPath $zipPath -Destination $rootArtifact -Force
    Write-Host "Portable package copied to: $rootArtifact"
}
