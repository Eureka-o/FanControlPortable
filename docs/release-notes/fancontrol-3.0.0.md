# FanControl 3.0.0

## 插件中心与官方市场
### Plugin Center & Official Marketplace

- **插件与扩展管理 / Plugin & Extension Management**
  在设置页新增「插件与扩展」，支持集中查看、搜索、按状态筛选并启用或停用内置扩展与官方插件；可查看插件运行日志、重启或卸载插件，也支持导入本地 `.zip` 插件包以及添加本地开发目录。独立运行的插件支持即时启停，部分核心集成插件在变更后会提示一键重启核心服务生效；未安装或停用时不会占用额外后台资源。
  Added **Plugins & Extensions** in Settings to browse, search, filter, and toggle built-in integrations and external plugins, with log viewing, restarts, uninstalls, local `.zip` package imports, and local dev-folder loading. Standalone plugins toggle on the fly while core-integrated plugins prompt to restart Core cleanly; disabled or uninstalled plugins consume no extra background resources.

- **官方插件市场 / Official Plugin Store**
  新增「插件市场」，支持在线浏览、搜索、按需安装与更新官方扩展（也可在菜单中直接打开插件与主题仓库或填写自定义目录源）。
  Added the **Plugin Store** for on-demand browsing, installation, and updates (with quick access to the plugin repository and custom catalog sources).

- **官方扩展：RTSS OSD / Official Plugin: RTSS OSD**
  在 RivaTuner Statistics Server 游戏叠加层中展示散热器风扇转速，支持调节刷新频率，并可在 FanControl 内微调显示坐标或配置定位图层后在 RTSS OverlayEditor 中拖拽位置。
  Displays cooler fan speeds inside the RivaTuner Statistics Server overlay with configurable refresh intervals, plus coordinate tuning inside FanControl or positioning-layer dragging in RTSS OverlayEditor.

- **官方扩展：HP OMEN / Official Plugin: HP OMEN**
  面向惠普暗影精灵笔记本的侧栏扩展页，提供双风扇曲线、性能模式切换、CPU/GPU 功耗调节、一键强冷及 OMEN Gaming Hub 共存/仅监测/积极接管模式，并支持退出或休眠时自动恢复系统控制。
  Dedicated sidebar control page for HP OMEN laptops featuring dual fan curves, performance mode switching, CPU/GPU power controls, max-cool toggle, and OMEN Gaming Hub coexistence modes with automatic BIOS restore on exit or sleep.

## 自定义主题与主题市场
### Custom Themes & Theme Store

- **官方自定义主题按需获取 / On-Demand Custom Themes**
  将原有的 9 套官方自定义主题从主程序安装包与便携包中拆分，主程序默认保留「浅色」「深色」「跟随系统」三种基础主题；在「系统设置 → 界面主题」新增「主题管理」与「主题市场」，支持按全部、可更新、未安装、已安装筛选，支持单个或全部一键更新，安装或更新后无需重启即可立即切换，并继续兼容旧版自定义主题。
  Decoupled the 9 official custom themes from the installer and portable packages while keeping Light, Dark, and Follow System built in. Added **Theme Management** and **Theme Store** under Settings → Theme with status filters, single or batch updates, instant switching without restarts, and continued compatibility with legacy custom themes.

## 界面排版与交互细节
### UI Layout & Interaction Polish

- **首页视图与分组设置页 / Homepage Views & Grouped Settings**
  调整首页状态卡片、设备顶栏居中排版及半圆仪表盘弧线比例，移除温度与风扇历史图下方多余的阴影填充，保留侧栏当前图标高光并去掉外圈橙色边框；支持开启温度转速与功耗历史图悬浮信息分图显示；将系统设置按「界面与语言」「监控与快捷键」「启动与连接」分组归类，并在系统设置中提供「后台历史记录」开关与保留时长设置（与首页、曲线页同步）。
  Refined homepage status cards, centered device headers, and semi-gauge proportions while removing extra shading under history charts and removing the orange border around the sidebar dock; added split hover tooltips between telemetry and power history charts, organized System Settings into clear groups, and exposed background history recording and retention controls synced across pages.

- **独显休眠状态提示与按需读取 / Discrete GPU Sleep Notice & On-Demand Probing**
  在自动读取模式下遇到独立显卡休眠时，首页状态卡片明确显示「独显未启用」及未读取原因，避免无意义唤醒独显增加功耗；在「风扇控制 → 温度基准」设置中新增「读取传感器」按钮，可临时唤醒独显读取传感器列表，离开设置页后自动恢复休眠。
  When GPU monitoring is set to Auto and the discrete GPU is sleeping, the homepage now explains why reading was skipped instead of waking the GPU; added a temporary sensor-probing button in Settings → Temperature Baseline to inspect GPU sensors on demand and let the GPU sleep again after leaving the page.

