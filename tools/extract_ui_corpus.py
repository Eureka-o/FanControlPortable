#!/usr/bin/env python3
"""Organize required UI characters into tools/ui-font-corpus.txt, rebuild theme font subsets, and verify 100% coverage."""

from __future__ import annotations

import argparse
import glob
import io
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
from dataclasses import dataclass
from pathlib import Path

from fontTools.merge import Merger
from fontTools.subset import Options, Subsetter
from fontTools.ttLib import TTFont
from fontTools.varLib.instancer import instantiateVariableFont


EXTRA_UI_SYMBOLS = "℃℉°±×÷≤≥≠≈←→↑↓⇒•·…—–‘’“”、。「」『』【】（）《》：；？！％★●○✓"
AUTOSKIP_GO_FILES = {"platform_other.go"}
JAPANESE_KANA_RE = re.compile(r"[\u3040-\u309F\u30A0-\u30FF]")


@dataclass(frozen=True)
class CorpusGroups:
    ascii_chars: set[int]
    symbols: set[int]
    frontend_zh: set[int]
    themes_extra: set[int]
    backend_extra: set[int]

    @property
    def ui_base(self) -> set[int]:
        """Static frontend, theme metadata, ASCII, and UI symbols."""
        return self.ascii_chars | self.symbols | self.frontend_zh | self.themes_extra

    @property
    def all_chars(self) -> set[int]:
        """Complete curated software UI character set."""
        return self.ui_base | self.backend_extra


def strip_js_comments(src: str) -> str:
    """Remove // and /* ... */ comments outside JS/CSS string and template literals."""
    out: list[str] = []
    i, n = 0, len(src)
    while i < n:
        ch = src[i]
        if ch in ('"', "'", "`"):
            quote = ch
            out.append(ch)
            i += 1
            while i < n:
                cur = src[i]
                out.append(cur)
                if cur == "\\":
                    i += 1
                    if i < n:
                        out.append(src[i])
                    i += 1
                    continue
                if cur == quote:
                    i += 1
                    break
                i += 1
        elif ch == "/" and i + 1 < n and src[i + 1] == "/":
            i += 2
            while i < n and src[i] != "\n":
                i += 1
        elif ch == "/" and i + 1 < n and src[i + 1] == "*":
            i += 2
            while i + 1 < n and not (src[i] == "*" and src[i + 1] == "/"):
                i += 1
            i += 2
        else:
            out.append(ch)
            i += 1
    return "".join(out)


def extract_quoted_literals(src: str) -> list[str]:
    """Extract string literals from Go or C# source while ignoring comments and struct tags."""
    literals: list[str] = []
    i, n = 0, len(src)
    while i < n:
        ch = src[i]
        if ch == "/" and i + 1 < n and src[i + 1] == "/":
            while i < n and src[i] != "\n":
                i += 1
        elif ch == "/" and i + 1 < n and src[i + 1] == "*":
            i += 2
            while i + 1 < n and not (src[i] == "*" and src[i + 1] == "/"):
                i += 1
            i += 2
        elif ch == '"':
            i += 1
            buf: list[str] = []
            while i < n:
                cur = src[i]
                if cur == "\\":
                    i += 1
                    if i < n:
                        esc = src[i]
                        if esc == "u" and i + 4 < n:
                            try:
                                buf.append(chr(int(src[i + 1 : i + 5], 16)))
                                i += 5
                                continue
                            except ValueError:
                                pass
                        elif esc in ("n", "r", "t", "\\", '"'):
                            buf.append({"n": "\n", "r": "\r", "t": "\t", "\\": "\\", '"': '"'}[esc])
                            i += 1
                            continue
                        buf.append(esc)
                    i += 1
                    continue
                if cur == '"':
                    i += 1
                    break
                buf.append(cur)
                i += 1
            literals.append("".join(buf))
        elif ch == "`":
            i += 1
            buf = []
            while i < n and src[i] != "`":
                buf.append(src[i])
                i += 1
            i += 1
            text = "".join(buf)
            if not text.strip().startswith("json:"):
                literals.append(text)
        elif ch == "'":
            i += 1
            while i < n:
                if src[i] == "\\":
                    i += 2
                elif src[i] == "'":
                    i += 1
                    break
                else:
                    i += 1
        else:
            i += 1
    return literals


