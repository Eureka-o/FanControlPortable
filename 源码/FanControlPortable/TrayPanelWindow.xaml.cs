using System.Text.Encodings.Web;
using System.Text.Json;
using System.Windows;
using Microsoft.Web.WebView2.Core;
using Forms = System.Windows.Forms;

namespace FanControlPortable;

public partial class TrayPanelWindow : Window
{
    private static readonly JsonSerializerOptions JsonOptions = new()
    {
        PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
        Encoder = JavaScriptEncoder.UnsafeRelaxedJsonEscaping
    };

    private readonly MainWindow _owner;
    private object? _pendingState;
    private bool _ready;
    private bool _navigationHooked;

    public TrayPanelWindow(MainWindow owner)
    {
        _owner = owner;
        InitializeComponent();
    }

    private async void Window_Loaded(object sender, RoutedEventArgs e)
    {
        await Browser.EnsureCoreWebView2Async();
        Browser.ZoomFactor = 0.9;
        Browser.CoreWebView2.Settings.AreDefaultContextMenusEnabled = false;
        Browser.CoreWebView2.Settings.AreDevToolsEnabled = false;
        Browser.CoreWebView2.WebMessageReceived += Browser_WebMessageReceived;
        if (!_navigationHooked)
        {
            Browser.CoreWebView2.NavigationCompleted += Browser_NavigationCompleted;
            _navigationHooked = true;
        }
        Browser.NavigateToString(WebUi.TrayHtml);
    }

    private void Browser_NavigationCompleted(object? sender, CoreWebView2NavigationCompletedEventArgs e)
    {
        _ready = true;
        if (_pendingState != null) UpdateState(_pendingState);
    }

    private async void Browser_WebMessageReceived(object? sender, CoreWebView2WebMessageReceivedEventArgs e)
    {
        var command = JsonSerializer.Deserialize<WebCommand>(e.WebMessageAsJson, JsonOptions);
        if (command == null) return;
        await _owner.HandleCommand(command);
        if (command.Command is "showWindow" or "exit")
            Hide();
    }

    public void ShowNearCursor()
    {
        var cursor = Forms.Cursor.Position;
        var screen = Forms.Screen.FromPoint(cursor).WorkingArea;
        Left = Math.Min(Math.Max(screen.Left, cursor.X - Width + 18), screen.Right - Width);
        Top = Math.Min(Math.Max(screen.Top, cursor.Y - Height + 18), screen.Bottom - Height);
        Show();
        Activate();
    }

    public void UpdateState(object state)
    {
        _pendingState = state;
        if (!_ready || Browser.CoreWebView2 == null) return;
        Browser.CoreWebView2.PostWebMessageAsJson(JsonSerializer.Serialize(state, JsonOptions));
    }

    private void Window_Deactivated(object? sender, EventArgs e)
    {
        Hide();
    }
}
