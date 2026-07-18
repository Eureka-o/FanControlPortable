# FanControl 主题 DIY 完整指南

> 适用于 FanControl 2.5.x 及之后版本的基础主题和高级主题。本文既是设计说明，也是可以交给作者、设计师或 AI 使用的工程规范。

本文解释主题包的每一个文件、字段、CSS 作用域、页面钩子、组件钩子、资源路径、字体子集、调试方法和发布验收规则。主题只修改 `theme.json`、`theme.css` 和主题资源，不修改 FanControl 源码。

---

## 0. 先理解主题系统

FanControl 的主题不是一个独立插件进程，也不是一个网页。它是一组被 GUI 读取的静态资源：

1. GUI 启动时，后端扫描可执行文件旁边的 `themes/` 目录，并读取每个主题的 `theme.json`。
2. 前端主题同步组件根据配置中的 `themeMode` 决定使用系统、浅色、深色或自定义主题。
3. 选择自定义主题时，GUI 后端读取该主题的 `theme.css`。
4. CSS 被放入页面的 `<style id="thrm-custom-theme-style">`，并给 `<html>` 加上 `data-theme` 属性。
5. CSS 中的 `html[data-theme="主题ID"]` 选择器因此只影响当前主题。
6. CSS 引用的本地图片、SVG 和字体通过 `/theme-assets/<主题ID>/<相对路径>` 提供。

主题只运行在 GUI 层，不会进入 FanControl Core，也不会直接触碰设备、温度采集或风扇控制逻辑。联网下载主题时，网络任务同样应该属于 GUI，不应阻塞 Core 的监控循环。

### 0.1 主题加载关系

```text
theme.json
    |
    v
主题列表 -----> 设置页主题选择器
    |
    v
theme.css -----> <style> + html[data-theme="id"]
    |
    v
相对资源 -----> /theme-assets/id/path
    |
    v
页面组件上的 glacier-* / data-theme-* 钩子
```

上面的每一条箭头都对应一个约束：清单负责“我是谁”，CSS 负责“怎么画”，资源路由负责“从哪里取文件”，组件钩子负责“影响哪一个界面部件”。

### 0.2 当前实现中可以复用的代码

主题作者通常不需要阅读 Go 代码，但理解下面几个入口有助于排查问题：

| 文件 | 作用 |
| --- | --- |
| `internal/theme/theme.go` | 扫描清单、迁移旧主题、读取 CSS、读取资源、校验主题 ID 和资源路径 |
| `internal/guiapp/theme_api.go` | 向前端提供 `ListThemes`、`GetThemeCSS`、`OpenThemesFolder` |
| `main.go` | 注册 `/theme-assets/` 资源处理器 |
| `frontend/src/app/components/SystemThemeSync.tsx` | 读取配置并应用系统或自定义主题 |
| `frontend/src/app/components/settings/SystemSettingsSection.tsx` | 在设置页加载主题列表并保存选择 |
| `frontend/src/app/globals.css` | 定义 FanControl 本身的基础外壳和高级钩子 |
| `docs/theme-template/basic/` | 兼容性优先的基础模板 |
| `docs/theme-template/advanced/` | 可使用页面、卡片、字体、贴纸和背景资源的高级模板 |

---

## 1. 选择模板：Basic 还是 Advanced

### 1.1 Basic 基础主题

Basic 适合颜色、背景、卡片、图表和左侧徽标调整。它只依赖稳定的 CSS 变量、`.glacier-*` 外壳类和原生 HTML 属性，适合希望同时兼容 THRM 或旧版本的作者。

Basic 的设计目标是“少改结构、保持可读”。它不应该依赖 FanControl 新增的 `data-theme-card` 和 `data-theme-ui` 细粒度钩子。这样即使旧版本忽略某些高级选择器，基础颜色和布局仍然可用。

### 1.2 Advanced 高级主题

Advanced 适合完整视觉设计，包括首页插画、背景幕布、贴纸、字体子集、页面级背景、卡片透明度、hover 动效和高级组件钩子。它主要面向 FanControl 2.3.0 及之后版本。

Advanced 仍然应该保留基础层的颜色变量和可读性兜底。高级选择器被旧软件忽略时，页面应该退化为清晰的基础主题，而不是变成无法操作的空白界面。

### 1.3 选择决策

| 需求 | 推荐 |
| --- | --- |
| 只改颜色、圆角、卡片和图表 | Basic |
| 要兼容 THRM 或旧版本 | Basic |
| 要使用首页插画、幕布和贴纸 | Advanced |
| 要绑定 FanControl 的首页、曲线页、设置页 | Advanced |
| 要使用主题专用字体 | Advanced，并使用字体子集 |
| 不确定自己能否维护复杂 CSS | Basic |

---

## 2. 主题目录结构

### 2.1 最小可用结构

```text
my-theme/
  theme.json       # 主题身份和元数据
  theme.css        # 主题样式
  README.md        # 可选：作者说明、许可证、预览说明
```

### 2.2 高级主题结构

```text
my-theme/
  theme.json
  theme.css
  README.md
  hero.webp                    # 首页插画或主视觉
  logo.svg                     # 可选：侧栏徽标
  assets/
    fonts/
      my-theme-ui.woff2       # 可选：UI 字体子集
      LICENSE-font.txt        # 字体许可证
    stickers/
      curtain.webp             # 可选：背景幕布
  decorations/
    cloud.svg
    star.svg
```

### 2.3 分享和上传时的 ZIP 结构

ZIP 内建议保留一层主题目录：

```text
my-theme-1.0.0.zip
  my-theme/
    theme.json
    theme.css
    README.md
    hero.webp
    assets/
    decorations/
```

这样用户解压后可以直接把 `my-theme/` 放进 `themes/`，也便于安装流程进行目录校验。

不要放入主题包：

| 不应包含 | 原因 |
| --- | --- |
| `Cache/`、`node_modules/`、`.git/` | 开发过程文件，会增大包体积 |
| PSD、AI、工程源文件 | 设计源文件不属于运行时资源 |
| 截图原图和中间渲染图 | 只保留最终预览和运行时资源 |
| `C:\Users\...`、`D:\...` 路径 | 其他电脑无法读取 |
| `.exe`、`.dll`、`.js`、`.html` | 主题只需要 CSS 和静态资源 |
| 未授权的完整商业字体 | 可能违反字体许可证 |

---

## 3. 第一次创建主题

### 3.1 复制模板

基础主题从这里复制：

```text
docs/theme-template/basic/
```

高级主题从这里复制：

```text
docs/theme-template/advanced/
```

不要直接在内置主题目录上改动。复制模板的目的，是保留主题所需的变量兜底、页面结构兼容和注释。

### 3.2 修改主题 ID

假设新主题叫 `cute-blue`，需要同步修改三个位置：

1. 文件夹名改为 `cute-blue`。
2. `theme.json` 中的 `id` 改为 `cute-blue`。
3. `theme.css` 中所有 `html[data-theme="advanced"]` 或 `html[data-theme="basic"]` 改为 `html[data-theme="cute-blue"]`。

如果只改了文件夹或只改了 JSON，主题可能出现在列表里，但 CSS 不会生效。ID 同步是最常见、也最容易漏掉的步骤。

### 3.3 ID 命名规则

主题 ID 必须满足：

```text
^[a-z0-9_-]{1,64}$
```

含义如下：

