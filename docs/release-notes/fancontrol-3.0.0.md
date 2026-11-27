# FanControl 3.0.0

## 插件中心与官方市场
### Plugin Center & Official Marketplace

- **插件与扩展管理 / Plugin & Extension Management**
  在设置页新增「插件与扩展」，支持查看、搜索、按状态筛选并启用或停用内置扩展与官方插件；可查看运行日志、重启或卸载插件，也支持导入本地 `.zip` 插件包或添加本地开发目录。独立运行的插件支持即时启停，部分核心集成插件变更后会提示重启核心服务生效；未安装或停用时不占用后台资源。
  Added **Plugins & Extensions** in Settings to browse, search, filter, and toggle built-in integrations and external plugins, with log viewing, restarts, uninstalls, local `.zip` package imports, and local dev-folder loading. Standalone plugins toggle on the fly while core-integrated plugins prompt for a clean core restart; disabled or uninstalled plugins leave no extra background overhead.

- **官方插件市场与多源下载 / Official Plugin Store & Multi-Source Downloads**
  新增「插件市场」，支持在线浏览、搜索、按需安装与更新官方扩展，也可在菜单中打开插件与主题仓库或填写自定义目录源；应用内检查更新与资源下载支持多镜像测速与自动回退，提升连接成功率。
  Added the **Plugin Store** for on-demand browsing, installation, and updates, with quick access to repositories and custom catalog sources; in-app updates and resource downloads now race multiple mirrors with automatic fallback for better reliability.

- **官方扩展：RTSS OSD / Official Plugin: RTSS OSD**
  在 RivaTuner Statistics Server 游戏叠加层中显示散热器风扇转速，支持调节刷新频率，并可在 FanControl 内微调显示坐标或配置定位图层后在 RTSS OverlayEditor 中拖拽位置。
  Displays cooler fan speeds inside the RivaTuner Statistics Server overlay with configurable refresh intervals, plus coordinate tuning inside FanControl or positioning-layer dragging in RTSS OverlayEditor.

- **官方扩展：HP OMEN / Official Plugin: HP OMEN**
  面向惠普暗影精灵笔记本的侧栏扩展页，提供双风扇曲线、性能模式切换、CPU/GPU 功耗调节、一键强冷及 OMEN Gaming Hub 共存 / 仅监测 / 积极接管模式，并支持退出或休眠时自动恢复系统控制。
  Dedicated sidebar control page for HP OMEN laptops featuring dual fan curves, performance mode switching, CPU/GPU power controls, max-cool toggle, and OMEN Gaming Hub coexistence modes with automatic BIOS restore on exit or sleep.

## 自定义主题与主题市场
### Custom Themes & Theme Store

- **官方自定义主题按需获取 / On-Demand Custom Themes**
  将原有 9 套官方自定义主题从主程序安装包与便携包中拆分，主程序保留「浅色」「深色」「跟随系统」三种基础主题；在「系统设置 → 界面主题」新增「主题管理」与「主题市场」，支持按状态筛选及单个或一键更新，安装或更新后无需重启即可切换，并继续兼容旧版自定义主题。
  Split the 9 official custom themes out of the installer and portable packages while keeping Light, Dark, and Follow System built in. Added **Theme Management** and **Theme Store** under Settings → Theme with status filters, single or batch updates, instant switching without restarts, and continued compatibility with legacy custom themes.

## 界面排版与监控视图
### UI Layout & Monitoring Views

- **仅监控模式首页与历史合并 / Monitor-Only Homepage & Integrated History**
  仅监控模式首页改为 CPU 与 GPU 双栏视图，集中展示温度表盘、实时功耗与近 60 秒功耗变化折线，移除无设备连接时多余的风扇转速与控制卡片，并将「温度与功耗历史」直接合并到首页展示（同时隐藏仅监控模式下无需使用的风扇曲线侧栏入口）。
  Redesigned the Monitor-Only homepage into a two-card CPU/GPU view with live wattage and 60-second power trend charts, removing redundant fan/control tiles when no device is connected, integrating the full Temperature & Power History panel onto the homepage, and hiding the unused Fan Curve tab.

- **首页仪表盘与分组设置优化 / Gauge Alignment & Grouped Settings**
  统一正常模式与仅监控模式的半圆仪表盘高度与弧线比例，将自动模式下「跳过独显读取」提示移至 GPU 表盘上方并保持间距稳定；历史图去掉多余下方阴影并支持转速与功耗分图悬浮提示，侧栏移除选中图标外圈橙色边框；系统设置按「界面与语言」「监控与快捷键」「启动与连接」归类，并同步后台历史记录开关与保留时长设置。
  Aligned semi-gauge proportions across Normal and Monitor-Only views and moved the Auto-mode discrete GPU notice above the GPU dial to keep card spacing steady; removed extra shading under history charts, split hover tooltips between telemetry and power charts, cleaned up the sidebar active-icon border, organized System Settings into three clear groups, and synced background history recording and retention controls across pages.

