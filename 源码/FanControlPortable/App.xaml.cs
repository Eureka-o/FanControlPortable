using System.Diagnostics;
using System.Security.Principal;
using System.Windows;

namespace FanControlPortable;

public partial class App : System.Windows.Application
{
    private SingleInstanceCoordinator? _singleInstance;

    protected override void OnStartup(StartupEventArgs e)
    {
        base.OnStartup(e);

        if (!IsAdministrator() && e.Args.All(arg => arg != "--no-elevate"))
        {
            try
            {
                Process.Start(new ProcessStartInfo
                {
                    FileName = PlatformCompat.CurrentProcessPath(),
                    UseShellExecute = true,
                    Verb = "runas",
                    WorkingDirectory = AppContext.BaseDirectory
                });
                Shutdown();
                return;
            }
            catch
            {
                System.Windows.MessageBox.Show(
                    "未以管理员权限启动，部分 CPU 温度可能读取不到。",
                    "风扇控制便携版",
                    MessageBoxButton.OK,
                    MessageBoxImage.Warning);
            }
        }

        if (!SingleInstanceCoordinator.TryBecomePrimary(out _singleInstance, out var singleInstanceMessage))
        {
            if (!string.IsNullOrWhiteSpace(singleInstanceMessage) &&
                singleInstanceMessage.Contains("未响应", StringComparison.OrdinalIgnoreCase))
            {
                System.Windows.MessageBox.Show(singleInstanceMessage, "风扇控制便携版", MessageBoxButton.OK, MessageBoxImage.Warning);
            }

            Shutdown();
            return;
        }

        if (!PawnIoGuard.IsInstalled())
        {
            var notice = new DriverNoticeWindow();
            notice.ShowDialog();
        }

        if (!WebView2RuntimeGuard.IsInstalled())
        {
            var message = WebView2RuntimeGuard.HasBundledInstaller()
                ? "当前系统缺少 Microsoft Edge WebView2 Runtime，主界面无法加载。\n\n是否现在运行随包安装程序？安装完成后请重新启动本软件。"
                : "当前系统缺少 Microsoft Edge WebView2 Runtime，主界面无法加载。\n\n随包安装程序未找到，请先安装 WebView2 Runtime 后再启动本软件。";
            var result = System.Windows.MessageBox.Show(
                message,
                "缺少 WebView2 运行时",
                WebView2RuntimeGuard.HasBundledInstaller() ? MessageBoxButton.YesNo : MessageBoxButton.OK,
                MessageBoxImage.Warning);

            if (result == MessageBoxResult.Yes)
            {
                try
                {
                    WebView2RuntimeGuard.LaunchBundledInstaller();
                }
                catch (Exception ex)
                {
                    System.Windows.MessageBox.Show("打开 WebView2 安装包失败：" + ex.Message, "WebView2", MessageBoxButton.OK, MessageBoxImage.Warning);
                }
            }

            Shutdown();
            return;
        }

        var settings = AppSettings.Load();
        var window = new MainWindow(settings);
        MainWindow = window;
        window.Show();
        _singleInstance?.StartServer(window.ShowFromTray);
    }

    protected override void OnExit(ExitEventArgs e)
    {
        _singleInstance?.Dispose();
        base.OnExit(e);
    }

    private static bool IsAdministrator()
    {
        using var identity = WindowsIdentity.GetCurrent();
        var principal = new WindowsPrincipal(identity);
        return principal.IsInRole(WindowsBuiltInRole.Administrator);
    }
}
