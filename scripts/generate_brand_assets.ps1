param(
    [string]$SourceDir = "D:\Downloads\thrm",
    [string]$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot ".."))
)

$ErrorActionPreference = "Stop"
Add-Type -AssemblyName System.Drawing

function New-DirectoryIfMissing {
    param([string]$Path)
    if (-not (Test-Path $Path)) {
        New-Item -ItemType Directory -Path $Path | Out-Null
    }
}

function Clamp-Byte {
    param([double]$Value)
    if ($Value -lt 0) { return [byte]0 }
    if ($Value -gt 255) { return [byte]255 }
    return [byte][Math]::Round($Value)
}

function Convert-ToTransparentLogo {
    param(
        [string]$Path,
        [ValidateSet("light", "dark")][string]$Mode
    )

    $source = [System.Drawing.Bitmap]::new($Path)
    $target = [System.Drawing.Bitmap]::new($source.Width, $source.Height, [System.Drawing.Imaging.PixelFormat]::Format32bppArgb)

    for ($y = 0; $y -lt $source.Height; $y++) {
        for ($x = 0; $x -lt $source.Width; $x++) {
            $c = $source.GetPixel($x, $y)
            if ($Mode -eq "light") {
                $dist = [Math]::Sqrt([Math]::Pow(255 - $c.R, 2) + [Math]::Pow(255 - $c.G, 2) + [Math]::Pow(255 - $c.B, 2)) / [Math]::Sqrt(3)
                $alpha = Clamp-Byte (($dist - 10) * 3.8)
            } else {
                $dist = [Math]::Sqrt([Math]::Pow($c.R, 2) + [Math]::Pow($c.G, 2) + [Math]::Pow($c.B, 2)) / [Math]::Sqrt(3)
                $alpha = Clamp-Byte (($dist - 8) * 4.2)
            }

            if ($alpha -lt 8) {
                $alpha = [byte]0
            }
            $target.SetPixel($x, $y, [System.Drawing.Color]::FromArgb($alpha, $c.R, $c.G, $c.B))
        }
    }

    $source.Dispose()
    return $target
}

function Get-AlphaBounds {
    param([System.Drawing.Bitmap]$Bitmap, [int]$Threshold = 12)

    $minX = $Bitmap.Width
    $minY = $Bitmap.Height
    $maxX = -1
    $maxY = -1

    for ($y = 0; $y -lt $Bitmap.Height; $y++) {
        for ($x = 0; $x -lt $Bitmap.Width; $x++) {
            if ($Bitmap.GetPixel($x, $y).A -gt $Threshold) {
                if ($x -lt $minX) { $minX = $x }
                if ($y -lt $minY) { $minY = $y }
                if ($x -gt $maxX) { $maxX = $x }
                if ($y -gt $maxY) { $maxY = $y }
            }
        }
    }

    if ($maxX -lt 0 -or $maxY -lt 0) {
        return [System.Drawing.Rectangle]::new(0, 0, $Bitmap.Width, $Bitmap.Height)
    }

    return [System.Drawing.Rectangle]::new($minX, $minY, $maxX - $minX + 1, $maxY - $minY + 1)
}

