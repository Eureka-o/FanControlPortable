# FanControl 2.9.1

## 性能与后台占用
### Performance & Background Usage

- **后台轮询与历史写入 / Background Polling & History Saves**
  最小化到托盘或仅监控模式下，减少非当前设备和空闲状态下的轮询与托盘刷新频率；温度历史改用异步定期写入，降低主循环阻塞与后台 CPU 唤醒。
  Reduced redundant polling and tray refreshes while minimized or in monitor-only mode, and switched periodic temperature-history writes to async flushes.

- **独显状态切换 / Discrete GPU State Transitions**
  独立显卡在活跃与休眠间切换时不再重建整套传感器监控，持续深度休眠后延长探测间隔，减少不必要的独显唤醒。
  Stopped reinitializing the full sensor monitor during dGPU active/sleep transitions and lengthened probe intervals after extended deep sleep.

- **界面加载与刷新 / UI Loading & Updates**
  控制设置、高级设备、关于页以及图表组件和非当前语言包改为按需加载，缩小每秒温控刷新引起的界面重绘范围。
  Added on-demand loading for secondary pages, charts, and inactive locales, and narrowed per-second telemetry updates to avoid full-shell repaints.

## 设备控制与曲线
### Device Control & Fan Curves

- **关机退出飞智控制 / FlyDigi Exit Behavior on Shutdown**
  Windows 关机或注销时主动退出飞智 HID 实时转速模式，开启智能启停的设备交回固件接管停转，并在退出前保存未写入的温度历史。
  Exits FlyDigi HID real-time RPM mode on Windows shutdown or logoff so smart start-stop returns to firmware control, and flushes pending temperature history before exit.

- **设备重连与状态同步 / Reconnect & Status Sync**
  调整 WiFi 与蓝牙自动重连优先级及连续写入失败后的重连恢复，合并高频设备状态事件，减少重复下发指令。
  Refined WiFi/BLE reconnect priority, recovery after repeated write failures, and status event coalescing to cut redundant device writes.

- **曲线范围与提示 / Curve Range & Notices**
  为通用风扇曲线补全低温段左端默认延展范围，并在历史保留时长保存失败时显示错误提示。
  Extended default low-temperature padding on generic fan curves and added an error notice when saving history retention fails.

## 主题与安装包体积
### Themes & Package Size

- **主题字体子集补齐 / Theme Font Subsets**
  按界面文案、设置名称、主题元数据及设备状态用字重建「初音未来」「小八 Plus」「蜡笔小新」三套主题的字体子集，补齐此前部分汉字回退为系统字体的问题；同时调整 Classic 与 Doro 主题背景动画的绘制方式以降低显卡开销。
  Rebuilt font subsets for the Miku, Xiaoba Plus, and Crayon Shin-chan themes to cover UI copy, settings labels, theme names, and device strings, and switched Classic and Doro background animations to lighter composited transforms.

- **安装包与便携包精简 / Smaller Installer & Portable Package**
  压缩主题图片、字体子集与屏保预设图并移除未引用的冗余文件，同时保留全部 9 套内置主题；相比 2.9.0，安装包和便携包体积均减小约 4 MB。
  Compressed theme images, font subsets, and screen presets while removing unused files and keeping all 9 built-in themes, reducing both the installer and portable package by roughly 4 MB from 2.9.0.

## 兼容性说明
### Compatibility Notes

- **覆盖安装 / In-place Upgrade**
  可直接覆盖安装旧版本；设备配置、风扇曲线、学习数据、历史记录、主题、托盘设置和 IP 设置会继续保留。
  You can install over previous versions; device settings, fan curves, learning data, history, themes, tray settings, and IP settings are preserved.