| 片段 | 意思 |
| --- | --- |
| `a-z` | 只能使用小写英文字母 |
| `0-9` | 可以使用数字 |
| `-` | 短横线可用于分词 |
| `_` | 下划线可用于兼容旧命名 |
| `{1,64}` | 长度至少 1，最多 64 |

显示名称 `name` 不受这个限制，可以写中文、空格和品牌名称。

---

## 4. `theme.json` 逐字段说明

JSON 不支持 `//` 注释。模板使用 `$comment` 和 `$help`，因为它们是普通 JSON 字段，解析器可以安全忽略。

### 4.1 完整示例

```json
{
  "$comment": "这里写给主题作者看的说明，运行时会忽略。",
  "$help": {
    "id": "唯一 ID，必须和文件夹名一致。",
    "name": "显示在设置页里的名称。",
    "base": "light 或 dark。",
    "layer": "basic 或 advanced。",
    "author": "作者名。",
    "version": "主题版本。",
    "description": "主题简介。"
  },
  "id": "cute-blue",
  "name": "Cute Blue",
  "base": "light",
  "layer": "advanced",
  "author": "FanControl User",
  "version": "1.0.0",
  "description": "蓝白配色的轻量高级主题。"
}
```

### 4.2 字段解释

| 字段 | 必填 | 允许值 | 运行时作用 |
| --- | --- | --- | --- |
| `$comment` | 否 | 任意字符串 | 给作者阅读，不参与主题身份 |
| `$help` | 否 | 对象 | 给复制模板的人阅读，不参与主题身份 |
| `id` | 推荐 | 小写字母、数字、`-`、`_` | 目录名、`data-theme` 值和资源路由中的主题名 |
| `name` | 否 | 任意字符串 | 设置页显示名称；空时退回 ID |
| `base` | 否 | `light` 或 `dark` | 应用主题 CSS 前使用的明暗基础 |
| `layer` | 否 | `basic` 或 `advanced` | 告诉前端该主题是否使用高级钩子 |
| `interface` | 兼容 | `basic` 或 `advanced` | 老模板字段，会被当作 `layer` 使用 |
| `author` | 否 | 任意字符串 | 设置页和主题说明中显示作者 |
| `version` | 否 | 建议使用 `主.次.修` | 主题升级和迁移时比较版本 |
| `description` | 否 | 任意字符串 | 主题列表和作者说明 |

### 4.3 `base` 的含义

`base` 不是主题的完整样式，它只是告诉应用先使用浅色还是深色基础变量：

```text
base = light  -> html 不加 dark 类
base = dark   -> html 加 dark 类
```

自定义主题不是 `system`。如果用户选择自定义主题，主题自己的 `base` 决定明暗；系统深浅变化不会自动把自定义主题改成另一套颜色。

### 4.4 `layer` 的含义

`basic` 代表只依赖通用结构；`advanced` 代表可以使用 FanControl 高级钩子。该字段不会自动授予主题权限，也不会替作者生成 CSS，只用于前端选择正确的兼容层和主题状态。

### 4.5 版本号规则

建议使用语义化版本：

```text
1.0.0       首个公开版本
1.1.0       新增视觉功能或资源
1.1.1       修复颜色、路径或可读性问题
2.0.0       大幅改变布局或资源结构
```

不要用时间戳代替版本号。安装器会按数字片段比较版本；`1.10.0` 应该高于 `1.9.0`。

---

## 5. CSS 的核心规则：作用域、顺序和变量

### 5.1 所有选择器都要进入主题作用域

正确写法：

```css
/* 主题入口：下面所有规则只对 cute-blue 生效。 */
html[data-theme="cute-blue"] {
  --primary: #2f6df6; /* 主色，按钮、选中态和强调文字会复用。 */
}

/* 只修改主题外壳，不影响其他主题。 */
html[data-theme="cute-blue"] .glacier-shell {
  background: var(--background); /* 使用应用已经计算好的背景变量。 */
}
```

错误写法：

```css
/* 错误：会影响系统主题和其他自定义主题。 */
body {
  background: red;
}

/* 错误：没有 data-theme 限制作用域。 */
.glacier-shell {
  background: red;
}
```

### 5.2 为什么不应该直接覆盖 `body`

FanControl 的窗口背景、原生材质、系统明暗和自定义主题是多层组合。全局 `body` 规则容易覆盖基础主题的透明处理，也可能让切换回系统主题后留下自定义颜色。主题应当从 `html[data-theme="id"]` 开始，并尽量改变量而不是重写布局。

### 5.3 CSS 变量覆盖优先于重复布局

推荐顺序：

1. 先修改现有 CSS 变量。
2. 再修改 `.glacier-*` 的颜色、边框、阴影和背景。
3. 只有确实需要时，才使用 `data-theme-card` 和 `data-theme-ui`。
4. 不要通过负边距、绝对定位和大范围 `!important` 重建整个页面。

### 5.4 CSS 文件组织顺序

建议把 `theme.css` 分为以下区块，并用注释标明：

```css
/* 1. Theme identity: ID prefix and main variables. */
/* 2. Global shell: window, title bar, sidebar, content. */
/* 3. Cards and controls: hero, metrics, charts, inputs. */
/* 4. Page hooks: status, curve, control, about, devices. */
/* 5. Assets and fonts: @font-face and local URLs. */
/* 6. Responsive rules: narrow windows and reduced motion. */
/* 7. Final guards: clipping, readability, and known component fixes. */
```

文件末尾的“Final guards”不是鼓励堆叠覆盖，而是给已知组件留下一个清晰的最终校正区。每条覆盖都应说明它修复了哪个组件和为什么放在末尾。

### 5.5 代码块的详细规范

代码块不是把几行 CSS 粘贴进文档就结束了。每个示例都应该让读者能回答三个问题：它作用于哪个节点、它解决什么视觉问题、删掉它会发生什么。推荐在代码块前写一句目的说明，在复杂规则旁保留一条“原因 + 约束”注释。

| 代码位置 | 必须说明 | 示例问题 |
| --- | --- | --- |
| 选择器第一行 | 页面、卡片或控件范围 | 这是首页主卡片，还是所有页面的按钮？ |
| CSS 变量区 | 变量的语义和回退 | `--cute-blue-border` 是边框色，不是文字色 |
| 视觉属性 | 修改后影响的层级 | `box-shadow` 只表达层级，不要盖住文字 |
| 交互状态 | 普通、hover、focus、checked 是否完整 | 只写 hover 会导致键盘 focus 不可见 |
| 最终覆盖区 | 为什么必须放在文件末尾 | 它修复的是曲线图内部圆角，而不是重复布局 |

示例应保持“一个代码块解决一个问题”。下面的代码只负责设置行的颜色和 focus，不应顺便改动页面宽度：

```css
/* 目标：设置行可读，键盘 focus 可见，且 hover 不改变布局尺寸。 */
html[data-theme="cute-blue"] [data-theme-ui="setting-row"] {
  background: var(--cute-blue-setting-row-background);
  border-color: var(--cute-blue-border);
}

html[data-theme="cute-blue"] [data-theme-ui="setting-row"]:focus-within {
  outline: 2px solid var(--cute-blue-focus-ring);
  outline-offset: 2px;
}
```

不要把整份 `theme.css` 放进指南正文。完整主题应该放在模板目录，指南只保留能解释接口的最小片段；这样作者可以逐行理解，也不会在复制时带入与当前页面无关的规则。

