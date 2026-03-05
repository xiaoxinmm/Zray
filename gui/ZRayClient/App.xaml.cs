using System.Diagnostics;

namespace ZRayClient
{
    public partial class App : System.Windows.Application
    {
        protected override void OnExit(System.Windows.ExitEventArgs e)
        {
            // 兜底：杀所有残留的 zray-client 进程
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
