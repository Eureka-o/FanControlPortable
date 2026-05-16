namespace FanControlPortable;

internal static class AppInfo
{
    public const string Version = "1.1.0";
    public const string RepositoryOwner = "Eureka-o";
    public const string RepositoryName = "FanControlPortable";
    public const string ReleasesUrl = "https://github.com/Eureka-o/FanControlPortable/releases";

#if EDITION_LITE
    public const string Edition = "Lite";
    public const string PackageFileName = "FanControlPortable-lite.zip";
#elif EDITION_STANDARD
    public const string Edition = "Standard";
    public const string PackageFileName = "FanControlPortable.zip";
#else
    public const string Edition = "Compat";
    public const string PackageFileName = "FanControlPortable-compat.zip";
#endif

    public static string DisplayName => Edition == "Standard"
        ? "FanControlPortable"
        : $"FanControlPortable {Edition}";
}
