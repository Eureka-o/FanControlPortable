# FanControl

FanControl 是面向 Windows 的多设备散热控制工具，用于连接和控制外置散热器、风扇控制器和 DIY 主控。它支持 WiFi、HID、BLE 与虚拟串口/COM 等连接方式，可以读取 CPU/GPU 温度与功耗，按风扇曲线或手动挡位调节转速，并提供设备扫描、设备档案、托盘后台运行、主题系统和配置继承等能力。

FanControl is a Windows fan-control tool for external coolers, fan controllers and DIY controller boards. It supports WiFi, HID, BLE and virtual serial/COM transports, with device profiles, fan curves, manual gears, tray mode, themes and configuration migration.

当前版本已内置 **Slim压风散热器Pro** 与多款飞智（FlyDigi）BS 系列设备档案；WiFi 与虚拟串口/COM 设备也可以通过兼容模式手动添加或扫描发现。

The current version includes built-in profiles for **Slim压风散热器Pro** and multiple FlyDigi BS-series devices. WiFi and virtual serial/COM devices can also be added manually or discovered through compatibility mode.

[下载最新版](https://github.com/Eureka-o/FanControlPortable/releases/latest)

## 适用范围

- 系统 / System：Windows 10 / Windows 11 64 位。
- 内置设备 / Built-in devices：Slim压风散热器Pro、飞智（FlyDigi）BS1、BS2、BS2PRO、BS3、BS3PRO。
- 连接方式 / Transports：WiFi、HID、BLE、虚拟串口/COM；飞智 BLE/HID 设备走原生自动识别通道。
- 兼容模式 / Compatibility mode：面向 DIY 主控、WiFi 设备和虚拟串口/COM 设备，支持手动添加地址、动态 IP 兼容和扫描发现。
- 主题系统 / Theme system：内置基础主题和高级主题，也支持导入/编辑第三方主题资源。

## 已适配设备

FanControl 现在以内置设备档案的方式支持多种设备。不同设备会使用不同的连接方式和控制能力，切换设备后软件会按设备档案加载对应的控制单位、曲线和可用功能。

| 设备 | 连接方式 | 控制单位 | 当前支持 |
| --- | --- | --- | --- |
| Slim压风散热器Pro | WiFi | 百分比 | 速度读取、速度下发、曲线控制、查找 IP、动态 IP 兼容、新固件智能启停 |
| 飞智（FlyDigi）BS1 | BLE | RPM | 速度读取、RPM 下发、手动挡位、通电自启 |
| 飞智（FlyDigi）BS2 | HID | RPM | 速度读取、RPM 下发、手动挡位、挡位灯、灯光、亮度、通电自启、智能启停 |
| 飞智（FlyDigi）BS2PRO | HID | RPM | 速度读取、RPM 下发、手动挡位、挡位灯、灯光、亮度、通电自启、智能启停 |
| 飞智（FlyDigi）BS3 | HID | RPM | 速度读取、RPM 下发、手动挡位、挡位灯、灯光、亮度、通电自启、智能启停 |
| 飞智（FlyDigi）BS3PRO | HID | RPM | 速度读取、RPM 下发、手动挡位、挡位灯、灯光、亮度、通电自启、智能启停 |

飞智设备会优先按内置档案自动匹配。设置页点击「扫描设备」后会按内置档案扫描 HID/BLE 设备；如果开启兼容模式，也会同时扫描 WiFi 和虚拟串口/COM 兼容设备。BLE/HID 不需要在高级设备页选择或启用；WiFi 和虚拟串口/COM 仍保留手动添加和兼容扫描入口。

部分飞智功能仍需要更多真实设备反馈，尤其是不同批次设备的灯光、智能启停和状态回读表现。如果设备能连接但转速或功能状态不正常，建议导出诊断日志并反馈设备型号、连接方式和复现步骤。

## 下载哪个文件

- `FanControl-2.5.2-preview.3-amd64-installer.exe`：2.5.2 第三版预览安装包，升级时会保留已有配置。
- `FanControl-2.5.2-preview.3-portable.zip`：2.5.2 第三版预览便携包，解压到固定文件夹后运行 `FanControl.exe`。

首次启动时如果 Windows 弹出权限确认，请选择允许。软件需要管理员权限读取硬件温度，并可能安装或调用温度读取所需的辅助组件。

## 第一次使用

1. 确认电脑和散热器已经通电；如果使用 WiFi 设备，请确认电脑和散热器处在同一个网络环境中。
2. 打开 FanControl，进入设置页的「设备连接」区域。
3. 点击「扫描设备」。软件会扫描内置 HID/BLE 设备；开启兼容模式后，也会扫描 WiFi 和虚拟串口/COM 设备。
4. 在「已发现设备」中选择要连接的设备。WiFi 设备也可以使用「手动添加地址」，常见默认地址为 `192.168.137.2`，也支持 `127.0.0.1:18080` 这种带端口的写法。
5. 连接成功后，回到主页查看温度、当前风速、目标风速和控制模式。

连接成功后，可以使用自动曲线控制，也可以切换到手动速度进行固定百分比控制。

## 主要功能

- 实时查看 CPU/GPU 温度、功耗和散热器风速 / Real-time CPU/GPU temperature, power and fan status.
- 自动模式下按风扇曲线调节速度，也可以手动设定固定速度 / Curve-based auto control with manual fixed-speed control.
- 曲线方案和学习结果按设备独立保存，切换设备后会自动加载对应曲线 / Device-scoped curves and learning data.
- 百分比控制和学习机制支持 `0.1%` 内部精度，并在发包时按设备能力取整 / Internal `0.1%` precision for percent control and learning.
- 统一的设备扫描与连接入口，支持 HID/BLE、WiFi 和虚拟串口/COM 按兼容模式一起发现和连接 / Unified device discovery for HID/BLE, WiFi and virtual serial/COM devices.
- 支持 WiFi 查找 IP、动态 IP 兼容、温升预判 Beta 和 WiFi 智能启停 Beta / WiFi IP discovery, dynamic IP compatibility, temperature-rise prediction Beta and WiFi smart start/stop Beta.
- 最小化到系统托盘后后台运行，托盘中可查看温度、功耗和风扇状态 / Background tray mode with temperature, power and fan status.
- 安装版和便携版都会尽量保留已有配置 / Installer and portable builds both preserve existing configuration where possible.

## 最新版本

当前预览版本：`2.5.2-preview.3`

- 配置保存失败时保留原配置，并向界面返回明确失败结果。
- 统一正式版、Preview 和 nightly 的版本比较，旧预览版可以正确识别后续预览更新。
- 下载阶段和重试次数以后端状态为准，暂停、继续、取消和重试时的显示更一致。
- 改进睡眠/休眠后的设备重连，并在温度遥测连续失效时临时使用安全转速。
- 历史页将功耗与温度/风扇拆分为对齐的双图，任一图表悬浮时显示全部已启用数据。
- 设置分区切换时保留滚动位置，下载浮窗折叠后可以重新展开。

完整更新记录请查看 [GitHub Releases](https://github.com/Eureka-o/FanControlPortable/releases) 或 `docs/release-notes/`。

## 升级与配置继承

- 从 2.0 / 2.1 / 2.2 升级到新版时，会继续保留 WiFi IP、风扇曲线、启用曲线方案、学习数据、用户设备、设备档案、主题、托盘设置和开机启动设置。
- 如果旧版本曾把主题留在用户目录，新版会尽量整理回软件目录；同名主题以安装包内置版本为准。
- 如果旧版本把配置放在安装目录根部的 `config.json`，新版会在启动时迁移到当前的 `config/config.json`。
- 便携版更新时，请先从托盘退出 FanControl，再解压新版覆盖旧文件夹；不要删除原来的 `config` 目录。
- 仓库和更新地址仍然是 `Eureka-o/FanControlPortable`，用户可见软件名保持为 `FanControl`。

## 使用提示

- 关闭窗口不等于退出软件。FanControl 会继续留在系统托盘中运行；需要完全退出时，请右键托盘图标并选择退出。
- 如果同时使用其他硬件监控或风扇控制软件，遇到温度读取异常时可以先临时关闭它们再重试。
- 高级设备页适合已经了解主控协议的用户。原始调试命令可能影响设备状态，请只在确认命令含义后发送。
- WiFi 和虚拟串口/COM 的通用档案能力已经具备基础运行时，不同 DIY 设备仍需要按主控协议填写正确字段；BLE/HID 设备请使用设置页的「扫描设备」自动遍历。

## 常见问题

### 设备连接不上

- 确认散热器已通电，并且电脑和散热器处于同一网络。
- 检查设置页中的设备 IP 地址是否正确。
- 如果修改过散热器网络设置，请重新填写新的 IP 地址。
- 可以在浏览器中打开 `http://设备地址/api/data` 做简单确认；能看到返回内容通常说明网络连通。

### 温度为空或一直是 0

- 以管理员身份重新启动 FanControl。
- 安装器提示安装或更新温度读取组件时，请允许。
- 临时关闭其他硬件监控软件后再试一次。
- 如果便携版缺少 `bridge` 目录，请重新下载并完整解压便携包。

### 风速没有按预期变化

- 确认设备已经连接成功。
- 检查当前是自动模式还是手动模式。
- 手动模式会固定在你设定的百分比速度；自动模式会按风扇曲线计算目标速度。
- 速度下发后设备状态可能需要几秒钟刷新。

## 反馈

如果遇到问题，可以在 GitHub 提交 issue，发送邮件到 `1989005183@qq.com`，也可以加入 QQ 交流群 `928338191` 反馈设备适配和使用问题。

## 支持项目

如果 FanControl 对你有帮助，欢迎自愿赞助支持后续开发。

| 微信 | 支付宝 |
| --- | --- |
| <img src="docs/assets/sponsor-wechat.jpg" width="220" alt="微信赞助码"> | <img src="docs/assets/sponsor-alipay.jpg" width="220" alt="支付宝赞助码"> |

## 致谢

FanControl 基于 [TIANLI0/THRM](https://github.com/TIANLI0/THRM) 的开源工作进行改造和适配。感谢原作者提供的桌面端框架、托盘后台机制、温度读取思路、风扇曲线交互和安装器基础。

## 许可

本项目使用 MIT License，详见 [LICENSE](LICENSE)。

## 免责声明

FanControl 是第三方开源软件。请根据设备状态合理设置风扇速度，因使用本软件产生的风险由用户自行承担。
