package com.zray.client

import android.app.*
import android.content.Intent
import android.os.Build
import android.os.IBinder
import androidx.core.app.NotificationCompat
import java.io.File

class ProxyService : Service() {

    private var process: Process? = null

    companion object {
        const val CHANNEL_ID = "zray_channel"
        const val NOTIFICATION_ID = 1
    }

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onCreate() {
        super.onCreate()
        createNotificationChannel()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        val configPath = intent?.getStringExtra("config_path") ?: return START_NOT_STICKY

        val notification = NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle("ZRay")
            .setContentText("代理运行中")
            .setSmallIcon(android.R.drawable.ic_lock_lock)
            .setOngoing(true)
            .setContentIntent(
                PendingIntent.getActivity(
                    this, 0,
                    Intent(this, MainActivity::class.java),
                    PendingIntent.FLAG_IMMUTABLE
                )
            )
            .build()

        startForeground(NOTIFICATION_ID, notification)

        // Start the Go binary
        startProxy(configPath)

        return START_STICKY
    }

    private fun startProxy(configPath: String) {
        try {
            val binaryPath = File(filesDir, "zray-client").absolutePath
            val workDir = filesDir

            val pb = ProcessBuilder(binaryPath)
            pb.directory(workDir)
            pb.environment()["ZRAY_CONFIG"] = configPath
            pb.redirectErrorStream(true)

            process = pb.start()

            // Log output in background
            Thread {
                process?.inputStream?.bufferedReader()?.forEachLine { line ->
                    android.util.Log.d("ZRay", line)
                }
            }.start()

        } catch (e: Exception) {
            android.util.Log.e("ZRay", "启动代理失败", e)
            stopSelf()
        }
    }

    override fun onDestroy() {
        process?.destroy()
        process = null
        super.onDestroy()
    }

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                CHANNEL_ID,
                "ZRay Proxy",
                NotificationManager.IMPORTANCE_LOW
            ).apply {
                description = "ZRay 代理服务通知"
            }
            getSystemService(NotificationManager::class.java)
                .createNotificationChannel(channel)
        }
    }
}
