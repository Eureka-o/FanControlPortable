# FanControl 官方插件最终技术路线

日期：2026-07-15

状态：已整合并核对，作为后续实现的唯一技术基线

实现进度（2026-07-15）：

- 已完成通用插件清单扫描、路径和版本校验、启用状态持久化、设置页插件管理，以及删除/重置安全边界。
- 已完成外部后端进程的 JSONL 握手、通用调用和事件、latest-value 遥测、受控停止、退出与休眠/唤醒接入；运行状态通过 Core revision 快照推送到 GUI。
- 已完成受控插件资源路由、`FanControlPluginHost v1`、IIFE 页面注册、动态侧栏、首次打开懒加载，以及停用/失效后的资源清理和页面回退。
- 尚未完成 Windows Job Object、崩溃退避重启和心跳失效恢复。
- 尚未迁移正式 OMEN 后端，也未加入任何真实 OMEN 硬件写入。

## 0. 文档地位与核对结论

本文整合并取代以下旧路线中的架构结论和示例代码：

- `D:/Desktop/Fancontrol/OMEN_PLUGIN_PLAN.md`
- `D:/Desktop/Fancontrol/OMEN_PLUGIN_V2_SPEC.md`
- 本轮关于 GUI、Core 与 OMEN 后端挂载方式的最终讨论

旧文档仍可作为历史调研材料，但不得再直接指导实现。发生冲突时以本文为准。

本次已按当前代码核对关键落点：

- GUI 与 Core 已经是独立进程；普通退出 GUI 只断开 IPC，Core 可以继续运行，只有“完全退出”才停止 Core。这正好支持“页面挂在 GUI、控制挂在 Core”的目标。
- `internal/plugins/manager.go` 的全局锁边界已经修正；外部进程由独立 `Supervisor` 与单插件生命周期锁监管，阻塞操作不在注册表锁内执行。
- `frontend/src/app/components/AppShell.tsx` 已支持“内置页面 + 动态插件页面”，主页面外壳、滚动和切页动画仍由宿主控制。
- `main.go` 已沿用主题资源模式接入插件同源资源路由，并增加类型、大小、目录逃逸、符号链接和 `nosniff` 边界。
- `internal/coreapp/monitoring.go` 已通过非阻塞 latest-value 队列向声明了输入且处于 `ready` 的插件提交通用遥测，监控 tick 不等待插件 I/O。

硬件参考核对以本地最新可用的 OmenMon `d89340e` 和 OmenSuperHub `f4b3864` 为依据。旧 V2 文档中的 GPU/MUX、显示过驱、电池限制和 Dynamic Boost 命令映射存在命令类型混淆或缺少读回/恢复证据，不进入首版承诺。首版只采用两份参考实现能够相互印证的 OMEN 风扇等级闭环。

## 1. 目标

FanControl 第一阶段只支持由项目维护者审核和发布的官方插件。插件安装包同时携带可动态接入的后端进程和前端页面，主程序只提供通用的发现、监管、通信、资源加载和视觉宿主能力。

本设计首先用于 HP OMEN / 暗影精灵控制插件，但主程序内不得出现 OMEN 专用请求、状态字段或页面分支。

运行目标是：GUI 负责交互，Core 负责常驻监管和遥测，插件后端负责硬件领域逻辑。关闭或重启 GUI 不应中断已经由 Core 托管的风扇控制。

## 2. 非目标

- 第一阶段不开放第三方插件市场。
- 不使用 Go 原生 `plugin` 包。该机制不适合 Windows，且与 Go 编译器及依赖版本强耦合。
- 不把硬件插件作为 Goja JavaScript 运行在 Core 主进程中。
- 不恢复旧原型的 `new Function` 前端执行方式和 `127.0.0.1:8787` Mock HTTP 通信。
- 不在第一阶段加入插件目录文件监听依赖；启动扫描和用户触发刷新已经覆盖官方安装流程。
- 主安装包和主便携包不捆绑 OMEN 插件。
- 不把 OMEN 曲线、模式、WMI 命令或联合学习字段放进主 `AppConfig`、Core 类型和 Wails API。
- 不在第一阶段实现 FlyDigi 与 OMEN 的联合噪音优化。后续若需要，必须先抽象成与品牌无关的多执行器协调协议。