### 5.6 代码表格的排版和解释方法

代码表格用于建立“代码入口 -> 视觉结果”的映射，而不是替代正文。表格中的代码列应保持左对齐、使用等宽字体；说明列使用短句，每格只表达一个动作。需要长篇解释时，应放到表格前后的段落中。

| 代码入口 | 所在层 | 主要作用 | 不应承担的工作 |
| --- | --- | --- | --- |
| `html[data-theme="id"]` | 主题作用域 | 限制样式只作用于当前主题 | 不负责切换系统明暗 |
| `.glacier-shell` | 应用外壳 | 设置窗口背景和基础文字色 | 不重建页面 flex 布局 |
| `[data-theme-card="curve-editor"]` | 曲线卡片 | 修正编辑器外层的视觉层级 | 不假定它就是内部绘图区 |
| `[data-theme-ui="setting-row"]` | 交互组件 | 设置行背景、边框和 focus | 不在 hover 时修改高度 |
| `@font-face` | 资源层 | 注册授权的 WOFF2 子集 | 不加载远程字体 |

表格行的顺序建议从稳定入口到细粒度入口：先写主题作用域，再写外壳类、页面/卡片钩子，最后写交互组件。这样读者可以从上到下建立覆盖层级，也能快速定位“规则没有生效”是作用域问题还是选择器问题。

### 5.7 正文段落与对齐规则

正文每段只讲一个判断或一个操作。推荐先给结论，再解释原因，最后补充限制；一段通常保持两到四句。不要把表格中的每一行再逐字重复成一串短段落，也不要用连续单行文本模拟表格。

代码、路径、属性值放在行内代码中，中文解释保持正常比例字体。路径很长时在目录分隔符处换行，不要为了“看起来一行”缩小字号。代码块使用固定的行间距，表格使用较宽的行高；两者都不应通过负边距压缩。

段落和表格之间留出明确的垂直间距。表格标题行使用轻微底色，正文行使用交替浅色或留白区分；颜色只用于层级提示，不能成为理解内容的唯一条件。

---

## 6. FanControl 外壳组件（`.glacier-*`）

这些类是主题最稳定的入口。它们描述的是布局角色，而不是某一种颜色。

| 类名 | 组件角色 | 常见用途 |
| --- | --- | --- |
| `.glacier-shell` | 应用总外壳 | 背景、圆角、整体字体、窗口边界 |
| `.glacier-titlebar` | 自定义标题栏 | 高度、透明度、底部描边 |
| `.glacier-sidebar` | 左侧导航栏 | 背景、宽度适配、徽标、选中态 |
| `.glacier-content` | 右侧内容外层 | 内容区布局和滚动外层 |
| `.glacier-content-panel` | 实际滚动面板 | 页面背景、幕布、内容内边距 |
| `.glacier-native-backdrop` | 原生材质存在时的外壳 | 透明度、系统背景兜底 |
| `.glacier-hero-card` | 首页顶部大卡片 | 主视觉、设备名称、快捷操作 |
| `.glacier-hero-content` | 大卡片文字和按钮区域 | 留出插画空间、控制层级 |
| `.glacier-hero-actions` | 大卡片操作按钮区 | 对齐、胶囊容器、按钮间距 |
| `.glacier-hero-art` | 首页插画容器 | 插画裁切、渐隐和装饰标签 |
| `.glacier-hero-art-label` | 插画文字标签 | 小型装饰标题，不能遮挡主信息 |
| `.glacier-operator-art` | 插画图片元素 | `object-fit`、透明度和混合模式 |
| `.glacier-metric-card` | 温度、转速等指标卡 | 背景、环形图、hover 阴影 |
| `.glacier-control-card` | 控制与保护大卡片 | 模式、功耗、保护开关区域 |
| `.glacier-stat-tile` | 控制卡内的小磁贴 | 当前模式、温度状态、功耗数值 |
| `.glacier-chart-card` | 曲线和历史图卡片 | 图表外框、装饰、标题和状态 |
| `.glacier-chart-canvas` | 图表绘制区域 | 网格、内阴影、扫描光和背景 |
| `.glacier-page-card-fade` | 页面卡片进入动画 | 控制渐入，不要制造布局跳动 |

### 6.1 外壳示例

```css
/* 外壳只负责主题背景，不改变页面的 flex 和 overflow。 */
html[data-theme="cute-blue"] .glacier-shell {
  background: var(--cute-blue-shell-background);
  color: var(--foreground);
}

/* 侧栏是固定宽度区域，装饰不要改变它的尺寸。 */
html[data-theme="cute-blue"] .glacier-sidebar {
  background: var(--cute-blue-sidebar-background);
  border-color: var(--cute-blue-border);
}

/* 内容面板允许滚动，背景装饰应当使用伪元素而不是插入布局节点。 */
html[data-theme="cute-blue"] .glacier-content-panel {
  background: var(--cute-blue-content-background);
}
```

解释：第一段改变总背景和文字颜色；第二段只改变侧栏的视觉，不改变 `width`；第三段只改变滚动区域的底色，避免背景装饰参与内容高度计算。

---

## 7. 页面级钩子：`data-theme-page`

FanControl 会在 `.glacier-shell` 上标记当前页面。页面钩子适合做大块背景和页面级视觉差异，不适合写通用按钮颜色。

当前常用页面值：

| 值 | 页面 |
| --- | --- |
| `status` | 首页 / 设备状态 |
| `curve` | 风扇曲线和历史趋势 |
| `control` | 设置和控制面板 |
| `about` | 关于页 |
| `devices` | 高级设备信息页 |

示例：

```css
/* 首页可以有主视觉，但内容区仍然保持清晰。 */
html[data-theme="cute-blue"] .glacier-shell[data-theme-page="status"] .glacier-content-panel {
  background: var(--cute-blue-home-background);
}

/* 曲线页减少背景装饰，给图表留出稳定对比度。 */
html[data-theme="cute-blue"] .glacier-shell[data-theme-page="curve"] .glacier-content-panel {
  background: var(--cute-blue-curve-background);
}
```

旧版本如果没有 `data-theme-page`，这些规则会自然失效；因此页面钩子应该是增强层，不能是唯一的可读性来源。

---

## 8. 卡片级钩子：`data-theme-card`

卡片钩子用于“同一页面中的一个语义卡片”。它比通用类更精确，但只在 FanControl 新版本中可靠。

| 值 | 组件 |
| --- | --- |
| `settings-overview` | 设置页顶部设备概览 |
| `settings-overview-temperature` | 设置页顶部温度概览 |
| `settings-overview-device` | 设置页顶部设备概览 |
| `settings-offline-tip` | 设置页离线提示 |
| `device-hero` | 首页设备主信息卡 |
| `fan-curve-preview` | 首页小型曲线预览 |
| `temperature-history` | 首页温度 / 风扇历史图 |
| `cpu-temperature` | CPU 温度指标卡 |
| `gpu-temperature` | GPU 温度指标卡 |
| `fan-speed` | 风扇转速指标卡 |
| `control-mode` | 控制模式磁贴 |
| `temperature-state` | 温度状态磁贴 |
| `work-mode` | 工作模式磁贴 |
| `cpu-power` | CPU 功耗磁贴 |
| `gpu-power` | GPU 功耗磁贴 |
| `curve-header` | 曲线页标题和操作栏 |
| `curve-manual-gears` | 手动档位区域 |
| `curve-editor` | 曲线编辑器 |
| `curve-prediction` | 温度趋势预测区域 |
| `curve-learning` | 联合学习 / 学习控制区域 |
| `curve-history` | 历史趋势总卡片 |
| `curve-history-summary` | 历史摘要磁贴 |
| `curve-history-chart` | 历史图表卡片 |

