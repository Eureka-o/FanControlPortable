# FanControl 主题 DIY 规范与模板说明

这份文档面向两类人：

- 只想让 AI 或自己改一套简单主题的人。
- 想做小八 Plus 那种完整资源包主题的人。

模板已经拆成两个版本：

```text
docs/theme-template/
  basic/      基础版：THRM/FanControl 尽量通用，只改变量和 glacier-* 组件
  advanced/   高级版：基于小八 Plus，使用 FanControl 2.3.0 高级钩子和资源包
```

## 1. 选哪个模板

### Basic

适合：

- 只改主色、背景、卡片、图表、左侧徽标。
- 希望 THRM 也尽量能用。
- 不想处理复杂资源和页面级定制。

特点：

- 有完整默认浅色变量，不是空壳。
- 只使用 `html[data-theme="basic"]`、CSS 变量、`.glacier-*`、`button[role="switch"]` 等常见钩子。
- 不使用 `data-theme-card`、`data-theme-ui` 这类 FanControl 新增高级钩子。
- 演示了左侧徽标的文字写法和图片/SVG 写法。

### Advanced

适合：

- 想做完整视觉主题。
- 想用插画、贴纸、背景幕布、字体子集、页面级背景、卡片透明和按钮动效。
- 主要给 FanControl 2.3.0 及之后版本使用。

特点：

- 基于小八 Plus 的完整技术路线。
- 使用相对资源：`hero.webp`、`assets/stickers/curtain.webp`、`decorations/*.svg`、`assets/fonts/*.woff2`。
- 使用 FanControl 新增钩子：`data-theme-page`、`data-theme-card`、`data-theme-ui`。
- 旧版或 THRM 不支持的高级选择器会被浏览器忽略；基础变量和 `.glacier-*` 仍然尽量保留可读效果。

## 2. 主题包结构

最小结构：

```text
basic/
  theme.json
  theme.css
  README.md
```

高级结构：

```text
advanced/
  theme.json
  theme.css
  README.md
  hero.webp
  assets/
    fonts/
      xiaoba-round-subset.woff2
    stickers/
      curtain.webp
  decorations/
    cloud.svg
    heart.svg
    sparkles.svg
    star.svg
```

分享时保留一层主题文件夹：

```text
your-theme.zip
  your-theme/
    theme.json
    theme.css
    README.md
    hero.webp
    assets/
    decorations/
```

不要打包这些内容：

- `Cache`
- PSD 工程文件
- 截图
- 中间生成图
- 本机绝对路径依赖的文件

## 3. 改名步骤

以 `advanced` 为例：

1. 把文件夹 `advanced` 改成你的主题 ID，例如 `cute-blue`。
2. 打开 `theme.json`，把 `"id": "advanced"` 改成 `"id": "cute-blue"`。
3. 打开 `theme.css`，把所有 `html[data-theme="advanced"]` 改成 `html[data-theme="cute-blue"]`。
4. 可选：把 `--advanced-*` 变量前缀改成 `--cute-blue-*`。

主题 ID 规则：

- 只能使用小写英文字母、数字、短横线 `-`、下划线 `_`。
- 文件夹名必须和 `theme.json` 的 `id` 一致。
- 显示名可以写中文，放在 `theme.json` 的 `name` 字段。

## 4. theme.json 规范

JSON 不能写 `// 注释`。模板里用 `$comment` 和 `$help` 做说明，这是合法 JSON。

```json
{
  "$comment": "说明文字可以写这里，不能写 // 注释。",
  "$help": {
    "id": "主题唯一 ID，必须和文件夹名一致。",
    "name": "显示在设置页里的主题名，可以中文。",
    "base": "light 或 dark。",
    "author": "作者名。",
    "version": "主题版本。",
    "description": "一句话说明主题。"
  },
  "id": "basic",
  "name": "基础主题模板",
  "base": "light",
  "author": "FanControl User",
  "version": "1.0.0",
  "description": "基于默认浅色效果的基础主题模板。"
}
```

## 5. CSS 作用域

所有 CSS 都必须写在主题作用域里：

```css
html[data-theme="basic"] {
  --primary: #2f6df6;
}

html[data-theme="basic"] .glacier-shell {
  background: var(--background);
}
```

不要写全局样式：

```css
body { background: red; }
.glacier-shell { background: red; }
```

原因：全局样式会污染其他主题。

## 6. 资源怎么选

### SVG

适合：

