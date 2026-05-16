# FanControlPortable

FanControlPortable is a portable Windows controller for an ESP8266-based laptop cooling pad. It reads local CPU/GPU temperatures, applies a fan curve, and sends speed commands to the cooler over Wi-Fi.

## Downloads

Start with `FanControlPortable-lite.zip` for most modern Windows PCs.

| Package | Recommended For | Includes |
| --- | --- | --- |
| `FanControlPortable-lite.zip` | Default choice. Smallest package for prepared systems. | .NET Framework 4.8 app only |
| `FanControlPortable.zip` | Standard package when sensor driver help may be needed. | .NET Framework 4.8 app + PawnIO installer |
| `FanControlPortable-compat.zip` | Compatibility package for unknown Windows environments. | Self-contained .NET 8 app + WebView2 installer + PawnIO installer |

The app may create a `FanControlPortable.exe.WebView2` folder next to the executable. That is WebView2 runtime cache. Keeping it avoids unnecessary reloads and is the least disruptive choice.

## Main Features

- First-run setup card for IP, connection test, temperature source, and auto mode.
- Human-readable connection diagnostics for empty IP, wrong format, timeout, wrong device/API, and rejected speed commands.
- GitHub release update check with matching package download link.
- Built-in fan curve presets: Quiet, Balanced, Cooling, and Game.
- Clear status dashboard: CPU/GPU temperature, fan speed, target speed, mode, online state, and last successful command time.
- Recent cooler IP list and one-click retest.
- Tray quick actions for monitor/manual/auto/off and common speeds.

## Cooler Connection

The cooler is expected to expose:

- `GET /api/data`
- `POST /api/speed` with JSON `{ "speed": 0-100 }`

Enter the cooler IP or `IP:port` in the top bar, then click **Test Connection**. If the controller is in access point mode, connect to its AP first, configure Wi-Fi, then use the station IP after it joins the normal network.

## Package Guidance

- Choose `lite` first.
- If CPU/GPU sensors are unavailable, try `FanControlPortable.zip` and install PawnIO from `Resources/assets`.
- If the app cannot start because .NET or WebView2 is missing, use `FanControlPortable-compat.zip`.

## 中文快速说明

- 大多数电脑先下载 `FanControlPortable-lite.zip`。
- 如果温度传感器读不到，再用标准包 `FanControlPortable.zip`，并安装 `Resources/assets` 里的 PawnIO。
- 如果软件打不开、缺 .NET 或 WebView2，再用 `FanControlPortable-compat.zip`。
- 软件目录里可能出现 `FanControlPortable.exe.WebView2` 文件夹，这是 WebView2 缓存。保留它最稳，能避免每次重新加载界面。

## Build

Compatibility package:

```powershell
dotnet publish "源码\FanControlPortable\FanControlPortable.csproj" -c Release -o "源码\FanControlPortable\bin\Release\publish\portable-single"
```

Standard package:

```powershell
dotnet publish "源码\FanControlPortable\FanControlPortable.csproj" -c Release -p:LightweightPackage=true -p:IncludePawnIoInstaller=true -o "源码\FanControlPortable\bin\Release\publish\standard-net48"
```

Lite package:

```powershell
dotnet publish "源码\FanControlPortable\FanControlPortable.csproj" -c Release -p:LightweightPackage=true -p:IncludePawnIoInstaller=false -o "源码\FanControlPortable\bin\Release\publish\lite-net48"
```
