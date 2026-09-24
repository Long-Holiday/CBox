#!/usr/bin/env bash
set -euo pipefail

# Ensure ~/.local/bin is in PATH
export PATH="${HOME}/.local/bin:${PATH}"

echo "========================================================="
echo "               CBox Installation & Setup                "
echo "========================================================="

# 1. Check Go environment
echo "==> [1/6] Checking Go compiler..."
if ! command -v go >/dev/null 2>&1; then
    echo "Error: 'go' is not installed. CBox requires Go >= 1.24." >&2
    echo "Please install Go: https://go.dev/doc/install" >&2
    exit 1
fi
GO_VER=$(go version | awk '{print $3}' | sed 's/go//')
echo "Found Go version: ${GO_VER}"

# 2. Check system utilities
echo "==> [2/6] Checking required system tools (ssh, rsync, tmux)..."
MISSING_TOOLS=()
for tool in ssh rsync tmux git; do
    if ! command -v "$tool" >/dev/null 2>&1; then
        MISSING_TOOLS+=("$tool")
    fi
done

if [ ${#MISSING_TOOLS[@]} -gt 0 ]; then
    echo "Warning: Missing system tools: ${MISSING_TOOLS[*]}"
    if command -v apt-get >/dev/null 2>&1; then
        echo "Attempting to install missing tools via apt-get..."
        if [ "$(id -u)" -eq 0 ]; then
            apt-get update -qq && apt-get install -y -qq "${MISSING_TOOLS[@]}"
        elif command -v sudo >/dev/null 2>&1; then
            sudo apt-get update -qq && sudo apt-get install -y -qq "${MISSING_TOOLS[@]}" || true
        fi
    fi
else
    echo "All required system tools are present."
fi

# 3. Check and install uv
echo "==> [3/6] Checking uv package manager..."
if ! command -v uv >/dev/null 2>&1; then
    echo "uv not found. Installing uv..."
    curl -LsSf https://astral.sh/uv/install.sh | sh
    export PATH="${HOME}/.local/bin:${PATH}"
fi
echo "uv is ready: $(uv --version)"

# 4. Install google-colab-cli using uv tool
echo "==> [4/6] Installing google-colab-cli via uv tool..."
uv tool install google-colab-cli \
  --with "jupyter-kernel-client @ git+https://github.com/googlecolab/jupyter-kernel-client.git" \
  --force

echo "google-colab-cli installed successfully at: $(command -v colab)"

# 5. Build and install CBox binaries
echo "==> [5/6] Building and installing CBox (cbox and cboxd)..."
make build

INSTALL_DIR="${HOME}/.local/bin"
if [ "$(id -u)" -eq 0 ]; then
    INSTALL_DIR="/usr/local/bin"
fi

mkdir -p "$INSTALL_DIR"
cp -f bin/cbox "${INSTALL_DIR}/cbox"
cp -f bin/cboxd "${INSTALL_DIR}/cboxd"
chmod +x "${INSTALL_DIR}/cbox" "${INSTALL_DIR}/cboxd"

# Initialize directories and default config
mkdir -p "${HOME}/.config/cbox"
mkdir -p "${HOME}/.local/share/cbox/keys"
mkdir -p "${HOME}/.local/share/cbox/ssh"
mkdir -p "${HOME}/.local/share/cbox/profiles/default/home"

CONFIG_FILE="${HOME}/.config/cbox/config.yaml"
if [ ! -f "$CONFIG_FILE" ]; then
    cat << 'EOF' > "$CONFIG_FILE"
engine:
  data_dir: ~/.local/share/cbox

runtime:
  idle_timeout: 30m

sync:
  interval: 10m
  transfer: auto

ssh:
  binary: ssh
  control_persist: 10m

rsync:
  binary: rsync

provider:
  colab_binary: colab

recovery:
  enabled: true
  max_attempts: 3

logging:
  level: info
EOF
    echo "Created default configuration: ${CONFIG_FILE}"
fi

# Optional systemd service
SYSTEMD_USER_DIR="${HOME}/.config/systemd/user"
mkdir -p "$SYSTEMD_USER_DIR"
cat << EOF > "${SYSTEMD_USER_DIR}/cboxd.service"
[Unit]
Description=CBox Runtime Engine Daemon
After=network-online.target

[Service]
ExecStart=${INSTALL_DIR}/cboxd
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
EOF

echo "Installed cbox and cboxd to ${INSTALL_DIR}"

# 6. Colab CLI interactive authentication
echo "==> [6/6] Running Colab CLI for authentication..."
echo "---------------------------------------------------------"
echo "请按照终端输出的 Colab CLI 提示进行认证授权（如访问 OAuth 链接登录）。"
echo "如果已登录或已配置凭据，将直接显示当前会话状态："
echo "---------------------------------------------------------"
colab sessions || true

echo ""
echo "========================================================="
echo "           CBox 安装与环境初始化全部完成！               "
echo "========================================================="
echo "使用说明："
echo "1. 启动守护进程："
echo "   cboxd"
echo "   或者通过 systemd 后台运行："
echo "   systemctl --user daemon-reload && systemctl --user enable --now cboxd"
echo "2. 使用 CLI 执行命令："
echo "   cbox ps"
echo "   cbox build -t mmseg:latest ."
echo "   cbox run --gpu T4 mmseg:latest python train.py"
echo "========================================================="
