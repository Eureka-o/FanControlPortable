using System.Windows;

namespace FanControlPortable;

public partial class DriverNoticeWindow : Window
{
    public DriverNoticeWindow()
    {
        InitializeComponent();
        InstallerPathText.Text = PawnIoGuard.HasBundledInstaller()
            ? "安装包：" + PawnIoGuard.BundledInstallerPath
            : "未找到随包安装包 resources\\assets\\PawnIO_setup.exe";
        InstallButton.IsEnabled = PawnIoGuard.HasBundledInstaller();
    }

    private void InstallButton_Click(object sender, RoutedEventArgs e)
    {
        try
        {
            PawnIoGuard.LaunchBundledInstaller(false);
            Close();
        }
        catch (Exception ex)
        {
            System.Windows.MessageBox.Show("打开安装包失败：" + ex.Message, "PawnIO", MessageBoxButton.OK, MessageBoxImage.Warning);
        }
    }

    private void LaterButton_Click(object sender, RoutedEventArgs e)
    {
        Close();
    }
}
