using System;
using System.Diagnostics;
using System.IO;
using System.Net.Http;
using System.Text.Json;
using System.Windows;
using System.Windows.Media;
using System.Windows.Threading;
using Wpf.Ui.Controls;
using H.NotifyIcon;

namespace ZRayClient
{
    public partial class MainWindow : FluentWindow
    {
        private Process? _coreProcess;
        private readonly HttpClient _http = new();
        private readonly DispatcherTimer _timer;
        private bool _isConnected;
        private TaskbarIcon? _trayIcon;
        private const string API = "http://127.0.0.1:18790";
        private const string CORE = "zray-client.exe";
        private const string REPO = "xiaoxinmm/Zray";
        private const string VERSION = "2.3.3";

        public MainWindow()
        {
            try
            {
                InitializeComponent();
                _timer = new DispatcherTimer { Interval = TimeSpan.FromSeconds(1) };
                _timer.Tick += OnTick;
                Title = $"ZRay v{VERSION}";
                InitTrayIcon();
                LoadConfig();
                CheckUpdateAsync();
            }
            catch (Exception ex)
            {
                System.Windows.MessageBox.Show($"初始化失败: {ex}", "ZRay 错误");
            }
        }

        // === System Tray ===
        private void InitTrayIcon()
        {
            try
            {
                _trayIcon = new TaskbarIcon
                {
                    ToolTipText = $"ZRay v{VERSION}",
                    Visibility = Visibility.Visible,
                    ContextMenu = CreateTrayMenu(),
                };

                // 尝试加载图标
                try
                {
                    _trayIcon.IconSource = new System.Windows.Media.Imaging.BitmapImage(
                        new Uri("pack://application:,,,/Assets/icon.ico"));
                }
                catch { }

                _trayIcon.TrayLeftMouseDown += (s, e) => ShowWindow();
            }
            catch (Exception ex)
            {
                // 托盘失败不影响主程序
                System.Diagnostics.Debug.WriteLine($"托盘初始化失败: {ex.Message}");
            }
        }

        private System.Windows.Controls.ContextMenu CreateTrayMenu()
        {
            var menu = new System.Windows.Controls.ContextMenu();

            var showItem = new System.Windows.Controls.MenuItem { Header = "显示主窗口" };
            showItem.Click += (s, e) => ShowWindow();
            menu.Items.Add(showItem);

            menu.Items.Add(new System.Windows.Controls.Separator());

            var exitItem = new System.Windows.Controls.MenuItem { Header = "退出" };
            exitItem.Click += (s, e) => { StopCore(); _trayIcon?.Dispose(); Application.Current.Shutdown(); };
            menu.Items.Add(exitItem);

            return menu;
        }

        private void ShowWindow()
        {
            Show();
            WindowState = WindowState.Normal;
            Activate();
        }

        // === Connection ===
        private void OnToggleConnect(object sender, RoutedEventArgs e)
        {
            if (_isConnected) StopCore(); else StartCore();
        }

        private void StartCore()
        {
            try
            {
                SaveConfig();
                var path = Path.Combine(AppDomain.CurrentDomain.BaseDirectory, CORE);
                if (!File.Exists(path)) { Show("找不到 " + CORE); return; }

                _coreProcess = new Process
                {
                    StartInfo = new ProcessStartInfo
                    {
                        FileName = path,
                        WorkingDirectory = AppDomain.CurrentDomain.BaseDirectory,
                        CreateNoWindow = true, UseShellExecute = false,
                        RedirectStandardOutput = true, RedirectStandardError = true,
                    },
                    EnableRaisingEvents = true
                };
                _coreProcess.Exited += (s, e) => Dispatcher.Invoke(SetOff);
                _coreProcess.Start();
                SetOn();
                _timer.Start();
                if (_trayIcon != null) _trayIcon.ToolTipText = $"ZRay v{VERSION} - 已连接";
            }
            catch (Exception ex) { Show("启动失败: " + ex.Message); }
        }

        private void StopCore()
        {
            _timer.Stop();
            try { if (_coreProcess is { HasExited: false }) { _coreProcess.Kill(); _coreProcess.WaitForExit(3000); } } catch { }
            _coreProcess = null;
            SetOff();
            if (_trayIcon != null) _trayIcon.ToolTipText = $"ZRay v{VERSION} - 未连接";
        }

        private void SetOn()
        {
            _isConnected = true;
            BtnConnect.Content = "断开";
            BtnConnect.Appearance = Wpf.Ui.Controls.ControlAppearance.Danger;
            StatusText.Text = "已连接";
            StatusDot.Fill = new SolidColorBrush(Color.FromRgb(0x53, 0xCF, 0x5E));
            TxtServer.IsEnabled = false; TxtPort.IsEnabled = false; TxtHash.IsEnabled = false;
        }

        private void SetOff()
        {
            _isConnected = false;
            BtnConnect.Content = "连 接";
            BtnConnect.Appearance = Wpf.Ui.Controls.ControlAppearance.Primary;
            StatusText.Text = "未连接";
            StatusDot.Fill = new SolidColorBrush(Colors.Gray);
            UpSpeed.Text = "0 B/s"; DownSpeed.Text = "0 B/s";
            ConnCount.Text = "0"; DirectCount.Text = "0"; ProxyCount.Text = "0";
            LatencyText.Text = "- ms";
            TxtServer.IsEnabled = true; TxtPort.IsEnabled = true; TxtHash.IsEnabled = true;
        }

