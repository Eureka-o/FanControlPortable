using System.IO;
using System.Text.Encodings.Web;
using System.Text.Json;
using System.Windows;
using Microsoft.Win32;
using Microsoft.Web.WebView2.Core;
using Drawing = System.Drawing;
using Forms = System.Windows.Forms;

namespace FanControlPortable;

public partial class MainWindow : Window
{
    private const string StartupRegistryKeyPath = @"Software\Microsoft\Windows\CurrentVersion\Run";
    private const string StartupRegistryValueName = "FanControlPortable";

    private static readonly JsonSerializerOptions JsonOptions = new()
    {
        PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
        Encoder = JavaScriptEncoder.UnsafeRelaxedJsonEscaping
    };

    private readonly AppSettings _settings;
    private readonly FanControlRuntime _runtime;
    private readonly Dictionary<string, Forms.ToolStripMenuItem> _trayModeItems = new(StringComparer.OrdinalIgnoreCase);
    private FanRuntimeSnapshot? _lastSnapshot;
    private Forms.NotifyIcon? _trayIcon;
    private Forms.ContextMenuStrip? _trayMenu;
    private bool _allowExit;
    private bool _browserReady;
    private bool _browserReleasedForBackground;
    private CoreWebView2? _browserEventSource;
    private Task? _browserInitializationTask;

    public MainWindow(AppSettings settings)
    {
        _settings = settings;
        _runtime = new FanControlRuntime(settings);
        InitializeComponent();
        StateChanged += Window_StateChanged;
        InitializeTrayIcon();
        _runtime.SnapshotReady += Runtime_SnapshotReady;
    }

    private async void Window_Loaded(object sender, RoutedEventArgs e)
    {
        SyncStartupRegistrationWithSetting();
        await EnsureBrowserAsync();
        _runtime.Start();
        await _runtime.ForceApplyAsync();
        if (_settings.StartMinimized)
            HideToTray();
    }

    private async Task EnsureBrowserAsync()
    {
        if (_browserInitializationTask != null)
        {
            await _browserInitializationTask;
            return;
        }

        _browserInitializationTask = EnsureBrowserCoreAsync();
        try
        {
            await _browserInitializationTask;
        }
        finally
        {
            _browserInitializationTask = null;
        }
    }

    private async Task EnsureBrowserCoreAsync()
    {
        if (Browser == null)
            RecreateBrowserControl();

        var browser = Browser ?? throw new InvalidOperationException("WebView2 控件未创建。");
        var shouldNavigate = browser.CoreWebView2 == null || _browserReleasedForBackground || !_browserReady;
        await browser.EnsureCoreWebView2Async();
        var core = browser.CoreWebView2 ?? throw new InvalidOperationException("WebView2 初始化失败。");
        browser.ZoomFactor = 1.0;
        core.Settings.IsWebMessageEnabled = true;
        core.Settings.AreDevToolsEnabled = false;
        core.Settings.AreDefaultContextMenusEnabled = false;
        var assetsDirectory = Path.Combine(AppContext.BaseDirectory, "Resources", "assets");
        if (Directory.Exists(assetsDirectory))
        {
            core.SetVirtualHostNameToFolderMapping(
                "appassets.local",
                assetsDirectory,
                CoreWebView2HostResourceAccessKind.Allow);
        }

        if (!ReferenceEquals(_browserEventSource, core))
        {
            DetachBrowserEvents();
            _browserEventSource = core;
            _browserEventSource.WebMessageReceived += Browser_WebMessageReceived;
            _browserEventSource.NavigationCompleted += Browser_NavigationCompleted;
            _browserEventSource.ProcessFailed += Browser_ProcessFailed;
        }

        _browserReleasedForBackground = false;
        if (shouldNavigate)
            browser.NavigateToString(WebUi.MainHtml);
    }

    private void RecreateBrowserControl()
    {
        Browser = new Microsoft.Web.WebView2.Wpf.WebView2
        {
            DefaultBackgroundColor = Drawing.Color.FromArgb(17, 21, 27)
        };
        BrowserHost.Children.Clear();
        BrowserHost.Children.Add(Browser);
    }

    private void Window_Closing(object? sender, System.ComponentModel.CancelEventArgs e)
    {
        if (!_allowExit && _settings.CloseToTray)
        {
            e.Cancel = true;
            HideToTray();
            return;
        }

        DisposeTrayIcon();
        DetachBrowserEvents();
        _runtime.Dispose();
        System.Windows.Application.Current.Shutdown();
    }

