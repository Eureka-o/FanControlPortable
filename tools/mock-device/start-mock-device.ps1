$ErrorActionPreference = 'Stop'

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$nodeScript = Join-Path $scriptDir 'mock-device.js'

if (-not (Test-Path -LiteralPath $nodeScript)) {
  throw "mock-device.js not found: $nodeScript"
}

node $nodeScript
