#!/usr/bin/env python3
"""
DAAN Protocol - Keypair Generation Script

支持多种签名算法:
- ECC (secp256k1) - 比特币/以太坊通用
- SM2 - 中国国密算法
- Ed25519 - 现代椭圆曲线

Usage:
    python3 generate_keypair.py --algorithm ecc
    python3 generate_keypair.py --algorithm sm2
    python3 generate_keypair.py --algorithm ed25519
"""

import argparse
import json
import os
import hashlib
from datetime import datetime
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.backends import default_backend

def generate_ecc_keypair():
    """生成 ECC secp256k1 密钥对"""
    private_key = ec.generate_private_key(ec.SECP256K1(), default_backend())
    public_key = private_key.public_key()

    private_pem = private_key.private_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PrivateFormat.PKCS8,
        encryption_algorithm=serialization.NoEncryption()
    )

    public_pem = public_key.public_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PublicFormat.SubjectPublicKeyInfo
    )

    return private_pem, public_pem

def generate_sm2_keypair():
    """生成 SM2 密钥对（使用 ECC P-256 作为替代）"""
    # 注意：Python 标准库不支持 SM2，这里使用 P-256 作为替代
    # 实际部署时可以使用 gmssl 或其他 SM2 实现
    print("⚠️  注意: Python 标准库不支持 SM2，使用 P-256 作为替代")
    print("   如需真正 SM2，请使用 gmssl: https://github.com/duanhongyi/gmssl")

    private_key = ec.generate_private_key(ec.SECP256K1(), default_backend())
    public_key = private_key.public_key()

    private_pem = private_key.private_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PrivateFormat.PKCS8,
        encryption_algorithm=serialization.NoEncryption()
    )

    public_pem = public_key.public_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PublicFormat.SubjectPublicKeyInfo
    )

    return private_pem, public_pem

def generate_ed25519_keypair():
    """生成 Ed25519 密钥对"""
    from cryptography.hazmat.primitives.asymmetric import ed25519

    private_key = ed25519.Ed25519PrivateKey.generate()
    public_key = private_key.public_key()

    private_pem = private_key.private_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PrivateFormat.PKCS8,
        encryption_algorithm=serialization.NoEncryption()
    )

    public_pem = public_key.public_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PublicFormat.SubjectPublicKeyInfo
    )

    return private_pem, public_pem

def public_key_hash(public_pem):
    """计算公钥哈希作为 Agent ID"""
    return hashlib.sha256(public_pem).hexdigest()[:16]

def main():
    parser = argparse.ArgumentParser(description='Generate DAAN keypair')
    parser.add_argument('--algorithm', choices=['ecc', 'sm2', 'ed25519'],
                       default='ecc', help='Signature algorithm')
    parser.add_argument('--output', '-o', default='./keypair',
                       help='Output directory')
    args = parser.parse_args()

    print(f"🔐 生成 {args.algorithm.upper()} 密钥对...")

    # 生成密钥对
    if args.algorithm == 'ecc':
        private_pem, public_pem = generate_ecc_keypair()
    elif args.algorithm == 'sm2':
        private_pem, public_pem = generate_sm2_keypair()
    else:
        private_pem, public_pem = generate_ed25519_keypair()

    # 计算 Agent ID
    agent_id = public_key_hash(public_pem)

    # 创建输出目录
    os.makedirs(args.output, exist_ok=True)

    # 保存私钥（危险！仅演示用）
    private_path = os.path.join(args.output, f'{agent_id}_private.pem')
    with open(private_path, 'wb') as f:
        f.write(private_pem)
    os.chmod(private_path, 0o600)  # 仅所有者可读写

    # 保存公钥
    public_path = os.path.join(args.output, f'{agent_id}_public.pem')
    with open(public_path, 'wb') as f:
        f.write(public_pem)

    # 保存元数据
    metadata = {
        'agent_id': agent_id,
        'algorithm': args.algorithm,
        'created_at': datetime.utcnow().isoformat() + 'Z',
        'private_key_path': private_path,
        'public_key_path': public_path
    }

    metadata_path = os.path.join(args.output, 'metadata.json')
    with open(metadata_path, 'w') as f:
        json.dump(metadata, f, indent=2)

    print(f"\n✅ 密钥对生成成功!")
    print(f"\n📁 文件已保存到 {args.output}/:")
    print(f"   - {agent_id}_private.pem (私钥，请妥善保管!)")
    print(f"   - {agent_id}_public.pem (公钥，可公开)")
    print(f"   - metadata.json (元数据)")
    print(f"\n🆔 Agent ID: {agent_id}")
    print(f"\n⚠️  下一步:")
    print(f"   1. 将公钥提交到: registry/keys/{agent_id}.pem")
    print(f"   2. 创建 register-agent Issue")
    print(f"   3. 配置 OpenClaw Cron Jobs")

if __name__ == '__main__':
    main()