    private void Runtime_SnapshotReady(object? sender, FanRuntimeSnapshot snapshot)
    {
        _lastSnapshot = snapshot;
        Dispatcher.Invoke(SendFullState);
    }

    private async void Browser_WebMessageReceived(object? sender, CoreWebView2WebMessageReceivedEventArgs e)
    {
        try
        {
            var message = JsonSerializer.Deserialize<WebCommand>(e.WebMessageAsJson, JsonOptions);
            if (message == null) return;
            await HandleCommand(message);
        }
        catch (Exception ex)
        {
            PostToMain(new { type = "notice", message = ex.Message });
        }
    }

    private void Browser_NavigationCompleted(object? sender, CoreWebView2NavigationCompletedEventArgs e)
    {
        _browserReady = true;
        SendFullState();
    }

    private void Browser_ProcessFailed(object? sender, CoreWebView2ProcessFailedEventArgs e)
    {
        _browserReady = false;
        Dispatcher.InvokeAsync(async () =>
        {
            try
            {
                Browser.NavigateToString(WebUi.MainHtml);
                await Task.Delay(250);
                _browserReady = true;
                SendFullState();
            }
            catch
            {
                // WebView will be retried by the next user action or app restart.
            }
        });
    }

    private void DetachBrowserEvents()
    {
        if (_browserEventSource == null)
            return;

        try
        {
            _browserEventSource.WebMessageReceived -= Browser_WebMessageReceived;
            _browserEventSource.NavigationCompleted -= Browser_NavigationCompleted;
            _browserEventSource.ProcessFailed -= Browser_ProcessFailed;
        }
        catch { }
        _browserEventSource = null;
    }

    private void ReleaseBrowserForBackgroundIfEnabled()
    {
        if (!_settings.PerformanceReleaseWebViewInBackground || _browserReleasedForBackground || Browser == null)
            return;

        DetachBrowserEvents();
        _browserReady = false;
        try
        {
            BrowserHost.Children.Clear();
            (Browser as IDisposable)?.Dispose();
        }
        catch { }
        Browser = null!;
        _browserReleasedForBackground = true;
    }

    private async Task RestoreBrowserIfNeededAsync()
    {
        if (!_browserReleasedForBackground)
            return;

        await EnsureBrowserAsync();
        SendFullState();
    }

    private void Window_StateChanged(object? sender, EventArgs e)
    {
        if (WindowState == WindowState.Minimized)
        {
            _runtime.SetBackgroundMode(true);
            return;
        }

        if (IsVisible && ShowInTaskbar)
            _runtime.SetBackgroundMode(false);
    }

    public async Task HandleCommand(WebCommand command)
    {
        switch (command.Command)
        {
            case "ready":
                SendFullState();
                break;
            case "setMode":
                await SetModeAsync(command.Mode ?? "monitor");
                break;
            case "setSpeed":
                await SetManualSpeedAsync(command.Speed ?? _settings.FanControlManualSpeed, command.ForceMode ?? true);
                break;
            case "saveIp":
                _settings.FanControlDeviceIp = command.Ip?.Trim() ?? _settings.FanControlDeviceIp;
                _settings.Save();
                await _runtime.ForceApplyAsync();
                break;
            case "test":
                _settings.FanControlDeviceIp = command.Ip?.Trim() ?? _settings.FanControlDeviceIp;
                _settings.Save();
                _runtime.TestConnection();
                break;
            case "setSource":
                _settings.FanControlTemperatureSource = command.Source is "gpu" or "max" ? command.Source : "cpu";
                _settings.Save();
                await _runtime.ForceApplyAsync();
                break;
            case "setTemperatureSensor":
                SetTemperatureSensor(command.SensorKind, command.SensorId);
                _settings.Save();
                await _runtime.ForceApplyAsync();
                break;
            case "setCurve":
                if (FanControlService.TryNormalizeCurve(command.Curve, out var normalizedCurve, out var curveError))
                {
                    _settings.FanControlCurve = normalizedCurve;
                    _settings.Save();
                    await _runtime.ForceApplyAsync();
                }
                else
                {
                    PostToMain(new { type = "notice", message = curveError });
                    SendFullState();
                }
                break;
            case "resetCurve":
                _settings.FanControlCurve = AppSettings.DefaultCurve;
                _settings.Save();
                await _runtime.ForceApplyAsync();
                break;
            case "saveConfig":
                SaveConfig();
                break;
            case "loadConfig":
                await LoadConfig();
                break;
            case "toggleStartMinimized":
                _settings.StartMinimized = command.Value ?? false;
                _settings.Save();
                SendFullState();
                break;
            case "toggleStartWithWindows":
                SetStartWithWindows(command.Value ?? false);
                SendFullState();
                break;
            case "toggleCloseToTray":
                _settings.CloseToTray = command.Value ?? true;
                _settings.Save();
                SendFullState();
                break;
            case "toggleReleaseWebViewInBackground":
                _settings.PerformanceReleaseWebViewInBackground = command.Value ?? false;
                _settings.Save();
                SendFullState();
                break;
            case "toggleTrimWorkingSetInBackground":
                _settings.PerformanceTrimWorkingSetInBackground = command.Value ?? false;
                _settings.Save();
                SendFullState();
                break;
            case "setNavigationPlacement":
                _settings.UiNavigationPlacement = command.Placement == "side" ? "side" : "top";
                _settings.Save();
                SendFullState();
                break;
            case "showWindow":
                ShowFromTray();
                break;
            case "exit":
                _allowExit = true;
                Close();
                break;
            case "windowMinimize":
                WindowState = WindowState.Minimized;
                break;
            case "windowToggleMaximize":
                WindowState = WindowState == WindowState.Maximized ? WindowState.Normal : WindowState.Maximized;
                break;
            case "windowClose":
                Close();
                break;
        }
    }

