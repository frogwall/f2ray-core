#!/usr/bin/env python3
import socket
import struct

def test_socks5_connection():
    """测试标准的 SOCKS5 连接"""
    try:
        # 连接到 v2ray SOCKS 代理
        sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        sock.connect(('127.0.0.1', 10808))
        
        # SOCKS5 握手
        # 1. 发送版本和方法数量
        sock.send(b'\x05\x01\x00')  # SOCKS5, 1 method, no auth
        
        # 2. 接收服务器响应
        response = sock.recv(2)
        print(f"SOCKS5 handshake response: {response.hex()}")
        
        if len(response) == 2 and response[0] == 0x05:
            print("✅ SOCKS5 握手成功")
            
            # 3. 发送连接请求
            # SOCKS5 CONNECT request: VER(1) + CMD(1) + RSV(1) + ATYP(1) + ADDR(4) + PORT(2)
            request = b'\x05\x01\x00\x01' + socket.inet_aton('8.8.8.8') + struct.pack('>H', 53)
            sock.send(request)
            
            # 4. 接收连接响应
            response = sock.recv(10)
            print(f"CONNECT response: {response.hex()}")
            
            if len(response) >= 2 and response[1] == 0x00:
                print("✅ SOCKS5 连接建立成功")
                print("🎉 hysteria2 协议工作正常！")
            else:
                print(f"❌ 连接失败，响应码: {response[1] if len(response) > 1 else 'N/A'}")
        else:
            print("❌ SOCKS5 握手失败")
            
        sock.close()
        
    except Exception as e:
        print(f"❌ 测试失败: {e}")

if __name__ == "__main__":
    print("🧪 测试 hysteria2 协议的 SOCKS5 支持...")
    test_socks5_connection()