def walk_json_strings(obj):
    if isinstance(obj, str):
        yield obj
    elif isinstance(obj, dict):
        for val in obj.values():
            yield from walk_json_strings(val)
    elif isinstance(obj, list):
        for val in obj:
            yield from walk_json_strings(val)


def is_usable_ui_char(codepoint: int) -> bool:
    if codepoint < 0x80 or codepoint in (0x7F, 0xFEFF):
        return False
    if 0x3040 <= codepoint <= 0x30FF:  # Exclude Japanese Hiragana/Katakana
        return False
    if 0xE000 <= codepoint <= 0xF8FF:  # Exclude Private Use Area
        return False
    return True


def collect_ui_corpus(root: Path) -> CorpusGroups:
    ascii_chars = set(range(0x20, 0x7F))
    symbols = {ord(c) for c in EXTRA_UI_SYMBOLS}

    frontend_non_ascii: set[int] = set()
    for rel in ("frontend/src/app/locales/zh-CN/translation.json", "frontend/src/app/locales/en-US/translation.json"):
        data = json.loads((root / rel).read_text(encoding="utf-8"))
        for text in walk_json_strings(data):
            frontend_non_ascii.update(ord(c) for c in text if is_usable_ui_char(ord(c)))

    fe_src = root / "frontend/src"
    for dirpath, _, filenames in os.walk(fe_src):
        if "locales" in Path(dirpath).parts:
            continue
        for fn in filenames:
            if fn.endswith((".ts", ".tsx", ".css")):
                cleaned = strip_js_comments((Path(dirpath) / fn).read_text(encoding="utf-8"))
                frontend_non_ascii.update(ord(c) for c in cleaned if is_usable_ui_char(ord(c)))

    frontend_zh = frontend_non_ascii - symbols

    themes_all: set[int] = set()
    for tj in sorted(glob.glob(str(root / "themes/*/theme.json"))):
        for text in walk_json_strings(json.loads(Path(tj).read_text(encoding="utf-8"))):
            themes_all.update(ord(c) for c in text if is_usable_ui_char(ord(c)))
    for tc in sorted(glob.glob(str(root / "themes/*/theme.css"))):
        cleaned = strip_js_comments(Path(tc).read_text(encoding="utf-8"))
        for m in re.finditer(r'content\s*:\s*(["\'])(.*?)\1', cleaned):
            themes_all.update(ord(c) for c in m.group(2) if is_usable_ui_char(ord(c)))
    themes_extra = themes_all - symbols - frontend_zh

    backend_all: set[int] = set()
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [
            d for d in dirnames if d not in (".git", "node_modules", "frontend", "build", "Cache", "exported-themes", "themes")
        ]
        rel_dp = Path(dirpath).relative_to(root).as_posix()
        if rel_dp.startswith("cmd/brbprobe"):
            continue
        for fn in filenames:
            if fn.endswith(".go") and not fn.endswith("_test.go") and fn not in AUTOSKIP_GO_FILES:
                raw = (Path(dirpath) / fn).read_text(encoding="utf-8")
                # Ignore Windows schtasks Japanese output matcher (not displayed in UI)
                raw = raw.replace("実行するタスク:", "")
                for lit in extract_quoted_literals(raw):
                    backend_all.update(ord(c) for c in lit if is_usable_ui_char(ord(c)))

    for cs in sorted(glob.glob(str(root / "bridge/TempBridge/*.cs"))):
        for lit in extract_quoted_literals(Path(cs).read_text(encoding="utf-8")):
            backend_all.update(ord(c) for c in lit if is_usable_ui_char(ord(c)))

    backend_extra = backend_all - symbols - frontend_zh - themes_extra

    return CorpusGroups(
        ascii_chars=ascii_chars,
        symbols=symbols,
        frontend_zh=frontend_zh,
        themes_extra=themes_extra,
        backend_extra=backend_extra,
    )