        // === Stats ===
        private async void OnTick(object? s, EventArgs e)
        {
            try
            {
                var j = await _http.GetStringAsync($"{API}/stats");
                var d = JsonSerializer.Deserialize<Stats>(j);
                if (d == null) return;
                UpSpeed.Text = Fmt(d.up_speed);
                DownSpeed.Text = Fmt(d.down_speed);
                ConnCount.Text = d.active.ToString();
                DirectCount.Text = d.direct.ToString();
                ProxyCount.Text = d.proxied.ToString();
                LatencyText.Text = d.latency_ms > 0 ? $"{d.latency_ms} ms" : d.latency_ms < 0 ? "超时" : "...";
            }
            catch { }
        }

        static string Fmt(long b) => b >= 1048576 ? $"{b / 1048576.0:F1} MB/s" : b >= 1024 ? $"{b / 1024.0:F1} KB/s" : $"{b} B/s";

        // === ZA Link Import ===
        private void OnImportLink(object sender, RoutedEventArgs e)
        {
            var za = TxtZALink.Text.Trim();
            if (string.IsNullOrEmpty(za) || !za.StartsWith("ZA://", StringComparison.OrdinalIgnoreCase))
            { Show("请输入 ZA:// 链接"); return; }
            try
            {
                var core = Path.Combine(AppDomain.CurrentDomain.BaseDirectory, CORE);
                var proc = new Process
                {
                    StartInfo = new ProcessStartInfo
                    {
                        FileName = core,
                        Arguments = $"--link \"{za}\"",
                        WorkingDirectory = AppDomain.CurrentDomain.BaseDirectory,
                        CreateNoWindow = true, UseShellExecute = false,
                        RedirectStandardOutput = true, RedirectStandardError = true,
                    }
                };
                proc.Start();
                System.Threading.Thread.Sleep(2000);
                if (!proc.HasExited) proc.Kill();
                LoadConfig();
                Show($"导入成功: {TxtServer.Text}:{TxtPort.Text}");
                TxtZALink.Text = "";
            }
            catch (Exception ex) { Show("导入失败: " + ex.Message); }
        }

        // === Update Check ===
        private async void CheckUpdateAsync()
        {
            try
            {
                _http.DefaultRequestHeaders.UserAgent.ParseAdd("ZRay");
                var j = await _http.GetStringAsync($"https://api.github.com/repos/{REPO}/releases/latest");
                var doc = JsonDocument.Parse(j);
                var tag = doc.RootElement.GetProperty("tag_name").GetString()?.TrimStart('v') ?? "";
                if (!string.IsNullOrEmpty(tag) && string.Compare(tag, VERSION, StringComparison.Ordinal) > 0)
                {
                    var url = doc.RootElement.GetProperty("html_url").GetString() ?? "";
                    if (System.Windows.MessageBox.Show($"新版本 v{tag} 可用\n当前 v{VERSION}\n\n是否下载？", "ZRay 更新",
                        System.Windows.MessageBoxButton.YesNo) == System.Windows.MessageBoxResult.Yes)
                        Process.Start(new ProcessStartInfo(url) { UseShellExecute = true });
                }
            }
            catch { }
        }

        // === Config ===
        private void SaveConfig()
        {
            var cfg = new
            {
                smart_port = $"127.0.0.1:{SmartPortText.Text.Trim()}",
                global_port = $"127.0.0.1:{GlobalPortText.Text.Trim()}",
                remote_host = TxtServer.Text.Trim(),
                remote_port = int.TryParse(TxtPort.Text.Trim(), out var p) ? p : 64433,
                user_hash = TxtHash.Text.Trim(),
                enable_tfo = false,
                geosite_path = "rules/geosite-cn.txt"
            };
            File.WriteAllText(
                Path.Combine(AppDomain.CurrentDomain.BaseDirectory, "config.json"),
                JsonSerializer.Serialize(cfg, new JsonSerializerOptions { WriteIndented = true }));
        }

        private void LoadConfig()
        {
            try
            {
                var p = Path.Combine(AppDomain.CurrentDomain.BaseDirectory, "config.json");
                if (!File.Exists(p)) return;
                var doc = JsonDocument.Parse(File.ReadAllText(p));
                var r = doc.RootElement;
                if (r.TryGetProperty("remote_host", out var h)) TxtServer.Text = h.GetString() ?? "";
                if (r.TryGetProperty("remote_port", out var port)) TxtPort.Text = port.GetInt32().ToString();
                if (r.TryGetProperty("user_hash", out var hash)) TxtHash.Text = hash.GetString() ?? "";
                if (r.TryGetProperty("smart_port", out var sp)) SmartPortText.Text = (sp.GetString() ?? ":1080").Split(':')[^1];
                if (r.TryGetProperty("global_port", out var gp)) GlobalPortText.Text = (gp.GetString() ?? ":1081").Split(':')[^1];
            }
            catch { }
        }

        // === Window Lifecycle ===
        protected override void OnStateChanged(EventArgs e)
        {
            if (WindowState == WindowState.Minimized)
            {
                Hide(); // 最小化到托盘
            }
            base.OnStateChanged(e);
        }

        protected override void OnClosing(System.ComponentModel.CancelEventArgs e)
        {
            StopCore();
            _trayIcon?.Dispose();
            base.OnClosing(e);
        }

        protected override void OnClosed(EventArgs e)
        {
            StopCore();
            Application.Current.Shutdown();
            base.OnClosed(e);
        }

        static void Show(string msg) => System.Windows.MessageBox.Show(msg, "ZRay");

        class Stats
        {
            public long upload { get; set; }
            public long download { get; set; }
            public long up_speed { get; set; }
            public long down_speed { get; set; }
            public long active { get; set; }
            public long direct { get; set; }
            public long proxied { get; set; }
            public long latency_ms { get; set; }
        }
    }
}
