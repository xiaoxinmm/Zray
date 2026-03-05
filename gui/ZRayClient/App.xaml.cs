using System.Diagnostics;

namespace ZRayClient
{
    public partial class App : System.Windows.Application
    {
        protected override void OnExit(System.Windows.ExitEventArgs e)
        {
            // 兜底：确保所有 zray-client 子进程都被杀掉
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