def format_corpus_file(groups: CorpusGroups) -> str:
    def render_sorted(cps: set[int]) -> str:
        return "".join(chr(c) for c in sorted(cps))

    lines = [
        "# FanControl UI Font Corpus (generated by tools/extract_ui_corpus.py)",
        f"# Total: {len(groups.all_chars)} characters (ASCII={len(groups.ascii_chars)}, Non-ASCII={len(groups.all_chars - groups.ascii_chars)})",
        "#",
        f"# [1] Printable ASCII (A-Z, a-z, 0-9, Punctuation - {len(groups.ascii_chars)} chars)",
        render_sorted(groups.ascii_chars),
        "",
        f"# [2] UI Symbols & Full-Width Punctuation ({len(groups.symbols)} chars)",
        render_sorted(groups.symbols),
        "",
        f"# [3] Frontend zh-CN Locale (incl. All Settings) + TSX/TS UI Chinese ({len(groups.frontend_zh)} chars)",
        render_sorted(groups.frontend_zh),
        "",
        f"# [4] Built-in Theme Metadata & CSS Content Delta ({len(groups.themes_extra)} chars)",
        render_sorted(groups.themes_extra),
        "",
        f"# [5] Backend Go & TempBridge Runtime UI Delta ({len(groups.backend_extra)} chars)",
        render_sorted(groups.backend_extra),
        "",
    ]
    return "\n".join(lines)


def load_corpus_file(path: Path) -> set[int]:
    chars: set[int] = set()
    for line in path.read_text(encoding="utf-8").splitlines():
        if line.startswith("#") or not line:
            continue
        chars.update(ord(c) for c in line)
    return chars


def cmap_of(tt: TTFont) -> set[int]:
    out: set[int] = set()
    for table in tt["cmap"].tables:
        out.update(table.cmap.keys())
    return out


def format_unicode_ranges(codepoints: set[int]) -> str:
    if not codepoints:
        return ""
    ordered = sorted(codepoints)
    ranges: list[str] = []
    start = prev = ordered[0]
    for cp in ordered[1:]:
        if cp == prev + 1:
            prev = cp
            continue
        ranges.append(f"U+{start:X}" if start == prev else f"U+{start:X}-{prev:X}")
        start = prev = cp
    ranges.append(f"U+{start:X}" if start == prev else f"U+{start:X}-{prev:X}")
    return ", ".join(ranges)


def parse_unicode_range_css(expr: str) -> set[int]:
    cps: set[int] = set()
    for part in expr.split(","):
        token = part.strip().upper()
        if not token.startswith("U+"):
            continue
        body = token[2:]
        if "-" in body:
            a, b = body.split("-", 1)
            cps.update(range(int(a, 16), int(b, 16) + 1))
        elif "?" in body:
            start = int(body.replace("?", "0"), 16)
            end = int(body.replace("?", "F"), 16)
            cps.update(range(start, end + 1))
        else:
            cps.add(int(body, 16))
    return cps


def update_font_face_unicode_range(css_path: Path, font_filename: str, codepoints: set[int]) -> None:
    raw = css_path.read_text(encoding="utf-8")
    newline = "\r\n" if "\r\n" in raw else "\n"
    range_str = format_unicode_ranges(codepoints)
    pattern = re.compile(
        r"(@font-face\s*\{[^}]*?url\([\"'][^\"']*" + re.escape(font_filename) + r"[\"']\)[^}]*?unicode-range:\s*)([^;]+)(;)",
        re.DOTALL,
    )
    updated, count = pattern.subn(rf"\g<1>{range_str}\g<3>", raw, count=1)
    if count != 1:
        raise RuntimeError(f"Expected 1 @font-face unicode-range block for {font_filename} in {css_path}, found {count}")
    css_path.write_bytes(updated.replace("\r\n", "\n").replace("\n", newline).encode("utf-8"))


