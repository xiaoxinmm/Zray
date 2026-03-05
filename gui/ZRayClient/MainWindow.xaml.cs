using System;
using System.Diagnostics;
using System.IO;
using System.Net.Http;
using System.Text.Json;
using System.Windows;
using System.Windows.Media;
using System.Windows.Threading;

namespace ZRayClient
{
    public partial class MainWindow : Window
    {
        private Process? _coreProcess;
        private readonly HttpClient _http = new();
        private readonly DispatcherTimer _timer;
        private bool _isConnected;
        private const string API_BASE = "http://127.0.0.1:18790";
        private const string CORE_EXE = "zray-client.exe";
        private const string GITHUB_REPO = "xiaoxinmm/Zray";
        private string _currentVersion = "2.2.0";

        public MainWindow()
        {
            InitializeComponent();
            _timer = new DispatcherTimer { Interval = TimeSpan.FromSeconds(1) };
            _timer.Tick += OnTimerTick;
            LoadConfig();
            Title = $"ZRay v{_currentVersion}";
            CheckUpdateAsync();
        }

        // === Connection Toggle ===
        private void OnToggleConnect(object sender, RoutedEventArgs e)
        {
            if (_isConnected) StopCore();
            else StartCore();
        }

        private void StartCore()
        {
            try
            {
                SaveConfig();
                var corePath = Path.Combine(AppDomain.CurrentDomain.BaseDirectory, CORE_EXE);
                if (!File.Exists(corePath))
                {
                    MessageBox.Show($"找不到核心: {corePath}", "错误", MessageBoxButton.OK, MessageBoxImage.Error);
                    return;
                }

                _coreProcess = new Process
                {
                    StartInfo = new ProcessStartInfo
                    {
                        FileName = corePath,
                        WorkingDirectory = AppDomain.CurrentDomain.BaseDirectory,
                        CreateNoWindow = true,
                        UseShellExecute = false,
                        RedirectStandardOutput = true,
                        RedirectStandardError = true,
                    },
                    EnableRaisingEvents = true
                };
                _coreProcess.Exited += (s, e) => Dispatcher.Invoke(SetDisconnected);
                _coreProcess.Start();
                SetConnected();
                _timer.Start();
            }
            catch (Exception ex)
            {
                MessageBox.Show($"启动失败: {ex.Message}", "错误");
            }
        }

        private void StopCore()
        {
            _timer.Stop();
            try
            {
                if (_coreProcess != null && !_coreProcess.HasExited)
                {
                    _coreProcess.Kill();
                    _coreProcess.WaitForExit(3000);
                }
            }
            catch { }
            _coreProcess = null;
            SetDisconnected();
        }

        // === UI State ===
        private void SetConnected()
        {
            _isConnected = true;
            BtnConnect.Content = "断开连接";
            BtnConnect.Background = new SolidColorBrush((Color)ColorConverter.ConvertFromString("#E94560"));
            StatusText.Text = "已连接";
            StatusDot.Fill = new SolidColorBrush((Color)ColorConverter.ConvertFromString("#53CF5E"));
            TxtServer.IsEnabled = false;
            TxtPort.IsEnabled = false;
            TxtHash.IsEnabled = false;
        }

        private void SetDisconnected()
        {
            _isConnected = false;
            BtnConnect.Content = "连 接";
            BtnConnect.Background = new SolidColorBrush((Color)ColorConverter.ConvertFromString("#0F3460"));
            StatusText.Text = "未连接";
            StatusDot.Fill = new SolidColorBrush(Colors.Gray);
            UpSpeed.Text = "0 B/s";
            DownSpeed.Text = "0 B/s";
            ConnCount.Text = "0";
            LatencyText.Text = "- ms";
            TxtServer.IsEnabled = true;
            TxtPort.IsEnabled = true;
            TxtHash.IsEnabled = true;
        }

        // === Stats Polling (每秒) ===
        private async void OnTimerTick(object? sender, EventArgs e)
        {
            try
            {
                var json = await _http.GetStringAsync($"{API_BASE}/stats");
                var stats = JsonSerializer.Deserialize<CoreStats>(json);
                if (stats == null) return;

                UpSpeed.Text = FormatSpeed(stats.up_speed);
                DownSpeed.Text = FormatSpeed(stats.down_speed);
                ConnCount.Text = stats.active.ToString();
                DirectCount.Text = $"🎯 直连: {stats.direct}";
                ProxyCount.Text = $"🌐 代理: {stats.proxied}";

                // 延迟显示
                if (stats.latency_ms > 0)
                    LatencyText.Text = $"{stats.latency_ms} ms";
                else if (stats.latency_ms < 0)
                    LatencyText.Text = "超时";
                else
                    LatencyText.Text = "测量中...";
            }
            catch { }
        }