- 左侧徽标
- 小图标
- 贴纸线稿
- 简单图形
- 抓痕、星星、云朵、箭头等可缩放装饰

优点：

- 文件小。
- 放大不糊。
- 可以直接改颜色或用 CSS 调透明度。

示例：

```css
--basic-sidebar-logo-image: url("logo.svg");

html[data-theme="basic"] .glacier-sidebar::after {
  background: var(--basic-sidebar-logo-image) center / 70% 70% no-repeat;
}
```

### WebP

适合：

- 人物插画
- 照片
- 复杂背景
- 纸纹、幕布、贴纸拼图
- 高细节颜色图片

优点：

- 比 PNG 小。
- 比 SVG 更适合复杂图像。

示例：

```css
--advanced-hero-image: url("hero.webp");
--advanced-curtain-image: url("assets/stickers/curtain.webp");
```

不要使用：

```css
url("C:/Users/xxx/Desktop/hero.webp")
url("D:/xxx/hero.webp")
url("https://example.com/hero.webp")
```

原因：别人电脑上会失效，或者网络图片加载不稳定。

## 7. 左侧徽标怎么做

### 文字徽标

适合：

- 主题名短。
- 想保持包很小。
- 想做纵向字样或花体字样。

Basic 模板示例：

```css
--basic-sidebar-logo-text: "Basic";
```

优点：不用额外资源。
缺点：不同电脑字体不一样，显示效果可能有差异。

### SVG/图片徽标

适合：

- 品牌字样、角色 logo、复杂轮廓。
- 希望每台电脑显示一致。

Basic 模板示例：

```css
--basic-sidebar-logo-image: url("logo.svg");
--basic-sidebar-logo-text: "";
```

优点：效果稳定。
缺点：需要额外文件。

## 8. 字体怎么选

推荐顺序：

1. 系统自带字体：最轻量，不增加主题包体积。
2. 小型 woff2 子集：适合主题名、少量标题、中文装饰字。
3. 完整字体：不推荐，体积容易过大。

高级模板里有字体子集示例：

```css
@font-face {
  font-family: "Xiaoba Round";
  src: url("assets/fonts/xiaoba-round-subset.woff2") format("woff2");
  font-weight: 400 700;
  font-style: normal;
  font-display: swap;
}
```

注意：

- 字体子集要包含主题名里用到的字。
- 如果主题名显示成方块或系统字体，通常是字体子集没包含这些字。
- 正文 UI 字体不要太花，否则设置页可读性会下降。

## 9. 背景和透明度

### 背景 WebP 怎么选

适合做背景的图：

- 对比度低。
- 没有大片高饱和文字。
- 边缘可以裁切。
- 不会压住按钮、曲线和正文。

不适合做背景的图：

- 细节非常密。
- 有很多文字。
- 高对比强光。
- 人物脸刚好在文字区域后面。

### 不想画背景怎么办

Basic 模板：

```css
--basic-background-image: none;
```

Advanced 模板：

```css
--advanced-curtain-image: none;
```

然后删除或注释掉对应的 `::before` 背景幕布块即可。

### 透明度怎么选

经验值：

- `0.90 - 1.00`：最清楚，适合设置页。
- `0.65 - 0.85`：能透出背景，也比较稳。
- `0.30 - 0.60`：视觉强，但容易看不清字。

例子：

```css
background: rgba(255, 255, 255, 0.72);
```

想更清楚：把 `0.72` 改成 `0.85`。
想更透：把 `0.72` 改成 `0.55`。

## 10. 渐变怎么写

线性渐变：

```css
background: linear-gradient(135deg, #f6f8fc 0%, #f9fbff 52%, #eef4ff 100%);
```

局部光斑：

```css
background:
  radial-gradient(circle at 78% 10%, rgba(47, 109, 246, 0.08), transparent 26rem),
  linear-gradient(135deg, #f6f8fc 0%, #f9fbff 52%, #eef4ff 100%);
```

多层背景规则：

- 第一层写在最前面，显示在最上面。
- 最后一层通常放纯色或主渐变兜底。
- WebP 背景建议放第一层或第二层。

## 11. 不同界面怎么分

FanControl 新版会在外壳上标出页面：

```css
.glacier-shell[data-theme-page="status"]   /* 首页 */
.glacier-shell[data-theme-page="control"]  /* 设置/控制页 */
.glacier-shell[data-theme-page="curve"]    /* 曲线页 */
.glacier-shell[data-theme-page="about"]    /* 关于页 */
.glacier-shell[data-theme-page="devices"]  /* 高级设备页 */
```

