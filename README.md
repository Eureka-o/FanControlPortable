# FanControl

FanControl 是面向 Windows 的多设备散热控制工具，用于连接和控制外置散热器、风扇控制器和 DIY 主控。它支持设备扫描、风扇曲线、手动转速、插件扩展、在线主题与插件市场、托盘后台运行以及配置继承。

FanControl is a Windows tool for controlling external coolers, fan controllers, and DIY boards. It provides device discovery, fan curves, manual speed control, plugins, an online plugin/theme store, tray operation, and configuration migration.

当前版本内置 Slim 压风散热器 Pro、飞智（FlyDigi）BS 系列和黑鲨（Black Shark）风神 Pro 的设备档案。WiFi 与虚拟串口/COM 仍可通过兼容模式手动配置；黑鲨风神 Pro 仅支持 USB 有线和 BLE。

The current build includes profiles for Slim 压风散热器 Pro, FlyDigi BS-series devices, and Black Shark Fengshen Pro. WiFi and virtual serial/COM remain available through compatibility mode; Black Shark Fengshen Pro supports USB wired and BLE only.

[下载最新版 / Download latest](https://github.com/Eureka-o/FanControlPortable/releases/latest)

## 适用范围 / Scope

- 系统 / System：Windows 10 或 Windows 11（64 位） / Windows 10 or Windows 11 (64-bit).
- 内置设备 / Built-in devices：Slim 压风散热器 Pro、飞智（FlyDigi）BS1/BS2/BS2PRO/BS3/BS3PRO、黑鲨（Black Shark）风神 Pro。
- 兼容设备 / Compatibility devices：WiFi 与虚拟串口/COM 设备可手动添加地址或扫描发现 / WiFi and virtual serial/COM devices can be added manually or discovered by scanning.
- 插件 / Plugins：内置轻量扩展与官方「插件市场」（提供 RTSS OSD 叠加层转速显示、HP OMEN 笔记本控制等扩展），支持独立启停、本地包导入与按需安装 / Built-in extensions and the Official Plugin Store (including RTSS OSD overlay telemetry and HP OMEN laptop control), supporting toggling, local `.zip` import, and on-demand installation.
- 主题 / Themes：主程序内置「浅色 / 深色 / 跟随系统」三种基础主题；官方自定义主题可通过「主题管理 / 主题市场」按需下载与更新，也支持导入第三方主题（模板与 DIY 制作指南见 [FanControl-Plugins](https://github.com/Eureka-o/FanControl-Plugins)） / Light, Dark, and Follow System themes are built in; official custom themes can be installed and updated on demand via Theme Management / Theme Store, with support for importing third-party themes (see [FanControl-Plugins](https://github.com/Eureka-o/FanControl-Plugins) for templates and DIY guides).

## 已适配设备 / Supported devices

设备切换后，FanControl 会按设备档案加载对应的连接方式、转速单位、曲线和可用功能。

FanControl loads the matching transport, speed unit, curves, and capabilities from the selected device profile.

| 设备 / Device | 连接方式 / Transport | 控制单位 / Unit | 当前支持 / Supported features |
| --- | --- | --- | --- |
| Slim 压风散热器 Pro | WiFi | 百分比 / Percent | 速度读取与下发、曲线控制、IP 查找、动态 IP 兼容、智能启停 / Speed read/write, curve control, IP discovery, dynamic-IP compatibility, smart start/stop |
| 飞智（FlyDigi）BS1 | BLE | RPM | 速度读取与下发、手动挡位、通电自启 / Speed read/write, manual gears, power-on start |
| 飞智（FlyDigi）BS2 / BS2PRO / BS3 / BS3PRO | HID | RPM | 速度读取与下发、手动挡位、挡位灯、灯光、亮度、通电自启、智能启停 / Speed read/write, manual gears, gear indicator, lighting, brightness, power-on start, smart start/stop |
| 黑鲨（Black Shark）风神 Pro | BLE | RPM | 速度读取与下发、灯光、亮度、屏幕图片上传（速度较慢） / Speed read/write, lighting, brightness, screen image upload (slower) |
| 黑鲨（Black Shark）风神 Pro | USB 有线 / USB wired | RPM | 速度读取与下发、灯光、亮度、屏幕图片上传 / Speed read/write, lighting, brightness, screen image upload |

飞智和黑鲨设备会优先通过内置档案自动匹配。点击设置页的「扫描设备」后，软件会扫描原生 HID/BLE/USB 设备；开启兼容模式后，还会扫描 WiFi 和虚拟串口/COM 设备。

FlyDigi and Black Shark devices are matched through built-in profiles first. The device scanner checks native HID/BLE/USB devices; compatibility mode additionally scans WiFi and virtual serial/COM devices.

## 下载文件 / Downloads

- `FanControl-3.0.0-amd64-installer.exe`：安装版，升级时保留已有配置 / Installer; preserves existing configuration during upgrades.
- `FanControl-3.0.0-portable.zip`：便携版，解压后运行 `FanControl.exe` / Portable package; extract and run `FanControl.exe`.

首次启动时如 Windows 弹出权限确认，请选择允许。软件可能需要管理员权限读取硬件温度或安装温度读取组件。

When Windows asks for permission on first launch, allow it. Administrator access may be required for hardware telemetry or temperature bridge installation.

## 第一次使用 / First use

1. 确认设备已经通电 / Make sure the device is powered on.
2. 打开 FanControl，进入设置页的「设备连接」/ Open FanControl and go to **Device Connection** in Settings.
3. 点击「扫描设备」/ Select **Scan Devices**.
4. 在发现列表中选择设备；WiFi 和虚拟串口/COM 设备也可以手动添加地址 / Select a discovered device; WiFi and virtual serial/COM devices can also be added by address.
5. 回到主页查看温度、功耗、当前转速和目标转速 / Return to the home page to view temperature, power, current speed, and target speed.

## 主要功能 / Features

- 实时查看 CPU/GPU 温度、功耗和散热器转速 / View CPU/GPU temperature, power, and cooler speed in real time.
- 独显休眠时默认跳过读取避免唤醒，并支持在温度基准设置中临时唤醒探测传感器 / Skip discrete GPU polling while sleeping to avoid wakeups, with on-demand temporary sensor probing in settings.
- 按曲线自动调节或设定固定转速，提供节点微调、整体偏移、步进吸附、`Ctrl+Z` 撤销、多方案管理与学习偏移写回曲线功能 / Curve-based automatic or manual speed control with precision point tuning, global shift, snap-to-step, `Ctrl+Z` undo, multi-profile management, and applying learned offsets back to base curves.
- 曲线方案、手动挡位和学习结果按设备独立保存 / Keep curves, manual gears, and learning data scoped to each device.
- 统一的原生设备扫描与兼容设备入口 / Use one discovery flow for native and compatibility devices.
- 提供「插件与扩展」与官方「插件市场」，支持 RTSS OSD 转速叠加与 HP OMEN 控制等按需扩展 / Built-in Plugins & Extensions and Official Plugin Store for on-demand extensions such as RTSS OSD telemetry and HP OMEN control.
- 提供「主题管理」与「主题市场」，支持官方自定义主题在线安装、更新、免重启切换与本地包导入 / Theme Management and Theme Store for online custom theme installation, updates, hot-switching, and local imports.
- 最小化到托盘后继续后台运行 / Continue running in the system tray when minimized.
- 安装版和便携版升级时保留配置、历史数据和已有主题 / Preserve configuration, history, and existing themes across installer or portable upgrades.

## 最新版本 / Latest version

当前构建版本：`3.0.0` / Current build: `3.0.0`

- 新增「插件与扩展」及官方「插件市场」，支持启停管理、日志查看、本地 `.zip` 导入，首发提供 **RTSS OSD** 屏幕转速显示与 **HP OMEN** 双风扇/性能功耗控制插件 / Added Plugins & Extensions and the Official Plugin Store with toggles, logs, local `.zip` import, and launch plugins for **RTSS OSD** and **HP OMEN**.
- 将 9 套官方自定义主题从安装包中拆分为按需获取，新增「主题管理」与「主题市场」支持筛选、一键更新与免重启切换，并显著减小安装包与便携包体积 / Decoupled the 9 official custom themes into on-demand downloads via Theme Management and Theme Store with filtering, batch updates, hot-switching, and smaller distribution packages.
- 重构首页状态卡片、仪表盘比例与分组设置页，新增曲线节点微调、整体微调、步进吸附、`Ctrl+Z` 撤销、低转速提示、方案管理升级及一键将学习偏移应用到曲线 / Redesigned homepage status cards, gauges, and grouped settings, and added point tuning, global shift, snap-to-step, `Ctrl+Z` undo, low-RPM guidance, upgraded profile management, and applying learned offsets to curves.
- 新增独显休眠原因提示与临时传感器探测、菜单防截断及折叠动效，修复休眠唤醒无法调速与退出后温度辅助进程残留问题，并精简安装与卸载等待流程 / Added dGPU sleep notices with temporary sensor probing, non-clipping menus, collapse transitions, sleep-resume fan recovery, clean helper shutdown on exit, and streamlined install/uninstall flows.

完整记录请查看 [GitHub Releases](https://github.com/Eureka-o/FanControlPortable/releases) 或 [docs/release-notes/](docs/release-notes/)。

See [GitHub Releases](https://github.com/Eureka-o/FanControlPortable/releases) or [docs/release-notes/](docs/release-notes/) for the complete changelog.

## 升级与配置继承 / Upgrade and migration

- 升级会保留 WiFi IP、风扇曲线、启用方案、学习数据、设备档案、主题、托盘设置和开机启动设置 / Upgrades preserve WiFi IPs, fan curves, active profiles, learning data, device profiles, themes, tray settings, and autostart settings.
- 官方自定义主题现改为按需下载；从旧版本升级后现有本地主题仍可继续使用，后续可在「设置 → 系统设置 → 主题管理 / 主题市场」安装或更新 / Official custom themes are now downloaded on demand; existing local themes remain usable after upgrading and can be installed or updated under Settings → System → Theme Management / Theme Store.
- 便携版更新前请退出 FanControl，再解压新版覆盖旧文件夹；不要删除 `config` 目录 / Before updating a portable build, exit FanControl and extract the new package over the old folder; keep the `config` directory.
- 关闭窗口不等于退出软件；需要完全退出时，请使用托盘菜单中的「退出」/ Closing the window leaves FanControl in the tray; use the tray **Exit** command to quit completely.

## 常见问题 / FAQ

### 设备连接不上 / Device cannot connect

- 确认设备已通电，并检查连接方式是否正确 / Make sure the device is powered on and the transport is correct.
- 黑鲨风神 Pro 使用 USB 有线或 BLE；两种模式都支持图传，但 BLE 速度较慢，建议优先使用 USB 有线连接 / Black Shark Fengshen Pro uses USB wired or BLE; both support image transfer, but BLE is slower and USB wired mode is recommended.
- WiFi 兼容设备需确认电脑和设备处于同一网络 / WiFi compatibility devices require the computer and device to be on the same network.
- 如果修改过 WiFi 设备网络设置，请重新填写 IP 地址 / Re-enter the IP address after changing a WiFi device's network settings.

### 温度为空或一直是 0 / Temperature is empty or zero

- 以管理员身份重新启动 FanControl / Restart FanControl as administrator.
- 允许安装器安装或更新温度读取组件 / Allow the installer to install or update the temperature bridge.
- 临时关闭其他硬件监控软件后再试 / Temporarily close other hardware monitoring software and retry.

## 反馈 / Feedback

请在 GitHub 提交 issue，或发送邮件至 `1989005183@qq.com`；也可以加入 QQ 交流群 `928338191` 反馈设备适配和使用问题。

Open an issue on GitHub, email `1989005183@qq.com`, or join QQ group `928338191` for device compatibility and usage feedback.

## 支持项目 / Support

如果 FanControl 对你有帮助，欢迎自愿赞助支持后续开发。

If FanControl is useful to you, voluntary sponsorship is appreciated.

| 微信 / WeChat | 支付宝 / Alipay |
| --- | --- |
| <img src="docs/assets/sponsor-wechat.jpg" width="220" alt="微信赞助码 / WeChat sponsorship"> | <img src="docs/assets/sponsor-alipay.jpg" width="220" alt="支付宝赞助码 / Alipay sponsorship"> |

## 致谢 / Acknowledgements

FanControl 基于 [TIANLI0/THRM](https://github.com/TIANLI0/THRM) 的开源工作进行改造和适配。感谢原作者提供的桌面端框架、托盘后台机制、温度读取思路、风扇曲线交互和安装器基础。

FanControl is adapted from the open-source work of [TIANLI0/THRM](https://github.com/TIANLI0/THRM). Thanks to the original authors for the desktop framework, tray architecture, telemetry approach, fan-curve interaction, and installer foundation.

## 许可 / License

本项目使用 MIT License，详见 [LICENSE](LICENSE)。 / This project is licensed under the MIT License; see [LICENSE](LICENSE).

## 免责声明 / Disclaimer

FanControl 是第三方开源软件。请根据设备状态合理设置风扇速度，因使用本软件产生的风险由用户自行承担。

FanControl is third-party open-source software. Set fan speeds responsibly according to device conditions; use of the software is at your own risk.
