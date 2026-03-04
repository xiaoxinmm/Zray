package com.zray.client

import android.app.Activity
import android.content.Context
import android.content.Intent
import android.content.SharedPreferences
import android.net.VpnService
import android.os.Bundle
import android.view.View
import android.widget.*
import java.io.File

class MainActivity : Activity() {

    private lateinit var prefs: SharedPreferences
    private lateinit var btnConnect: Button
    private lateinit var btnMode: Button
    private lateinit var tvStatus: TextView
    private lateinit var tvStats: TextView
    private lateinit var tvSmartPort: TextView
    private lateinit var tvGlobalPort: TextView
    private lateinit var etServer: EditText
    private lateinit var etPort: EditText
    private lateinit var etHash: EditText
    private lateinit var cardStatus: View

    private var isConnected = false
    private var isSmartMode = true

    companion object {
        const val VPN_REQUEST_CODE = 100
        const val PREFS_NAME = "zray_config"
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)

        prefs = getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)

        initViews()
        loadConfig()
        updateUI()

        // Extract binary on first run
        extractBinary()
    }

    private fun initViews() {
        btnConnect = findViewById(R.id.btn_connect)
        btnMode = findViewById(R.id.btn_mode)
        tvStatus = findViewById(R.id.tv_status)
        tvStats = findViewById(R.id.tv_stats)
        tvSmartPort = findViewById(R.id.tv_smart_port)
        tvGlobalPort = findViewById(R.id.tv_global_port)
        etServer = findViewById(R.id.et_server)
        etPort = findViewById(R.id.et_port)
        etHash = findViewById(R.id.et_hash)
        cardStatus = findViewById(R.id.card_status)

        btnConnect.setOnClickListener { toggleConnection() }
        btnMode.setOnClickListener { toggleMode() }
    }

    private fun loadConfig() {
        etServer.setText(prefs.getString("server", ""))
        etPort.setText(prefs.getString("port", "64433"))
        etHash.setText(prefs.getString("hash", ""))
        isSmartMode = prefs.getBoolean("smart_mode", true)
    }

    private fun saveConfig() {
        prefs.edit()
            .putString("server", etServer.text.toString())
            .putString("port", etPort.text.toString())
            .putString("hash", etHash.text.toString())
            .putBoolean("smart_mode", isSmartMode)
            .apply()
    }

    private fun toggleConnection() {
        if (isConnected) {
            stopProxy()
        } else {
            saveConfig()
            if (etServer.text.isNullOrEmpty() || etHash.text.isNullOrEmpty()) {
                Toast.makeText(this, "请填写服务器地址和密钥", Toast.LENGTH_SHORT).show()
                return
            }
            startProxy()
        }
    }

    private fun toggleMode() {
        isSmartMode = !isSmartMode
        saveConfig()
        updateUI()
        if (isConnected) {
            // Restart to apply new mode
            stopProxy()
            startProxy()
        }
    }

    private fun startProxy() {
        // Write config file
        val configFile = File(filesDir, "config.json")
        val config = """
        {
            "smart_port": "127.0.0.1:1080",
            "global_port": "127.0.0.1:1081",
            "remote_host": "${etServer.text}",
            "remote_port": ${etPort.text},
            "user_hash": "${etHash.text}",
            "enable_tfo": false,
            "geosite_path": "${File(filesDir, "geosite-cn.txt").absolutePath}"
        }
        """.trimIndent()
        configFile.writeText(config)

        // Start proxy service
        val intent = Intent(this, ProxyService::class.java)
        intent.putExtra("config_path", configFile.absolutePath)
        startForegroundService(intent)

        isConnected = true
        updateUI()
    }

    private fun stopProxy() {
        stopService(Intent(this, ProxyService::class.java))
        isConnected = false
        updateUI()
    }

    private fun updateUI() {
        if (isConnected) {
            btnConnect.text = "断开连接"
            btnConnect.setBackgroundColor(0xFFE94560.toInt())
            tvStatus.text = "● 已连接"
            tvStatus.setTextColor(0xFF53CF5E.toInt())
            cardStatus.setBackgroundColor(0xFF1A3A2A.toInt())
        } else {
            btnConnect.text = "连接"
            btnConnect.setBackgroundColor(0xFF0F3460.toInt())
            tvStatus.text = "● 未连接"
            tvStatus.setTextColor(0xFF888888.toInt())
            cardStatus.setBackgroundColor(0xFF16213E.toInt())
        }

        if (isSmartMode) {
            btnMode.text = "🎯 智能分流"
            tvSmartPort.text = "SOCKS5  127.0.0.1:1080  (活跃)"
            tvGlobalPort.text = "SOCKS5  127.0.0.1:1081"
        } else {
            btnMode.text = "🌐 全局代理"
            tvSmartPort.text = "SOCKS5  127.0.0.1:1080"
            tvGlobalPort.text = "SOCKS5  127.0.0.1:1081  (活跃)"
        }

        etServer.isEnabled = !isConnected
        etPort.isEnabled = !isConnected
        etHash.isEnabled = !isConnected
    }

    private fun extractBinary() {
        val binaryFile = File(filesDir, "zray-client")
        if (!binaryFile.exists()) {
            try {
                assets.open("zray-client-android-arm64").use { input ->
                    binaryFile.outputStream().use { output ->
                        input.copyTo(output)
                    }
                }
                binaryFile.setExecutable(true)
            } catch (e: Exception) {
                // Binary will be downloaded or extracted later
            }
        }

        // Extract geosite rules
        val geositeFile = File(filesDir, "geosite-cn.txt")
        if (!geositeFile.exists()) {
            try {
                assets.open("geosite-cn.txt").use { input ->
                    geositeFile.outputStream().use { output ->
                        input.copyTo(output)
                    }
                }
            } catch (e: Exception) {}
        }
    }
}
