# FanControl 2.1.0 Release Notes

FanControl 2.1.0 是正式发行版。用户可见软件名保持为 **FanControl**，仓库、更新检查和发布地址继续使用 `Eureka-o/FanControlPortable`。

## 下载

- `FanControl-2.1.0-amd64-installer.exe`：推荐安装版，适合大多数用户。
- `FanControl-2.1.0-portable.zip`：免安装便携版，解压后运行 `FanControl.exe`。

SHA256:

- `FanControl-2.1.0-amd64-installer.exe`
  - `C7E31F390E9EE0A02EB529CF09BCF5C815AEB0D5B797CD1D27F5AA0C2A08717C`
- `FanControl-2.1.0-portable.zip`
  - `64E3F00617916DCD78D6C29C5445EAFD998C595135229D5F8169F8E2EDA5C4C4`

## 重点变化

- 新增高级“设备”页面，用于已支持设备、用户录入设备和 DIY 主控档案管理。
- 设备库和模板库分离：模板描述协议/控制方式，设备是实际启用和控制的对象。
- 每种连接方式最多启用一个设备，可同时记住 WIFI、BLE、虚拟串口/COM、legacy HID/RPM 的启用设备。
- 新增 `.fcdp` 设备文件导入/导出：支持文件选择、拖入导入、并集合并和原生保存路径导出。
- 普通设置页只保留日常连接字段：WIFI 填 IP，虚拟串口/COM 填端口和波特率；高级协议、发包模板、解析规则保留在设备页。
- 默认 WiFi 设备名称在主页、设备库和设置刷新里保持一致。
- 首页在设备未连接时会显示对应启用设备的等待/断开状态，不再显示空的风速仪表。
- 软件图标已更新，并同步到主程序、Core、托盘资源、安装器、favicon 和前端品牌图。

## 控制与设备档案

- 百分比控制和学习机制使用 FanControl 自有路径，内部精度为 `0.1%`，发包时按设备能力取整。
- legacy RPM/HID 路径保留独立的 RPM 单位和参考式学习逻辑；默认构建中 legacy HID/RPM 会明确显示为未启用的隔离路径。
- WiFi 自定义档案支持自定义状态接口、调速接口、HTTP 方法、命令模板、响应解析、发送限速、重试和回退策略。
- 命令模板支持 `json`、`ascii`、`raw`、`hex` 编码；`ascii/raw/hex` 支持 `none`、`sum8`、`xor8`、`crc16` 校验。
- 虚拟串口/COM 档案支持端口、波特率、数据位、停止位、校验位、帧分隔符、命令模板和响应解析。
- BLE 档案支持手动扫描、广播信息匹配、GATT 服务/特征探测建议，以及连接、读取状态、设置速度的后端基础。
- 新增不保存配置的设备构建测试控件：连接、读取状态、设置速度。
- 调试区域保留有界日志，原始命令发送前会再次确认，成功的调试结果可作为设备档案草稿起点。

## 升级兼容

- 从 FanControl 2.0 升级到 2.1.0 时，会保留 WiFi IP、风扇曲线、启用曲线方案、学习数据、用户设备和设备档案。
- 旧版安装目录根部的 `config.json` 会迁移到当前 `config/config.json`。
- 安装器升级会备份并恢复当前配置和旧版配置位置，减少升级时丢失配置的风险。
- 保留旧 FanControlPortable 配置、任务、历史数据和资产迁移兼容。
- 不影响原作者 THRM 版本；FanControl 的安装、任务、进程、IPC 和配置路径继续保持隔离。

## 已验证

- `go test ./...`
- `npx tsc --noEmit`
- `npm run build`
- `build.bat` 正式构建 `FanControl.exe`、`FanControl Core.exe`、TempBridge 和 NSIS 安装器
- locale JSON key parity for `en-US` / `zh-CN` / `ja-JP`
- `git diff --check`
- portable zip 内容检查
- `FanControl.exe`、`FanControl Core.exe`、安装器文件版本检查
- 主程序、Core、安装器关联图标提取检查

## 注意

- BLE 和虚拟串口/COM 的通用档案能力已经具备基础运行时，但不同 DIY 设备仍需要按主控协议填写正确字段；真实硬件兼容性需要设备用户继续反馈。
- 高级设备页适合已经了解主控协议的用户。原始调试命令可能影响设备状态，请只在确认命令含义后发送。