def save_subset_woff2(tt: TTFont, unicodes: set[int], out_path: Path, drop_hinting: bool = False) -> set[int]:
    opts = Options()
    opts.no_hinting = drop_hinting
    opts.desubroutinize = False
    opts.layout_features = ["*"]
    opts.name_IDs = ["*"]
    opts.name_languages = ["*"]
    if drop_hinting:
        opts.drop_tables += ["vhea", "vmtx", "cvt ", "fpgm", "prep", "gasp"]
    else:
        opts.drop_tables += ["vhea", "vmtx"]
    sub = Subsetter(options=opts)
    avail = unicodes & cmap_of(tt)
    sub.populate(unicodes=avail)
    sub.subset(tt)
    tt.flavor = "woff2"
    out_path.parent.mkdir(parents=True, exist_ok=True)
    tt.save(out_path)
    return cmap_of(TTFont(out_path))


def build_maoken_woff2(root: Path, unicodes: set[int], out_path: Path) -> set[int]:
    shards = sorted(glob.glob(str(root / "Cache/maoken-fontsource/*.woff2")))
    if not shards:
        raise FileNotFoundError("Missing Cache/maoken-fontsource/*.woff2 shards")
    char_to_shard: dict[int, str] = {}
    for sp in shards:
        for cp in cmap_of(TTFont(sp)):
            if cp not in char_to_shard:
                char_to_shard[cp] = sp

    selected = unicodes & set(char_to_shard.keys())
    buckets: dict[str, set[int]] = {}
    for cp in selected:
        buckets.setdefault(char_to_shard[cp], set()).add(cp)

    tmpdir = Path(tempfile.mkdtemp(prefix="maoken_build_"))
    try:
        part_paths: list[str] = []
        for idx, (sp, cps) in enumerate(buckets.items()):
            tt = TTFont(sp)
            opts = Options()
            opts.no_hinting = True
            opts.layout_features = ["*"]
            opts.name_IDs = ["*"]
            opts.name_languages = ["*"]
            opts.drop_tables += ["vhea", "vmtx"]
            sub = Subsetter(options=opts)
            sub.populate(unicodes=cps)
            sub.subset(tt)
            tt.flavor = None
            pp = tmpdir / f"part_{idx}.ttf"
            tt.save(pp)
            part_paths.append(str(pp))

        merged = Merger().merge(part_paths)
        merged.flavor = "woff2"
        out_path.parent.mkdir(parents=True, exist_ok=True)
        merged.save(out_path)
        return cmap_of(TTFont(out_path))
    finally:
        shutil.rmtree(tmpdir, ignore_errors=True)


def load_git_or_disk_font(root: Path, rel_path: str, git_ref: str | None) -> TTFont:
    if git_ref:
        raw = subprocess.check_output(["git", "-C", str(root), "show", f"{git_ref}:{rel_path}"])
        return TTFont(io.BytesIO(raw))
    return TTFont(root / rel_path)