- **独显休眠可读性与按需探测 / Discrete GPU Sleep Readout & On-Demand Probing**
  自动读取模式下独立显卡休眠时，首页状态卡片直接标示「独显未启用」及未读取原因，功耗继续保留 `0 W` 单位展示（CPU 功耗低于有效阈值时显示 `--` 而非误报 `0 W`），避免无意义唤醒独显；在「风扇控制 → 温度基准」设置中新增「读取传感器」按钮，可临时唤醒独显读取传感器列表，离开设置页后自动恢复休眠。
  When discrete GPU monitoring is set to Auto and the GPU is sleeping, homepage cards show "GPU Inactive" with the skip reason while keeping muted `0 W` units (and displaying `--` instead of `0 W` when CPU power is below valid reporting thresholds) without waking the GPU; added an on-demand **Read Sensors** button in Settings → Temperature Baseline that restores sleep behavior after leaving the page.

- **菜单弹出与界面交互修复 / Menus, Collapse Transitions & Theme Font Polish**
  修复插件页与市场弹窗二级菜单靠近边界时被截断的问题；为可折叠面板增加展开与收起过渡动画，消除按钮焦点框残留与列表触底卡滞；修正刚打开软件时短暂闪现温度桥接异常提示的问题，并补齐官方自定义主题在全界面下的字体覆盖。
  Fixed secondary dropdown menu clipping near dialog edges across Plugins and Marketplace views, added expand/collapse transitions for collapsible panels, removed focus-ring artifacts and scroll-edge stalls, suppressed transient startup bridge alerts, and completed glyph coverage across official custom themes.

## 风扇曲线编辑与智能控温
### Fan Curve Editor & Smart Control

- **精细曲线编辑与低转速提醒 / Precision Curve Editing & Low-RPM Guidance**
  新增「节点微调」面板（支持方向键微调温度/转速及左右切换节点）、一键「整体微调」、拖动「步进吸附」（按住 `Shift` 切换吸附粒度）与 `Ctrl+Z` 撤销；曲线图新增目标温区标识，节点拖入过低转速区间时显示「低转速提示」并支持一键调整至 1000 RPM。
  Added a precision point tuner (arrow-key step tuning and node switching), global curve shift, drag snap-to-step (with `Shift` modifier), `Ctrl+Z` undo, target temperature band markers, and low-RPM guidance with quick adjustment to 1000 RPM.

- **方案管理与自适应学习 / Profile Management & Adaptive Learning**
  曲线方案弹窗拆分为「方案列表」与「导入 / 导出」，支持查看当前使用方案、启用、重命名、复制副本及全选或仅导出当前方案；自适应学习新增一键「应用到曲线」将学习偏移写回基础曲线，改进小幅温差收敛与负载温升预判，并补充 12 档自定义转速范围与单调校验。
  Reorganized curve profiles into Profile List and Import/Export tabs with activation, renaming, duplication, and selective export; added one-click application of learned offsets to the base curve, refined small-delta learning convergence and rise prediction, and added range/monotonic checks for 12-gear custom speeds.

## 稳定性、体积与安装卸载
### Stability, Package Size & Setup

- **睡眠唤醒与后台资源控制 / Sleep Recovery & Lower Idle Overhead**
  修复系统从睡眠或休眠恢复后偶发无法继续调节风扇转速的问题；退出软件时确保温度读取辅助进程一并退出；无情景规则、未启用扩展或处于空闲状态时自动停止无效轮询并减少内存占用。
  Fixed occasional loss of fan control responsiveness after waking from Windows sleep or hibernation, ensured background temperature helpers exit cleanly with the app, and reduced idle background polling and memory usage when features or devices are inactive.

- **更小包体与安装卸载流程 / Smaller Packages & Setup Polish**
  拆分官方自定义主题后减小了安装包与便携包体积；安装与卸载过程中缩短关闭旧进程的等待时间并在完成后自动进入完成页，彻底清理桌面快捷方式，交互式卸载新增是否保留个人配置的确认提示（静默卸载时不弹窗）。
  Reduced installer and portable package sizes by splitting out official custom themes, shortened process-close waits during install/uninstall, auto-advanced to completion pages, improved desktop shortcut cleanup, and added a confirmation prompt before removing user configurations on interactive uninstall.

## 兼容性说明
### Compatibility Notes

- **覆盖安装与数据保留 / In-place Upgrade & Data Preservation**
  可直接覆盖安装旧版本；已有设备配置、风扇曲线、自适应学习数据、历史记录、托盘设置、IP 设置与本地自定义主题会继续保留。从含内置主题的旧版升级后，如需更新或管理官方自定义主题，可在「设置 → 系统设置 → 主题管理 / 主题市场」中操作。
  You can install over previous versions with existing device settings, fan curves, adaptive learning data, history, tray settings, IP settings, and local custom themes preserved. After upgrading from older versions with bundled themes, official themes can be updated or managed anytime under Settings → System → Theme Management / Theme Store.