        private static string FormatSpeed(long bytesPerSec)
        {
            if (bytesPerSec < 1024) return $"{bytesPerSec} B/s";
            if (bytesPerSec < 1024 * 1024) return $"{bytesPerSec / 1024.0:F1} KB/s";
            return $"{bytesPerSec / (1024.0 * 1024.0):F1} MB/s";
        }

        // === ZA Link Import ===
        private void OnImportLink(object sender, RoutedEventArgs e)
        {
            var zaLink = TxtZALink.Text.Trim();
            if (string.IsNullOrEmpty(zaLink) || !zaLink.StartsWith("ZA://", StringComparison.OrdinalIgnoreCase))
            {
                MessageBox.Show("请输入有效的 ZA:// 链接", "提示");
                return;
            }
            try
            {
                var corePath = Path.Combine(AppDomain.CurrentDomain.BaseDirectory, CORE_EXE);
                var proc = Process.Start(new ProcessStartInfo
                {
                    FileName = corePath,
                    Arguments = $"--link \"{zaLink}\" --dry-run",
                    CreateNoWindow = true,
                    UseShellExecute = false,
                    RedirectStandardOutput = true,
                });
                proc?.WaitForExit(5000);
                MessageBox.Show("链接导入成功！", "成功");
                LoadConfig();
            }
            catch (Exception ex)
            {
                MessageBox.Show($"导入失败: {ex.Message}", "错误");
            }
        }

        // === #7 Auto Update ===
        private async void CheckUpdateAsync()
        {
            try
            {
                _http.DefaultRequestHeaders.UserAgent.ParseAdd("ZRay-Client");
                var json = await _http.GetStringAsync($"https://api.github.com/repos/{GITHUB_REPO}/releases/latest");
                var doc = JsonDocument.Parse(json);
                var latest = doc.RootElement.GetProperty("tag_name").GetString()?.TrimStart('v') ?? "";
                if (!string.IsNullOrEmpty(latest) && string.Compare(latest, _currentVersion, StringComparison.Ordinal) > 0)
                {
                    var url = doc.RootElement.GetProperty("html_url").GetString() ?? "";
                    var result = MessageBox.Show(
                        $"发现新版本 v{latest}\n当前版本 v{_currentVersion}\n\n是否打开下载页面？",
                        "ZRay 更新", MessageBoxButton.YesNo, MessageBoxImage.Information);
                    if (result == MessageBoxResult.Yes)
                    {
                        Process.Start(new ProcessStartInfo(url) { UseShellExecute = true });
                    }
                }
            }
            catch { } // 静默失败
        }

        // === Config ===
        private void SaveConfig()
        {
            var config = new
            {
                smart_port = "127.0.0.1:1080",
                global_port = "127.0.0.1:1081",
                remote_host = TxtServer.Text.Trim(),
                remote_port = int.TryParse(TxtPort.Text.Trim(), out var p) ? p : 64433,
                user_hash = TxtHash.Text.Trim(),
                enable_tfo = false,
                geosite_path = "rules/geosite-cn.txt"
            };
            var json = JsonSerializer.Serialize(config, new JsonSerializerOptions { WriteIndented = true });
            File.WriteAllText(Path.Combine(AppDomain.CurrentDomain.BaseDirectory, "config.json"), json);
        }

        private void LoadConfig()
        {
            try
            {
                var path = Path.Combine(AppDomain.CurrentDomain.BaseDirectory, "config.json");
                if (!File.Exists(path)) return;
                var doc = JsonDocument.Parse(File.ReadAllText(path));
                var root = doc.RootElement;
                if (root.TryGetProperty("remote_host", out var h)) TxtServer.Text = h.GetString() ?? "";
                if (root.TryGetProperty("remote_port", out var port)) TxtPort.Text = port.GetInt32().ToString();
                if (root.TryGetProperty("user_hash", out var hash)) TxtHash.Text = hash.GetString() ?? "";
                if (root.TryGetProperty("smart_port", out var sp))
                    SmartPortText.Text = (sp.GetString() ?? ":1080").Split(':')[^1];
                if (root.TryGetProperty("global_port", out var gp))
                    GlobalPortText.Text = (gp.GetString() ?? ":1081").Split(':')[^1];
            }
            catch { }
        }

        // === #6 最小化到托盘 ===
        protected override void OnStateChanged(EventArgs e)
        {
            if (WindowState == WindowState.Minimized)
            {
                Hide();
                // 用 NotifyIcon 需要 WinForms 引用，这里简化：任务栏仍可见
                // 完整托盘需要 Hardcodet.NotifyIcon.Wpf 包
            }
            base.OnStateChanged(e);
        }

        protected override void OnClosed(EventArgs e)
        {
            StopCore();
            base.OnClosed(e);
        }

        private class CoreStats
        {
            public long upload { get; set; }
            public long download { get; set; }
            public long up_speed { get; set; }
            public long down_speed { get; set; }
            public long active { get; set; }
            public long direct { get; set; }
            public long proxied { get; set; }
            public long latency_ms { get; set; }
            public bool running { get; set; }
        }
    }
}