Basic 模板只轻量演示页面背景：

```css
html[data-theme="basic"] .glacier-shell[data-theme-page="status"] .glacier-content-panel {
  background: var(--theme-page-home-background, transparent);
}
```

Advanced 模板会分页面处理卡片、幕布、状态页小卡片、设置页和关于页。

## 12. 关键组件清单

基础组件：

```css
.glacier-shell          /* 应用整体 */
.glacier-titlebar       /* 顶部栏 */
.glacier-sidebar        /* 左侧菜单 */
.glacier-content        /* 右侧内容外层 */
.glacier-content-panel  /* 右侧滚动内容区域 */
.glacier-hero-card      /* 首页顶部大卡片 */
.glacier-metric-card    /* 首页温度/转速大卡片 */
.glacier-control-card   /* 控制与保护大卡片 */
.glacier-stat-tile      /* 控制与保护小卡片 */
.glacier-chart-card     /* 图表卡片 */
.glacier-chart-canvas   /* 图表画布 */
```

FanControl 高级钩子：

```css
[data-theme-card="fan-curve-preview"]
[data-theme-card="temperature-history"]
[data-theme-card="control-mode"]
[data-theme-card="cpu-power"]
[data-theme-ui="setting-section"]
[data-theme-ui="setting-card"]
[data-theme-ui="sidebar-item"]
[data-theme-ui="select-trigger"]
```

Basic 模板不要依赖高级钩子。Advanced 模板可以用。

## 13. 怎么好加东西

建议新增资源时这样做：

1. 把资源放进主题文件夹，例如 `decorations/moon.svg`。
2. 在变量区声明：

```css
--advanced-moon: url("decorations/moon.svg");
```

3. 在组件里使用：

```css
html[data-theme="advanced"] .glacier-chart-card::after {
  background: var(--advanced-moon) center / contain no-repeat;
}
```

这样以后删资源时，只需要删变量和对应组件块。

## 14. 怎么好删东西

删主题效果时优先按块删除：

- 不要左侧徽标：删 `.glacier-sidebar::after`。
- 不要首页插画：删 `.glacier-hero-art::before`。
- 不要背景幕布：删 `.glacier-content-panel::before`。
- 不要小贴纸：删 `.glacier-metric-card::after`、`.glacier-stat-tile::before` 等装饰块。
- 不要动画：删 `@keyframes` 和 `animation`。
- 不要字体：删 `@font-face`，把字体变量改回系统字体。

## 15. AI 提示词

```text
请帮我制作 FanControl 自定义主题。

请基于模板：
- basic：只用默认浅色变量和 glacier-* 组件，尽量兼容 THRM。
- advanced：基于小八 Plus 技术路线，可以使用资源包、字体、贴纸和 FanControl data-theme-* 高级钩子。

要求：
1. 不修改软件源码，只改 theme.json、theme.css 和主题资源。
2. CSS 必须写在 html[data-theme="主题ID"] 作用域下。
3. JSON 不能写 // 注释，说明文字用 $comment 或 $help。
4. 资源必须使用相对路径，例如 url("hero.webp")。
5. 不要使用 C:/、D:/ 绝对路径，不要依赖网络图片。
6. 保证首页、曲线页、设置页、关于页文字可读。
7. 不要让装饰遮挡按钮、进度条、图标、曲线和文字。
8. 如果用了透明、blur、动画和大图，要控制性能。

主题需求：
- 主题名：
- 主题 ID：
- basic 还是 advanced：
- 主色：
- 辅色：
- 角色/插画：
- 背景元素：
- 左侧徽标用文字还是图片：
- 想要的风格：
- 不想要的元素：
```

## 16. 验收清单

- 文件夹名和 `theme.json` 的 `id` 一致。
- `theme.css` 中所有选择器都以 `html[data-theme="主题id"]` 开头。
- 没有 `C:\Users\...`、`D:\...`、`https://...` 这类不可移植资源。
- 所有 `url("...")` 指向的文件都在主题包里。
- 首页、曲线页、设置页、关于页都能看清文字。
- 左侧菜单、顶部栏、按钮、开关、下拉框没有被装饰遮挡。
- 曲线图、历史图、温度/转速/功耗卡片没有直角露底或奇怪边框。
- 如果主题用了很多图片、透明、blur 或动画，观察 CPU/GPU 占用；卡顿就减少装饰层、动画和大图。