def rebuild_fonts(root: Path, groups: CorpusGroups, shinchan_ref: str | None = None) -> None:
    target = groups.all_chars

    # 1. miku-01 (Xiaolai-Regular.ttf covers 100%)
    xiaolai_src = root / "Cache/miku-01/fonts/xiaolai-v3.126/Xiaolai-Regular.ttf"
    if not xiaolai_src.exists():
        raise FileNotFoundError(xiaolai_src)

    ui_base_set = groups.ui_base
    runtime_delta_set = target - ui_base_set

    miku_ui_path = root / "themes/miku-01/assets/fonts/miku-xiaolai-ui-subset.woff2"
    miku_rt_path = root / "themes/miku-01/assets/fonts/miku-xiaolai-runtime-patch.woff2"
    miku_ui_cps = save_subset_woff2(TTFont(xiaolai_src), ui_base_set, miku_ui_path, drop_hinting=True)
    miku_rt_cps = save_subset_woff2(TTFont(xiaolai_src), runtime_delta_set, miku_rt_path, drop_hinting=True)

    miku_css = root / "themes/miku-01/theme.css"
    update_font_face_unicode_range(miku_css, "miku-xiaolai-runtime-patch.woff2", miku_rt_cps)
    update_font_face_unicode_range(miku_css, "miku-xiaolai-ui-subset.woff2", miku_ui_cps)
    print(f"[miku-01] ui-subset: {miku_ui_path.stat().st_size:,} B ({len(miku_ui_cps)} chars), runtime-patch: {miku_rt_path.stat().st_size:,} B ({len(miku_rt_cps)} chars)")

    # 2. xiaoba-deluxe (MaokenZhuyuanTi from Cache/maoken-fontsource covers all Chinese + ASCII)
    xiaoba_round_path = root / "themes/xiaoba-deluxe/assets/fonts/xiaoba-round-subset.woff2"
    maoken_cps = build_maoken_woff2(root, target, xiaoba_round_path)
    maoken_missing = target - maoken_cps
    print(f"[xiaoba-deluxe] xiaoba-round-subset.woff2: {xiaoba_round_path.stat().st_size:,} B ({len(maoken_cps)} chars), missing from Maoken: {[chr(c) for c in sorted(maoken_missing)]}")

    # 3. shinchan (ChillRoundM + ChillReunion subset + NotoSansSC patch)
    cr_rel = "themes/shinchan/assets/fonts/shinchan-chillround-ui-subset.woff2"
    cu_rel = "themes/shinchan/assets/fonts/shinchan-chillreunion-title-subset.woff2"
    cr_font = load_git_or_disk_font(root, cr_rel, shinchan_ref)
    cu_font = load_git_or_disk_font(root, cu_rel, shinchan_ref)

    cr_cps = save_subset_woff2(cr_font, target, root / cr_rel, drop_hinting=False)
    cu_cps = save_subset_woff2(cu_font, target, root / cu_rel, drop_hinting=False)

    # Single-glyph rounded .woff patches kept alongside UI font
    woff_patch_cps: set[int] = set()
    for wp in sorted(glob.glob(str(root / "themes/shinchan/assets/fonts/advanced-glyph-patch-*.woff"))):
        woff_patch_cps.update(cmap_of(TTFont(wp)))

    # 4. Shared NotoSansSC-VF wght=600 patch for any corpus char missing in ChillRoundM / ChillReunion / Maoken
    shinchan_missing = target - cr_cps - woff_patch_cps
    patch_needed = shinchan_missing | (target - cu_cps - cr_cps) | maoken_missing

    noto_src = root / "Cache/font-sources/noto-sans-sc/NotoSansSC-VF.ttf"
    if not noto_src.exists():
        raise FileNotFoundError(noto_src)
    noto_inst = instantiateVariableFont(TTFont(noto_src), {"wght": 600}, inplace=False)

    shinchan_patch_path = root / "themes/shinchan/assets/fonts/advanced-glyph-patch-ui.woff2"
    xiaoba_patch_path = root / "themes/xiaoba-deluxe/assets/fonts/advanced-glyph-patch-ui.woff2"
    patch_cps = save_subset_woff2(noto_inst, patch_needed, shinchan_patch_path, drop_hinting=True)
    shutil.copy2(shinchan_patch_path, xiaoba_patch_path)

    update_font_face_unicode_range(root / "themes/shinchan/theme.css", "advanced-glyph-patch-ui.woff2", shinchan_missing & patch_cps)
    update_font_face_unicode_range(root / "themes/xiaoba-deluxe/theme.css", "advanced-glyph-patch-ui.woff2", maoken_missing & patch_cps)

    print(f"[shinchan] chillround: {(root / cr_rel).stat().st_size:,} B ({len(cr_cps)} chars), chillreunion: {(root / cu_rel).stat().st_size:,} B ({len(cu_cps)} chars), patch: {shinchan_patch_path.stat().st_size:,} B ({len(patch_cps)} chars)")


