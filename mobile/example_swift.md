# iOS Swift 使用示例

## 基础使用

### 1. 导入 Framework

```swift
import F2Ray
```

### 2. 创建 F2Ray 管理器

```swift
import Foundation
import F2Ray

class F2RayManager {
    private var instance: MobileF2RayInstance?
    private var isRunning = false
    
    static let shared = F2RayManager()
    
    private init() {}
    
    // 启动 F2Ray
    func start(configJSON: String) throws {
        guard !isRunning else {
            throw F2RayError.alreadyRunning
        }
        
        do {
            instance = try MobileStartF2Ray(configJSON)
            isRunning = true
            print("✅ F2Ray 启动成功")
        } catch {
            throw F2RayError.startFailed(error.localizedDescription)
        }
    }
    
    // 停止 F2Ray
    func stop() throws {
        guard isRunning else {
            return
        }
        
        do {
            try instance?.stop()
            instance = nil
            isRunning = false
            print("✅ F2Ray 已停止")
        } catch {
            throw F2RayError.stopFailed(error.localizedDescription)
        }
    }
    
    // 获取版本
    func getVersion() -> String {
        return MobileGetVersion()
    }
    
    // 测试配置
    func testConfig(_ configJSON: String) -> Bool {
        let result = MobileTestConfig(configJSON)
        return result.isEmpty
    }
    
    // 查询统计
    func queryStats(pattern: String) throws -> String {
        guard let instance = instance else {
            throw F2RayError.notRunning
        }
        
        do {
            return try instance.queryStats(pattern)
        } catch {
            throw F2RayError.queryFailed(error.localizedDescription)
        }
    }
}

// 错误定义
enum F2RayError: LocalizedError {
    case alreadyRunning
    case notRunning
    case startFailed(String)
    case stopFailed(String)
    case queryFailed(String)
    
    var errorDescription: String? {
        switch self {
        case .alreadyRunning:
            return "F2Ray 已经在运行"
        case .notRunning:
            return "F2Ray 未运行"
        case .startFailed(let message):
            return "启动失败: \(message)"
        case .stopFailed(let message):
            return "停止失败: \(message)"
        case .queryFailed(let message):
            return "查询失败: \(message)"
        }
    }
}
```

### 3. 在 ViewController 中使用

```swift
import UIKit

class ViewController: UIViewController {
    
    @IBOutlet weak var statusLabel: UILabel!
    @IBOutlet weak var startButton: UIButton!
    @IBOutlet weak var stopButton: UIButton!
    @IBOutlet weak var versionLabel: UILabel!
    
    override func viewDidLoad() {
        super.viewDidLoad()
        
        // 显示版本
        versionLabel.text = "F2Ray \(F2RayManager.shared.getVersion())"
        updateUI()
    }
    
    @IBAction func startButtonTapped(_ sender: UIButton) {
        let config = getConfig()
        
        // 测试配置
        guard F2RayManager.shared.testConfig(config) else {
            showAlert(title: "错误", message: "配置无效")
            return
        }
        
        // 启动
        do {
            try F2RayManager.shared.start(configJSON: config)
            statusLabel.text = "运行中"
            updateUI()
        } catch {
            showAlert(title: "启动失败", message: error.localizedDescription)
        }
    }
    
    @IBAction func stopButtonTapped(_ sender: UIButton) {
        do {
            try F2RayManager.shared.stop()
            statusLabel.text = "已停止"
            updateUI()
        } catch {
            showAlert(title: "停止失败", message: error.localizedDescription)
        }
    }
    
    private func updateUI() {
        // 更新按钮状态
    }
    
    private func showAlert(title: String, message: String) {
        let alert = UIAlertController(title: title, message: message, preferredStyle: .alert)
        alert.addAction(UIAlertAction(title: "确定", style: .default))
        present(alert, animated: true)
    }
    
    private func getConfig() -> String {
        return """
        {
          "log": {
            "loglevel": "info"
          },
          "inbounds": [
            {
              "port": 1080,
              "protocol": "socks",
              "settings": {
                "auth": "noauth",
                "udp": true
              }
            }
          ],
          "outbounds": [
            {
              "protocol": "vmess",
              "settings": {
                "vnext": [
                  {
                    "address": "server.example.com",
                    "port": 443,
                    "users": [
                      {
                        "id": "your-uuid-here",
                        "alterId": 0,
                        "security": "auto"
                      }
                    ]
                  }
                ]
              },
              "streamSettings": {
                "network": "ws",
                "security": "tls"
              }
            }
          ]
        }
        """
    }
}
```

