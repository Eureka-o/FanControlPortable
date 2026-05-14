using System.Diagnostics;
using System.IO;
using Microsoft.Web.WebView2.Core;

namespace FanControlPortable;

public static class WebView2RuntimeGuard
{
    public static bool IsInstalled()
    {
        try
        {
            _ = CoreWebView2Environment.GetAvailableBrowserVersionString();
            return true;
        }
        catch
        {
            return false;
        }
    }

    public static bool HasBundledInstaller() => File.Exists(BundledInstallerPath);

    public static void LaunchBundledInstaller()
    {
        if (!HasBundledInstaller())
            throw new FileNotFoundException("未找到 Microsoft Edge WebView2 Runtime 安装包。", BundledInstallerPath);

        Process.Start(new ProcessStartInfo
        {
            FileName = BundledInstallerPath,
            Arguments = "/silent /install",
            UseShellExecute = true,
            Verb = "runas",
            WindowStyle = ProcessWindowStyle.Normal
        });
    }

    public static string BundledInstallerPath =>
        Path.Combine(AppContext.BaseDirectory, "Resources", "assets", "MicrosoftEdgeWebview2Setup.exe");
}
