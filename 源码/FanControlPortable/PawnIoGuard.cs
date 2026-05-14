using System.Diagnostics;
using System.IO;
using Microsoft.Win32;

namespace FanControlPortable;

public static class PawnIoGuard
{
    public static bool IsInstalled()
    {
        try
        {
            const string uninstallPath = @"SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\PawnIO";
            using var x64 = RegistryKey.OpenBaseKey(RegistryHive.LocalMachine, RegistryView.Registry64).OpenSubKey(uninstallPath);
            if (x64 != null) return true;

            using var x86 = RegistryKey.OpenBaseKey(RegistryHive.LocalMachine, RegistryView.Registry32).OpenSubKey(uninstallPath);
            if (x86 != null) return true;

            using var svc = Registry.LocalMachine.OpenSubKey(@"SYSTEM\CurrentControlSet\Services\PawnIO");
            return svc != null;
        }
        catch
        {
            return false;
        }
    }

    public static bool HasBundledInstaller() => File.Exists(BundledInstallerPath);

    public static void LaunchBundledInstaller(bool silent)
    {
        if (!HasBundledInstaller())
            throw new FileNotFoundException("未找到 PawnIO 安装包", BundledInstallerPath);

        Process.Start(new ProcessStartInfo
        {
            FileName = BundledInstallerPath,
            Arguments = silent ? "-install -silent" : "",
            UseShellExecute = true,
            Verb = "runas",
            WindowStyle = silent ? ProcessWindowStyle.Hidden : ProcessWindowStyle.Normal
        });
    }

    public static string BundledInstallerPath =>
        Path.Combine(AppContext.BaseDirectory, "resources", "assets", "PawnIO_setup.exe");
}