function Crop-AlphaBitmap {
    param(
        [System.Drawing.Bitmap]$Bitmap,
        [int]$Padding = 24,
        [int]$Threshold = 12
    )

    $bounds = Get-AlphaBounds $Bitmap $Threshold
    $x = [Math]::Max(0, $bounds.X - $Padding)
    $y = [Math]::Max(0, $bounds.Y - $Padding)
    $right = [Math]::Min($Bitmap.Width, $bounds.Right + $Padding)
    $bottom = [Math]::Min($Bitmap.Height, $bounds.Bottom + $Padding)
    $rect = [System.Drawing.Rectangle]::new($x, $y, $right - $x, $bottom - $y)

    $cropped = [System.Drawing.Bitmap]::new($rect.Width, $rect.Height, [System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
    $g = [System.Drawing.Graphics]::FromImage($cropped)
    $g.CompositingMode = [System.Drawing.Drawing2D.CompositingMode]::SourceCopy
    $g.DrawImage($Bitmap, [System.Drawing.Rectangle]::new(0, 0, $rect.Width, $rect.Height), $rect, [System.Drawing.GraphicsUnit]::Pixel)
    $g.Dispose()
    return $cropped
}

function Clear-EdgeBand {
    param(
        [System.Drawing.Bitmap]$Bitmap,
        [int]$Band = 10
    )

    $transparent = [System.Drawing.Color]::Transparent
    for ($y = 0; $y -lt $Bitmap.Height; $y++) {
        for ($x = 0; $x -lt $Bitmap.Width; $x++) {
            if ($x -lt $Band -or $y -lt $Band -or $x -ge ($Bitmap.Width - $Band) -or $y -ge ($Bitmap.Height - $Band)) {
                $Bitmap.SetPixel($x, $y, $transparent)
            }
        }
    }
}

function Resize-Bitmap {
    param(
        [System.Drawing.Bitmap]$Bitmap,
        [int]$Width,
        [int]$Height
    )

    $resized = [System.Drawing.Bitmap]::new($Width, $Height, [System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
    $g = [System.Drawing.Graphics]::FromImage($resized)
    $g.CompositingQuality = [System.Drawing.Drawing2D.CompositingQuality]::HighQuality
    $g.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
    $g.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::HighQuality
    $g.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
    $g.Clear([System.Drawing.Color]::Transparent)
    $g.DrawImage($Bitmap, 0, 0, $Width, $Height)
    $g.Dispose()
    return $resized
}

function Save-LogoWidth {
    param(
        [System.Drawing.Bitmap]$Bitmap,
        [string]$Path,
        [int]$Width
    )

    $height = [Math]::Max(1, [int][Math]::Round($Bitmap.Height * ($Width / $Bitmap.Width)))
    $resized = Resize-Bitmap $Bitmap $Width $height
    $resized.Save($Path, [System.Drawing.Imaging.ImageFormat]::Png)
    $resized.Dispose()
}

function New-RoundedRectPath {
    param([System.Drawing.RectangleF]$Rect, [float]$Radius)
    $path = [System.Drawing.Drawing2D.GraphicsPath]::new()
    $d = $Radius * 2
    $path.AddArc($Rect.X, $Rect.Y, $d, $d, 180, 90)
    $path.AddArc($Rect.Right - $d, $Rect.Y, $d, $d, 270, 90)
    $path.AddArc($Rect.Right - $d, $Rect.Bottom - $d, $d, $d, 0, 90)
    $path.AddArc($Rect.X, $Rect.Bottom - $d, $d, $d, 90, 90)
    $path.CloseFigure()
    return $path
}

function New-AppIcon {
    param(
        [System.Drawing.Bitmap]$Mark,
        [int]$Size
    )

    $canvas = [System.Drawing.Bitmap]::new($Size, $Size, [System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
    $g = [System.Drawing.Graphics]::FromImage($canvas)
    $g.CompositingQuality = [System.Drawing.Drawing2D.CompositingQuality]::HighQuality
    $g.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
    $g.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::AntiAlias
    $g.Clear([System.Drawing.Color]::Transparent)

    $pad = $Size * 0.035
    $rect = [System.Drawing.RectangleF]::new($pad, $pad, $Size - $pad * 2, $Size - $pad * 2)
    $radius = $Size * 0.22
    $path = New-RoundedRectPath $rect $radius
    $brush = [System.Drawing.Drawing2D.LinearGradientBrush]::new($rect, [System.Drawing.Color]::FromArgb(255, 255, 255, 255), [System.Drawing.Color]::FromArgb(255, 235, 241, 250), 90)
    $g.FillPath($brush, $path)
    $pen = [System.Drawing.Pen]::new([System.Drawing.Color]::FromArgb(42, 30, 64, 175), [Math]::Max(1, $Size * 0.012))
    $g.DrawPath($pen, $path)

    $markTarget = [int][Math]::Round($Size * 0.68)
    $markX = [int][Math]::Round(($Size - $markTarget) / 2)
    $markY = [int][Math]::Round(($Size - $markTarget) / 2)
    $g.DrawImage($Mark, $markX, $markY, $markTarget, $markTarget)

    $pen.Dispose()
    $brush.Dispose()
    $path.Dispose()
    $g.Dispose()
    return $canvas
}

function Save-IcoBitmapBytes {
    param([System.Drawing.Bitmap]$Bitmap)

    $width = $Bitmap.Width
    $height = $Bitmap.Height
    $xorSize = $width * $height * 4
    $maskStride = [int]([Math]::Floor(($width + 31) / 32)) * 4
    $maskSize = $maskStride * $height
    $stream = [System.IO.MemoryStream]::new()
    $writer = [System.IO.BinaryWriter]::new($stream)

    $writer.Write([UInt32]40)
    $writer.Write([Int32]$width)
    $writer.Write([Int32]($height * 2))
    $writer.Write([UInt16]1)
    $writer.Write([UInt16]32)
    $writer.Write([UInt32]0)
    $writer.Write([UInt32]($xorSize + $maskSize))
    $writer.Write([Int32]0)
    $writer.Write([Int32]0)
    $writer.Write([UInt32]0)
    $writer.Write([UInt32]0)

    for ($y = $height - 1; $y -ge 0; $y--) {
        for ($x = 0; $x -lt $width; $x++) {
            $c = $Bitmap.GetPixel($x, $y)
            $writer.Write([byte]$c.B)
            $writer.Write([byte]$c.G)
            $writer.Write([byte]$c.R)
            $writer.Write([byte]$c.A)
        }
    }

    for ($y = $height - 1; $y -ge 0; $y--) {
        $row = New-Object byte[] $maskStride
        for ($x = 0; $x -lt $width; $x++) {
            $c = $Bitmap.GetPixel($x, $y)
            if ($c.A -lt 128) {
                $byteIndex = [int][Math]::Floor($x / 8)
                $bit = 0x80 -shr ($x % 8)
                $row[$byteIndex] = [byte]($row[$byteIndex] -bor $bit)
            }
        }
        $writer.Write($row)
    }

    $writer.Flush()
    $bytes = $stream.ToArray()
    $writer.Dispose()
    $stream.Dispose()
    return ,([byte[]]$bytes)
}

function New-IcoFile {
    param(
        [System.Drawing.Bitmap]$Source,
        [string]$Path,
        [int[]]$Sizes = @(16, 24, 32, 48, 64, 128, 256)
    )

    $entries = @()
    foreach ($size in $Sizes) {
        $bitmap = Resize-Bitmap $Source $size $size
        $bytes = [byte[]](Save-IcoBitmapBytes $bitmap)
        $bitmap.Dispose()
        $entries += [pscustomobject]@{ Size = $size; Bytes = [byte[]]$bytes }
    }

    $stream = [System.IO.File]::Open($Path, [System.IO.FileMode]::Create, [System.IO.FileAccess]::Write)
    $writer = [System.IO.BinaryWriter]::new($stream)
    $writer.Write([UInt16]0)
    $writer.Write([UInt16]1)
    $writer.Write([UInt16]$entries.Count)

    $offset = 6 + (16 * $entries.Count)
    foreach ($entry in $entries) {
        $w = if ($entry.Size -eq 256) { [byte]0 } else { [byte]$entry.Size }
        $writer.Write($w)
        $writer.Write($w)
        $writer.Write([byte]0)
        $writer.Write([byte]0)
        $writer.Write([UInt16]1)
        $writer.Write([UInt16]32)
        $writer.Write([UInt32]$entry.Bytes.Length)
        $writer.Write([UInt32]$offset)
        $offset += $entry.Bytes.Length
    }

    foreach ($entry in $entries) {
        $writer.Write([byte[]]$entry.Bytes)
    }

    $writer.Dispose()
    $stream.Dispose()
}

$logoLightSource = Join-Path $SourceDir "1.png"
$logoDarkSource = Join-Path $SourceDir "2.png"
$markSource = Join-Path $SourceDir "3.png"
foreach ($path in @($logoLightSource, $logoDarkSource, $markSource)) {
    if (-not (Test-Path $path)) {
        throw "Missing source asset: $path"
    }
}

$brandDir = Join-Path $RepoRoot "frontend\public\brand"
New-DirectoryIfMissing $brandDir

$lightLogoSource = Convert-ToTransparentLogo $logoLightSource "light"
$darkLogoSource = Convert-ToTransparentLogo $logoDarkSource "dark"
$markSourceBitmap = Convert-ToTransparentLogo $markSource "light"
Clear-EdgeBand $lightLogoSource 14
Clear-EdgeBand $darkLogoSource 14
Clear-EdgeBand $markSourceBitmap 14

$lightLogo = Crop-AlphaBitmap $lightLogoSource 34 12
$darkLogo = Crop-AlphaBitmap $darkLogoSource 34 12
$mark = Crop-AlphaBitmap $markSourceBitmap 60 12

Save-LogoWidth $lightLogo (Join-Path $brandDir "wordmark-light.png") 900
Save-LogoWidth $darkLogo (Join-Path $brandDir "wordmark-dark.png") 900

$markLarge = Resize-Bitmap $mark 720 720
$markLarge.Save((Join-Path $brandDir "mark.png"), [System.Drawing.Imaging.ImageFormat]::Png)

$appIcon = New-AppIcon $markLarge 1024
$appIcon.Save((Join-Path $RepoRoot "build\appicon.png"), [System.Drawing.Imaging.ImageFormat]::Png)
$appIcon.Save((Join-Path $brandDir "appicon.png"), [System.Drawing.Imaging.ImageFormat]::Png)

$coreIcon = Resize-Bitmap $appIcon 256 256
$coreIcon.Save((Join-Path $RepoRoot "cmd\core\winres\icon.png"), [System.Drawing.Imaging.ImageFormat]::Png)

New-IcoFile $appIcon (Join-Path $RepoRoot "build\windows\icon.ico")
New-IcoFile $appIcon (Join-Path $RepoRoot "cmd\core\icon.ico")
New-IcoFile $appIcon (Join-Path $RepoRoot "frontend\src\app\favicon.ico") @(16, 32, 48)

$lightLogo.Dispose()
$darkLogo.Dispose()
$mark.Dispose()
$markLarge.Dispose()
$appIcon.Dispose()
$coreIcon.Dispose()
$lightLogoSource.Dispose()
$darkLogoSource.Dispose()
$markSourceBitmap.Dispose()

Write-Host "Brand assets generated from $SourceDir"