示例：

```css
/* 只让 CPU 和 GPU 功耗磁贴使用强调色，避免影响其他统计项。 */
html[data-theme="cute-blue"] [data-theme-card="cpu-power"],
html[data-theme="cute-blue"] [data-theme-card="gpu-power"] {
  border-color: var(--cute-blue-power-border);
  background: var(--cute-blue-power-background);
}

/* 曲线编辑器外层有时只是布局锚点，真正的圆角卡片在内部。 */
html[data-theme="cute-blue"] [data-theme-card="curve-editor"] > .relative.rounded-3xl {
  border-radius: 1.25rem;
  overflow: hidden;
}
```

第二条规则解释了一个常见问题：如果只给外层设置背景，内部图表卡片可能仍然出现方角；如果只给内部设置圆角，外层装饰又可能露出原生窗口背景。需要同时检查外层和内部真实绘图区。

---

## 9. 组件级钩子：`data-theme-ui`

UI 钩子用于按钮、输入、设置行、切换器和弹窗等交互组件。它们适合调节边框、hover、focus、选中态和可读性。

| 值 | 组件 |
| --- | --- |
| `brand-mark` | 品牌或左上角标记 |
| `sidebar-item` | 左侧导航按钮 |
| `settings-tabs` | 设置页分区标签栏 |
| `settings-tab` | 设置页单个分区标签 |
| `settings-panels` | 设置页分区内容容器 |
| `settings-panel` | 设置页单个内容面板 |
| `setting-section` | 设置分区外框 |
| `setting-section-header` | 设置分区标题栏 |
| `setting-row` | 设置行 |
| `setting-row-icon` | 设置行左侧图标 |
| `setting-row-control` | 设置行右侧控制区 |
| `connection-panel` | 设备连接面板 |
| `compatibility-submenu` | 兼容模式子菜单 |
| `compatibility-submenu-row` | 兼容模式子菜单行 |
| `compatibility-nested` | 兼容模式嵌套内容 |
| `compatibility-nested-row` | 嵌套设置行 |
| `compatibility-nested-panel` | 嵌套控制面板 |
| `select-trigger` | 下拉选择器按钮 |
| `switch` | 开关轨道 |
| `switch-thumb` | 开关滑块 |
| `slider` | 滑块容器 |
| `manual-gear-dot` | 手动档位视觉节点 |
| `learning-target-temp` | 学习目标温度控制 |
| `curve-hints` | 曲线页提示标签 |
| `history-display-dialog` | 历史显示弹窗 |
| `history-display-row` | 历史显示项 |
| `hero-actions` | 首页主卡片操作区 |

### 9.1 设置行示例

```css
/* 设置行默认保持清楚的文字和边界。 */
html[data-theme="cute-blue"] [data-theme-ui="setting-row"] {
  background: var(--cute-blue-setting-row-background);
  border-color: var(--cute-blue-border);
}

/* hover 只增加层次，不移动行，防止设置页滚动位置跳动。 */
html[data-theme="cute-blue"] [data-theme-ui="setting-row"]:hover {
  background: var(--cute-blue-setting-row-hover);
  box-shadow: inset 2px 0 0 var(--cute-blue-accent);
}
```

不要在 hover 时修改 `padding`、`margin`、`height` 或 `border-width`，这些变化会造成整页布局抖动。

### 9.2 下拉框和开关示例

```css
/* 下拉触发按钮要同时处理普通、hover、打开和 focus-visible 状态。 */
html[data-theme="cute-blue"] [data-theme-ui="select-trigger"] {
  background: var(--cute-blue-control-background);
  border-color: var(--cute-blue-control-border);
  color: var(--foreground);
}

html[data-theme="cute-blue"] [data-theme-ui="select-trigger"]:hover,
html[data-theme="cute-blue"] [data-theme-ui="select-trigger"][data-state="open"] {
  border-color: var(--cute-blue-accent);
}

html[data-theme="cute-blue"] [data-theme-ui="select-trigger"]:focus-visible {
  outline: 2px solid var(--cute-blue-focus-ring);
  outline-offset: 2px;
}

/* 开关要同时覆盖未选中、选中和滑块颜色。 */
html[data-theme="cute-blue"] [data-theme-ui="switch"] {
  background: var(--cute-blue-switch-off);
}

html[data-theme="cute-blue"] [data-theme-ui="switch"][data-state="checked"] {
  background: var(--cute-blue-switch-on);
}

html[data-theme="cute-blue"] [data-theme-ui="switch-thumb"] {
  background: var(--cute-blue-switch-thumb);
}
```

---

## 10. Basic 模板变量逐组说明

Basic 模板的变量区应该是主题最先修改的地方。变量名以主题模板中的实际前缀为准；复制后建议将 `--basic-` 改为自己的前缀，例如 `--cute-blue-`。

### 10.1 变量分类总览

主题变量不是同一种东西。控制颜色决定文字、边框、按钮和状态是否可读；纹理变量只负责材质、渐变和图片；几何变量决定尺寸和层级。先判断变量类型，再决定它应该放在控制组件还是装饰层。

| 变量类别 | 典型变量 | CSS 值类型 | 负责什么和可感知范围 |
| --- | --- | --- | --- |
| 控制颜色 | `--background`、`--foreground`、`--card`、`--border`、`--primary` | `<color>` | 页面、文字、卡片、边框和主操作按钮；影响所有页面和交互控件。 |
| 状态颜色 | `--primary-foreground`、`--muted-foreground`、`--focus-ring`、`--switch-on` | `<color>` 或带透明度的 `<color>` | hover、focus、checked、禁用和错误状态；影响键盘操作、开关、下拉框和提示。 |
| 图表颜色 | `--chart-1` 至 `--chart-4` | `<color>` | CPU、GPU、风扇和功耗趋势；影响曲线页、历史图和悬浮提示。 |
| 纹理/材质 | `--basic-sidebar-background`、`--advanced-hero-image`、`--advanced-curtain-image` | `<image>`、`linear-gradient()` 或 `background` 简写 | 渐变、插画、幕布和背景质感；只影响首页、侧栏和内容面板，不改变控件语义。 |
| 透明保护层 | `--advanced-hero-overlay`、`--dialog-overlay` | 带 alpha 的 `<color>` | 在图片上方保护文字和按钮对比度；影响插画、弹窗和下拉菜单。 |
| 阴影层级 | `--basic-dialog-shadow`、`--card-shadow` | `<shadow>` | 表达卡片、弹窗和浮层的前后关系；影响边界、弹窗和 hover。 |
| 几何/层级 | `--radius`、`--radius-sm`、`--z-content`、`--z-overlay` | `<length>` 或 `<integer>` | 控制圆角、层级和装饰覆盖顺序；影响所有页面的布局稳定性。 |
| 资源/文字 | `--basic-sidebar-logo-image`、`--basic-sidebar-logo-text` | `<image>` 或 `<string>` | 注册侧栏图片徽标和文字徽标；影响左侧导航和品牌标记。 |

