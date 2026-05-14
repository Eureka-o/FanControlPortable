using System.Diagnostics;
using System.IO;
using System.IO.Pipes;
using System.Text;
using System.Threading;

namespace FanControlPortable;

public sealed class SingleInstanceCoordinator : IDisposable
{
    private const string MutexName = @"Global\FanControlPortable.SingleInstance";
    private const string PipeName = "FanControlPortable.SingleInstancePipe";

    private readonly Mutex _mutex;
    private CancellationTokenSource? _serverCts;
    private Task? _serverTask;

    private SingleInstanceCoordinator(Mutex mutex)
    {
        _mutex = mutex;
    }

    public static bool TryBecomePrimary(out SingleInstanceCoordinator? coordinator, out string message)
    {
        coordinator = null;
        message = "";

        if (TrySendCommand("show", 700))
        {
            message = "已打开正在后台运行的窗口";
            return false;
        }

        KillExistingProcesses();

        var mutex = new Mutex(true, MutexName, out var createdNew);
        if (!createdNew)
        {
            Thread.Sleep(800);
            mutex.Dispose();
            mutex = new Mutex(true, MutexName, out createdNew);
        }

        if (!createdNew)
        {
            mutex.Dispose();
            if (TrySendCommand("show", 700))
            {
                message = "已打开正在后台运行的窗口";
                return false;
            }

            message = "已有后台实例仍在运行，且未响应唤醒请求";
            return false;
        }

        coordinator = new SingleInstanceCoordinator(mutex);
        return true;
    }

    public void StartServer(Action showWindow)
    {
        _serverCts = new CancellationTokenSource();
        _serverTask = Task.Run(() => ServerLoopAsync(showWindow, _serverCts.Token));
    }

    private static async Task ServerLoopAsync(Action showWindow, CancellationToken cancellationToken)
    {
        while (!cancellationToken.IsCancellationRequested)
        {
            try
            {
                using var pipe = new NamedPipeServerStream(
                    PipeName,
                    PipeDirection.In,
                    1,
                    PipeTransmissionMode.Byte,
                    PipeOptions.Asynchronous);

                await pipe.WaitForConnectionAsync(cancellationToken);
                using var reader = new StreamReader(pipe, Encoding.UTF8);
                var command = await reader.ReadLineAsync(cancellationToken);
                if (string.Equals(command, "show", StringComparison.OrdinalIgnoreCase))
                {
                    System.Windows.Application.Current.Dispatcher.Invoke(showWindow);
                }
            }
            catch (OperationCanceledException)
            {
                break;
            }
            catch
            {
                await Task.Delay(300, cancellationToken).ContinueWith(_ => { }, TaskScheduler.Default);
            }
        }
    }

    private static bool TrySendCommand(string command, int timeoutMilliseconds)
    {
        try
        {
            using var pipe = new NamedPipeClientStream(".", PipeName, PipeDirection.Out);
            pipe.Connect(timeoutMilliseconds);
            using var writer = new StreamWriter(pipe, Encoding.UTF8) { AutoFlush = true };
            writer.WriteLine(command);
            return true;
        }
        catch
        {
            return false;
        }
    }

    private static void KillExistingProcesses()
    {
        var current = Process.GetCurrentProcess();
        var currentDirectory = NormalizeDirectory(Path.GetDirectoryName(PlatformCompat.CurrentProcessPath()) ?? AppContext.BaseDirectory);

        foreach (var process in Process.GetProcesses())
        {
            if (process.Id == current.Id)
            {
                process.Dispose();
                continue;
            }

            if (!IsLikelySameAppProcess(process, current.ProcessName, currentDirectory))
            {
                process.Dispose();
                continue;
            }

            try
            {
                process.Kill(entireProcessTree: true);
                process.WaitForExit(3000);
            }
            catch
            {
                try
                {
                    process.CloseMainWindow();
                    process.WaitForExit(1500);
                }
                catch { }
            }
            finally
            {
                process.Dispose();
            }
        }
    }

    private static bool IsLikelySameAppProcess(Process process, string currentProcessName, string currentDirectory)
    {
        var processName = process.ProcessName;
        if (string.Equals(processName, currentProcessName, StringComparison.OrdinalIgnoreCase))
            return true;

        var executablePath = TryGetExecutablePath(process);
        if (!string.IsNullOrWhiteSpace(executablePath))
        {
            var fileName = Path.GetFileNameWithoutExtension(executablePath);
            var directory = NormalizeDirectory(Path.GetDirectoryName(executablePath) ?? "");
            if (fileName.StartsWith("FanControlPortable", StringComparison.OrdinalIgnoreCase) &&
                string.Equals(directory, currentDirectory, StringComparison.OrdinalIgnoreCase))
            {
                return true;
            }
        }

        return processName.StartsWith("FanControlPortable", StringComparison.OrdinalIgnoreCase);
    }

    private static string? TryGetExecutablePath(Process process)
    {
        try { return process.MainModule?.FileName; }
        catch { return null; }
    }

    private static string NormalizeDirectory(string directory)
    {
        try
        {
            return Path.GetFullPath(directory).TrimEnd(Path.DirectorySeparatorChar, Path.AltDirectorySeparatorChar);
        }
        catch
        {
            return directory.TrimEnd(Path.DirectorySeparatorChar, Path.AltDirectorySeparatorChar);
        }
    }

    public void Dispose()
    {
        try
        {
            _serverCts?.Cancel();
            _serverTask?.Wait(500);
        }
        catch { }

        _serverCts?.Dispose();
        try { _mutex.ReleaseMutex(); } catch { }
        _mutex.Dispose();
    }
}
