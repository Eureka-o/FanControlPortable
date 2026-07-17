param(
    [string]$OutputRoot = ""
)

$ErrorActionPreference = "Stop"

$pluginRoot = Split-Path -Parent $PSCommandPath
$repoRoot = [System.IO.Path]::GetFullPath((Join-Path $pluginRoot "..\.."))
if ([string]::IsNullOrWhiteSpace($OutputRoot)) {
    $OutputRoot = Join-Path $repoRoot "build\bin"
} elseif (-not [System.IO.Path]::IsPathRooted($OutputRoot)) {
    $OutputRoot = Join-Path $repoRoot $OutputRoot
}
$OutputRoot = [System.IO.Path]::GetFullPath($OutputRoot)
$pluginOutput = [System.IO.Path]::GetFullPath((Join-Path $OutputRoot "plugins\omen-fan"))
$expectedParent = [System.IO.Path]::GetFullPath((Join-Path $OutputRoot "plugins"))

if (-not $pluginOutput.StartsWith($expectedParent + [System.IO.Path]::DirectorySeparatorChar, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Refusing to stage the plugin outside the output plugins directory: $pluginOutput"
}

New-Item -ItemType Directory -Force -Path $OutputRoot | Out-Null
New-Item -ItemType Directory -Force -Path $expectedParent | Out-Null
if (Test-Path -LiteralPath $pluginOutput) {
    Remove-Item -LiteralPath $pluginOutput -Recurse -Force
}
New-Item -ItemType Directory -Force -Path (Join-Path $pluginOutput "backend") | Out-Null

Push-Location $repoRoot
try {
    & go build -buildvcs=false -trimpath -ldflags "-s -w" -o (Join-Path $pluginOutput "backend\omen-fan-driver.exe") ./plugins/omen-fan/backend
    if ($LASTEXITCODE -ne 0) {
        throw "OMEN preview backend build failed with exit code $LASTEXITCODE"
    }
} finally {
    Pop-Location
}

Copy-Item -LiteralPath (Join-Path $pluginRoot "src\plugin.json") -Destination (Join-Path $pluginOutput "plugin.json")
Copy-Item -LiteralPath (Join-Path $pluginRoot "src\ui") -Destination (Join-Path $pluginOutput "ui") -Recurse
Copy-Item -LiteralPath (Join-Path $pluginRoot "THIRD_PARTY_NOTICES.md") -Destination (Join-Path $pluginOutput "THIRD_PARTY_NOTICES.md")

$zipPath = Join-Path $OutputRoot "omen-fan-0.1.0-windows-amd64.zip"
if (Test-Path -LiteralPath $zipPath) {
    Remove-Item -LiteralPath $zipPath -Force
}
Compress-Archive -LiteralPath $pluginOutput -DestinationPath $zipPath -CompressionLevel Optimal

$makensisCandidates = @(
    "C:\Program Files (x86)\NSIS\makensis.exe",
    "C:\Program Files\NSIS\makensis.exe"
)
$makensis = $makensisCandidates | Where-Object { Test-Path -LiteralPath $_ } | Select-Object -First 1
$setupPath = Join-Path $OutputRoot "omen-fan-setup.exe"
if ($makensis) {
    $installerScript = Join-Path $pluginRoot "installer\omen-fan-setup.nsi"
    & $makensis "/DPLUGIN_SOURCE=$pluginOutput" "/DOUTPUT_FILE=$setupPath" "/DPLUGIN_VERSION=0.1.0" $installerScript
    if ($LASTEXITCODE -ne 0) {
        throw "OMEN plugin installer build failed with exit code $LASTEXITCODE"
    }
} else {
    Write-Warning "NSIS was not found; the plugin folder and ZIP were built without an installer."
}

Write-Host "OMEN preview plugin staged at: $pluginOutput"
Write-Host "OMEN preview plugin ZIP: $zipPath"
if (Test-Path -LiteralPath $setupPath) {
    Write-Host "OMEN preview plugin installer: $setupPath"
}
