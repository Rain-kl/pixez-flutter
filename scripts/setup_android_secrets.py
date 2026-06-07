#!/usr/bin/env python3
"""
为 Android Release 工作流一键生成所需的 GitHub Secrets 设置命令。

用法:
    python3 scripts/setup_android_secrets.py [keystore_path]

脚本会交互式地引导你完成所有配置，最终输出可直接复制粘贴到终端执行的
 gh secret set 命令。

前置条件:
    - GitHub CLI (gh) 已安装并已登录
    - 已有 JDK (keytool) 或让脚本自动生成 keystore

所需 Secrets:
    RELEASE_KEY_STORE      — keystore 文件的 base64 编码内容
    RELEASE_KEY_ALIAS      — 密钥别名
    RELEASE_KEY_PASSWORD   — 密钥密码
    RELEASE_STORE_PASSWORD — keystore 密码
"""

import base64
import shutil
import subprocess
import sys
import os


def read_file_as_base64(path: str) -> str:
    with open(path, "rb") as f:
        return base64.b64encode(f.read()).decode("utf-8")


def prompt(prompt_text: str, default: str = "") -> str:
    suffix = f" [{default}]" if default else ""
    value = input(f"{prompt_text}{suffix}: ").strip()
    return value if value else default


def prompt_yes_no(prompt_text: str, default: bool = True) -> bool:
    hint = "Y/n" if default else "y/N"
    value = input(f"{prompt_text} [{hint}]: ").strip().lower()
    if not value:
        return default
    return value in ("y", "yes")


def generate_keystore(
    ks_path: str,
    alias: str,
    store_password: str,
    key_password: str,
    validity_years: int = 25,
    cn: str = "Android Release",
    org: str = "Android",
    ou: str = "Android",
) -> bool:
    """调用 keytool 生成新的签名 keystore。"""
    keytool = shutil.which("keytool")
    if not keytool:
        print("\n✘ 未找到 keytool，请确认 JDK 已安装并加入 PATH。")
        return False

    validity_days = validity_years * 365

    cmd = [
        keytool,
        "-genkeypair",
        "-v",
        "-keystore", ks_path,
        "-alias", alias,
        "-keyalg", "RSA",
        "-keysize", "2048",
        "-validity", str(validity_days),
        "-storepass", store_password,
        "-keypass", key_password,
        "-dname", f"CN={cn}, OU={ou}, O={org}, L=Unknown, ST=Unknown, C=US",
    ]

    print(f"\n  正在生成 keystore: {ks_path}")
    print(f"  别名: {alias}")
    print(f"  有效期: {validity_years} 年")
    print()

    result = subprocess.run(cmd, capture_output=True, text=True)

    if result.returncode != 0:
        print(f"✘ keytool 执行失败:\n{result.stderr}")
        return False

    print(f"✔ keystore 已生成: {ks_path}")
    return True


def main():
    print("=" * 52)
    print("  Android Release 工作流 — Secrets 生成工具")
    print("=" * 52)
    print()

    # ── 1. Keystore 文件 ──────────────────────────────
    ks_default = sys.argv[1] if len(sys.argv) > 1 else "release.jks"
    ks_path = prompt("请输入 keystore 文件路径", ks_default)

    # ── 2. 签名信息（先收集，生成 keystore 时也需要） ─────
    key_alias = prompt("请输入 key alias", "release")
    store_password = prompt("请输入 store password", "")
    key_password = prompt("请输入 key password (留空则与 store password 相同)", "")
    if not key_password:
        key_password = store_password

    if not all([key_alias, store_password, key_password]):
        print("\n✘ 所有字段均为必填，不能为空。")
        sys.exit(1)

    # ── 3. 读取或生成 keystore ─────────────────────────
    if os.path.isfile(ks_path):
        print(f"\n✔ 使用已有 keystore: {ks_path}")
    else:
        print(f"\n未找到 keystore 文件: {ks_path}")
        if not prompt_yes_no("是否自动生成新的签名 keystore?"):
            print("✘ 请提供有效的 keystore 文件后重新运行。")
            sys.exit(1)

        cn = prompt("  证书 CN (Common Name)", "Android Release")
        org = prompt("  证书 O  (Organization)", "Android")
        if not generate_keystore(ks_path, key_alias, store_password, key_password,
                                 cn=cn, org=org):
            sys.exit(1)

    ks_b64 = read_file_as_base64(ks_path)

    # ── 4. 输出可复制的命令 ─────────────────────────────
    repo = prompt("请输入 GitHub 仓库 (格式: owner/repo)", "")
    repo_flag = f" -R {repo}" if repo else ""

    commands = [
        ("RELEASE_KEY_STORE", ks_b64),
        ("RELEASE_KEY_ALIAS", key_alias),
        ("RELEASE_KEY_PASSWORD", key_password),
        ("RELEASE_STORE_PASSWORD", store_password),
    ]

    print()
    print("=" * 52)
    print("  ↓ 复制以下命令到终端执行即可设置全部 Secrets ↓")
    print("=" * 52)
    print()

    for name, value in commands:
        # 用 heredoc 避免特殊字符在 shell 中被解析
        print(f"gh secret set {name}{repo_flag} <<< '{value}'")

    print()
    print(f"共 {len(commands)} 个 Secrets。设置完成后可到仓库 Settings → Secrets 页面确认。")


if __name__ == "__main__":
    main()
