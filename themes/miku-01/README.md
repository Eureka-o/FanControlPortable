# Hatsune Miku / Digital Stage

FanControl advanced light theme using the existing advanced-theme layout with a dedicated Miku visual system.

## Design

- The full advanced-theme cascade is retained for the home page, curve editor, settings, device library, dialogs, scrollbars, typography, and narrow-window behavior.
- The title bar, sidebar, and content area share one continuous surface without hard panel separators; the sidebar uses a Dune-style inset rounded glass frame.
- Cards reuse the Crayon Shin-chan theme's sparse glass hierarchy, recolored with Miku theme tokens so the wallpaper remains visible without sacrificing readability.
- The full-body Miku artwork stays aligned to the right and is fitted by height so the complete character remains in frame.
- The device Hero follows the Xiaoba Plus layout, stays against the right edge, and fades in without reserving an oversized blank area.
- Home metric and chart cards lift on hover; the five compact status tiles use a smaller lift with semantic Phosphor Fill watermarks. Cards on other pages stay still.
- UI text, headings, values, and chart labels use one local rounded Xiaolai subset for a consistent cute Miku style.

## External Assets

All runtime assets are external files. The CSS contains no Base64 artwork or fonts.

- `assets/character/wallpaper-miku-user.webp`: compressed runtime copy of the user-provided full-body Miku artwork with the bottom typography removed.
- `assets/character/hero-miku-q-cutout.png`: small optimized runtime Hero with only the edge-connected white background removed; PNG keeps WebView and cached-theme compatibility.
- The original PNG sources, including the unused `hero-miku-q.png`, are kept in `Cache/miku-01/source-images/`.
- `assets/fonts/miku-xiaolai-ui-subset.woff2`: local UI subset generated from Xiaolai Regular under SIL Open Font License 1.1.
- `assets/fonts/miku-xiaolai-runtime-patch.woff2`: a UI/runtime patch generated from the frontend source and installed FanControl `config.json`, placed before the main subset so small cards, settings labels, and saved device/profile names all use Xiaolai.
- The reusable generator is `tools/make_theme_font_subset.py`; the full source font and generated corpora are kept in `Cache/miku-01/fonts/xiaolai-v3.126/`.

To refresh the runtime patch after changing UI text or the installed config, run the generator with `--input frontend/src --input "D:\\Programs Files\\FanControl\\config\\config.json"` and `--base-font assets/fonts/miku-xiaolai-ui-subset.woff2`; keep the generated WOFF2 in `assets/fonts/` and the corpus in `Cache/`.
- `assets/icons/iconoir/*.svg`: thin technical accents from Iconoir under the MIT License.
- `assets/icons/phosphor-fill/*.svg`: rounded compact-card watermarks from Phosphor Icons under the MIT License.

The two character images are for this personal-use theme only. Do not publish or redistribute the theme until their original sources and permissions are confirmed.

See the font license files under `assets/fonts/`, the icon license files under `licenses/`, and `licenses/ASSET-SOURCES.md`.
