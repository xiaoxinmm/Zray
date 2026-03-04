using System;
using System.Diagnostics;
using System.IO;
using System.Net.Http;
using System.Text.Json;
using System.Threading.Tasks;
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

        public MainWindow()
        {
            InitializeComponent();

            _timer = new DispatcherTimer { Interval = TimeSpan.FromSeconds(1) };
            _timer.Tick += OnTimerTick;

            LoadConfig();
        }

        // === Connection Toggle ===
        private void OnToggleConnect(object sender, RoutedEventArgs e)
        {
            if (_isConnected)
                StopCore();
            else
                StartCore();
        }

        private void StartCore()
        {
            try
            {
                SaveConfig();

                var corePath = Path.Combine(AppDomain.CurrentDomain.BaseDirectory, CORE_EXE);
                if (!File.Exists(corePath))
                {
                    MessageBox.Show($"找不到核心程序: {corePath}", "错误", MessageBoxButton.OK, MessageBoxImage.Error);
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

                _coreProcess.Exited += (s, e) =>
                {
                    Dispatcher.Invoke(() => SetDisconnected());
                };

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
            TxtServer.IsEnabled = true;
            TxtPort.IsEnabled = true;
            TxtHash.IsEnabled = true;
        }

        // === Stats Polling ===
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

            // Decode: we delegate to the core binary
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
                var output = proc?.StandardOutput.ReadToEnd();
                proc?.WaitForExit();

                if (!string.IsNullOrEmpty(output) && output.Contains(":"))
                {
                    // Parse "host:port" from output
                    MessageBox.Show("链接导入成功！请重新连接。", "成功");
                    LoadConfig(); // Reload
                }
                else
                {
                    MessageBox.Show("链接解析失败", "错误");
                }
            }
            catch (Exception ex)
            {
                MessageBox.Show($"导入失败: {ex.Message}", "错误");
            }
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
            var path = Path.Combine(AppDomain.CurrentDomain.BaseDirectory, "config.json");
            File.WriteAllText(path, json);
        }

        private void LoadConfig()
        {
            try
            {
                var path = Path.Combine(AppDomain.CurrentDomain.BaseDirectory, "config.json");
                if (!File.Exists(path)) return;

                var json = File.ReadAllText(path);
                var doc = JsonDocument.Parse(json);
                var root = doc.RootElement;

                if (root.TryGetProperty("remote_host", out var h)) TxtServer.Text = h.GetString() ?? "";
                if (root.TryGetProperty("remote_port", out var port)) TxtPort.Text = port.GetInt32().ToString();
                if (root.TryGetProperty("user_hash", out var hash)) TxtHash.Text = hash.GetString() ?? "";
                if (root.TryGetProperty("smart_port", out var sp))
                {
                    var parts = (sp.GetString() ?? ":1080").Split(':');
                    SmartPortText.Text = parts[^1];
                }
                if (root.TryGetProperty("global_port", out var gp))
                {
                    var parts = (gp.GetString() ?? ":1081").Split(':');
                    GlobalPortText.Text = parts[^1];
                }
            }
            catch { }
        }

        protected override void OnClosed(EventArgs e)
        {
            StopCore();
            base.OnClosed(e);
        }

        // === Stats Model ===
        private class CoreStats
        {
            public long upload { get; set; }
            public long download { get; set; }
            public long up_speed { get; set; }
            public long down_speed { get; set; }
            public long active { get; set; }
            public long direct { get; set; }
            public long proxied { get; set; }
            public bool running { get; set; }
        }
    }
}