- **菜单弹出与折叠动效改进 / Menus, Collapse Transitions & Startup Notices**
  修复插件页与市场弹窗的二级操作菜单靠近边界时被截断的问题；为可折叠配置面板补充展开与收起过渡动画，消除按钮点击后的焦点框残留与列表滚动到底部时的卡滞；修正刚打开软件时短暂闪现温度桥接异常提示的现象，并补齐官方主题在全界面下的字体覆盖与视觉细节。
  Fixed secondary dropdown menus clipping near dialog edges across Plugins and Marketplace views, added expand/collapse transitions for collapsible panels, removed focus-ring artifacts and scroll-edge stalls, suppressed transient startup bridge alerts, and improved UI glyph coverage across official custom themes.

## 风扇曲线编辑与智能控温
### Fan Curve Editor & Smart Control

- **精细曲线编辑与安全提醒 / Precision Curve Editing & Guidance**
  新增「节点微调」面板（支持方向键微调温度/转速及左右切换节点）、一键「整体微调」、拖动「步进吸附」（按住 `Shift` 切换吸附粒度）以及 `Ctrl+Z` 撤销；曲线图新增目标温区标识，将节点拖入过低转速区间时显示「低转速提示」并支持一键调整至 1000 RPM。
  Added a precision point tuner (arrow-key step tuning and node switching), global curve shift, drag snap-to-step (with `Shift` modifier), `Ctrl+Z` undo, target temperature band markers, and low-RPM guidance with quick adjustment to 1000 RPM.

- **方案管理与自适应学习完善 / Profile Management & Adaptive Learning**
  曲线方案弹窗拆分为「方案列表」与「导入 / 导出」，支持查看当前使用方案、启用、重命名、复制副本及全选或仅导出当前方案；自适应学习新增一键「应用到曲线」，可将学习到的各温区偏移写入基础曲线，并改进小幅温差收敛、负载温升预判及 12 档自定义转速范围校验。
  Reorganized curve profiles into Profile List and Import/Export tabs with activation, renaming, duplication, and selective export; added one-click application of learned offsets back to the base curve, refined small-delta learning convergence and rise prediction, and added range/monotonic checks for 12-gear custom speeds.

## 稳定性、体积与安装卸载
### Stability, Package Size & Setup

- **睡眠唤醒与资源占用改善 / Sleep Recovery & Lower Idle Overhead**
  修复系统从睡眠或休眠恢复后偶发无法继续调节风扇转速的问题；退出软件时确保温度读取辅助进程一并退出；无情景规则、未启用扩展或处于空闲状态时减少后台轮询与冗余内存占用。
  Fixed occasional loss of fan control responsiveness after waking from Windows sleep or hibernation, ensured background temperature helpers exit cleanly with the app, and reduced idle background activity and memory footprint when features or devices are inactive.

- **更小包体与安装卸载体验 / Smaller Packages & Setup Polish**
  拆出自定义主题后显著缩小安装包与便携包体积；安装与卸载过程中缩短关闭旧进程的等待时间并在完成后自动进入完成页，清理桌面快捷方式更彻底，交互式卸载新增是否删除个人配置的确认提示（静默卸载时不弹窗）。
  Noticeably reduced installer and portable bundle sizes after decoupling custom themes, shortened process-close waits during install/uninstall, auto-advanced to completion pages, improved desktop shortcut cleanup, and added a confirmation prompt before removing user configurations on interactive uninstall.

## 兼容性说明
### Compatibility Notes

- **覆盖安装与数据保留 / In-place Upgrade & Data Preservation**
  可直接覆盖安装旧版本；已有设备配置、风扇曲线、自适应学习数据、历史记录、托盘设置、IP 设置与本地自定义主题会继续保留。从含内置主题的旧版升级后，如需获取官方自定义主题更新或集中卸载重装，可在「设置 → 系统设置 → 主题管理 / 主题市场」中操作。
  You can install over previous versions with existing device settings, fan curves, adaptive learning data, history, tray settings, IP settings, and local custom themes preserved. After upgrading from older versions with bundled themes, official themes can be updated or managed anytime under Settings → System → Theme Management / Theme Store.
