# FanControl 高级主题模板

这个版本基于「小八 Plus」的主题结构，适合做完整角色主题。它包含本地 WebP 插画、SVG 装饰、WebP 幕布、字体子集、页面级背景、卡片透明、hover 动效和 FanControl 新增的 `data-theme-*` 钩子。

## 怎么改成自己的主题

1. 把文件夹 `advanced` 改成你的主题 ID，例如 `cute-blue`。
2. 打开 `theme.json`，把 `id` 改成同样的 ID，把 `name` 改成显示名称。
3. 打开 `theme.css`，把所有 `html[data-theme="advanced"]` 改成你的主题 ID。
4. 可选：把变量名里的 `--advanced-*` 改成自己的前缀。
5. 替换 `hero.webp`、`assets/stickers/curtain.webp` 或 `decorations/*.svg` 时，保持相对路径不变最省事。

## 资源结构

```text
advanced/
  theme.json
  theme.css
  hero.webp                         首页插画，复杂图像推荐 WebP
  assets/
    fonts/
      xiaoba-round-subset.woff2     字体子集，尽量小
    stickers/
      curtain.webp                  背景幕布，复杂纹理推荐 WebP
  decorations/
    cloud.svg                       小装饰，推荐 SVG
    heart.svg
    sparkles.svg
    star.svg
```

## 常改变量

```css
--advanced-hero-image: url("hero.webp");
--advanced-curtain-image: url("assets/stickers/curtain.webp");
--advanced-sidebar-logo-text: "Advanced";
--advanced-sidebar-logo-image: none;
```

左侧徽标用文字：改 `--advanced-sidebar-logo-text`。
左侧徽标用图片：放入 `logo.svg` 或 `logo.webp`，设置 `--advanced-sidebar-logo-image: url("logo.svg")`，再把文字改成 `""`。

## 删减方式

- 不要首页插画：把 `--advanced-hero-image` 改成 `none`，或删除 `.glacier-hero-art::before`。
- 不要幕布背景：把 `--advanced-curtain-image` 改成 `none`，或删除 `.glacier-content-panel::before`。
- 不要贴纸：把 `decorations/*.svg` 相关变量改成 `none`，或删除对应 `::before` / `::after`。
- 不要字体：删除 `@font-face`，把字体变量改回系统字体。
- 觉得太透明：把 `rgba(..., 0.3)` 调到 `0.65-0.85`。

## 适用范围

- 适合 FanControl 2.3.0 及之后版本。
- 使用了 FanControl 新增的 `data-theme-card`、`data-theme-ui`、`data-theme-page` 等高级钩子。
- 如果拿给 THRM 或旧版软件使用，基础变量和 `.glacier-*` 部分通常能生效，但高级页面钩子可能被忽略。
