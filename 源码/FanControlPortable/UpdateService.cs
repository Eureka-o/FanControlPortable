using System.Net.Http;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace FanControlPortable;

internal sealed class UpdateService
{
    private readonly HttpClient _http = new()
    {
        Timeout = TimeSpan.FromSeconds(6)
    };

    public async Task<UpdateCheckResult> CheckLatestAsync(CancellationToken cancellationToken = default)
    {
        using var request = new HttpRequestMessage(
            HttpMethod.Get,
            $"https://api.github.com/repos/{AppInfo.RepositoryOwner}/{AppInfo.RepositoryName}/releases/latest");
        request.Headers.UserAgent.ParseAdd("FanControlPortable/" + AppInfo.Version);
        request.Headers.Accept.ParseAdd("application/vnd.github+json");

        using var response = await _http.SendAsync(request, cancellationToken);
        var body = await response.Content.ReadAsStringAsync();
        if (!response.IsSuccessStatusCode)
        {
            return UpdateCheckResult.Failed($"GitHub 返回 HTTP {(int)response.StatusCode}，可稍后重试或手动打开发布页。", AppInfo.ReleasesUrl);
        }

        var release = JsonSerializer.Deserialize<GitHubRelease>(body);
        if (release == null)
            return UpdateCheckResult.Failed("没有读到发布版本信息。", AppInfo.ReleasesUrl);

        var latestVersionText = NormalizeVersionText(release.TagName ?? release.Name);
        var hasNewer = TryParseVersion(latestVersionText, out var latestVersion) &&
                       TryParseVersion(AppInfo.Version, out var currentVersion) &&
                       latestVersion.CompareTo(currentVersion) > 0;
        var asset = SelectAsset(release.Assets);
        var downloadUrl = asset?.BrowserDownloadUrl ?? release.HtmlUrl ?? AppInfo.ReleasesUrl;
        var notes = string.IsNullOrWhiteSpace(release.Body) ? "暂无更新说明。" : release.Body!.Trim();

        return new UpdateCheckResult(
            Success: true,
            HasUpdate: hasNewer,
            CurrentVersion: AppInfo.Version,
            LatestVersion: string.IsNullOrWhiteSpace(latestVersionText) ? "unknown" : latestVersionText,
            Message: hasNewer ? "发现新版本，可下载对应安装包。" : "当前已是最新版本。",
            DownloadUrl: downloadUrl,
            Notes: notes,
            AssetName: asset?.Name ?? "");
    }

    private static GitHubAsset? SelectAsset(IReadOnlyList<GitHubAsset>? assets)
    {
        if (assets == null || assets.Count == 0)
            return null;

        return assets.FirstOrDefault(asset => string.Equals(asset.Name, AppInfo.PackageFileName, StringComparison.OrdinalIgnoreCase)) ??
               assets.FirstOrDefault(asset => asset.Name.EndsWith(".zip", StringComparison.OrdinalIgnoreCase));
    }

    private static string NormalizeVersionText(string? text)
    {
        text = (text ?? "").Trim();
        return text.StartsWith("v", StringComparison.OrdinalIgnoreCase) ? text.Substring(1) : text;
    }

    private static bool TryParseVersion(string? text, out Version version)
    {
        version = new Version(0, 0, 0);
        if (string.IsNullOrWhiteSpace(text))
            return false;

        var cleaned = new string(text.TakeWhile(ch => char.IsDigit(ch) || ch == '.').ToArray());
        var parsed = Version.TryParse(cleaned, out var parsedVersion);
        version = parsedVersion ?? new Version(0, 0, 0);
        return parsed;
    }
}

internal sealed record UpdateCheckResult(
    bool Success,
    bool HasUpdate,
    string CurrentVersion,
    string LatestVersion,
    string Message,
    string DownloadUrl,
    string Notes,
    string AssetName)
{
    public static UpdateCheckResult Failed(string message, string url) =>
        new(false, false, AppInfo.Version, "", message, url, "", "");
}

internal sealed class GitHubRelease
{
    [JsonPropertyName("tag_name")] public string? TagName { get; set; }
    [JsonPropertyName("name")] public string? Name { get; set; }
    [JsonPropertyName("html_url")] public string? HtmlUrl { get; set; }
    [JsonPropertyName("body")] public string? Body { get; set; }
    [JsonPropertyName("assets")] public List<GitHubAsset>? Assets { get; set; }
}

internal sealed class GitHubAsset
{
    [JsonPropertyName("name")] public string Name { get; set; } = "";
    [JsonPropertyName("browser_download_url")] public string? BrowserDownloadUrl { get; set; }
}
