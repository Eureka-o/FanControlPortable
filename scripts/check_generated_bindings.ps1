param(
    [Parameter(Mandatory = $true)]
    [string]$SnapshotPath,
    [Parameter(Mandatory = $true)]
    [ValidateSet("Capture", "Verify")]
    [string]$Mode
)

$ErrorActionPreference = "Stop"
$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$paths = @(
    "frontend\wailsjs\go\main\App.d.ts",
    "frontend\wailsjs\go\main\App.js",
    "frontend\wailsjs\go\models.ts",
    "frontend\wailsjs\runtime\runtime.d.ts",
    "frontend\wailsjs\runtime\runtime.js",
    "frontend\wailsjs\runtime\package.json"
)

$utf8NoBom = New-Object System.Text.UTF8Encoding($false)
$sha256 = [System.Security.Cryptography.SHA256]::Create()
try {
    $current = foreach ($relativePath in $paths) {
        $path = Join-Path $repoRoot $relativePath
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
            throw "Generated Wails binding is missing: $relativePath"
        }
        $normalizedText = [System.IO.File]::ReadAllText($path, $utf8NoBom) -replace "`r`n", "`n"
        $hash = [System.BitConverter]::ToString($sha256.ComputeHash($utf8NoBom.GetBytes($normalizedText))).Replace("-", "").ToLowerInvariant()
        [pscustomobject]@{ Path = $relativePath; Hash = $hash }
    }
}
finally {
    $sha256.Dispose()
}

if ($Mode -eq "Capture") {
    $current | ConvertTo-Json -Compress | Set-Content -LiteralPath $SnapshotPath -Encoding UTF8
    exit 0
}

if (-not (Test-Path -LiteralPath $SnapshotPath -PathType Leaf)) {
    throw "Wails binding snapshot is missing: $SnapshotPath"
}

$expected = Get-Content -Raw -LiteralPath $SnapshotPath | ConvertFrom-Json
$changes = foreach ($item in $current) {
    $old = $expected | Where-Object { $_.Path -eq $item.Path }
    if ($null -eq $old -or $old.Hash -ne $item.Hash) {
        $item.Path
    }
}

if ($changes.Count -gt 0) {
    Write-Error ("Wails generated bindings changed during build: " + ($changes -join ", ") + ". Regenerate and review them explicitly before building.")
    exit 1
}

Write-Host "Wails generated bindings unchanged."