    private async Task SetModeAsync(string mode)
    {
        _settings.FanControlEnabled = true;
        _settings.FanControlMode = FanControlService.NormalizeMode(mode);
        _settings.Save();
        SendFullState();
        await _runtime.ForceApplyAsync();
    }

    private async Task SetManualSpeedAsync(int speed, bool forceMode)
    {
        _settings.FanControlManualSpeed = Math.Clamp(speed, 0, 100);
        if (forceMode)
            _settings.FanControlMode = "manual";
        _settings.Save();
        SendFullState();
        await _runtime.ForceApplyAsync();
    }

    private void SaveConfig()
    {
        var dialog = new Microsoft.Win32.SaveFileDialog
        {
            Title = "\u4fdd\u5b58\u98ce\u6247\u914d\u7f6e",
            Filter = "JSON \u914d\u7f6e|*.json",
            FileName = "fan-control-profile.json"
        };

        if (dialog.ShowDialog(this) == true)
            _settings.SaveTo(dialog.FileName);
    }

    private async Task LoadConfig()
    {
        var dialog = new Microsoft.Win32.OpenFileDialog
        {
            Title = "\u8bfb\u53d6\u98ce\u6247\u914d\u7f6e",
            Filter = "JSON \u914d\u7f6e|*.json"
        };

        if (dialog.ShowDialog(this) != true) return;
        var loaded = AppSettings.LoadFrom(dialog.FileName);
        if (loaded == null) return;
        _settings.CopyFrom(loaded);
        _settings.Save();
        SendFullState();
        await _runtime.ForceApplyAsync();
    }

    private void SendFullState()
    {
        var snapshot = _lastSnapshot ?? new FanRuntimeSnapshot(_runtime.CurrentHardware, _runtime.CurrentStatus, _runtime.HardwareError);
        var payload = BuildState(snapshot);
        PostToMain(payload);
        UpdateTrayModeItems(_settings.FanControlMode);
        SetTrayToolTip($"\u98ce\u6247 {ValueOrDash(snapshot.Status.CurrentSpeed)}% | {ModeLabel(snapshot.Status.Mode)} | \u76ee\u6807 {snapshot.Status.TargetSpeed}%");
    }

    private object BuildState(FanRuntimeSnapshot snapshot)
    {
        var hardware = snapshot.Hardware;
        var status = snapshot.Status;
        var hardwareMessage = !hardware.CpuTemperature.HasValue && !hardware.GpuTemperature.HasValue && !string.IsNullOrWhiteSpace(hardware.Diagnostic)
            ? hardware.Diagnostic
            : null;