def verify_coverage(root: Path, groups: CorpusGroups) -> list[str]:
    errors: list[str] = []
    target = groups.all_chars

    corpus_path = root / "tools/ui-font-corpus.txt"
    if not corpus_path.exists():
        errors.append(f"Missing {corpus_path}")
    else:
        recorded = load_corpus_file(corpus_path)
        missing_in_file = target - recorded
        if missing_in_file:
            errors.append(f"ui-font-corpus.txt missing {len(missing_in_file)} live chars: {''.join(chr(c) for c in sorted(missing_in_file))}")

    def check_theme(theme_name: str, font_specs: list[tuple[str, str | None]]):
        covered: set[int] = set()
        css_text = (root / "themes" / theme_name / "theme.css").read_text(encoding="utf-8")
        for font_rel, range_file in font_specs:
            font_cps = cmap_of(TTFont(root / "themes" / theme_name / "assets/fonts" / font_rel))
            if range_file:
                m = re.search(
                    r"@font-face\s*\{[^}]*?url\([\"'][^\"']*" + re.escape(range_file) + r"[\"']\)[^}]*?unicode-range:\s*([^;]+);",
                    css_text,
                    re.DOTALL,
                )
                if not m:
                    errors.append(f"[{theme_name}] missing @font-face unicode-range for {range_file}")
                    continue
                declared = parse_unicode_range_css(m.group(1))
                uncovered_in_font = declared - font_cps
                if uncovered_in_font:
                    errors.append(f"[{theme_name}] {range_file} unicode-range declares glyphs absent from binary font: {''.join(chr(c) for c in sorted(uncovered_in_font))}")
                covered.update(font_cps & declared)
            else:
                covered.update(font_cps)

        missing = target - covered
        if missing:
            errors.append(f"[{theme_name}] missing {len(missing)} required UI chars: {''.join(chr(c) for c in sorted(missing))}")

    check_theme(
        "miku-01",
        [
            ("miku-xiaolai-ui-subset.woff2", "miku-xiaolai-ui-subset.woff2"),
            ("miku-xiaolai-runtime-patch.woff2", "miku-xiaolai-runtime-patch.woff2"),
        ],
    )

    woff_names = sorted(Path(p).name for p in glob.glob(str(root / "themes/xiaoba-deluxe/assets/fonts/advanced-glyph-patch-*.woff")))
    check_theme(
        "xiaoba-deluxe",
        [("xiaoba-round-subset.woff2", None), ("advanced-glyph-patch-ui.woff2", "advanced-glyph-patch-ui.woff2")]
        + [(w, w) for w in woff_names],
    )
    check_theme(
        "shinchan",
        [
            ("shinchan-chillround-ui-subset.woff2", None),
            ("shinchan-chillreunion-title-subset.woff2", None),
            ("advanced-glyph-patch-ui.woff2", "advanced-glyph-patch-ui.woff2"),
        ]
        + [(w, w) for w in woff_names],
    )

    return errors


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parent.parent)
    parser.add_argument("--write-corpus", action="store_true", help="Write tools/ui-font-corpus.txt")
    parser.add_argument("--rebuild-fonts", action="store_true", help="Rebuild theme WOFF2 subsets and CSS unicode-ranges")
    parser.add_argument("--shinchan-ref", default=None, help="Optional git ref (e.g. 63f05700) to recover pre-trimmed Chill fonts before subsetting")
    parser.add_argument("--verify", action="store_true", help="Verify binary theme fonts and CSS unicode-ranges cover 100% of live UI characters")
    args = parser.parse_args()

    groups = collect_ui_corpus(args.root)
    if args.write_corpus:
        out_path = args.root / "tools/ui-font-corpus.txt"
        out_path.write_text(format_corpus_file(groups), encoding="utf-8", newline="\n")
        print(f"Wrote {out_path} ({len(groups.all_chars)} unique chars)")

    if args.rebuild_fonts:
        rebuild_fonts(args.root, groups, shinchan_ref=args.shinchan_ref)

    if args.verify:
        errors = verify_coverage(args.root, groups)
        if errors:
            for err in errors:
                print("ERROR:", err, file=sys.stderr)
            return 1
        print(f"VERIFIED: 100% of {len(groups.all_chars)} curated UI characters are covered in miku-01, xiaoba-deluxe, and shinchan.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
