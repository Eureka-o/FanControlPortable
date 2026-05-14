# FanControlPortable

FanControlPortable is a Windows desktop controller for an ESP8266-based automatic laptop cooling pad. It reads local CPU/GPU temperatures, applies a configurable fan curve, and sends speed commands to the cooler over Wi-Fi.

The app is designed for portable use: unzip one package, run the executable, enter the cooler address, choose a mode, and keep the controller in the tray.

## Downloads

| Package | Best For | Includes | Approx. Zip / Unpacked |
| --- | --- | --- | --- |
| `FanControlPortable-兼容版.zip` | Maximum compatibility | Self-contained .NET 8 app, WebView2 installer, PawnIO installer | 75.8 MB / 82.1 MB |
| `FanControlPortable-lite.zip` | Modern Windows PCs that may need sensor driver help | .NET Framework 4.8 app, PawnIO installer | 6.8 MB / 10.1 MB |
| `FanControlPortable-superlite.zip` | Smallest package for prepared systems | .NET Framework 4.8 app only | 2.1 MB / 5.1 MB |

Use the compatibility package if you are sending the app to someone and do not know their Windows setup. Use `lite` or `superlite` when the target PC already has Microsoft Edge WebView2 Runtime and .NET Framework 4.8.

## Main Features

- CPU/GPU temperature monitoring with selectable sensor sources.
- Automatic, manual, off, and monitor-only operating modes.
- Configurable fan curve and quick speed presets.
- Background tray mode with current mode shown in the right-click menu.
- Optional startup with Windows.
- Performance options for lower background memory use.
- Import/export of the local profile.

## Cooler / Controller Notes

The cooler is expected to expose a small HTTP API over Wi-Fi. A typical ESP8266 controller page shows information such as:

- device uptime and chip/flash status;
- Wi-Fi station/AP state, SSID, IP address, hostname, and MAC address;
- firmware/about information and build date;
- OTA update entry;
- Wi-Fi reset/erase option;
- pages such as menu, Wi-Fi configuration, parameter page, info page, update page, restart, and erase.

The Windows app only needs the cooler IP or `IP:port` to communicate with it. If the controller is in access point mode, connect to its AP first, configure Wi-Fi, then return to the normal network and use the station IP.

## Package Guidance

`兼容版` is the safest package. It keeps the .NET runtime inside the app bundle and includes installer helpers for WebView2 and PawnIO.

`lite` is smaller, but depends on Windows having .NET Framework 4.8 and WebView2 Runtime. It still includes PawnIO so users can install the hardware driver if CPU/GPU sensor access is limited.

`superlite` is the smallest package. It does not include a `Resources` folder or helper installers. Use it only when the target PC already has the required runtime, WebView2, and driver environment.

## Build

```powershell
dotnet build "源码\FanControlPortable\FanControlPortable.csproj" -c Release
dotnet publish "源码\FanControlPortable\FanControlPortable.csproj" -c Release -o "源码\FanControlPortable\bin\Release\publish\portable-single"
```

Lightweight builds:

```powershell
dotnet publish "源码\FanControlPortable\FanControlPortable.csproj" -c Release -p:LightweightPackage=true -p:IncludePawnIoInstaller=true -o "源码\FanControlPortable\bin\Release\publish\lite-net48"
dotnet publish "源码\FanControlPortable\FanControlPortable.csproj" -c Release -p:LightweightPackage=true -p:IncludePawnIoInstaller=false -o "源码\FanControlPortable\bin\Release\publish\superlite-net48"
```

## 中文说明

FanControlPortable 是一个 Windows 便携式风扇控制软件，用来控制基于 ESP8266 的笔记本散热底座。软件读取本机 CPU/GPU 温度，根据风扇曲线自动调速，也支持手动、关闭和仅监控模式。

三个包的区别：

- `FanControlPortable-兼容版.zip`：最稳，适合直接发给别人，包含自带 .NET 运行时、WebView2 安装器和 PawnIO 安装器。
- `FanControlPortable-lite.zip`：轻量版，包含 PawnIO 安装器，但不包含 WebView2 安装器。
- `FanControlPortable-superlite.zip`：最小版，不包含 `Resources` 文件夹，也不包含安装器，适合运行环境已经准备好的电脑。

控制器侧通常会有 Wi-Fi 配置、设备信息、OTA 更新、擦除 Wi-Fi 配置等页面。软件端只需要填写散热器的 IP 或 `IP:端口` 即可连接。
