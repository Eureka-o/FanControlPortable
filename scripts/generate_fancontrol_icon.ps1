param(
    [string]$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")),
    [string]$SourceImage = ""
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

function New-HexagonPath {
    param(
        [float]$CenterX,
        [float]$CenterY,
        [float]$Radius
    )

    $path = [System.Drawing.Drawing2D.GraphicsPath]::new()
    $points = @()
    for ($i = 0; $i -lt 6; $i++) {
        $angle = (-90 + $i * 60) * [Math]::PI / 180
        $points += [System.Drawing.PointF]::new(
            [float]($CenterX + [Math]::Cos($angle) * $Radius),
            [float]($CenterY + [Math]::Sin($angle) * $Radius)
        )
    }
    $path.AddPolygon([System.Drawing.PointF[]]$points)
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
        [System.Drawing.PointF]::new($Center + 25 * $Scale, $Center - 18 * $Scale),
        [System.Drawing.PointF]::new($Center + 94 * $Scale, $Center - 70 * $Scale),
        [System.Drawing.PointF]::new($Center + 178 * $Scale, $Center - 34 * $Scale),
        [System.Drawing.PointF]::new($Center + 186 * $Scale, $Center + 48 * $Scale)
    )
    $path.AddBezier(
        [System.Drawing.PointF]::new($Center + 186 * $Scale, $Center + 48 * $Scale),
        [System.Drawing.PointF]::new($Center + 112 * $Scale, $Center + 84 * $Scale),
        [System.Drawing.PointF]::new($Center + 48 * $Scale, $Center + 48 * $Scale),
        [System.Drawing.PointF]::new($Center + 10 * $Scale, $Center + 16 * $Scale)
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

    $shadowRect = [System.Drawing.RectangleF]::new($Size * 0.072, $Size * 0.088, $Size * 0.856, $Size * 0.842)
    $shadowPath = New-RoundedRectPath $shadowRect ($Size * 0.205)
    $shadowBrush = [System.Drawing.SolidBrush]::new([System.Drawing.Color]::FromArgb(38, 18, 55, 94))
    $g.FillPath($shadowBrush, $shadowPath)

    $rect = [System.Drawing.RectangleF]::new($Size * 0.055, $Size * 0.055, $Size * 0.89, $Size * 0.89)
    $bgPath = New-RoundedRectPath $rect ($Size * 0.21)
    $bgBrush = [System.Drawing.Drawing2D.LinearGradientBrush]::new(
        $rect,
        [System.Drawing.Color]::FromArgb(255, 255, 255, 255),
        [System.Drawing.Color]::FromArgb(255, 224, 237, 248),
        135
    )
    $g.FillPath($bgBrush, $bgPath)

    $innerGlowRect = [System.Drawing.RectangleF]::new($Size * 0.083, $Size * 0.083, $Size * 0.834, $Size * 0.834)
    $innerGlowPath = New-RoundedRectPath $innerGlowRect ($Size * 0.19)
    $innerGlowPen = [System.Drawing.Pen]::new([System.Drawing.Color]::FromArgb(150, 255, 255, 255), [Math]::Max(2, $Size * 0.018))
    $g.DrawPath($innerGlowPen, $innerGlowPath)

    $rimPen = [System.Drawing.Pen]::new([System.Drawing.Color]::FromArgb(230, 36, 95, 178), [Math]::Max(3, $Size * 0.014))
    $g.DrawPath($rimPen, $bgPath)

    $accentRect = [System.Drawing.RectangleF]::new($Size * 0.655, $Size * 0.15, $Size * 0.18, $Size * 0.047)
    $accentPath = New-RoundedRectPath $accentRect ($Size * 0.022)
    $accentBrush = [System.Drawing.Drawing2D.LinearGradientBrush]::new(
        $accentRect,
        [System.Drawing.Color]::FromArgb(255, 0, 95, 255),
        [System.Drawing.Color]::FromArgb(255, 75, 202, 255),
        0
    )
    $g.FillPath($accentBrush, $accentPath)

    $scale = $Size / 1024.0
    $center = $Size / 2.0
    $hexPath = New-HexagonPath ([float]$center) ([float]($center + $Size * 0.015)) ([float]($Size * 0.295))
    $hexShadow = New-HexagonPath ([float]($center + $Size * 0.012)) ([float]($center + $Size * 0.033)) ([float]($Size * 0.295))
    $hexShadowPen = [System.Drawing.Pen]::new([System.Drawing.Color]::FromArgb(66, 8, 24, 50), [Math]::Max(12, $Size * 0.078))
    $hexShadowPen.LineJoin = [System.Drawing.Drawing2D.LineJoin]::Round
    $g.DrawPath($hexShadowPen, $hexShadow)

    $hexBrush = [System.Drawing.Drawing2D.LinearGradientBrush]::new(
        [System.Drawing.RectangleF]::new($Size * 0.22, $Size * 0.22, $Size * 0.56, $Size * 0.56),
        [System.Drawing.Color]::FromArgb(255, 4, 12, 30),
        [System.Drawing.Color]::FromArgb(255, 27, 43, 72),
        35
    )
    $hexPen = [System.Drawing.Pen]::new($hexBrush, [Math]::Max(14, $Size * 0.074))
    $hexPen.LineJoin = [System.Drawing.Drawing2D.LineJoin]::Round
    $g.DrawPath($hexPen, $hexPath)

    $hexHighlightPen = [System.Drawing.Pen]::new([System.Drawing.Color]::FromArgb(72, 96, 148, 216), [Math]::Max(4, $Size * 0.013))
    $hexHighlightPen.LineJoin = [System.Drawing.Drawing2D.LineJoin]::Round
    $matrix = [System.Drawing.Drawing2D.Matrix]::new()
    $matrix.Translate([float](-$Size * 0.012), [float](-$Size * 0.014))
    $hexPath.Transform($matrix)
    $g.DrawPath($hexHighlightPen, $hexPath)
    $matrix.Dispose()

    $fanShadowBrush = [System.Drawing.SolidBrush]::new([System.Drawing.Color]::FromArgb(70, 0, 58, 130))
    foreach ($angle in @(0, 120, 240)) {
        $state = $g.Save()
        $g.TranslateTransform([float]($Size * 0.015), [float]($Size * 0.022))
        Add-FanBlade $g $center $scale $angle $fanShadowBrush
        $g.Restore($state)
    }

    $bladeBrush = [System.Drawing.Drawing2D.LinearGradientBrush]::new(
        [System.Drawing.RectangleF]::new($Size * 0.29, $Size * 0.29, $Size * 0.42, $Size * 0.42),
        [System.Drawing.Color]::FromArgb(255, 0, 92, 255),
        [System.Drawing.Color]::FromArgb(255, 52, 220, 235),
        35
    )
    foreach ($angle in @(0, 120, 240)) {
        Add-FanBlade $g $center $scale $angle $bladeBrush
    }

    $ringPen = [System.Drawing.Pen]::new([System.Drawing.Color]::FromArgb(255, 20, 84, 170), [Math]::Max(6, $Size * 0.025))
    $g.DrawEllipse($ringPen, $Size * 0.357, $Size * 0.357, $Size * 0.286, $Size * 0.286)

    $ringHighlightPen = [System.Drawing.Pen]::new([System.Drawing.Color]::FromArgb(145, 145, 231, 255), [Math]::Max(3, $Size * 0.01))
    $g.DrawArc($ringHighlightPen, $Size * 0.363, $Size * 0.363, $Size * 0.274, $Size * 0.274, 210, 145)

    $hubRect = [System.Drawing.RectangleF]::new($Size * 0.415, $Size * 0.415, $Size * 0.17, $Size * 0.17)
    $hubBrush = [System.Drawing.Drawing2D.LinearGradientBrush]::new(
        $hubRect,
        [System.Drawing.Color]::FromArgb(255, 255, 255, 255),
        [System.Drawing.Color]::FromArgb(255, 206, 232, 246),
        135
    )
    $hubPen = [System.Drawing.Pen]::new([System.Drawing.Color]::FromArgb(255, 8, 70, 150), [Math]::Max(4, $Size * 0.018))
    $g.FillEllipse($hubBrush, $hubRect)
    $g.DrawEllipse($hubPen, $hubRect)

    $innerBrush = [System.Drawing.Drawing2D.LinearGradientBrush]::new(
        [System.Drawing.RectangleF]::new($Size * 0.472, $Size * 0.472, $Size * 0.056, $Size * 0.056),
        [System.Drawing.Color]::FromArgb(255, 0, 112, 255),
        [System.Drawing.Color]::FromArgb(255, 91, 225, 242),
        35
    )
    $g.FillEllipse($innerBrush, $Size * 0.472, $Size * 0.472, $Size * 0.056, $Size * 0.056)

    foreach ($obj in @($innerBrush, $hubPen, $hubBrush, $ringHighlightPen, $ringPen, $bladeBrush, $fanShadowBrush, $hexHighlightPen, $hexPen, $hexBrush, $hexShadowPen, $hexShadow, $hexPath, $accentBrush, $accentPath, $innerGlowPen, $innerGlowPath, $rimPen, $bgBrush, $bgPath, $shadowBrush, $shadowPath, $g)) {
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

$sourceIconPath = $SourceImage
if ([string]::IsNullOrWhiteSpace($sourceIconPath)) {
    $candidate = Join-Path $RepoRoot "build\appicon.png"
    if (Test-Path $candidate) {
        $sourceIconPath = $candidate
    }
}

if (-not [string]::IsNullOrWhiteSpace($sourceIconPath)) {
    $sourceIconPath = [string](Resolve-Path -LiteralPath $sourceIconPath)
    $sourceIcon = [System.Drawing.Bitmap]::new($sourceIconPath)
    $icon = Resize-Bitmap $sourceIcon 1024 1024
    $sourceIcon.Dispose()
} else {
    $icon = New-AppIcon 1024
}
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
Write-Host "Generated FanControl icon assets in $RepoRoot"
