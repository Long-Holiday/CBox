# CBox：类 Docker 的即用即弃 GPU 运行时引擎
[English](README.md) | [简体中文](README_zh.md)

[![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.26.5-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20amd64-blue?style=flat&logo=linux)](https://github.com/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Tested Colab CLI](https://img.shields.io/badge/Colab%20CLI-v0.7.2%20测试通过-orange?style=flat&logo=googlecolab)](https://colab.research.google.com/)

**CBox** 是一个使用 Go 语言开发的轻量级、高性能即用即弃式（Ephemeral）远程 GPU 运行时引擎。它为算法研究员与开发者提供了与 **Docker 极度相似的命令行体验**，支持**蓝图镜像构建**、**智能数据卷同步**、**后台无缝常驻执行**以及**多服务 Compose 编排**。

目前首选实现的云端提供商为 **Google Colab**。CBox 将底层复杂的 SSH 隧道隧道配置、OAuth 凭据交换、tmux 后台保持以及云端会话生命周期彻底封装，让云端即用即弃的 GPU 虚拟机用起来就像本地 Docker 容器一样简单自然。

---

## 目录

- [核心特性](#核心特性)
- [系统架构](#系统架构)
- [核心概念深度解析](#核心概念深度解析)
  - [1. 镜像与 Cboxfile](#1-镜像与-cboxfile)
  - [2. 远程同步数据卷 (Volumes)](#2-远程同步数据卷-volumes)
  - [3. GPU 优先级调度与运行时池](#3-gpu-优先级调度与运行时池)
- [环境依赖与安装](#环境依赖与安装)
  - [系统要求](#系统要求)
  - [一键自动安装](#一键自动安装)
  - [手动源码编译](#手动源码编译)
  - [环境体检 (doctor)](#环境体检-doctor)
- [Colab 认证与配置文件隔离](#colab-认证与配置文件隔离)
- [快速上手指南](#快速上手指南)
- [多容器编排 (Compose)](#多容器编排-compose)
- [CLI 命令速查手册](#cli-命令速查手册)
- [技术要点与最佳实践](#技术要点与最佳实践)
- [未来路线图 (Roadmap)](#未来路线图-roadmap)
- [开源协议](#开源协议)

---

## 核心特性

- 🐳 **类 Docker 极简 CLI 体验**：完美贴合现有操作习惯（`cbox run`、`cbox ps`、`cbox logs`、`cbox exec`、`cbox stop`、`cbox rm`、`cbox stats`）。
- 📦 **蓝图镜像（`Cboxfile`）**：支持 `FROM`、`APT`、`PIP`、`RUN` 声明式构建，具备清单哈希计算与远程镜像增量缓存机制。
- 🔄 **智能远程数据卷同步**：提供 4 种挂载模式（`ro` 只读源、`rw` 双向同步、`output` 定期增量产物同步、`cache` 远端高速缓存），支持进程退出时的可靠强制同步。
- ⚡ **独立守护进程引擎（`cboxd`）**：纯 Go 实现（基于 `modernc.org/sqlite`，零 CGO 依赖），提供 Unix Socket HTTP API 与透明自动拉起机制。
- 🎯 **GPU 智能调度与实例复用**：支持按优先级申报算力（如 `--gpu L4,T4`），运行时实例池化复用，空闲超时自动回收，避免冷启动等待。
- 🖥️ **OpenSSH 多路复用与原生 PTY 交互**：利用 OpenSSH `ControlMaster` 长连接与 Colab `ProxyCommand`，支持交互式伪终端直连（`cbox exec -it`）。
- 🎼 **多容器服务编排（Compose）**：支持通过 `cbox-compose.yaml` 声明式定义并一键拉起/销毁多个 GPU 任务。
- 🩺 **内置环境诊断与一键配置**：内置 `cbox doctor` 进行全链路诊断，`cbox setup` 自动完成 Colab CLI 及依赖安装。

---

## 系统架构

```text
                        开发者 / 用户终端
                                │
                                ▼
                     ┌────────────────────┐
                     │    cbox (CLI)      │
                     │    Cobra 命令行    │
                     └──────────┬─────────┘
                                │
                       Unix HTTP REST API
                                │
                                ▼
                     ┌────────────────────┐
                     │   cboxd (后台引擎)  │
                     │   Daemon Engine    │
                     └──────────┬─────────┘
                                │
        ┌───────────────────────┼───────────────────────┐
        │                       │                       │
        ▼                       ▼                       ▼
 ┌──────────────┐        ┌──────────────┐        ┌──────────────┐
 │  容器生命周期  │        │   镜像构建   │        │   数据卷管理  │
 │  Container   │        │    Image     │        │    Volume    │
 └──────┬───────┘        └──────────────┘        └──────────────┘
        │
        ▼
 ┌──────────────┐
 │   调度算法   │  (GPU 规格优先级匹配、运行时池实例分配)
 │  Scheduler   │
 └──────┬───────┘
        │
        ▼
 ┌──────────────┐
 │ 运行时生命周期│  (会话保活、心跳监测、空闲实例自动回收)
 │   Runtime    │
 └──────┬───────┘
        │
        ▼
 ┌──────────────┐
 │ 云厂商抽象接口│  (可插拔 Provider 架构，未来兼容 RunPod/GCP/SSH)
 │   Provider   │
 └──────┬───────┘
        │
        ▼
 ┌──────────────┐
 │ColabProvider │  (驱动 google-colab-cli、OAuth 会话凭据与隧道)
 └──────┬───────┘
        │
        ▼
 ┌──────────────┐
 │ Colab GPU VM │  (云端 GPU 虚拟机，通过 SSH 代理与多路复用通道连接)
 └──────────────┘
```

---

## 核心概念深度解析

### 1. 镜像与 `Cboxfile`

`Cboxfile` 用于在云端运行时上声明可复现的执行环境。其语法结构清晰简洁：

```dockerfile
# 基础镜像环境
FROM colab/python:3

# 远端系统依赖包（通过 apt-get 自动安装）
APT git rsync libgl1 tmux

# 远端 Python 库依赖（通过 pip 自动安装）
PIP torch torchvision torchaudio
PIP mmengine mmcv mmsegmentation

# 环境变量与工作目录
ENV PYTHONPATH=/workspace
WORKDIR /workspace

# 默认启动命令
CMD ["bash"]
```

执行 `cbox build -t my-task:latest .` 时，CBox 会计算配置清单的 SHA-256 哈希值并在远端运行时记录，避免重复安装相同依赖，极速复用环境。

### 2. 远程同步数据卷 (Volumes)

CBox 底层基于安全的 SSH 隧道与 `rsync` 协议实现本地与远程环境之间的数据同步。针对深度学习训练场景设计了 4 种专用挂载模式：

| 模式 | 同步流向 | 同步时机与行为 | 典型适用场景 |
| :--- | :--- | :--- | :--- |
| `ro` (只读) | 本地 $\to$ 远端 | 容器启动前单向推送；远端若有修改不会回传覆盖本地。 | 大规模数据集、预训练模型权重、代码配置 |
| `rw` (双向读写) | 本地 $\leftrightarrow$ 远端 | 启动前拉取，容器运行期间或退出后双向同步。 | 协同工作区、交互式调试代码 |
| `output` (产物输出) | 远端 $\to$ 本地 | 容器运行期间定期增量同步（默认每 10 分钟），进程退出时强制最终同步。 | 训练检查点（checkpoints）、评估指标、训练日志 |
| `cache` (远端缓存) | 远端保留 | 仅保留在远程虚拟机特定路径中，跨容器生命周期复用。 | Pip 缓存轮子、Hugging Face / PyTorch Hub 缓存 |

### 3. GPU 优先级调度与运行时池

在提交任务时，你可以指定 GPU 的首选与降级偏好（逗号分隔）：

```bash
cbox run --gpu L4,T4 ...
```

CBox 的调度器（Scheduler）策略：
1. **优先复用**：检查本地运行时池（Runtime Pool）中是否存在符合规格且处于空闲（idle）状态的已有实例；若存在则直接秒级复用，免去虚拟机冷启动时间。
2. **动态申请**：若没有可用实例，调用 Provider 动态申请匹配规格的云端实例。
3. **保活等待**：容器执行完毕后，该云端实例不会立刻被注销，而是保留在池中继续保活（默认空闲保留 30 分钟）。若连续跑批或快速排查，后续容器可以即刻启动。

---

## 环境依赖与安装

### 系统要求

- **操作系统**：Linux (amd64)
- **Go 语言环境**：Go **1.26.5** 或更高版本
- **系统工具**：OpenSSH 客户端 (`ssh`)、`rsync`、`git`、`tmux`
- **Python 工具**：`uv`（现代极速 Python 包管理器）
- **Colab 工具**：`google-colab-cli`（建议版本 `0.7.2` 及以上）

### 一键自动安装

项目内置了一键配置与安装脚本，会自动安装系统依赖、uv、Colab CLI、编译 CBox 二进制文件并配置好默认环境：

```bash
bash scripts/install.sh
```

- 普通用户执行时，程序将安装至 `~/.local/bin/`。
- `root` 用户执行时，程序将安装至 `/usr/local/bin/`。

请确保 `~/.local/bin` 已添加到环境变量中：

```bash
export PATH="${HOME}/.local/bin:${PATH}"
```

### 手动源码编译

```bash
# 1. 编译本地二进制文件至 ./bin/ (cbox 与 cboxd)
make build

# 2. 静态编译并安装至 ~/.local/bin/
make install

# 3. 自动安装 Colab CLI 依赖并引导 Google 账户 OAuth 登录
cbox setup

# 4. 全面体检依赖与凭证健康状态
cbox doctor
```

如果更新了源码并重新编译，重启后台守护进程即可：

```bash
cbox daemon stop
make install
cbox daemon start
```

### 环境体检 (doctor)

随时可以使用 `cbox doctor` 检查本地依赖与网络连接健康度：

```bash
cbox doctor
```

输出示例：
```text
CBox Environment Diagnosis:
----------------------------------------------------------------------
[✔] SSH Client             : OpenSSH_9.6p1
[✔] Rsync                  : version 3.2.7
[✔] Git                    : version 2.43.0
[✔] uv Package Manager     : uv 0.4.18
[✔] Colab CLI              : colab 0.7.2
[✔] Colab Credentials      : OK
[✔] CBox Daemon Engine     : Running (PID: 12345)
----------------------------------------------------------------------
All required dependencies are satisfied! CBox is ready to use.
```

---

## Colab 认证与配置文件隔离

Google Colab 在初次调用或凭据过期时会触发浏览器/终端 OAuth 授权。

- **默认 Profile**：复用存储在 `~/.config/colab-cli/token.json` 中的用户凭证。其实例会话状态保存在 `<data_dir>/profiles/default/config.json`。
- **默认数据目录**：默认路径为 `~/.local/share/cbox`（严格遵循 `XDG_DATA_HOME` 规范）。
- **手动查询与销毁云端实例**：所有 Colab 实例的管理命令均使用统一的会话状态文件。如果需要手动排查或强制释放 CBox 的云端实例，请在子命令**前**传入 `--config` 全局参数：

```bash
# 查看 CBox 当前正在托管的 Colab 会话
colab --config ~/.local/share/cbox/profiles/default/config.json sessions

# 手动强制释放指定会话（SESSION 为实例 ID）
colab --config ~/.local/share/cbox/profiles/default/config.json stop -s <SESSION_ID>
```

- **多租户/命名 Profile**：使用 `<data_dir>/profiles/<name>/home` 隔离凭据；可选通过 `--client-oauth-config` 挂载企业或独立客户端配置。

---

## 快速上手指南

### 第一步：启动后台引擎

```bash
cbox daemon start
```

> **提示**：如果在执行客户端命令时 `cboxd` 未处于运行状态，客户端会自动无缝将其在后台拉起。若需前台排查，可使用 `cbox daemon run --debug`。

### 第二步：编写 Cboxfile 并构建镜像

```bash
mkdir -p /tmp/cbox-gpu-demo
cat << 'EOF' > /tmp/cbox-gpu-demo/Cboxfile
FROM colab/python:3
APT tmux rsync
PIP numpy
EOF

# 构建镜像并打标签
cbox build -t gpu-demo:latest /tmp/cbox-gpu-demo

# 查看已构建的镜像
cbox images
```

### 第三步：在后台启动 GPU 容器

申请一块 `T4`（或按需设为 `L4,T4`）GPU，在容器中启动后台长跑任务：

```bash
cbox run -d \
  --name gpu-demo \
  --gpu T4 \
  --workdir /content \
  -e LD_LIBRARY_PATH=/usr/lib64-nvidia \
  gpu-demo:latest -- bash -c 'echo "CBox GPU 任务已启动"; nvidia-smi; sleep 600'
```

> **注意**：Colab 远程环境的 NVIDIA 驱动运行库默认位于 `/usr/lib64-nvidia`。建议在容器启动（`-e`）以及非交互式 `cbox exec` 中传入 `LD_LIBRARY_PATH=/usr/lib64-nvidia`。

### 第四步：监控、日志与交互

```bash
# 查看正在运行中的容器列表
cbox ps

# 实时跟随容器的输出日志
cbox logs -f gpu-demo

# 在正在运行的容器中执行单条 GPU 命令
cbox exec gpu-demo -- nvidia-smi --query-gpu=name,memory.total --format=csv,noheader

# 验证远端 PyTorch 与 CUDA 算力状态
cbox exec gpu-demo -- python3 -c \
  'import torch; print("CUDA 是否可用:", torch.cuda.is_available()); print("设备型号:", torch.cuda.get_device_name(0))'

# 接入交互式终端（分配独立 PTY，全彩终端）
cbox exec -it gpu-demo -- bash

# 查看容器低层底层完整元数据 (JSON)
cbox inspect gpu-demo
```

### 第五步：停止与清理

```bash
# 停止容器并移除容器记录
cbox stop gpu-demo
cbox rm gpu-demo
```

容器移除后，底层的 Colab 实例会在空闲池中继续维持 30 分钟，在此期间若再次运行新容器将获得近乎瞬间启动的体验。

---

## 多容器编排 (Compose)

CBox 支持通过类似 Docker Compose 的语法管理多个关联服务的声明式配置。

### 示例配置文件：`cbox-compose.yaml`

```yaml
version: "1"

services:
  training-l4:
    image: gpu-demo:latest
    gpu:
      preference:
        - L4
        - T4
    workdir: /content
    env:
      LD_LIBRARY_PATH: /usr/lib64-nvidia
    volumes:
      - source: ./data
        target: /data
        mode: ro
      - source: ./checkpoints
        target: /checkpoints
        mode: output
    command:
      - python3
      - -c
      - 'import torch, time; print("正在使用设备训练:", torch.cuda.get_device_name(0)); time.sleep(60)'
    restart: unless-stopped

  eval-t4:
    image: gpu-demo:latest
    gpu:
      preference:
        - T4
    workdir: /content
    env:
      LD_LIBRARY_PATH: /usr/lib64-nvidia
    command:
      - nvidia-smi
```

### Compose 管理命令

```bash
# 在后台一键拉起所有服务
cbox compose up -d

# 查看编排服务运行状态
cbox compose ps

# 查看所有服务的聚合日志
cbox compose logs

# 停止并清理编排的所有服务
cbox compose down
```

---

## CLI 命令速查手册

### 容器管理命令 (Container)

| 命令 | 说明 | 常用参数 |
| :--- | :--- | :--- |
| `cbox run` | 创建并在远端启动新容器 | `-d` (后台), `--name`, `--gpu`, `-v`, `-e`, `-w`, `--restart` |
| `cbox create` | 仅创建容器元数据，不启动 | `--name`, `--gpu`, `-v`, `-e`, `-w` |
| `cbox start` | 启动已停止的容器 | `-a, --attach` (前台并接收日志) |
| `cbox stop` | 优雅停止运行中的容器 | `-t, --time <秒数>` (超时强制杀死) |
| `cbox restart` | 重启容器 | `-t, --time <秒数>` |
| `cbox rm` | 移除容器记录 | `-f, --force` (强制删除运行中容器) |
| `cbox ps` | 列出容器列表 | `-a, --all`, `--format json` |
| `cbox inspect` | 查询容器底层详细状态 JSON | `cbox inspect <容器名或ID>` |
| `cbox logs` | 查看或跟踪容器输出日志 | `-f, --follow` (实时流式输出) |
| `cbox exec` | 在容器中执行命令 | `-i` (保持标准输入), `-t` (分配 PTY 终端) |
| `cbox stats` | 查看容器资源使用统计 | `cbox stats <容器名或ID>` |

### 蓝图镜像管理命令 (Image)

| 命令 | 说明 | 常用参数 |
| :--- | :--- | :--- |
| `cbox build` | 基于 `Cboxfile` 构建镜像 | `-t, --tag <名称:标签>`, `-f, --file` |
| `cbox images` | 列出本地所有镜像元数据 | `--format json` |
| `cbox rmi` | 删除指定镜像标签 | `cbox rmi <镜像名称或ID>` |

### 存储与数据卷管理 (Volume)

| 命令 | 说明 | 常用参数 |
| :--- | :--- | :--- |
| `cbox volume create` | 创建命名数据卷定义 | `--source <路径>`, `--mode <ro\|rw\|output\|cache>` |
| `cbox volume ls` | 列出所有数据卷 | `--format json` |
| `cbox volume inspect`| 查看数据卷详细元数据 | `cbox volume inspect <卷名>` |
| `cbox volume rm` | 删除数据卷配置 | `cbox volume rm <卷名>` |

### 服务编排管理 (Compose)

| 命令 | 说明 | 常用参数 |
| :--- | :--- | :--- |
| `cbox compose up` | 创建并拉起全部服务 | `-f <文件>`, `-d, --detach` |
| `cbox compose down` | 停止并移除编排服务 | `-f <文件>` |
| `cbox compose ps` | 查看编排服务的运行状态 | `-f <文件>` |
| `cbox compose logs` | 查看编排服务的聚合日志 | `-f <文件>` |

### 上下文与守护引擎 (Context & Daemon)

| 命令 | 说明 |
| :--- | :--- |
| `cbox context create` | 创建新的云服务提供商或环境上下文 (`--provider`, `--profile`) |
| `cbox context ls` | 列出所有已配置的上下文 |
| `cbox context use` | 切换当前活动的上下文 |
| `cbox daemon start` | 在后台启动 `cboxd` 守护引擎 |
| `cbox daemon stop` | 停止正在运行的 `cboxd` 引擎 |
| `cbox daemon restart` | 重启 `cboxd` 引擎 |
| `cbox daemon status` | 检查守护引擎进程 PID、Socket 路径及存活状态 |
| `cbox daemon run` | 在前台启动守护引擎（支持 `--debug` 与 `--tcp` 监听） |

### 诊断与系统工具 (Doctor & Setup)

| 命令 | 说明 |
| :--- | :--- |
| `cbox doctor` | 全面检查系统工具、Python 环境、网络与凭据状态 |
| `cbox setup` | 自动化安装 Colab CLI 并引导完成 Google 授权流程 |
| `cbox events` | 获取并实时监听服务端产生的事件流 (`-n <条数>`) |
| `cbox version` | 查看客户端与服务端的版本号、编译参数及平台架构 |

---

## 技术要点与最佳实践

1. **GPU 驱动运行库路径 (`LD_LIBRARY_PATH`)**：
   在 Colab 提供的非交互式 SSH 环境中，CUDA 驱动的用户态文件存放在 `/usr/lib64-nvidia`。运行 GPU 任务或执行 Python 脚本前，务必指定环境变量 `-e LD_LIBRARY_PATH=/usr/lib64-nvidia`，或者在命令中预设 `export LD_LIBRARY_PATH=/usr/lib64-nvidia`。
2. **命令参数安全隔离符 (`--`)**：
   当要执行的远程命令包含它自己的 Flag（例如 `-c`、`--query-gpu` 等）时，应在命令前使用 `--` 分隔，例如：`cbox run ... image -- bash -c '...'`。这样可以保证参数不被 `cbox` 本身的命令行解析器拦截。
3. **会话保活与即时释放**：
   容器退出后，对应的 Colab GPU 虚拟机会在实例池中保活等待 30 分钟。如果你不需要再次执行任务且希望立即释放云端 GPU 配额，可通过 `cbox inspect <name>` 获取 `runtime_id`，然后运行以下命令释放：
   ```bash
   colab --config ~/.local/share/cbox/profiles/default/config.json stop -s <RUNTIME_ID>
   ```
4. **数据卷同步安全细节**：
   - `ro` 卷在容器启动前完成同步，属于单向推送，不会回写本地。
   - `output` 卷由后台 worker 定期增量回传（默认 10 分钟一次），在容器退出清理时会触发最后一次阻断式拉取，确保模型权重或重要日志绝对不丢失。
5. **交互式 PTY 终端**：
   通过 `cbox exec -it <容器名> -- bash` 进入容器时，底层直接调起 OpenSSH 会话并申请独立 PTY，完美支持方向键、Tab 自动补全、Ctrl+C 中断以及全彩色终端显示。

---

## 开源协议

本项目采用 [MIT 许可证](LICENSE) 开源。