## 3. 参考项目取舍

Gopeed 的扩展系统提供了适合复用的产品模型：清单、安装、启停、更新、设置、独立存储、开发模式、事件激活和小型宿主接口。

Gopeed 后端扩展实际由 Goja 在 Go 主进程内执行 JavaScript，不是动态加载 Go 代码。这个执行模型适合下载链接解析，但不适合 WMI、HID 和需要恢复原始硬件状态的控制插件。FanControl 只借用其管理模型，不借用其运行时。

## 4. 总体架构

```text
┌──────────────────────────── GUI 进程 ────────────────────────────┐
│ 动态侧栏 / 设置插件分区 / React 页面注册 / invoke / subscribe     │
│                   FanControlPluginHost v1                        │
└──────────────────────────────┬───────────────────────────────────┘
                               │ Wails + GUI/Core 通用插件 IPC
┌──────────────────────────────▼───────────────────────────────────┐
│                            Core 进程                              │
│ 发现与启用状态 / 进程监管 / JSONL / 遥测投递 / 休眠恢复 / 状态修订 │
└──────────────────────────────┬───────────────────────────────────┘
                               │ stdin/stdout JSON Lines
┌──────────────────────────────▼───────────────────────────────────┐
│                      官方插件后端 EXE                            │
│ 曲线与目标计算 / fan-level 转换 / WMI 写入读回 / 限频 / 安全恢复   │
└──────────────────────────────────────────────────────────────────┘
```

插件包同时注入前端和后端：