        return new
        {
            type = "state",
            settings = new
            {
                ip = _settings.FanControlDeviceIp,
                mode = FanControlService.NormalizeMode(_settings.FanControlMode),
                source = _settings.FanControlTemperatureSource,
                cpuSensorId = _settings.FanControlCpuTemperatureSensorId,
                gpuSensorId = _settings.FanControlGpuTemperatureSensorId,
                manualSpeed = _settings.FanControlManualSpeed,
                curve = _settings.FanControlCurve,
                startMinimized = _settings.StartMinimized,
                startWithWindows = _settings.StartWithWindows,
                closeToTray = _settings.CloseToTray,
                releaseWebViewInBackground = _settings.PerformanceReleaseWebViewInBackground,
                trimWorkingSetInBackground = _settings.PerformanceTrimWorkingSetInBackground,
                navigationPlacement = _settings.UiNavigationPlacement
            },
            hardware = new
            {
                cpuTemp = hardware.CpuTemperature,
                cpuSensor = hardware.CpuSensorName,
                gpuTemp = hardware.GpuTemperature,
                gpuSensor = hardware.GpuSensorName,
                cpuSensors = hardware.CpuTemperatureSensors,
                gpuSensors = hardware.GpuTemperatureSensors,
                sensorCount = hardware.TemperatureSensorCount,
                diagnostic = hardware.Diagnostic
            },
            fan = new
            {
                online = status.Online,
                current = status.CurrentSpeed,
                target = status.TargetSpeed,
                deviceTemp = status.DeviceTemperature,
                deviceTarget = status.DeviceTargetSpeed,
                deviceMode = status.DeviceControlMode,
                wifiControl = status.DeviceWifiControl,
                controlTemp = SelectControlTemperature(hardware),
                smoothedTemp = status.SmoothedTemperature,
                mode = status.Mode,
                message = TranslateStatus(snapshot.HardwareError != null ? "\u786c\u4ef6\u8bfb\u53d6\u4e0d\u53ef\u7528\uff1a" + snapshot.HardwareError : hardwareMessage ?? status.Message)
            }
        };
    }

    private void SetTemperatureSensor(string? sensorKind, string? sensorId)
    {
        sensorId = string.IsNullOrWhiteSpace(sensorId) ? "auto" : sensorId.Trim();
        if (string.Equals(sensorKind, "gpu", StringComparison.OrdinalIgnoreCase))
        {
            _settings.FanControlGpuTemperatureSensorId = sensorId;
            return;
        }

        _settings.FanControlCpuTemperatureSensorId = sensorId;
    }

    private void PostToMain(object payload)
    {
        if (!_browserReady || Browser == null || Browser.CoreWebView2 == null) return;
        Browser.CoreWebView2.PostWebMessageAsJson(JsonSerializer.Serialize(payload, JsonOptions));
    }

    private float? SelectControlTemperature(HardwareSnapshot hardware)
    {
        return (_settings.FanControlTemperatureSource ?? "cpu") switch
        {
            "gpu" => hardware.GpuTemperature ?? hardware.CpuTemperature,
            "max" when hardware.CpuTemperature.HasValue && hardware.GpuTemperature.HasValue => Math.Max(hardware.CpuTemperature.Value, hardware.GpuTemperature.Value),
            "max" => hardware.CpuTemperature ?? hardware.GpuTemperature,
            _ => hardware.CpuTemperature ?? hardware.GpuTemperature
        };
    }

    private void InitializeTrayIcon()
    {
        _trayMenu = new Forms.ContextMenuStrip
        {
            ShowImageMargin = true,
            BackColor = Drawing.Color.White,
            ForeColor = Drawing.Color.Black,
            Font = new Drawing.Font("Microsoft YaHei UI", 10F),
            Padding = new Forms.Padding(4, 7, 6, 7),
            MinimumSize = new Drawing.Size(260, 0),
            RenderMode = Forms.ToolStripRenderMode.Professional,
            Renderer = new Forms.ToolStripProfessionalRenderer(new PlainTrayMenuColorTable())
        };
        _trayMenu.Opening += (_, _) => UpdateTrayModeItems(_settings.FanControlMode);

        _trayMenu.Items.Add(CreateTrayMenuItem("打开完整面板", ShowFromTray));
        _trayMenu.Items.Add(CreateTraySeparator());
        _trayMenu.Items.Add(CreateTrayModeMenuItem("monitor", "只监控"));
        _trayMenu.Items.Add(CreateTrayModeMenuItem("manual", "手动"));
        _trayMenu.Items.Add(CreateTrayModeMenuItem("auto", "自动"));
        _trayMenu.Items.Add(CreateTrayModeMenuItem("off", "关闭"));
        _trayMenu.Items.Add(CreateTraySeparator());
        _trayMenu.Items.Add(CreateTrayMenuItem("退出", () =>
        {
            _allowExit = true;
            Close();
        }));

        _trayIcon = new Forms.NotifyIcon
        {
            Icon = LoadTrayIcon(),
            Text = "风扇控制便携版",
            ContextMenuStrip = _trayMenu,
            Visible = true
        };
        _trayIcon.MouseUp += TrayIcon_MouseUp;
        UpdateTrayModeItems(_settings.FanControlMode);
    }

