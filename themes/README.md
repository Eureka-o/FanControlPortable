# FanControl 自定义主题

这个文件夹用于存放 **自定义界面主题**。应用启动时会扫描这里的每个子文件夹，
把它们识别成「系统设置 → 界面主题」里可选的主题。

> 内置的「浅色 / 深色 / 跟随系统」三种基础主题不在这里，它们由程序自带；
> 官方自定义主题已独立托管在 [FanControl-Plugins](https://github.com/Eureka-o/FanControl-Plugins) 仓库并通过软件内「系统设置 → 界面主题 → 主题管理 → 主题下载」按需获取，也可自由删除。

## 目录在哪

- 安装目录下的 `themes\` 文件夹（一般就是本文件夹）。
- 旧版本用户目录中的主题只作为兼容迁移来源读取；新主题统一放在安装目录的 `themes\`。

可以在应用的「界面主题 → 主题管理」中点击「本地目录」，直接定位到这里。

## 一个主题 = 一个文件夹

每个主题是一个子文件夹，里面至少包含 `theme.json` 与 `theme.css`：

```
themes/
└─ mytheme/             ← 文件夹名建议与 theme.json 里的 id 一致
   ├─ theme.json        ← 主题清单（名字、id、基于浅色还是深色）
   └─ theme.css         ← 主题样式（改这里就能改外观）
```

### theme.json 字段

| 字段          | 必填 | 说明 |
|---------------|------|------|
| `id`          | 是   | 唯一标识，只能用小写字母/数字/`-`/`_`。会作为 `<html data-theme="id">` 注入页面。 |
| `name`        | 是   | 下拉菜单里显示的名字。 |
| `base`        | 否   | `light` 或 `dark`，本主题基于哪种底色，默认 `light`。 |
| `layer`       | 否   | `basic` 或 `advanced`，默认 `basic`。 |
| `contract`    | 是   | 新主题填写 `fancontrol/v1`；缺失时按旧主题兼容方式加载。 |
| `author`      | 否   | 作者。 |
| `version`     | 否   | 版本号。 |
| `description` | 否   | 简介。 |

### theme.css 写法（重要）

所有选择器都要以 `html[data-theme="你的id"]` 开头，样式才只在该主题被选中时生效：

```css
/* 把主色改成紫色 */
html[data-theme="mytheme"] {
  --primary: #7c5cff;
  --ring: #7c5cff;
  --chart-primary: #7c5cff;
}
```

> 完整的主题制作指南、Basic / Advanced 主题模板以及字体子集生成工具均已迁至官方主题/插件仓库 `FanControl-Plugins`（`docs/` 与 `tools/` 目录）。