变量后缀也有固定含义。`-background` 通常是表面颜色或材质，`-foreground` 是表面上的文字/图标，`-border` 是边界，`-hover`、`-active`、`-checked` 和 `-focus-ring` 是状态，`-image` 是本地资源，`-overlay` 是保护层，`-shadow` 是阴影，`-radius` 和 `-z-*` 是结构参数。不要把图片写进 `--primary`，也不要把按钮文字色写进 `--hero-image`。

控制颜色变量必须始终有可读的配对关系。例如 `--primary` 负责按钮背景，`--primary-foreground` 负责按钮文字；`--card` 负责卡片表面，`--card-foreground` 负责卡片文字；`--background` 和 `--foreground` 负责页面默认层。纹理变量可以被删除或替换，但删除后必须退回到纯色背景。

```css
html[data-theme="cute-blue"] {
  /* 控制颜色：直接影响文字、按钮和边框。 */
  --primary: #2f6df6;
  --primary-foreground: #ffffff;
  --border: #d8e1ef;

  /* 纹理/材质：只影响背景，不改变控件语义。 */
  --hero-image: url("hero.webp");
  --hero-overlay: rgb(255 255 255 / 65%);
}
```

### 10.2 基础颜色

```css
html[data-theme="basic"] {
  --background: #f6f8fc;       /* 页面底色。 */
  --foreground: #172033;       /* 主文字颜色。 */
  --card: #ffffff;             /* 卡片底色。 */
  --card-foreground: #172033;  /* 卡片内文字。 */
  --muted: #eaf0f8;            /* 次级背景和禁用区域。 */
  --muted-foreground: #65738a; /* 次级文字。 */
  --border: #d8e1ef;           /* 通用边框。 */
  --primary: #2f6df6;          /* 主操作色。 */
  --primary-foreground: #ffffff; /* 主操作按钮文字。 */
}
```

不要只改变 `--primary` 而不检查 `--primary-foreground`。浅色按钮配浅色文字或深色按钮配深色文字都会产生低对比度。

### 10.3 图表颜色

功耗、CPU 温度、GPU 温度和风扇趋势可能同时出现。颜色必须彼此区分：

```css
html[data-theme="basic"] {
  --chart-1: #2f6df6; /* CPU 或主曲线。 */
  --chart-2: #e05252; /* GPU 或第二条温度曲线。 */
  --chart-3: #2b9b67; /* 风扇或状态曲线。 */
  --chart-4: #d59025; /* 功耗或辅助曲线。 */
}
```

不要把所有曲线都设置成同一色相的浅色版本。相邻曲线在悬浮提示和截图中必须能快速区分。

### 10.4 圆角和层级

```css
html[data-theme="basic"] {
  --radius: 0.75rem;       /* 通用圆角。 */
  --radius-sm: 0.5rem;     /* 小控件圆角。 */
  --z-content: 1;           /* 内容层。 */
  --z-decoration: 2;        /* 装饰层。 */
  --z-overlay: 20;          /* 弹窗和浮层。 */
}
```

装饰层不应该高于弹窗、下拉框和 toast。层级变量是基础模板中的安全栏，不建议随意增大。

### 10.5 侧栏、内容区和弹窗

```css
html[data-theme="basic"] {
  --basic-sidebar-background: linear-gradient(180deg, #ffffff, #eef5ff);
  --basic-sidebar-foreground: #26364f;
  --basic-content-background: #f6f8fc;
  --basic-dialog-background: #ffffff;
  --basic-dialog-border: #c9d5e7;
  --basic-dialog-shadow: 0 18px 60px rgb(29 52 87 / 18%);
}
```

背景可以使用渐变，但正文卡片应该保持足够不透明。弹窗和下拉菜单尤其不能透到难以读取。

### 10.6 徽标变量

```css
html[data-theme="basic"] {
  --basic-sidebar-logo-text: "Basic"; /* 文字徽标。 */
  --basic-sidebar-logo-image: none;    /* 图片徽标；有图片时写 url。 */
}
```

二选一最稳定。若同时保留文字和图片，必须确认二者不会重叠。

---

## 11. Advanced 模板的页面和资源设计

Advanced 不是“把所有装饰都打开”。它仍然需要先建立一个可读的结构，再逐层增加视觉。

### 11.1 首页主视觉

```css
html[data-theme="advanced"] {
  --advanced-hero-image: url("hero.webp"); /* 首页主插画。 */
  --advanced-hero-overlay: rgb(255 255 255 / 65%); /* 文字保护层。 */
}

html[data-theme="advanced"] .glacier-hero-art::before {
  content: ""; /* 伪元素必须有内容，才能绘制背景。 */
  position: absolute; /* 让插画脱离文字布局。 */
  inset: 0; /* 填满插画容器。 */
  background: var(--advanced-hero-image) right center / contain no-repeat;
  opacity: 0.72; /* 降低插画对文字的干扰。 */
  pointer-events: none; /* 装饰不能拦截按钮点击。 */
}
```

首页插画应该放在 `glacier-hero-art` 中，并给文字区域保留足够空间。不要把高对比人物脸放在设备名称或操作按钮正后方。

### 11.2 背景幕布

```css
html[data-theme="advanced"] {
  --advanced-curtain-image: url("assets/stickers/curtain.webp");
}

html[data-theme="advanced"] .glacier-content-panel::before {
  content: ""; /* 创建幕布层。 */
  position: absolute; /* 不参与内容高度。 */
  inset: 0; /* 覆盖整个内容面板。 */
  background: var(--advanced-curtain-image) top center / cover no-repeat;
  opacity: 0.18; /* 幕布只能提供质感。 */
  pointer-events: none; /* 不挡滚动和点击。 */
}
```

内容面板需要有 `position: relative` 和合适的 z-index，正文需要位于伪元素上方。背景幕布在设置页通常应该比首页更淡。

### 11.3 贴纸和小装饰

```css
html[data-theme="advanced"] {
  --advanced-star: url("decorations/star.svg");
}

html[data-theme="advanced"] .glacier-metric-card::after {
  content: ""; /* 创建装饰层。 */
  position: absolute; /* 不改变卡片尺寸。 */
  right: 0.75rem; /* 与卡片右边保持距离。 */
  bottom: 0.75rem; /* 与卡片底部保持距离。 */
  width: 2rem; /* 固定尺寸，防止布局抖动。 */
  height: 2rem;
  background: var(--advanced-star) center / contain no-repeat;
  opacity: 0.35; /* 装饰不应盖过数值。 */
  pointer-events: none;
}
```

伪元素装饰必须有固定尺寸、低透明度和 `pointer-events: none`。它不能影响卡片的实际内容高度。

### 11.4 卡片透明度

```css
html[data-theme="advanced"] [data-theme-card="temperature-history"] {
  background: rgb(255 255 255 / 82%); /* 图表背景保持高可读性。 */
  border-color: rgb(76 105 145 / 25%); /* 边框只保留层次。 */
  backdrop-filter: blur(14px); /* 背景复杂时只使用中等 blur。 */
}
```

透明度低于约 60% 时，图表网格和文字容易与背景混在一起。设置页、曲线编辑器和历史图通常应使用更高不透明度。

---

## 12. 左侧徽标和品牌标记

### 12.1 文字徽标

