param(
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

function Add-FanBlade {
    param(
        [System.Drawing.Graphics]$Graphics,
        [float]$Center,
        [float]$Scale,
        [float]$Angle,
        [System.Drawing.Brush]$Brush
    )

    $path = [System.Drawing.Drawing2D.GraphicsPath]::new()
    $path.StartFigure()
    $path.AddBezier(
        [System.Drawing.PointF]::new($Center + 14 * $Scale, $Center - 10 * $Scale),
        [System.Drawing.PointF]::new($Center + 78 * $Scale, $Center - 46 * $Scale),
        [System.Drawing.PointF]::new($Center + 136 * $Scale, $Center - 20 * $Scale),
        [System.Drawing.PointF]::new($Center + 148 * $Scale, $Center + 36 * $Scale)
    )
    $path.AddBezier(
        [System.Drawing.PointF]::new($Center + 148 * $Scale, $Center + 36 * $Scale),
        [System.Drawing.PointF]::new($Center + 90 * $Scale, $Center + 62 * $Scale),
        [System.Drawing.PointF]::new($Center + 36 * $Scale, $Center + 36 * $Scale),
        [System.Drawing.PointF]::new($Center + 8 * $Scale, $Center + 10 * $Scale)
    )
    $path.CloseFigure()

    $matrix = [System.Drawing.Drawing2D.Matrix]::new()
    $matrix.RotateAt($Angle, [System.Drawing.PointF]::new($Center, $Center))
    $path.Transform($matrix)
    $Graphics.FillPath($Brush, $path)
    $matrix.Dispose()
    $path.Dispose()
}

function New-AppIcon {
    param([int]$Size = 1024)

    $canvas = [System.Drawing.Bitmap]::new($Size, $Size, [System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
    $g = [System.Drawing.Graphics]::FromImage($canvas)
    $g.CompositingQuality = [System.Drawing.Drawing2D.CompositingQuality]::HighQuality
    $g.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
    $g.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::AntiAlias
    $g.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
    $g.Clear([System.Drawing.Color]::Transparent)

    $shadowRect = [System.Drawing.RectangleF]::new($Size * 0.07, $Size * 0.09, $Size * 0.86, $Size * 0.84)
    $shadowPath = New-RoundedRectPath $shadowRect ($Size * 0.2)
    $shadowBrush = [System.Drawing.SolidBrush]::new([System.Drawing.Color]::FromArgb(32, 52, 96, 132))
    $g.FillPath($shadowBrush, $shadowPath)

    $rect = [System.Drawing.RectangleF]::new($Size * 0.055, $Size * 0.055, $Size * 0.89, $Size * 0.89)
    $bgPath = New-RoundedRectPath $rect ($Size * 0.21)
    $bgBrush = [System.Drawing.Drawing2D.LinearGradientBrush]::new(
        $rect,
        [System.Drawing.Color]::FromArgb(255, 252, 254, 255),
        [System.Drawing.Color]::FromArgb(255, 230, 244, 252),
        135
    )
    $g.FillPath($bgBrush, $bgPath)

    $rimPen = [System.Drawing.Pen]::new([System.Drawing.Color]::FromArgb(180, 179, 213, 233), [Math]::Max(2, $Size * 0.01))
    $g.DrawPath($rimPen, $bgPath)

    $accentRect = [System.Drawing.RectangleF]::new($Size * 0.66, $Size * 0.15, $Size * 0.17, $Size * 0.045)
    $accentPath = New-RoundedRectPath $accentRect ($Size * 0.022)
    $accentBrush = [System.Drawing.SolidBrush]::new([System.Drawing.Color]::FromArgb(185, 58, 155, 207))
    $g.FillPath($accentBrush, $accentPath)

    $scale = $Size / 1024.0
    $center = $Size / 2.0
    $bladeBrush = [System.Drawing.Drawing2D.LinearGradientBrush]::new(
        [System.Drawing.RectangleF]::new($Size * 0.22, $Size * 0.22, $Size * 0.56, $Size * 0.56),
        [System.Drawing.Color]::FromArgb(238, 40, 138, 196),
        [System.Drawing.Color]::FromArgb(238, 98, 211, 218),
        35
    )
    foreach ($angle in @(0, 120, 240)) {
        Add-FanBlade $g $center $scale $angle $bladeBrush
    }

    $ringPen = [System.Drawing.Pen]::new([System.Drawing.Color]::FromArgb(190, 62, 158, 207), [Math]::Max(4, $Size * 0.018))
    $g.DrawEllipse($ringPen, $Size * 0.275, $Size * 0.275, $Size * 0.45, $Size * 0.45)

    $hubBrush = [System.Drawing.SolidBrush]::new([System.Drawing.Color]::FromArgb(255, 248, 253, 255))
    $hubPen = [System.Drawing.Pen]::new([System.Drawing.Color]::FromArgb(230, 44, 145, 197), [Math]::Max(3, $Size * 0.014))
    $g.FillEllipse($hubBrush, $Size * 0.425, $Size * 0.425, $Size * 0.15, $Size * 0.15)
    $g.DrawEllipse($hubPen, $Size * 0.425, $Size * 0.425, $Size * 0.15, $Size * 0.15)

    $innerBrush = [System.Drawing.SolidBrush]::new([System.Drawing.Color]::FromArgb(255, 65, 177, 211))
    $g.FillEllipse($innerBrush, $Size * 0.482, $Size * 0.482, $Size * 0.036, $Size * 0.036)

    foreach ($obj in @($innerBrush, $hubPen, $hubBrush, $ringPen, $bladeBrush, $accentBrush, $accentPath, $rimPen, $bgBrush, $bgPath, $shadowBrush, $shadowPath, $g)) {
        $obj.Dispose()
    }
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

$brandDir = Join-Path $RepoRoot "frontend\public\brand"
$distBrandDir = Join-Path $RepoRoot "frontend\dist\brand"
New-DirectoryIfMissing (Join-Path $RepoRoot "build\windows")
New-DirectoryIfMissing (Join-Path $RepoRoot "cmd\core")
New-DirectoryIfMissing (Join-Path $RepoRoot "cmd\core\winres")
New-DirectoryIfMissing (Join-Path $RepoRoot "frontend\src\app")
New-DirectoryIfMissing $brandDir
New-DirectoryIfMissing $distBrandDir

$icon = New-AppIcon 1024
$icon.Save((Join-Path $RepoRoot "build\appicon.png"), [System.Drawing.Imaging.ImageFormat]::Png)
$icon.Save((Join-Path $brandDir "appicon.png"), [System.Drawing.Imaging.ImageFormat]::Png)
$icon.Save((Join-Path $brandDir "mark.png"), [System.Drawing.Imaging.ImageFormat]::Png)
$icon.Save((Join-Path $distBrandDir "appicon.png"), [System.Drawing.Imaging.ImageFormat]::Png)
$icon.Save((Join-Path $distBrandDir "mark.png"), [System.Drawing.Imaging.ImageFormat]::Png)

$coreIcon = Resize-Bitmap $icon 256 256
$coreIcon.Save((Join-Path $RepoRoot "cmd\core\winres\icon.png"), [System.Drawing.Imaging.ImageFormat]::Png)

New-IcoFile $icon (Join-Path $RepoRoot "build\windows\icon.ico")
New-IcoFile $icon (Join-Path $RepoRoot "cmd\core\icon.ico")
New-IcoFile $icon (Join-Path $RepoRoot "frontend\src\app\favicon.ico") @(16, 32, 48)

$coreIcon.Dispose()
$icon.Dispose()
Write-Host "Generated FanControlPortable icon assets in $RepoRoot"
