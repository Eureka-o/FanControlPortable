# FanControlPortable

FanControlPortable 2.0 是面向 **Slim压风散热器Pro** 的 Windows 散热控制工具。

软件会读取本机 CPU/GPU 温度，记录近期温度和风扇历史，按百分比风扇曲线进行控制，并通过 WiFi 向设备发送原生 `0-100%` 转速指令。

> 普通用户请直接前往 [Releases](https://github.com/Eureka-o/FanControlPortable/releases/latest) 下载 Windows 安装包或便携包，不需要安装 Go、Node.js、Wails、NSIS 或 .NET SDK。

## 普通用户先看

### 下载与运行

1. 打开 [Releases](https://github.com/Eureka-o/FanControlPortable/releases/latest)。
2. 下载 `FanControlPortable-2.0.0-amd64-installer.exe` 使用安装版，或下载 `FanControlPortable-2.0.0-portable.zip` 使用便携版。
3. 启动 `FanControlPortable.exe`。
4. 在设置页选择 WiFi 连接方式并填写设备地址，常见默认地址为 `192.168.137.2`。
5. 回到主页确认温度、风扇速度、连接状态和控制模式正常显示。

### 当前设备协议

- 状态读取：`GET http://<ip>/api/data`
- 转速控制：`POST http://<ip>/api/speed`
- 请求体：`{"speed": 0-100}`

风扇速度使用设备协议中的原生百分比，不把百分比和 RPM 做简单线性映射。

### 2.0 做了什么

- 将目标设备改为 **Slim压风散热器Pro**。
- 将 WiFi 控制逻辑改为当前设备使用的 HTTP 协议，通过 `/api/data` 读取状态，通过 `/api/speed` 发送 `0-100%` 转速。
- 在界面中预留蓝牙/BLE 连接模式，方便后续接入 BLE 协议。
- 重做运行身份：进程名、命名管道、互斥锁、配置目录、托盘身份、计划任务名和更新仓库都已换成 FanControlPortable 自己的值，可与原软件同时运行。
- 更新关于页、仓库链接、反馈邮箱、图标、品牌资源、托盘菜单和安装器文案。
- 保留并适配了实用的桌面端能力：后台核心服务、系统托盘、温度桥接、风扇曲线编辑、手动档位、学习机制接口、历史曲线、安装包和便携包。
- 新增 `tools/mock-device/` 本地模拟设备，用于没有硬件时测试连接、读包和发包。

### 配置与日志

- 便携版会优先读取可执行文件旁边的 `settings.json`。
- 安装版使用 FanControlPortable 独立的数据目录。
- 安装器升级时会保留已有 FanControlPortable 配置。
- 日志写入软件日志目录，不会提交到仓库。

### 常见问题

#### 设备无法连接

1. 确认电脑和散热器处在预期网络中。
2. 检查设置页 IP 地址；支持 `127.0.0.1:18080` 这种带端口写法。
3. 用浏览器或 `curl` 测试 `http://<ip>/api/data`。
4. 如需模拟设备，运行 `tools\mock-device\start-mock-device.ps1`，然后连接 `127.0.0.1:18080`。

#### 温度为空或显示 0

1. 尝试以管理员身份重启 FanControlPortable。
2. 确认 `bridge` 目录中的温度桥接文件完整。
3. 安装器提示时安装或更新 PawnIO。
4. 如果同时运行了其他硬件监控工具，先临时关闭后重试。

#### 关闭窗口后仍在后台

FanControlPortable 支持最小化到托盘运行。如需完全退出，请使用托盘菜单中的退出命令。

## 来源与致谢

本项目基于 [TIANLI0/THRM](https://github.com/TIANLI0/THRM) 的开源工作进行设备适配和改造。感谢原作者提供的 Wails 桌面端架构、托盘/后台机制、温度桥接思路、风扇曲线交互模型和安装器基础。

FanControlPortable 2.0 已针对新的目标设备、通信协议、运行身份、打包命名、更新仓库和用户文案做了独立化处理，避免与原软件互相冲突。

<details>
<summary>开发与构建说明</summary>

## 技术栈

- Go 1.26+
- Wails v2
- Next.js 16
- TypeScript
- Tailwind CSS 4
- C#/.NET 温度桥接
- NSIS 安装器

## 项目结构

```text
cmd/core/                 后台核心服务入口
internal/                 设备、配置、IPC、温度、托盘、更新和应用模块
bridge/TempBridge/        C# 温度桥接程序
frontend/                 Wails 前端
frontend/src/app/         主界面、状态页、风扇曲线页、设置页和关于页
frontend/src/components/  通用界面组件
build/windows/installer/  NSIS 安装器脚本
scripts/                  资源生成和辅助脚本
tools/mock-device/        WiFi 发包/读包测试用本地模拟设备
themes/                   主题资源
```

整体结构参考 THRM 的 Wails 桌面应用组织方式，同时将硬件协议、后台服务、GUI 绑定和前端界面模块拆开，方便后续继续重写控制与学习逻辑。

## 本地开发

```powershell
go mod tidy
npm install --prefix frontend
wails dev
```

## 构建

```powershell
npm install --prefix frontend
npm run build --prefix frontend
dotnet publish bridge\TempBridge\TempBridge.csproj -c Release --self-contained false -o build\bin\bridge /p:Platform=x64 /p:DebugType=none /p:DebugSymbols=false
.\build.bat
```

生成安装器需要安装 NSIS 3.x，并确保 `makensis.exe` 在 `PATH` 中。

`build.bat` 会生成：

```text
build/bin/FanControlPortable.exe
build/bin/FanControlPortable Core.exe
build/bin/bridge/
build/bin/FanControlPortable-amd64-installer.exe
```

安装器需要 `build/bin/PawnIO_setup.exe`。可以运行 `build_bridge.bat` 下载，也可以在打包前手动放入 `build/bin/`。

## 模拟设备

```powershell
tools\mock-device\start-mock-device.ps1
```

默认地址：

```text
http://127.0.0.1:18080
```

模拟设备实现 `GET /api/data` 和 `POST /api/speed`，用于测试真实 WiFi 读写路径。

## 发布

GitHub Releases 是正式分发渠道。Release 附件应包含：

```text
FanControlPortable-2.0.0-amd64-installer.exe
FanControlPortable-2.0.0-portable.zip
```

打包后的二进制文件不提交到 git，应作为 Release 附件上传。

</details>

## 许可证

本项目沿用上游开源基础的 MIT License，详见 [LICENSE](LICENSE)。

## 免责声明

FanControlPortable 是面向特定散热控制设备的第三方开源项目。使用本软件产生的风险由用户自行承担。