```css
html[data-theme="basic"] {
  --basic-sidebar-logo-text: "AUX"; /* 文字内容。 */
}

html[data-theme="basic"] .glacier-sidebar::after {
  content: var(--basic-sidebar-logo-text); /* 把变量写进伪元素。 */
  writing-mode: vertical-rl; /* 让短文本竖排。 */
  letter-spacing: 0.18em; /* 增加字间距。 */
  opacity: 0.55; /* 保持装饰性质。 */
}
```

字体应优先使用系统字体或主题已经提供的短文本字体。徽标文字过长会挤压侧栏。

### 12.2 图片徽标

```css
html[data-theme="advanced"] {
  --advanced-sidebar-logo-image: url("logo.svg"); /* 本地 SVG。 */
  --advanced-sidebar-logo-text: "";               /* 禁用备用文字。 */
}

html[data-theme="advanced"] .glacier-sidebar::after {
  content: ""; /* 使用图片时不输出文字。 */
  background: var(--advanced-sidebar-logo-image) center / 70% 70% no-repeat;
  opacity: 0.9; /* 品牌标记可以比普通装饰更清晰。 */
}
```

SVG 应尽量使用简单路径，不携带脚本、外链或嵌入 HTML。复杂插画用 WebP，不要把大图强行做成 SVG。

---

## 13. 图表、曲线和历史页

曲线页是最容易被装饰破坏可读性的页面。主题作者应先确保坐标轴、曲线、点、悬浮提示和下方控制区域清晰，再考虑背景。

### 13.1 曲线编辑器

```css
/* 外层只做定位锚点，不绘制一个额外的方形背景。 */
html[data-theme="advanced"] [data-theme-card="curve-editor"] {
  background: transparent;
  box-shadow: none;
}

/* 内层才是真正的图表卡片，需要负责圆角和裁切。 */
html[data-theme="advanced"] [data-theme-card="curve-editor"] > .relative.rounded-3xl {
  background: var(--advanced-chart-background);
  border-radius: 1.25rem;
  overflow: hidden;
}
```

### 13.2 手动档位

```css
html[data-theme="advanced"] [data-theme-card="curve-manual-gears"] {
  background: var(--advanced-control-background);
}

html[data-theme="advanced"] [data-theme-ui="manual-gear-dot"] {
  border-color: var(--advanced-accent);
  box-shadow: 0 0 0 3px rgb(47 109 246 / 12%);
}
```

手动档位点是操作目标，不能用装饰覆盖，也不要在 hover 时改变点的外部尺寸。

### 13.3 历史趋势

```css
html[data-theme="advanced"] [data-theme-card="curve-history"] {
  background: var(--advanced-history-background);
}

html[data-theme="advanced"] [data-theme-ui="history-display-dialog"] {
  background: var(--advanced-dialog-background);
  color: var(--foreground);
}

html[data-theme="advanced"] [data-theme-ui="history-display-row"] {
  border-color: var(--advanced-border);
}
```

历史图的功耗、温度和风扇趋势可能分成上下两个图。不要用同一张大背景图压住坐标轴，也不要给不同曲线使用相近颜色。

---

## 14. 设置页和设备连接页

设置页是信息密度最高的页面，视觉主题必须优先保护文字、下拉、开关、错误提示和滚动位置。

### 14.1 设置分区

```css
html[data-theme="advanced"] [data-theme-ui="setting-section"] {
  background: var(--advanced-setting-section-background);
  border-color: var(--advanced-border);
  box-shadow: var(--advanced-setting-shadow);
}

html[data-theme="advanced"] [data-theme-ui="setting-section-header"] {
  border-color: var(--advanced-border);
  color: var(--foreground);
}
```

设置分区的外框应该稳定。不要在切换分区时通过 CSS 动画改变高度，也不要让标题栏在 hover 时产生位移。

### 14.2 设备连接面板

```css
html[data-theme="advanced"] [data-theme-ui="connection-panel"] {
  background: var(--advanced-connection-background);
  border-color: var(--advanced-border);
}

html[data-theme="advanced"] [data-theme-ui="compatibility-submenu"] {
  background: var(--advanced-submenu-background);
  border-color: var(--advanced-border);
}
```

兼容模式下可能显示 IP、连接状态、扫描进度和重试按钮。装饰层必须位于这些控件之后，不能遮住状态文字。

---

## 15. 颜色、透明度、阴影和动效

### 15.1 颜色对比

至少检查以下组合：

| 前景 | 背景 | 必须检查 |
| --- | --- | --- |
| 标题文字 | 首页主卡片 | 设备名和模式名称清晰 |
| 次级文字 | 半透明卡片 | 描述和单位清晰 |
| 图表线 | 图表背景 | 多条线能区分 |
| 开关滑块 | 开关轨道 | 选中态明显 |
| 错误文字 | 错误背景 | 不只依靠红色，还要有文字 |
| focus ring | 控件边界 | 键盘操作可见 |

不要把可读性寄托在背景图片的“平均颜色”上。图片会因窗口尺寸、裁切和系统 DPI 改变。

### 15.2 阴影

阴影应表达层级，而不是装饰一切：

```css
html[data-theme="advanced"] .glacier-chart-card {
  box-shadow:
    0 8px 24px rgb(28 51 83 / 10%), /* 外部层次。 */
    inset 0 1px 0 rgb(255 255 255 / 45%); /* 内部高光。 */
}
```

过重的阴影会让透明背景变成灰块。设置页和曲线页应比首页少使用装饰阴影。

### 15.3 动效和减少动态效果

```css
html[data-theme="advanced"] [data-theme-card="fan-curve-preview"] {
  transition: box-shadow 180ms ease, transform 180ms ease;
}

html[data-theme="advanced"] [data-theme-card="fan-curve-preview"]:hover {
  transform: translateY(-1px); /* 只做轻微位移。 */
}

@media (prefers-reduced-motion: reduce) {
  html[data-theme="advanced"] * {
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.01ms !important;
  }
}
```

不要把扫描光、呼吸动画和大背景动画同时放在每个卡片上。动画应该是可关闭的增强效果，而不是信息传递的唯一方式。

---

## 16. 资源路径、图片和 SVG

### 16.1 只使用相对路径

正确：

```css
html[data-theme="cute-blue"] {
  --cute-blue-hero: url("hero.webp");
  --cute-blue-star: url("decorations/star.svg");
  --cute-blue-font: url("assets/fonts/cute-blue-ui.woff2");
}
```

错误：

```css
/* 绝对 Windows 路径：只在作者电脑上有效。 */
--hero: url("C:/Users/Alice/Desktop/hero.webp");

/* 外链：会受网络、域名和隐私策略影响。 */
--hero: url("https://example.com/hero.webp");
```

### 16.2 WebP 和 SVG 的选择

| 资源 | 推荐格式 | 说明 |
| --- | --- | --- |
| 复杂插画、照片、纹理 | WebP | 文件小，适合复杂像素内容 |
| 线稿、简单徽标、星星和云朵 | SVG | 可缩放，适合少量路径 |
| 字体 | WOFF2 | 体积小，浏览器支持好 |
| 透明复杂贴纸 | WebP 或 SVG | 视细节数量选择 |

### 16.3 资源大小建议

分享包和便携包都应该优先使用小资源：

| 资源 | 建议上限 |
| --- | --- |
| `hero.webp` | 约 500 KB |
| 单个贴纸 | 约 200 KB |
| 单个 SVG | 约 100 KB，尽量更小 |
| 单个字体子集 | 约 300 KB |
| 完整主题包 | 建议控制在 5 MB 内 |