```text
plugins/omen-fan/
|- plugin.json
|- backend/omen-fan-driver.exe
|- ui/index.js
|- ui/index.css
`- ui/assets/omen.png
```

职责边界固定为：

- GUI 拥有插件资源路由接入、动态导航、设置分区、React 页面注册、通用调用和事件订阅，以及 Core 状态的渲染。
- Core 拥有发现、启用状态、外部进程监管、协议、遥测投递、休眠/唤醒/退出协调和权威状态快照。
- OMEN 后端拥有 CPU/GPU 风扇曲线、目标计算、RPM 到 fan level 转换、WMI 写入与读回、去重限频、原始状态快照和恢复。

Core 不理解 OMEN 方法的业务含义，也不计算 OMEN 风扇目标。主程序只增加通用宿主代码；未安装插件时，OMEN 后端和前端均不占用主安装包体积，也不会参与首页渲染。

## 5. 插件清单

第一版清单保持最小，只声明运行和兼容性所需信息：

```json
{
  "id": "omen-fan",
  "name": "HP OMEN Fan Control",
  "version": "0.1.0",
  "platform": "windows-amd64",
  "minCoreVersion": "2.5.3",
  "protocolVersion": 1,
  "backend": "backend/omen-fan-driver.exe",
  "frontend": "ui/index.js",
  "style": "ui/index.css",
  "page": {"id": "control", "title": "HP OMEN", "icon": "fan", "iconAsset": "ui/assets/omen.png", "order": 500},
  "hostApiVersion": 1,
  "capabilities": ["status", "fan-control"],
  "telemetryInputs": ["cpu-temp", "gpu-temp", "cpu-power", "gpu-power"]
}
```

Core 必须校验 ID、版本、相对路径、平台、协议版本、Core 最低版本、入口文件和页面元数据。页面保留宿主白名单图标作为回退；可选 `iconAsset` 只能指向插件目录内经过同一安全校验的图片。标题长度和排序值必须限幅。任何绝对路径、路径穿越、符号链接逃逸或清单 ID 与目录 ID 不一致都必须拒绝。

第一阶段的官方信任由官方发布链保证：独立安装器使用项目发布渠道和校验值，主程序仍对本地路径、资源类型和消息大小执行输入校验。该模型不是第三方代码沙箱。

清单中的 `capabilities` 只表示插件可能提供的粗粒度能力。页面是否显示某项控制，以后端握手后的运行时能力和硬件只读检测结果为准；清单不能替代硬件检测。

## 6. 后端运行模块

现有 `internal/plugins` 模块需要从编译期生命周期列表深化为同时支持注册表和外部进程监管的模块。Core 只依赖它公开的插件列表、状态、启停、调用和生命周期接口，不直接处理进程管道。

### 6.1 发现和状态

启动时扫描 `<install-dir>/plugins/<id>/plugin.json`。设置页提供显式刷新操作；官方安装器关闭主程序后安装，下一次启动自然完成发现，因此第一版不需要 `fsnotify`。

运行状态固定为：

- `discovered`：文件存在且清单有效。
- `disabled`：已安装但未启用。
- `starting`：进程已启动，等待握手。
- `ready`：握手成功，可以处理请求。
- `suspending` / `suspended`：系统休眠流程中。
- `restarting`：异常退出后退避重启。
- `unsupported`：后端正常运行，但硬件检测不支持。
- `incompatible`：平台、Core、协议或宿主接口版本不兼容。
- `failed`：连续启动失败，停止自动重试。

### 6.2 进程监管

后端使用 Go 标准库 `os/exec` 启动。`stdin/stdout` 只传输逐行 JSON，`stderr` 由 Core 捕获并带插件 ID 写入日志。

监管规则：

- 启动后约 3 秒内必须完成握手。
- 异常退出按 1 秒、3 秒、10 秒退避重启。
- 短时间连续失败达到上限后进入 `failed`，等待用户重试。
- 禁用、休眠、更新和主程序退出期间不允许自动重启。
- 监管器不得在持有全局锁时等待进程启动、停止或请求响应。
- Core 退出时确保子进程随进程树终止，但正常退出必须先给插件恢复硬件的机会。

当前实现已经先完成 `plugins.Manager` 改造：管理器锁只用于注册表查找、上下文和快照复制，随后在锁外执行 `Start`、`Stop` 和 `Status`。每个外部插件实例拥有自己的生命周期互斥锁，负责串行化启动、停止和后续重启。不得用一个全局锁保护可能阻塞的进程操作。

Windows 上外部后端应加入带 `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` 的 Job Object，保证 Core 异常终止后不会遗留失控子进程。Job Object 只负责最终兜底；正常停止必须先走插件恢复协议，达到恢复期限后才能强制终止。

### 6.3 配置和数据

主配置只保存通用启用状态，例如插件 ID 到布尔值的映射。插件专属曲线、模式和学习数据保存在 `config/plugins/<id>/`。Core 创建并在初始化握手中传入该数据目录，官方插件后端自行使用原子写入保存，不把专属字段塞入主 `AppConfig`。

插件数据与安装目录分离，升级和重装插件不得清除用户数据。卸载时是否清除数据必须由用户显式选择。

### 6.4 Core 遥测与后台控制

Core 在温度监控循环获得新样本后，为每个声明了对应输入的 `ready` 插件生成一个只包含通用字段的遥测快照：

```json
{
  "type": "telemetry",
  "sequence": 1842,
  "sampledAt": 1784044800123,
  "payload": {
    "cpuTemp": {"value": 62, "valid": true},
    "gpuTemp": {"value": 58, "valid": true},
    "cpuPowerWatts": {"value": 34.5, "valid": true},
    "gpuPowerWatts": {"value": 71.2, "valid": true},
    "bridgeOk": true
  }
}
```

监控循环只能调用非阻塞 `SubmitTelemetry(snapshot)`。每个外部插件使用容量为 1 的 latest-value 队列：新样本到来时替换尚未发送的旧样本，由独立 writer goroutine 完成 JSON 序列化和管道写入。插件阻塞、退出或管道拥塞不得拖慢 `monitoring.go`。

OMEN 后端收到遥测后自行选择 CPU/GPU 温度、计算曲线目标并执行 WMI 控制。GUI 是否存在不影响这条路径。遥测超过后端声明的有效期、Core 心跳中断或 stdin EOF 时，后端必须停止继续沿用旧目标并进入硬件恢复流程。

## 7. 通信协议

协议版本 1 使用 JSON Lines，每行一个完整消息。握手、调用、遥测和事件都经过同一条受监管管道：

```json
{"type":"hello","protocolVersion":1,"pluginId":"omen-fan","version":"0.1.0","capabilities":["status","fan-control"]}
{"type":"host-init","protocolVersion":1,"instanceId":"...","dataDir":".../config/plugins/omen-fan","heartbeatIntervalMs":2000}
{"type":"telemetry","sequence":1842,"sampledAt":1784044800123,"payload":{"cpuTemp":{"value":62,"valid":true}}}
{"type":"request","id":"42","method":"set-mode","payload":{"mode":"balanced"}}
{"type":"response","id":"42","ok":true,"payload":{}}
{"type":"event","event":"status-changed","payload":{"cpuRpm":2200,"gpuRpm":2100}}
```

协议要求：

- Core 只提供 `invoke(pluginID, method, payload)`，不知道方法的业务含义。
- 每个请求都有唯一 ID、超时和消息大小上限。
- 插件 ID 和版本必须与清单一致。
- `stdout` 出现非 JSON 或超限消息视为协议错误。
- 写操作超时后不得自动重放，因为硬件命令可能已经执行。
- 读操作是否重试由调用方重新发起，不在管道层隐式重试。
- 插件事件统一以插件 ID 命名空间转发，防止不同插件事件冲突。
- `hello` 成功后 Core 才发送 `host-init` 和遥测；握手前的业务事件和请求一律拒绝。
- 遥测 `sequence` 单调递增，后端忽略旧序列；`sampledAt` 与字段级 `valid` 共同决定样本能否参与控制。
- 心跳、单行大小、请求并发数、响应超时和 stderr 缓冲都有固定上限，防止插件耗尽 Core 资源。

Core/GUI 之间只增加通用能力：`GetPluginSnapshot`、`RefreshPlugins`、`SetPluginEnabled`、`InvokePlugin`、插件状态事件和插件业务事件。不得增加 OMEN 专用 Wails 方法。

`GetPluginSnapshot` 返回全量状态、运行时能力、已校验的页面元数据及单调递增的 `revision`；每条插件状态事件也携带 `revision`。GUI 先安装事件监听并暂存事件，再请求快照，应用快照后只重放 `revision` 更大的事件。GUI/Core IPC 重连后重复同一流程，旧连接和旧修订事件一律丢弃。这避免“先取快照还是先订阅”产生的丢事件窗口。

`GetPluginSnapshot`、列表和只读状态请求可以在 GUI IPC 重连后重试；`SetPluginEnabled` 和 `InvokePlugin` 中的写调用沿用当前安全策略，响应丢失时只报告“结果未知”，不得自动重放。

## 8. 休眠、恢复和退出

休眠流程：

1. Core 阻止该插件接受新调用。
2. 发送 `prepare-suspend`。
3. 插件恢复进入控制前保存的原始硬件状态。
4. 插件确认后退出，Core 进入 `suspended`。
5. 超时则终止进程并记录恢复结果未知。

唤醒流程：

1. 不复用休眠前的 WMI、HID 或管道句柄。
2. 创建新进程并重新完成握手和能力检测。
3. 成功后恢复 `ready`，失败则进入退避重启。
4. 前端只根据状态事件更新，不主动轮询本地 HTTP 端口。

禁用、更新和主程序退出使用相同的受控停止流程。插件负责硬件专属的原始状态快照、脏状态标记和恢复；Core 只提供停止原因、超时和进程监管。

第一次硬件写入前，OMEN 后端必须完成以下事务：

1. 只读获取当前可恢复的模式、手动控制状态、最大风扇状态和风扇等级。
2. 将原始状态与插件版本原子写入 `config/plugins/omen-fan/hardware-state.json`。
3. 持久化 `dirty: true`，确认落盘后才允许第一次写入。

禁用、完全退出、休眠、遥测过期、Core 心跳丢失、stdin EOF 和正常更新都必须尝试恢复。只有硬件读回确认恢复成功后才能清除 dirty 标记。若后端崩溃来不及恢复，下次启动必须先检测 dirty 标记并执行恢复，成功前不得进入正常控制。

硬件写入超时表示结果未知，不允许自动重放。正常停止等待恢复完成；达到期限后 Core 记录“恢复结果未知”，再使用 Job Object 或进程终止作为最后手段。

## 9. 前端资源加载

Wails 已有自定义主题资源处理器。插件前端沿用同一模式，通过受控资源路径提供：

```text
/plugin-assets/<plugin-id>/ui/index.js?v=<plugin-version>
/plugin-assets/<plugin-id>/ui/index.css?v=<plugin-version>
```

资源处理器只允许已发现且兼容插件目录下的相对路径，仅开放 JavaScript、CSS、JSON、图片和字体白名单，拒绝后端可执行文件、目录逃逸、符号链接和超限文件。响应设置明确 MIME 与 `X-Content-Type-Options: nosniff`；匹配插件版本的查询参数使用 immutable 缓存，其余请求不缓存。

页面通过正常 `<script src>` 和 `<link>` 加载。禁止把文件读成字符串后使用 `eval` 或 `new Function`。插件禁用或更新时，宿主注销页面和事件订阅，并移除对应样式节点。

官方插件 UI 的发布格式固定为 IIFE：不包含第二份 React，不依赖 Node 运行时，也不要求宿主重新编译。脚本从 `window.FanControlPluginHost` 获取宿主 React 和白名单组件，执行后调用 `registerPage`。构建产物不得引用只能由插件源码触发生成的 Tailwind utility；插件差异样式使用带 `[data-plugin-id="<id>"]` 前缀的语义 CSS。

浏览器无法真正卸载已经执行的 JavaScript。禁用时可以注销页面、事件和 CSS，但不能回收脚本定义。因此 v1 的插件安装或更新必须重启 FanControl，显式刷新只用于重新扫描清单和状态，不支持运行中替换前端代码。

## 10. 前端宿主接口

`FanControlPluginHost v1` 保持小而稳定：

- `React`：宿主正在使用的 React 实例和必要 hooks。
- `registerPage(definition)`：注册与 manifest 页面 ID 匹配的 React 页面和释放函数。
- `invoke(pluginID, method, payload)`：调用后端。
- `subscribe(pluginID, event, handler)`：订阅插件事件并返回取消函数。
- `ui`：显式白名单的宿主 UI 工具箱和少量 Lucide 图标。
- `theme`：当前主题和稳定 CSS 变量信息。
- `locale`：当前语言。
- `toast`：统一通知。

不向插件接口公开完整 `apiService`、Zustand Store、主配置对象、任意 Wails 方法、全部 UI 导出或全部图标导出。

官方插件脚本仍与主页面运行在同一 WebView 中，因此小型宿主接口主要解决兼容性和维护问题，不构成安全沙箱。未来开放第三方插件时，必须改用隔离 WebView、签名和权限模型。

## 11. 导航和设置页

设置页增加第四个分区“插件”，负责：

- 官方插件介绍和安装状态。
- 版本、启用开关和硬件支持状态。
- 启动、重启、刷新、打开控制页和导出诊断。
- 更新可用状态；第一阶段安装和更新仍使用官方独立安装器。
- 失败原因和最近的后端日志摘要。

前端已将 `ActiveTab` 深化为内置页与 `plugin:<plugin-id>:<page-id>` 联合类型。插件达到可展示状态后，GUI 根据快照中的已校验 manifest 页面元数据生成懒加载入口。用户首次打开时才加载 CSS/IIFE，IIFE 的 `registerPage` 必须匹配当前脚本、manifest 插件 ID 和页面 ID，并提交真正的 React 组件。宿主继续拥有侧栏排序、加载/错误占位、内容容器、滚动复位和切页动画，插件不直接修改 `AppShell` DOM。

侧栏规则：

- 未安装或已禁用：不显示插件入口。
- 已启用且后端 `ready` 并支持当前硬件：显示控制入口。
- 首次检测为 `unsupported`：不显示主控制入口，在插件管理中展示原因和诊断。
- 已经进入 `ready` 后失去可用后端：注销页面并隐藏入口；当前正在打开时回到状态页并提示。
- 前端资源加载失败：保留入口并展示宿主错误页。
- 插件被用户禁用或卸载且当前正打开该页面：完成注销后切换到概览页，并显示一次明确通知。

首次启动、GUI 重启和 GUI/Core IPC 重连均使用“事件监听缓冲 -> 获取 revision 快照 -> 应用快照 -> 重放新事件”的流程。页面显示状态只能来自 Core 快照和事件，不能通过本地 HTTP 端口或插件进程轮询自行推断。

## 12. 视觉规范

主程序拥有页面外壳、内容宽度、滚动、切页动画、字体、间距和背景。插件页面只组合宿主 UI 工具箱：

- 页面标题、状态标签、告警条和分段页签。
- 指标格、设置组和设置行。
- 按钮、开关、滑块、选择器、对话框、提示框和 Toast。
- 共享风扇曲线编辑器、图表颜色和交互行为。
- Lucide 图标和宿主主题变量。

插件 CSS 必须限定在 `[data-plugin-id="<id>"]` 根节点内，禁止全局选择器、自建全局主题和另一套卡片系统。品牌视觉只用于图标和页面标题，不能覆盖 FanControl 的整体视觉。

OMEN 页面沿用旧 V2 文档中“紧凑状态区 + 快捷操作 + 分段内容”的信息结构，但必须使用宿主组件和当前 FanControl 视觉规范，不照搬旧 `glacier-*` 示例类名。第一版按能力展示：

- 概览：支持状态、CPU/GPU 温度、当前风扇等级或估算 RPM、当前模式。
- 风扇：按后端能力显示自动、全速、手动 CPU/GPU 目标转速；自定义曲线随后接入。所有命令只按硬件读回结果更新；仅在已验证进入/退出模式闭环时显示模式切换。
- 性能：不进入首版。后续只有在后端完成能力检测、写入、读回和恢复闭环后，才按能力增加功耗、MUX 或其他控制。

所有硬件写入控件在请求期间进入等待状态，成功后以硬件读回结果更新。失败时保留原显示值，不做盲目乐观更新。

不支持的高风险功能不显示空白卡片或占位控件；支持诊断只放在设置页插件详情中。旧 V2 文档中的 FlyDigi 联合噪音优化条暂不进入 OMEN 首版页面。

OMEN 前端 v0.1 使用以下插件私有方法和事件；Core 仍只转发字符串方法，不理解含义：

- `get-status`：返回完整状态、能力、可用模式、RPM 范围与最后读回时间。
- `set-fan-mode`：`{"mode":"auto|max|manual"}`，返回写后完整状态。
- `set-manual-speed`：`{"cpuRpm":2400,"gpuRpm":2200}`，返回写后完整状态。
- `set-thermal-mode`：仅当 `capabilities.thermalMode=true` 且模式出现在 `availableThermalModes` 时显示和调用。
- `status-changed`：推送完整或增量状态，GUI 与当前读回状态合并。

对 OmenSuperHub `f4b3864` 的前端能力核对确认其还暴露功耗、GPU 模式、超频、帧率和灯光控制；这些路径依赖额外驱动、NVAPI 或机型数据，且当前插件尚无统一读回与恢复事务，因此不进入本阶段 UI。

## 13. OMEN 首版硬件范围

### 13.1 已核对的风扇控制基线

OmenMon 与 OmenSuperHub 能够相互印证以下 WMI BIOS 路径：

| 操作 | CommandType | 命令 | 首版用途 |
|------|-------------|------|----------|
| SystemDesignData | Default | `0x28` | 热策略版本、软件风扇控制支持位 |
| 风扇数量 | Default | `0x10` | 确认双风扇/三风扇布局 |
| 风扇类型与能力 | Default | `0x2C` | CPU/GPU 风扇映射 |
| 读取 fan level | Default | `0x2D` | 写入前快照和写后读回 |
| 写入 fan level | Default | `0x2E` | 实际风扇控制 |
| 读取 fan table | Default | `0x2F` | 等级范围和保守限幅 |
| 设置风扇/性能模式 | Default | `0x1A` | 仅在机型闭环验证后使用 |
| 读取/设置最大风扇 | Default | `0x26` / `0x27` | 原始状态快照和恢复 |

命令 ID 不能脱离 CommandType、payload、返回长度和机型能力单独解释。首批真实写入只允许在明确支持的暗影精灵 11 硬件 ID 上开启；其他机型保持 `unsupported` 或只读。

RPM 是 UI 和曲线目标单位，WMI 实际写入 fan level。OmenSuperHub 使用 `rpm / 100` 作为换算依据，但首版后端仍必须结合 fan table、当前等级和实机标定进行限幅；未标定时显示“估算 RPM”。转换、限幅、写入、去重和读回全部属于 OMEN 后端，不进入 Core。

### 13.2 已否决或延期的旧 V2 结论

- GPU 模式在 OmenMon 中是 `Cmd.Legacy + 0x52` 读取、`Cmd.GpuMode + 0x52` 写入，不是旧文档描述的 `0x50/0x51` 后再提交 `0x52`。
- `Default + 0x27` 是最大风扇设置，`Default + 0x28` 是 SystemDesignData，不能作为旧文档所写的显示过驱读写对。
- 旧文档把若干 EC 偏移或未区分 CommandType 的值直接当作电池限制、Dynamic Boost 和能力查询命令，证据不足。
- `0x1A` 是带机型映射的模式写入，不是统一的“热力模式读取”；只有找到可靠的原始模式读取和恢复路径后才能对用户开放模式切换。

MUX、显示过驱、电池限制、Dynamic Boost 和功耗精调均延期。每项功能必须独立完成“能力检测 -> 只读状态 -> Mock 写入 -> 实机小步写入 -> 读回 -> 休眠/退出恢复”后，才能增加后端 capability 和页面控件。

### 13.3 首版写入门禁

1. Mock 协议、状态机和 dirty 恢复测试通过。
2. 非 OMEN、WMI 缺失、能力位不支持和未知硬件 ID 均安全返回，不执行写入。
3. 目标机完成 SystemDesignData、风扇数量/类型、当前等级、fan table、最大风扇和可恢复模式的只读快照。
4. 单次保守 fan level 写入后立即读回，确认 CPU/GPU 映射和范围。
5. 禁用、GUI 重启、Core 完全退出、休眠/唤醒、遥测过期和后端异常路径均验证恢复。

任一门禁失败时不得扩大真实写入范围。

## 14. 用户可感知行为

- 安装插件后，设置页显示“已安装，未启用”。
- 用户启用后进行只读能力检测；支持时侧栏出现插件入口。
- 插件页面首次打开时才加载前端资源，不影响首页启动和切换性能。
- 后端崩溃时页面显示“正在重连”，自动恢复成功后继续使用。
- 休眠期间显示“已暂停”，唤醒后显示“正在恢复”。
- 关闭或重启 GUI 时，Core 与插件后端继续执行已启用的风扇控制；选择“完全退出”时先恢复硬件再退出。
- 写入超时显示“结果未知，请刷新状态”，不重复发送硬件命令。
- 插件不支持当前设备时，设置页提供明确原因和诊断导出，不显示空白控制页。
- 页面只展示后端真实声明的能力，不显示尚未实现的占位控制。
- 安装或更新插件后提示需要重启 FanControl；单纯刷新不会热替换已经执行的前端脚本。

## 15. 安装、升级和体积

主安装包和便携包继续不携带 OMEN。当前主安装器升级只替换主程序拥有的文件，并使用非递归方式删除安装根目录，因此可以保留独立插件目录。

主程序新增内容只有通用注册表、监管器、IPC 路由、资源加载器和 UI 宿主。OMEN 前后端文件全部位于独立插件包。主程序无需新增 Goja、文件监听、第二份 React 或新的前端运行库；Windows Job Object 可复用现有 `golang.org/x/sys` 依赖。

无法承诺主程序二进制字节级零增长，但可以保证不携带任何 OMEN payload 和大型新运行时。每个阶段都要记录无插件构建的安装包/便携包体积和启动时间；若出现与通用宿主代码不相称的增长，必须在进入 OMEN 迁移前处理。

第一阶段的官方插件安装器在替换运行中的后端前必须请求 FanControl 完全退出并完成硬件恢复，避免监管器重启进程并与文件覆盖竞争。安装完成后要求重新启动 FanControl。协议稳定后，再由插件管理页协调停止、原子替换和重启，实现内置更新。

## 16. 验证策略

主程序平台测试：

- 清单验证、版本兼容和所有路径穿越情况。
- 假插件进程的握手、请求响应、事件、超时、崩溃和退避重启。
- `plugins.Manager` 生命周期方法不持有全局锁，慢插件不会阻塞其他插件状态读取。
- latest-value 遥测队列在管道阻塞时不阻塞温度监控 tick，并保留最新样本。
- 禁用、退出、休眠和唤醒状态转换。
- GUI 首次启动和 IPC 重连的 revision 快照/事件竞态、旧事件丢弃和页面保持。
- 插件资源 MIME、缓存版本和目录逃逸防护。
- IIFE 使用宿主 React、动态侧栏显示规则、页面加载失败、事件释放、CSS 移除和更新需重启提示。
- 主程序无插件时的启动时间和安装包体积对比。
- GUI 单独退出/重启期间后台控制保持，完全退出时恢复后端硬件。

OMEN 插件测试：

- Mock 协议和只读 WMI 检测。
- 风扇等级转换、去重、限速、写入读回和超时不重放。
- 恢复原始模式和风扇等级。
- dirty 标记残留后的下次启动恢复，以及恢复失败时禁止重新控制。
- 暗影精灵 11 实机上的休眠、唤醒、退出和异常终止验证。

真实硬件写入必须在 Mock、只读检测和恢复流程全部通过后单独开启。

## 17. 实施顺序

1. 基础并发：修正 `plugins.Manager` 锁边界，定义注册表、状态、revision 和单插件生命周期锁。
2. 假插件后端纵向切片：实现清单扫描、路径校验、进程握手、通用 invoke/event、latest-value 遥测、受控停止和测试。
3. 假插件前端纵向切片（已完成）：实现 Wails 插件资源路由、IIFE 宿主、动态页面注册、设置插件分区、快照重连和最小 React 页面。
4. 生命周期加固：补齐 Job Object、退避重启、休眠/唤醒、GUI 重启、更新需重启和无插件体积/性能回归测试。
5. OMEN 安全迁移：将现有 Mock 与只读 WMI 检测迁移到正式插件包，只显示真实能力，不执行真实写入。
6. OMEN 风扇闭环：实现 dirty 快照、曲线计算、fan-level 转换、限频、写入读回、遥测过期恢复和暗影精灵 11 实机门禁。
7. 可选能力：MUX、功耗、显示过驱、电池和 Dynamic Boost 各自作为独立后续阶段验证，不与首版风扇控制捆绑。
8. 协议稳定后再增加官方在线目录、一键安装和更新。

每一步都必须保持无插件主程序行为不变，并使用独立提交验证后再进入下一步。
