#!/usr/bin/env python3
"""Build a WOFF2 subset and optional unicode-range CSS for a theme font."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

from fontTools.subset import Options, Subsetter
from fontTools.ttLib import TTFont


TEXT_SUFFIXES = {".css", ".html", ".js", ".jsx", ".json", ".md", ".mjs", ".py", ".ts", ".tsx", ".txt"}


def iter_text_files(inputs: list[Path]):
    for path in inputs:
        if path.is_dir():
            yield from (item for item in path.rglob("*") if item.is_file() and item.suffix.lower() in TEXT_SUFFIXES)
        elif path.is_file():
            yield path
        else:
            raise FileNotFoundError(path)


def read_codepoints(inputs: list[Path], literals: list[str], include_ascii: bool) -> set[int]:
    chars = set(range(0x20, 0x7F)) if include_ascii else set()
    for path in iter_text_files(inputs):
        chars.update(ord(char) for char in path.read_text(encoding="utf-8-sig", errors="replace"))
    for literal in literals:
        chars.update(ord(char) for char in literal)
    return chars


def cmap(font: TTFont) -> set[int]:
    codepoints: set[int] = set()
    for table in font["cmap"].tables:
        codepoints.update(table.cmap)
    return codepoints


def format_range(start: int, end: int) -> str:
    if start == end:
        return f"U+{start:X}"
    return f"U+{start:X}-{end:X}"


def unicode_ranges(codepoints: set[int]) -> str:
    if not codepoints:
        return ""
    ordered = sorted(codepoints)
    ranges: list[str] = []
    start = previous = ordered[0]
    for codepoint in ordered[1:]:
        if codepoint == previous + 1:
            previous = codepoint
            continue
        ranges.append(format_range(start, previous))
        start = previous = codepoint
    ranges.append(format_range(start, previous))
    return ", ".join(ranges)


def write_css(path: Path, family: str, source_url: str, weight: str, codepoints: set[int]) -> None:
    css = (
        "@font-face {\n"
        f'  font-family: "{family}";\n'
        f'  src: url("{source_url}") format("woff2");\n'
        f"  font-weight: {weight};\n"
        "  font-style: normal;\n"
        "  font-display: swap;\n"
        f"  unicode-range: {unicode_ranges(codepoints)};\n"
        "}\n"
    )
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(css, encoding="utf-8", newline="\n")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", type=Path, required=True, help="Mother TTF/OTF/WOFF2 font")
    parser.add_argument("--output", type=Path, required=True, help="Output WOFF2 subset")
    parser.add_argument("--input", type=Path, action="append", default=[], help="Text file or directory; repeatable")
    parser.add_argument("--text", action="append", default=[], help="Literal text; repeatable")
    parser.add_argument(
        "--base-font",
        type=Path,
        action="append",
        default=[],
        help="Subtract codepoints covered by every base subset; repeatable for a shared patch",
    )
    parser.add_argument("--corpus-output", type=Path, help="Write the collected corpus")
    parser.add_argument("--css-output", type=Path, help="Write an @font-face unicode-range block")
    parser.add_argument("--css-url", default="", help="URL used by --css-output")
    parser.add_argument("--font-family", default="Theme Font")
    parser.add_argument("--font-weight", default="400 900")
    parser.add_argument("--no-ascii", action="store_true", help="Do not add printable ASCII automatically")
    args = parser.parse_args()

    source = TTFont(args.source)
    source_cmap = cmap(source)
    requested = read_codepoints(args.input, args.text, include_ascii=not args.no_ascii)
    missing = sorted(requested - source_cmap)
    selected = requested & source_cmap

    if args.base_font:
        covered_by_all = set.intersection(*(cmap(TTFont(path)) for path in args.base_font))
        selected -= covered_by_all

    if not selected:
        raise SystemExit("no source glyphs remain after filtering")

    if args.corpus_output:
        args.corpus_output.parent.mkdir(parents=True, exist_ok=True)
        args.corpus_output.write_text("".join(chr(codepoint) for codepoint in sorted(requested)), encoding="utf-8", newline="\n")

    options = Options()
    options.flavor = "woff2"
    options.layout_features = ["*"]
    options.no_hinting = True
    options.desubroutinize = True
    options.name_IDs = ["*"]
    options.name_languages = ["*"]
    subsetter = Subsetter(options=options)
    subsetter.populate(unicodes=selected)
    subsetter.subset(source)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    source.save(args.output)

    if args.css_output:
        write_css(args.css_output, args.font_family, args.css_url, args.font_weight, selected)

    print(json.dumps({"requested": len(requested), "selected": len(selected), "missing_from_source": missing}, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except ImportError as error:
        print(f"fontTools/brotli is required: {error}", file=sys.stderr)
        raise