这些是工程建议，不是软件强制的二进制限制；自动校验可以根据实际发布渠道调整。

---

## 17. 字体和字体子集

### 17.1 推荐顺序

1. 系统字体：体积为零，跨主题最稳定。
2. 小型 WOFF2 子集：用于标题、主题名或少量装饰文字。
3. 完整字体：除非许可证和包体积都明确允许，否则不推荐。

### 17.2 `@font-face` 逐行示例

```css
@font-face {
  font-family: "Cute Blue UI"; /* CSS 中引用的字体族名称。 */
  src: url("assets/fonts/cute-blue-ui.woff2") format("woff2"); /* 本地相对字体。 */
  font-weight: 400 700; /* 支持普通到粗体的可变范围。 */
  font-style: normal; /* 不声明斜体，避免浏览器合成假斜体。 */
  font-display: swap; /* 字体未加载时先用回退字体。 */
}

html[data-theme="cute-blue"] {
  --cute-blue-ui-font: "Cute Blue UI", "Segoe UI", "Microsoft YaHei UI", sans-serif;
}

html[data-theme="cute-blue"] .glacier-titlebar,
html[data-theme="cute-blue"] [data-theme-ui="setting-section-header"] {
  font-family: var(--cute-blue-ui-font); /* 只把字体用于适合的位置。 */
}
```

正文、设置描述和错误提示建议保留系统回退字体。主题字体只覆盖标题或明确的短文本区域，避免字形缺失导致整页方块。

### 17.3 字体子集脚本的设计

仓库可以提供 `tools/subset_theme_font.py`，开发者本地安装 `fonttools` 后运行。脚本不进入 FanControl 运行时，也不会增加安装包体积。

建议输入：

```text
原始字体文件 + 主题目录 + 主题模式(title/ui) + 可选文本文件
```

建议输出：

```text
assets/fonts/<theme-id>-<mode>-subset.woff2
assets/fonts/LICENSE-<font-name>.txt
```

脚本应完成以下工作：

1. 读取 `theme.json` 的 `name`、`description` 和作者说明。
2. 扫描 CSS 中的 `content: "..."` 字符和显式标题文本。
3. 合并 ASCII、数字、中文标点、主题专用字符和作者提供的文本文件。
4. 调用 `fontTools.pyftsubset`，输出 WOFF2。
5. 生成缺字报告，列出无法从源字体找到的字符。
6. 复制或检查字体许可证。

示例命令：

```powershell
# --font 是原始字体，--theme 是主题目录，--mode 决定字符集规模。
python tools/subset_theme_font.py `
  --font source.ttf `
  --theme themes/cute-blue `
  --mode ui `
  --output themes/cute-blue/assets/fonts/cute-blue-ui-subset.woff2
```

`title` 模式只保留主题名和装饰标题，包体积最小；`ui` 模式应额外包含软件常用标签和作者提供的界面文案。中文 UI 主题必须考虑用户切换语言后的英文、日文和标点回退。

### 17.4 字体缺字排查

| 现象 | 原因 | 修复 |
| --- | --- | --- |
| 主题名显示方块 | 子集没有主题名字符 | 把名称加入文本集合后重新裁剪 |
| 标题正常、正文全变系统字体 | 字体只覆盖标题或正文缺字 | 扩大 UI 子集或缩小字体作用范围 |
| 加粗时字形变化异常 | 子集没有对应权重或变量轴 | 保留正确 `font-weight` 范围 |
| 首次打开字体闪烁 | 字体尚未加载 | 保留 `font-display: swap` 和系统回退 |
| 包体积突然增大 | 打包了完整字体 | 使用 `pyftsubset` 生成 WOFF2 |

---

## 18. 外部资源和安全边界

主题 CSS 会被注入 GUI 页面，因此“只是 CSS”不等于完全没有风险。

### 18.1 可分享主题应禁止

- `@import` 外部样式表
- `url(http://...)`、`url(https://...)` 和远程字体
- JavaScript、HTML、可执行文件和 DLL
- 绝对文件路径
- 符号链接和 ZIP 路径穿越
- 未声明许可证的第三方字体和图片
- 覆盖官方主题 ID 的非官方包

本地 DIY 可以暂时保留实验性写法，但准备分享主题时必须改成本地资源。

### 18.2 为什么不允许远程 URL

远程 URL 会带来网络不可用、第三方服务下线、隐私请求、加载延迟和主题不可复现问题。主题运行时资源应当随主题包提供，并通过本地资源路由读取。

### 18.3 ZIP 安装安全

任何未来的安装器或手动解压流程都应该检查：

1. 每个路径都是相对路径。
2. 清理后的路径仍然位于临时主题目录内。
3. 不接受符号链接和特殊文件。
4. 文件总数、单文件大小和解压后总大小有上限。
5. 必须存在 `theme.json` 和 `theme.css`。
6. 安装完成后使用临时目录原子替换，失败时保留旧主题。

---

## 19. 当前产品边界

本文暂不规定主题市场、联网清单或上传流程。主题作者当前只需要准备符合本文规范的主题目录或 ZIP；任何发布渠道都应以本指南的文件、资源和安全约束为准。

---

## 20. 本地调试方法

### 21.1 让主题出现在列表

确认主题目录位于可执行文件同级：

```text
FanControl/
  FanControl.exe
  themes/
    cute-blue/
      theme.json
      theme.css
```

启动后打开设置页的主题选择器。如果主题没有出现，先检查 JSON 是否能被解析，再检查 ID 是否符合规则。

### 21.2 CSS 不生效

按以下顺序检查：

1. 文件名是否严格为 `theme.css`。
2. `theme.json` 的 `id` 是否与文件夹相同。
3. CSS 是否使用相同的 `html[data-theme="id"]` 前缀。
4. 自定义主题是否真正被选中，而不是仍然是 `system`、`light` 或 `dark`。
5. 浏览器开发者工具中是否存在 `html[data-theme="id"]`。
6. 是否被模板文件末尾的最终覆盖规则覆盖。

### 21.3 图片或字体不显示

检查 CSS 的相对路径和真实文件大小写：

```text
theme.css                         # 引用 url("hero.webp")
hero.webp                         # 必须位于同一个主题目录
assets/fonts/ui.woff2             # 路径必须与 CSS 完全一致
```

不要把 `/theme-assets/...` 手动写成其他主题 ID。应用会把同主题的相对路径自动改写为资源路由。

### 21.4 圆角露底或出现方角

先确认装饰元素和外壳的层级，再检查真实绘制卡片。曲线页常见结构是外层 `data-theme-card="curve-editor"` 加内部圆角图表卡片，因此只修改外层不一定能解决问题。

### 21.5 页面变卡

优先减少：

- 大图尺寸和数量
- `backdrop-filter: blur(...)`
- 持续运行的 `@keyframes`
- 每个卡片都使用的伪元素
- 多层 box-shadow

不要先重写整个页面或把图表改成另一种渲染技术。主题 CSS 的目标是视觉覆盖，不是重构应用布局。

---

## 21. 代码注释逐类说明

模板中的注释不是装饰，它们对应了维护时最容易出问题的区域。新主题应继续保留这种分区注释。

### 22.1 Basic 模板注释分组

