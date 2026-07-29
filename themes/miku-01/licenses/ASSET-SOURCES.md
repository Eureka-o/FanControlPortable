# Asset Sources

## Character artwork

- `assets/character/wallpaper-miku-user.webp`: WebP runtime derivative of the user-provided full-body artwork; bottom typography was removed from the white background and the character was not cropped.
- `assets/character/hero-miku-q-cutout.png`: optimized PNG runtime derivative of the local transparent-background Hero, retained for WebView and cached-CSS compatibility.
- Original PNG sources, including the unused `hero-miku-q.png`, are retained in `Cache/miku-01/source-images/`.
- Intended use: personal/local FanControl theme only.
- Redistribution: not cleared. Confirm the original sources and permissions before publishing or bundling this theme in a public release.

## Typography

- `assets/fonts/miku-xiaolai-ui-subset.woff2` is generated from the locally cached Xiaolai Regular v3.126 source using the current FanControl UI corpus plus ASCII and common unit symbols.
- `assets/fonts/miku-xiaolai-runtime-patch.woff2` is generated from `frontend/src` (including `frontend/src/app/locales/zh-CN/translation.json`) and the installed `D:\Programs Files\FanControl\config\config.json` with the same source font, subtracting glyphs already present in the main subset. This keeps settings, compact cards, and saved device/profile names covered without shipping the mother font.
- Xiaolai source: `lxgw/kose-font`, release `v3.126`; official source: `https://github.com/lxgw/kose-font`; license: `licenses/LICENSE-xiaolai.txt`.
- The full source font and generation corpora are kept outside the import package at `Cache/miku-01/fonts/xiaolai-v3.126/`.

## Interface icons

- All SVG files under `assets/icons/iconoir/` are from `iconoir-icons/iconoir`.
- License: `licenses/ICONOIR-MIT.txt`.
- All SVG files under `assets/icons/phosphor-fill/` are from `phosphor-icons/core`.
- License: `licenses/PHOSPHOR-MIT.txt`.
