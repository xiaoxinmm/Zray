using System;
using System.Diagnostics;
using System.IO;
using System.Net.Http;
using System.Text.Json;
using System.Windows;
using System.Windows.Media;
using System.Windows.Threading;
using Wpf.Ui.Controls;

namespace ZRayClient
{
    public partial class MainWindow : FluentWindow
    {
        private Process? _coreProcess;
        private readonly HttpClient _http = new();
        private readonly DispatcherTimer _timer;
        private bool _isConnected;
        private const string API = "http://127.0.0.1:18790";
        private const string CORE = "zray-client.exe";
        private const string REPO = "xiaoxinmm/Zray";
        private string _ver = "2.3.0";

        public MainWindow()
        {
            InitializeComponent();
            _timer = new DispatcherTimer { Interval = TimeSpan.FromSeconds(1) };
            _timer.Tick += OnTick;
            LoadConfig();
            CheckUpdateAsync();
        }

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
            }
            catch (Exception ex) { Show("启动失败: " + ex.Message); }
        }

        private void StopCore()
        {
            _timer.Stop();
            try { if (_coreProcess is { HasExited: false }) { _coreProcess.Kill(); _coreProcess.WaitForExit(3000); } } catch { }
            _coreProcess = null;
            SetOff();
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

        private void OnImportLink(object sender, RoutedEventArgs e)
        {
            var za = TxtZALink.Text.Trim();
            if (string.IsNullOrEmpty(za) || !za.StartsWith("ZA://", StringComparison.OrdinalIgnoreCase))
            { Show("请输入 ZA:// 链接"); return; }
            try
            {
                var core = Path.Combine(AppDomain.CurrentDomain.BaseDirectory, CORE);
                Process.Start(new ProcessStartInfo { FileName = core, Arguments = $"--link \"{za}\" --dry-run", CreateNoWindow = true, UseShellExecute = false })?.WaitForExit(5000);
                Show("导入成功"); LoadConfig();
            }
            catch (Exception ex) { Show("导入失败: " + ex.Message); }
        }

        private async void CheckUpdateAsync()
        {
            try
            {
                _http.DefaultRequestHeaders.UserAgent.ParseAdd("ZRay");
                var j = await _http.GetStringAsync($"https://api.github.com/repos/{REPO}/releases/latest");
                var doc = JsonDocument.Parse(j);
                var tag = doc.RootElement.GetProperty("tag_name").GetString()?.TrimStart('v') ?? "";
                if (!string.IsNullOrEmpty(tag) && string.Compare(tag, _ver, StringComparison.Ordinal) > 0)
                {
                    var url = doc.RootElement.GetProperty("html_url").GetString() ?? "";
                    if (System.Windows.MessageBox.Show($"新版本 v{tag} 可用，是否下载？", "更新", System.Windows.MessageBoxButton.YesNo) == System.Windows.MessageBoxResult.Yes)
                        Process.Start(new ProcessStartInfo(url) { UseShellExecute = true });
                }
            }
            catch { }
        }

        private void SaveConfig()
        {
            var cfg = new { smart_port = $"127.0.0.1:{SmartPortText.Text.Trim()}", global_port = $"127.0.0.1:{GlobalPortText.Text.Trim()}", remote_host = TxtServer.Text.Trim(), remote_port = int.TryParse(TxtPort.Text.Trim(), out var p) ? p : 64433, user_hash = TxtHash.Text.Trim(), enable_tfo = false, geosite_path = "rules/geosite-cn.txt" };
            File.WriteAllText(Path.Combine(AppDomain.CurrentDomain.BaseDirectory, "config.json"), JsonSerializer.Serialize(cfg, new JsonSerializerOptions { WriteIndented = true }));
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

        protected override void OnStateChanged(EventArgs e)
        {
            if (WindowState == WindowState.Minimized) Hide();
            base.OnStateChanged(e);
        }

        protected override void OnClosed(EventArgs e) { StopCore(); base.OnClosed(e); }

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
