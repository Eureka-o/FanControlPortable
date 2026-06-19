param(
    [Parameter(Mandatory = $true)]
    [string]$InstallThemesDir
)

$ErrorActionPreference = "SilentlyContinue"

function Test-ThemeId {
    param([string]$Id)

    return -not [string]::IsNullOrWhiteSpace($Id) -and $Id -match '^[a-z0-9_-]{1,64}$'
}

function Get-VersionParts {
    param([string]$Version)

    if ([string]::IsNullOrWhiteSpace($Version)) {
        return @(0)
    }

    $parts = [regex]::Matches($Version, '\d+') | ForEach-Object {
        [int]$_.Value
    }

    if ($parts.Count -eq 0) {
        return @(0)
    }

    return @($parts)
}

function Compare-ThemeVersion {
    param(
        [string]$Left,
        [string]$Right
    )

    $leftParts = @(Get-VersionParts $Left)
    $rightParts = @(Get-VersionParts $Right)
    $max = [Math]::Max($leftParts.Count, $rightParts.Count)

    for ($i = 0; $i -lt $max; $i++) {
        $leftValue = 0
        $rightValue = 0
        if ($i -lt $leftParts.Count) {
            $leftValue = $leftParts[$i]
        }
        if ($i -lt $rightParts.Count) {
            $rightValue = $rightParts[$i]
        }
        if ($leftValue -gt $rightValue) {
            return 1
        }
        if ($leftValue -lt $rightValue) {
            return -1
        }
    }

    return 0
}

function Read-ThemeManifest {
    param([string]$ManifestPath)

    try {
        return Get-Content -LiteralPath $ManifestPath -Raw -Encoding UTF8 | ConvertFrom-Json
    } catch {
        return $null
    }
}

function Get-LegacyThemeCandidates {
    param([string[]]$LegacyDirs)

    $candidates = @()
    foreach ($legacyDir in $LegacyDirs) {
        if (-not (Test-Path -LiteralPath $legacyDir -PathType Container)) {
            continue
        }

        Get-ChildItem -LiteralPath $legacyDir -Directory -ErrorAction SilentlyContinue | ForEach-Object {
            $manifestPath = Join-Path $_.FullName "theme.json"
            if (-not (Test-Path -LiteralPath $manifestPath -PathType Leaf)) {
                return
            }

            $manifest = Read-ThemeManifest $manifestPath
            if ($null -eq $manifest) {
                return
            }

            $themeId = [string]$manifest.id
            if ([string]::IsNullOrWhiteSpace($themeId)) {
                $themeId = $_.Name
            }
            if (-not (Test-ThemeId $themeId)) {
                return
            }

            $item = Get-Item -LiteralPath $manifestPath -ErrorAction SilentlyContinue
            $candidates += [pscustomobject]@{
                Id       = $themeId
                Dir      = $_.FullName
                Root     = $legacyDir
                Version  = [string]$manifest.version
                Modified = if ($item) { $item.LastWriteTimeUtc } else { [datetime]::MinValue }
            }
        }
    }

    return $candidates
}

function Select-NewestTheme {
    param([object[]]$Candidates)

    $best = $Candidates[0]
    foreach ($candidate in $Candidates | Select-Object -Skip 1) {
        $versionCompare = Compare-ThemeVersion $candidate.Version $best.Version
        if ($versionCompare -gt 0 -or ($versionCompare -eq 0 -and $candidate.Modified -gt $best.Modified)) {
            $best = $candidate
        }
    }

    return $best
}

function Remove-EmptyDirectory {
    param([string]$Path)

    if ([string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path -LiteralPath $Path -PathType Container)) {
        return
    }

    $children = @(Get-ChildItem -LiteralPath $Path -Force -ErrorAction SilentlyContinue)
    if ($children.Count -eq 0) {
        Remove-Item -LiteralPath $Path -Force -ErrorAction SilentlyContinue
    }
}

try {
    if ([string]::IsNullOrWhiteSpace($InstallThemesDir)) {
        exit 0
    }

    New-Item -ItemType Directory -Path $InstallThemesDir -Force | Out-Null

    $userProfile = $env:USERPROFILE
    if ([string]::IsNullOrWhiteSpace($userProfile)) {
        exit 0
    }

    $legacyDirs = @(
        (Join-Path $userProfile ".fancontrol\themes"),
        (Join-Path $userProfile ".fancontrolportable\themes"),
        (Join-Path $userProfile ".bs2pro-controller\themes")
    ) | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | Select-Object -Unique

    $candidates = @(Get-LegacyThemeCandidates $legacyDirs)
    if ($candidates.Count -eq 0) {
        exit 0
    }

    foreach ($group in ($candidates | Group-Object Id)) {
        $themeId = $group.Name
        $installManifest = Join-Path (Join-Path $InstallThemesDir $themeId) "theme.json"

        if (Test-Path -LiteralPath $installManifest -PathType Leaf) {
            foreach ($candidate in $group.Group) {
                Remove-Item -LiteralPath $candidate.Dir -Recurse -Force -ErrorAction SilentlyContinue
            }
            continue
        }

        $best = Select-NewestTheme @($group.Group)
        $destination = Join-Path $InstallThemesDir $themeId
        $tempDestination = Join-Path $InstallThemesDir ".$themeId.migration"

        Remove-Item -LiteralPath $tempDestination -Recurse -Force -ErrorAction SilentlyContinue
        Copy-Item -LiteralPath $best.Dir -Destination $tempDestination -Recurse -Force -ErrorAction Stop
        Remove-Item -LiteralPath $destination -Recurse -Force -ErrorAction SilentlyContinue
        Rename-Item -LiteralPath $tempDestination -NewName $themeId -Force -ErrorAction Stop

        foreach ($candidate in $group.Group) {
            Remove-Item -LiteralPath $candidate.Dir -Recurse -Force -ErrorAction SilentlyContinue
        }
    }

    foreach ($legacyDir in $legacyDirs) {
        Remove-EmptyDirectory $legacyDir
        Remove-EmptyDirectory (Split-Path -Parent $legacyDir)
    }
} catch {
    exit 0
}

exit 0
