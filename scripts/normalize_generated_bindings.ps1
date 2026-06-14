$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Resolve-Path (Join-Path $scriptDir "..")
$modelsPath = Join-Path $repoRoot "frontend\wailsjs\go\models.ts"

if (-not (Test-Path -LiteralPath $modelsPath -PathType Leaf)) {
    return
}

$text = [System.IO.File]::ReadAllText($modelsPath)
$normalized = [System.Text.RegularExpressions.Regex]::Replace($text, "(?m)[\t ]+$", "")
if ($normalized -ne $text) {
    [System.IO.File]::WriteAllText($modelsPath, $normalized, (New-Object System.Text.UTF8Encoding($false)))
    Write-Host "Normalized generated Wails bindings whitespace: $modelsPath"
}
