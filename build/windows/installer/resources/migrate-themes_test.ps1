$ErrorActionPreference = "Stop"

$root = Join-Path ([IO.Path]::GetTempPath()) ("fancontrol-theme-test-" + [guid]::NewGuid().ToString("N"))
$installDir = Join-Path $root "install\themes"
$bundleDir = Join-Path $root "bundle"
$userDir = Join-Path $root "user"
$scriptPath = Join-Path $PSScriptRoot "migrate-themes.ps1"

function Write-TestTheme {
    param(
        [string]$Root,
        [string]$Id,
        [string]$Version,
        [string]$Css
    )

    $dir = Join-Path $Root $Id
    New-Item -ItemType Directory -Path $dir -Force | Out-Null
    @{ id = $Id; name = $Id; base = "light"; version = $Version } |
        ConvertTo-Json -Compress |
        Set-Content -LiteralPath (Join-Path $dir "theme.json") -Encoding UTF8
    Set-Content -LiteralPath (Join-Path $dir "theme.css") -Value $Css -NoNewline -Encoding UTF8
}

try {
    Write-TestTheme $bundleDir "dune" "2.0.0" "bundled-dune"
    Write-TestTheme $bundleDir "thrm" "1.0.0" "bundled-thrm"
    Write-TestTheme $installDir "dune" "1.0.0" "user-edited-dune"
    Write-TestTheme $installDir "my-theme" "9.0.0" "user-theme"
    New-Item -ItemType Directory -Path $userDir -Force | Out-Null

    $previousUserProfile = $env:USERPROFILE
    $env:USERPROFILE = $userDir
    try {
        & "$env:SystemRoot\System32\WindowsPowerShell\v1.0\powershell.exe" -NoProfile -ExecutionPolicy Bypass -File $scriptPath -InstallThemesDir $installDir -BundledThemesDir $bundleDir
        if ($LASTEXITCODE -ne 0) {
            throw "migrate-themes.ps1 exited with $LASTEXITCODE"
        }
    } finally {
        $env:USERPROFILE = $previousUserProfile
    }

    if ((Get-Content -Raw -LiteralPath (Join-Path $installDir "dune\theme.css")) -ne "user-edited-dune") {
        throw "existing official theme was overwritten"
    }
    if ((Get-Content -Raw -LiteralPath (Join-Path $installDir "my-theme\theme.css")) -ne "user-theme") {
        throw "user theme was overwritten"
    }
    if ((Get-Content -Raw -LiteralPath (Join-Path $installDir "thrm\theme.css")) -ne "bundled-thrm") {
        throw "missing official theme was not installed"
    }

    Write-Host "theme migration preservation test passed"
} finally {
    Remove-Item -LiteralPath $root -Recurse -Force -ErrorAction SilentlyContinue
}