## 高级用法

### 1. 使用 Naive 协议

```swift
private func getNaiveConfig() -> String {
    return """
    {
      "log": {
        "loglevel": "info"
      },
      "inbounds": [
        {
          "port": 1080,
          "protocol": "socks",
          "settings": {
            "auth": "noauth"
          }
        }
      ],
      "outbounds": [
        {
          "protocol": "naive",
          "settings": {
            "address": "server.example.com",
            "port": 443,
            "username": "user",
            "password": "pass"
          },
          "streamSettings": {
            "security": "none"
          }
        }
      ]
    }
    """
}
```

### 2. 使用 Hysteria2 协议

```swift
private func getHysteria2Config() -> String {
    return """
    {
      "log": {
        "loglevel": "info"
      },
      "inbounds": [
        {
          "port": 1080,
          "protocol": "socks",
          "settings": {
            "auth": "noauth"
          }
        }
      ],
      "outbounds": [
        {
          "protocol": "hysteria2",
          "settings": {
            "servers": [
              {
                "address": "server.example.com",
                "port": 443,
                "password": "your_password"
              }
            ]
          }
        }
      ]
    }
    """
}
```

### 3. 使用 Brook 协议

```swift
private func getBrookConfig() -> String {
    return """
    {
      "log": {
        "loglevel": "info"
      },
      "inbounds": [
        {
          "port": 1080,
          "protocol": "socks",
          "settings": {
            "auth": "noauth"
          }
        }
      ],
      "outbounds": [
        {
          "protocol": "brook",
          "settings": {
            "servers": [
              {
                "address": "server.example.com",
                "port": 9999,
                "password": "your_password",
                "method": "tcp"
              }
            ]
          }
        }
      ]
    }
    """
}
```

## 网络扩展 (Network Extension)

### 1. 创建 Packet Tunnel Provider

```swift
import NetworkExtension
import V2Ray

class PacketTunnelProvider: NEPacketTunnelProvider {
    
    private var v2rayInstance: MobileV2RayInstance?
    
    override func startTunnel(options: [String : NSObject]?, 
                            completionHandler: @escaping (Error?) -> Void) {
        
        // 读取配置
        guard let config = loadConfig() else {
            completionHandler(NSError(domain: "V2Ray", code: -1, 
                                     userInfo: [NSLocalizedDescriptionKey: "配置加载失败"]))
            return
        }
        
        // 启动 V2Ray
        do {
            v2rayInstance = try MobileStartV2Ray(config)
            
            // 配置网络设置
            let settings = createTunnelSettings()
            setTunnelNetworkSettings(settings) { error in
                if let error = error {
                    completionHandler(error)
                } else {
                    completionHandler(nil)
                }
            }
        } catch {
            completionHandler(error)
        }
    }
    
    override func stopTunnel(with reason: NEProviderStopReason, 
                           completionHandler: @escaping () -> Void) {
        do {
            try v2rayInstance?.stop()
            v2rayInstance = nil
        } catch {
            print("停止失败: \(error)")
        }
        completionHandler()
    }
    
    private func createTunnelSettings() -> NEPacketTunnelNetworkSettings {
        let settings = NEPacketTunnelNetworkSettings(tunnelRemoteAddress: "127.0.0.1")
        
        // IPv4 设置
        let ipv4Settings = NEIPv4Settings(addresses: ["10.0.0.2"], 
                                         subnetMasks: ["255.255.255.0"])
        ipv4Settings.includedRoutes = [NEIPv4Route.default()]
        settings.ipv4Settings = ipv4Settings
        
        // DNS 设置
        let dnsSettings = NEDNSSettings(servers: ["8.8.8.8", "8.8.4.4"])
        settings.dnsSettings = dnsSettings
        
        return settings
    }
    
    private func loadConfig() -> String? {
        // 从 UserDefaults 或文件加载配置
        return UserDefaults(suiteName: "group.your.app.id")?.string(forKey: "v2ray_config")
    }
}
```

## 注意事项

1. **权限配置**
   - 需要在 `Info.plist` 中添加网络权限
   - Network Extension 需要配置 App Groups

2. **后台运行**
   - iOS 限制后台网络，建议使用 Network Extension
   - 普通 App 后台运行时间有限

3. **内存限制**
   - iOS 对 App 内存使用有严格限制
   - Network Extension 内存限制更严格（约 50MB）

4. **调试**
   - 使用 Console.app 查看日志
   - Network Extension 调试需要真机

5. **发布**
   - 需要申请 Network Extension 权限
   - 需要配置 Provisioning Profile