    private void DisposeTrayIcon()
    {
        if (_trayIcon != null)
        {
            _trayIcon.MouseUp -= TrayIcon_MouseUp;
            _trayIcon.Visible = false;
            _trayIcon.Dispose();
            _trayIcon = null;
        }

        _trayMenu?.Dispose();
        _trayMenu = null;
    }

    private void TrayIcon_MouseUp(object? sender, Forms.MouseEventArgs e)
    {
        if (e.Button == Forms.MouseButtons.Left)
            Dispatcher.Invoke(ShowFromTray);
    }

    private static Forms.ToolStripSeparator CreateTraySeparator() => new()
    {
        AutoSize = false,
        Height = 9,
        Margin = new Forms.Padding(0, 3, 0, 3),
        BackColor = Drawing.Color.White,
        ForeColor = Drawing.Color.FromArgb(209, 213, 219)
    };

    private Forms.ToolStripMenuItem CreateTrayMenuItem(string text, Action action)
    {
        var item = CreateTrayMenuItemBase(text);
        item.Click += (_, _) => Dispatcher.Invoke(action);
        return item;
    }

    private Forms.ToolStripMenuItem CreateTrayMenuItem(string text, Func<Task> action)
    {
        var item = CreateTrayMenuItemBase(text);
        item.Click += (_, _) =>
        {
            Dispatcher.InvokeAsync(async () =>
            {
                try
                {
                    await action();
                }
                catch (Exception ex)
                {
                    PostToMain(new { type = "notice", message = ex.Message });
                }
            });
        };
        return item;
    }

    private Forms.ToolStripMenuItem CreateTrayModeMenuItem(string mode, string text)
    {
        var item = CreateTrayMenuItem(text, () => SetModeAsync(mode));
        item.CheckOnClick = false;
        _trayModeItems[mode] = item;
        return item;
    }

    private static Forms.ToolStripMenuItem CreateTrayMenuItemBase(string text) => new(text)
    {
        AutoSize = false,
        Width = 244,
        Height = 40,
        Padding = new Forms.Padding(8, 0, 12, 0),
        Margin = new Forms.Padding(0, 1, 0, 1),
        BackColor = Drawing.Color.White,
        ForeColor = Drawing.Color.Black,
        DisplayStyle = Forms.ToolStripItemDisplayStyle.Text,
        TextAlign = Drawing.ContentAlignment.MiddleLeft
    };

    private static Drawing.Icon LoadTrayIcon()
    {
        var processPath = Environment.ProcessPath;
        if (!string.IsNullOrWhiteSpace(processPath))
        {
            var icon = Drawing.Icon.ExtractAssociatedIcon(processPath);
            if (icon != null) return icon;
        }

        return Drawing.SystemIcons.Application;
    }

    private void SetTrayToolTip(string text)
    {
        if (_trayIcon == null) return;
        _trayIcon.Text = text.Length <= 63 ? text : text[..60] + "...";
    }

    private void UpdateTrayModeItems(string? mode)
    {
        var normalized = FanControlService.NormalizeMode(mode);
        foreach (var (name, item) in _trayModeItems)
        {
            item.Checked = string.Equals(name, normalized, StringComparison.OrdinalIgnoreCase);
        }
    }

    private void HideToTray()
    {
        _runtime.SetBackgroundMode(true);
        Hide();
        ShowInTaskbar = false;
        ReleaseBrowserForBackgroundIfEnabled();
        if (_settings.PerformanceTrimWorkingSetInBackground)
            _ = Task.Run(WorkingSetTrimmer.TrimCurrentProcess);
    }

    public void ShowFromTray()
    {
        _runtime.SetBackgroundMode(false);
        ShowInTaskbar = true;
        Show();
        WindowState = WindowState.Normal;
        Activate();
        _ = Dispatcher.InvokeAsync(async () => await RestoreBrowserIfNeededAsync());
    }

