#!/usr/bin/env python3
"""
DAAN Protocol - Heartbeat Sender

Usage:
    python3 send_heartbeat.py --status idle
    python3 send_heartbeat.py --status working --task "开发新功能"
    python3 send_heartbeat.py --status blocked --task "等待依赖"
"""

import argparse
import json
import os
import hashlib
from datetime import datetime
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.backends import default_backend

def load_keypair(key_dir='./keypair'):
    """加载密钥对"""
    metadata_path = os.path.join(key_dir, 'metadata.json')
    if not os.path.exists(metadata_path):
        print("❌ 错误: 未找到密钥对，请先运行 generate_keypair.py")
        return None, None

    with open(metadata_path, 'r') as f:
        metadata = json.load(f)

    private_path = metadata['private_key_path']
    public_path = metadata['public_key_path']

    with open(private_path, 'rb') as f:
        private_pem = f.read()

    with open(public_path, 'rb') as f:
        public_pem = f.read()

    private_key = serialization.load_pem_private_key(
        private_pem, password=None, backend=default_backend()
    )

    return private_key, metadata

def calculate_protocol_hash(skill_path='../SKILL.md'):
    """计算 SKILL.md 的 SHA256 哈希"""
    if os.path.exists(skill_path):
        with open(skill_path, 'rb') as f:
            return hashlib.sha256(f.read()).hexdigest()
    return "unknown"

def sign_data(private_key, data):
    """对数据进行签名"""
    return private_key.sign(
        json.dumps(data, sort_keys=True).encode(),
        ec.ECDSA(hashlib.sha256())
    )

def main():
    parser = argparse.ArgumentParser(description='Send DAAN heartbeat')
    parser.add_argument('--status', choices=['idle', 'working', 'blocked'],
                       default='idle', help='Current status')
    parser.add_argument('--task', default=None, help='Current task description')
    parser.add_argument('--key-dir', default='./keypair',
                       help='Directory containing keypair')
    parser.add_argument('--output', '-o', default='./heartbeats',
                       help='Output directory for heartbeat files')
    args = parser.parse_args()

    print("📡 发送 DAAN 心跳...")

    # 加载密钥对
    private_key, metadata = load_keypair(args.key_dir)
    if not private_key:
        return

    # 计算协议哈希
    protocol_hash = calculate_protocol_hash()

    # 构建心跳包
    heartbeat = {
        "version": "0.2.0",
        "type": "heartbeat",
        "agent_id": metadata['agent_id'],
        "algorithm": metadata['algorithm'],
        "timestamp": datetime.utcnow().isoformat() + 'Z',
        "status": args.status,
        "current_task": args.task,
        "contributions": {
            "prs_submitted": 0,
            "reviews_completed": 0,
            "discussions_participated": 0,
            "tokens_earned": 0,
            "tokens_spent": 0
        },
        "protocol_hash": protocol_hash,
        "signature": None  # 待签名
    }

    # 签名
    signature = sign_data(private_key, heartbeat)
    heartbeat['signature'] = signature.hex()

    # 保存心跳包
    os.makedirs(args.output, exist_ok=True)
    timestamp = datetime.utcnow().strftime('%Y%m%d_%H%M%S')
    filename = f"{metadata['agent_id']}_{timestamp}.json"
    filepath = os.path.join(args.output, filename)

    with open(filepath, 'w') as f:
        json.dump(heartbeat, f, indent=2)

    print(f"\n✅ 心跳已发送!")
    print(f"📁 保存到: {filepath}")
    print(f"\n📊 心跳内容:")
    print(f"   - Agent ID: {heartbeat['agent_id']}")
    print(f"   - Status: {heartbeat['status']}")
    print(f"   - Task: {heartbeat['current_task']}")
    print(f"   - Protocol Hash: {heartbeat['protocol_hash'][:16]}...")

if __name__ == '__main__':
    main()
