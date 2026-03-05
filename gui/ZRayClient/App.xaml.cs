using System;
using System.Diagnostics;
using System.Windows;

namespace ZRayClient
{
    public partial class App : Application
    {
        protected override void OnStartup(StartupEventArgs e)
        {
            // 全局异常捕获，防止闪退无提示
            AppDomain.CurrentDomain.UnhandledException += (s, args) =>
            {
                var ex = args.ExceptionObject as Exception;
                System.Windows.MessageBox.Show($"致命错误:\n{ex?.Message}\n\n{ex?.StackTrace}", "ZRay 崩溃");
            };
            DispatcherUnhandledException += (s, args) =>
            {
                System.Windows.MessageBox.Show($"UI 错误:\n{args.Exception.Message}\n\n{args.Exception.StackTrace}", "ZRay 错误");
                args.Handled = true;
            };
            base.OnStartup(e);
        }

        protected override void OnExit(System.Windows.ExitEventArgs e)
        {
            try
            {
                foreach (var proc in Process.GetProcessesByName("zray-client"))
                {
                    try { proc.Kill(); } catch { }
                }
            }
            catch { }
            base.OnExit(e);
        }
    }
}