    private static string ValueOrDash(int? value) => value.HasValue ? value.Value.ToString() : "--";

    private static string ModeLabel(string? mode) => FanControlService.NormalizeMode(mode) switch
    {
        "manual" => "\u624b\u52a8",
        "auto" => "\u81ea\u52a8",
        "off" => "\u5173\u95ed",
        _ => "\u53ea\u76d1\u63a7"
    };

    private void SetStartWithWindows(bool enabled)
    {
        try
        {
            ApplyStartupRegistration(enabled);
            _settings.StartWithWindows = enabled;
            _settings.Save();
        }
        catch (Exception ex)
        {
            _settings.StartWithWindows = false;
            _settings.Save();
            PostToMain(new { type = "notice", message = "\u5f00\u673a\u81ea\u542f\u8bbe\u7f6e\u5931\u8d25\uff1a" + ex.Message });
        }
    }

    private void SyncStartupRegistrationWithSetting()
    {
        try
        {
            ApplyStartupRegistration(_settings.StartWithWindows);
        }
        catch
        {
            _settings.StartWithWindows = false;
            _settings.Save();
        }
    }

    private static void ApplyStartupRegistration(bool enabled)
    {
        using var key = Registry.CurrentUser.OpenSubKey(StartupRegistryKeyPath, writable: true)
            ?? Registry.CurrentUser.CreateSubKey(StartupRegistryKeyPath, writable: true)
            ?? throw new InvalidOperationException("\u65e0\u6cd5\u6253\u5f00 Windows \u81ea\u542f\u6ce8\u518c\u8868\u9879\u3002");

        if (enabled)
        {
            key.SetValue(StartupRegistryValueName, QuoteCommandPath(StartupExecutablePath()), RegistryValueKind.String);
            return;
        }

        key.DeleteValue(StartupRegistryValueName, throwOnMissingValue: false);
    }

    private static string StartupExecutablePath()
    {
        var appPath = Path.Combine(AppContext.BaseDirectory, "FanControlPortable.exe");
        if (File.Exists(appPath))
            return Path.GetFullPath(appPath);

        return string.IsNullOrWhiteSpace(Environment.ProcessPath)
            ? Path.GetFullPath(appPath)
            : Path.GetFullPath(Environment.ProcessPath);
    }

    private static string QuoteCommandPath(string path) => "\"" + path + "\"";

    private static string TranslateStatus(string? message)
    {
        if (string.IsNullOrWhiteSpace(message)) return "\u51c6\u5907\u5c31\u7eea";
        return message switch
        {
            "OK" => "\u72b6\u6001\u6b63\u5e38",
            "Speed command sent" => "\u8f6c\u901f\u547d\u4ee4\u5df2\u4e0b\u53d1",
            "Device connected" => "\u8bbe\u5907\u5df2\u8fde\u63a5",
            "Waiting for temperature data" => "\u7b49\u5f85\u6e29\u5ea6\u6570\u636e",
            "Fan control disabled" => "\u98ce\u6247\u63a7\u5236\u672a\u542f\u7528",
            _ => message
        };
    }

    private sealed class PlainTrayMenuColorTable : Forms.ProfessionalColorTable
    {
        public override Drawing.Color ToolStripDropDownBackground => Drawing.Color.White;
        public override Drawing.Color ImageMarginGradientBegin => Drawing.Color.White;
        public override Drawing.Color ImageMarginGradientMiddle => Drawing.Color.White;
        public override Drawing.Color ImageMarginGradientEnd => Drawing.Color.White;
        public override Drawing.Color MenuBorder => Drawing.Color.FromArgb(209, 213, 219);
        public override Drawing.Color MenuItemBorder => Drawing.Color.FromArgb(147, 197, 253);
        public override Drawing.Color MenuItemSelected => Drawing.Color.FromArgb(219, 234, 254);
        public override Drawing.Color SeparatorDark => Drawing.Color.FromArgb(209, 213, 219);
        public override Drawing.Color SeparatorLight => Drawing.Color.White;
    }
}

public sealed class WebCommand
{
    public string? Command { get; set; }
    public string? Mode { get; set; }
    public string? Source { get; set; }
    public string? Curve { get; set; }
    public string? Ip { get; set; }
    public int? Speed { get; set; }
    public bool? ForceMode { get; set; }
    public bool? Value { get; set; }
    public string? SensorKind { get; set; }
    public string? SensorId { get; set; }
    public string? Placement { get; set; }
}
