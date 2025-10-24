#!/usr/bin/env python3
import socket
import struct
import time

def test_password_debug():
    """测试密码调试信息"""
    try:
        # 连接到 v2ray SOCKS 代理
        sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        sock.connect(('127.0.0.1', 10808))
        
        # SOCKS5 握手
        sock.send(b'\x05\x01\x00')  # SOCKS5, 1 method, no auth
        response = sock.recv(2)
        print(f"SOCKS5 handshake response: {response.hex()}")
        
        if len(response) == 2 and response[0] == 0x05:
            # 发送连接请求
            request = b'\x05\x01\x00\x01' + socket.inet_aton('8.8.8.8') + struct.pack('>H', 53)
            sock.send(request)
            
            # 接收连接响应
            response = sock.recv(10)
            print(f"CONNECT response: {response.hex()}")
            
            if len(response) >= 2 and response[1] == 0x00:
                print("✅ 连接成功，检查日志中的密码信息")
            else:
                print(f"❌ 连接失败，响应码: {response[1] if len(response) > 1 else 'N/A'}")
                
        sock.close()
        
    except Exception as e:
        print(f"❌ 测试失败: {e}")

if __name__ == "__main__":
    print("🔍 测试密码传递调试信息...")
    test_password_debug()