| 注释主题 | 代码意思 |
| --- | --- |
| 文件头和主题 ID | 提醒复制主题时同步修改 JSON ID 和 CSS 前缀 |
| 滚动条 | 说明滚动条不应使用过重颜色抢走视觉 |
| 图表颜色 | 说明 CPU、GPU、风扇和功耗曲线需要区分 |
| 次级背景和弹窗 | 说明下拉、toast 和辅助按钮使用的层级 |
| 左侧导航栏 | 说明侧栏颜色和选中状态 |
| 内容面板和卡片 | 说明基础版尽量不依赖重 blur |
| 层级变量 | 提醒不要让装饰盖住弹窗和控件 |
| 图标描边 | 说明描边粗细只能调视觉，不能改变图标盒子尺寸 |
| 页面背景 | 说明页面属性在旧版本中可能被忽略 |
| 控件 hover/focus | 说明 hover 不应造成布局移动，focus 必须可见 |
| 开关兼容写法 | 说明 Basic 使用 `button[role="switch"]` 兼容旧版本 |
| 左侧标记 | 说明文字徽标和图片徽标的二选一关系 |
| 仪表和半环 | 说明只改变颜色和光效，不重建 SVG 或图表结构 |

### 22.2 Advanced 模板注释分组

| 注释主题 | 代码意思 |
| --- | --- |
| 外壳圆角填充 | 防止透明窗口在四角露出原生背景 |
| 高级外观层 | 集中放贴纸、幕布和玻璃效果，方便整段删除 |
| 字体与卡片微调 | 控制 UI 字体、透明度、hover 稳定性 |
| 字体子集 | 说明 WOFF2 子集的目的和完整字体的体积风险 |
| 透明卡片层 | 让背景透出但保留文字可读性 |
| 幕布可见性 | 说明觉得太花时可以删除整段 |
| 左侧按钮点击反馈 | 说明轻微缩放和光圈不应改变布局 |
| 曲线编辑器修复 | 说明外层是布局锚点，内部才是真实圆角卡片 |

### 22.3 注释的写法

推荐写“原因 + 约束”，不推荐写无信息量的句子：

```css
/* 好：伪元素只做装饰，不能挡住按钮点击。 */
pointer-events: none;

/* 差：设置 pointer-events。 */
pointer-events: none;
```

复杂区块前保留一段短注释即可。不要在每一行都重复 CSS 属性的字面意思；但对于 z-index、透明度、固定尺寸、资源路径和最终覆盖顺序，应该说明为什么这样写。

---

## 22. 完整验收清单

### 23.1 文件和清单

- [ ] 文件夹名与 `theme.json` 的 `id` 完全一致。
- [ ] `id` 只包含小写字母、数字、短横线或下划线。
- [ ] `name`、`author`、`version`、`description` 已填写。
- [ ] `base` 是 `light` 或 `dark`。
- [ ] `layer` 是 `basic` 或 `advanced`。
- [ ] 没有在 JSON 中写 `//` 注释。
- [ ] 主题包至少包含 `theme.json` 和 `theme.css`。

### 23.2 CSS 和组件

- [ ] 所有主题选择器都从 `html[data-theme="主题ID"]` 开始。
- [ ] 没有无作用域的 `body`、`:root` 或 `.glacier-shell` 覆盖。
- [ ] 页面背景没有遮挡正文。
- [ ] 卡片透明度足以读取标题、单位和数值。
- [ ] 下拉框、开关、滑块、按钮和 focus ring 可见。
- [ ] 设置行 hover 不改变布局尺寸。
- [ ] 曲线编辑器、历史图和功耗图没有露出方角。
- [ ] 曲线颜色彼此区分。
- [ ] 主题在首页、曲线页、设置页和关于页都测试过。

### 23.3 资源和字体

- [ ] 所有 `url(...)` 都是主题包内的相对路径。
- [ ] 没有绝对路径或远程资源。
- [ ] SVG 不含脚本和外链。
- [ ] 图片经过裁剪和压缩，适合窗口比例。
- [ ] 字体是授权可分发的字体。
- [ ] 字体子集包含主题名和主题专用字符。
- [ ] 字体有系统回退，缺字时不会出现方块。
- [ ] `font-display: swap` 已设置。

### 23.4 性能和可访问性

- [ ] 没有在每张卡片上运行持续动画。
- [ ] `backdrop-filter` 使用数量有限。
- [ ] `prefers-reduced-motion` 下动画会关闭或缩短。
- [ ] 信息不依赖颜色单独表达。
- [ ] 键盘 focus 状态清楚。
- [ ] 窄窗口下装饰会隐藏或自动收敛。

### 23.5 分享前检查

- [ ] 预览图是清晰的 WebP，比例稳定。
- [ ] 包含许可证和作者署名。
- [ ] ZIP 内没有开发缓存、工程文件或可执行文件。
- [ ] 如果发布渠道需要校验，已经生成并记录 SHA-256。
- [ ] `minAppVersion` 与使用的高级钩子匹配。
- [ ] 更新说明写明视觉变化和兼容性变化。
- [ ] 提交前在干净的便携目录中安装测试。

---

## 23. 给 AI 或协作者的标准提示词

```text
请为 FanControl 制作一个自定义主题，只修改 theme.json、theme.css 和主题资源，不修改软件源码。

请先选择模板：
- basic：只使用基础颜色变量、glacier-* 外壳和 button[role="switch"]，尽量兼容 THRM。
- advanced：可以使用 data-theme-page、data-theme-card、data-theme-ui、相对图片、贴纸和字体子集。

必须遵守：
1. 文件夹名、theme.json 的 id 和 html[data-theme="id"] 前缀一致。
2. 所有 CSS 选择器都放在 html[data-theme="id"] 作用域内。
3. JSON 不能使用 // 注释，说明请写在 $comment 或 $help。
4. 资源只能使用主题包内的相对路径，不能使用 C:/、D:/ 或远程 URL。
5. 不要让装饰覆盖按钮、下拉框、开关、进度条、图表和正文。
6. 设置页、曲线页、首页和关于页都要保证文字可读。
7. 字体优先使用系统字体；使用自定义字体时必须使用授权的 WOFF2 子集。
8. 任何 blur、动画、大图和多层阴影都要考虑性能和减少动态效果。
9. 每个复杂 CSS 区块前写一条说明“它解决什么问题、为什么放在这里”。
10. 最后输出文件清单、许可证、已知兼容版本和验收结果。

主题需求：
- 主题名：
- 主题 ID：
- basic 或 advanced：
- 基础明暗：
- 主色和辅助色：
- 首页插画或背景：
- 左侧徽标：文字还是图片：
- 是否使用字体子集：
- 希望强调的页面：
- 不希望出现的元素：
```

---

## 24. 参考文件

- 基础模板：`docs/theme-template/basic/`
- 高级模板：`docs/theme-template/advanced/`
- 主题目录：`themes/`
- 已导出主题：`exported-themes/`
- 主题资源和路径实现：`internal/theme/theme.go`
- 主题 GUI API：`internal/guiapp/theme_api.go`
- 前端主题同步：`frontend/src/app/components/SystemThemeSync.tsx`
- 设置页主题选择：`frontend/src/app/components/settings/SystemSettingsSection.tsx`
- 主题设计指南 PDF 源码：`docs/theme-diy-guide-latex/`

这份指南解释的是当前版本公开的主题接口。新增页面或组件时，应同时更新组件索引、模板示例和验收清单，避免作者只能通过阅读源码猜测钩子含义。
