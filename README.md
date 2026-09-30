# CBox: Docker-like Ephemeral GPU Runtime Engine (Go)

[English](README.md) | [简体中文](README_zh.md)

[![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.26.5-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20amd64-blue?style=flat&logo=linux)](https://github.com/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Tested Colab CLI](https://img.shields.io/badge/Colab%20CLI-v0.7.2%20tested-orange?style=flat&logo=googlecolab)](https://colab.research.google.com/)

**CBox** is a high-performance, lightweight ephemeral runtime engine written in Go. It empowers developers and researchers to manage cloud GPU runtimes and remote processes through familiar **Docker-style commands**, **blueprint images**, **synchronized volumes**, **detached background execution**, and **multi-service compose orchestration**.

Google Colab is the currently supported primary cloud runtime provider, shielding users from raw SSH configurations, OAuth token dances, and ephemeral session lifecycles.

---

## Table of Contents

- [Features](#features)
- [Architecture](#architecture)
- [Core Concepts](#core-concepts)
  - [Images and Cboxfile](#1-images-and-cboxfile)
  - [Remote Synchronized Volumes](#2-remote-synchronized-volumes)
  - [GPU Preference & Runtime Pool](#3-gpu-preference--runtime-pool)
- [Prerequisites & Installation](#prerequisites--installation)
  - [Quick Automated Setup](#quick-automated-setup)
  - [Manual Compilation](#manual-compilation)
  - [Environment Health Check](#environment-health-check)
- [Colab Authentication & Profiles](#colab-authentication--profiles)
- [Quick Start](#quick-start)
- [Multi-Service Compose](#multi-service-compose)
- [CLI Reference](#cli-reference)
- [Implementation Notes & Best Practices](#implementation-notes--best-practices)
- [Roadmap](#roadmap)
- [License](#license)

---

## Features

- 🐳 **Docker-like CLI Experience**: Familiar syntax (`cbox run`, `cbox ps`, `cbox logs`, `cbox exec`, `cbox stop`, `cbox start`, `cbox rm`, `cbox stats`).
- 📦 **Blueprint Images (`Cboxfile`)**: Build manifests with `FROM`, `APT`, `PIP`, and `RUN`, complete with manifest hashing and remote image caching.
- 🔄 **Synchronized Remote Volumes**: Flexible mount modes (`ro`, `rw`, `output`, `cache`) with periodic background rsync and final guaranteed exit sync.
- ⚡ **Detached Daemon Engine (`cboxd`)**: Background Go daemon backed by pure Go SQLite (`modernc.org/sqlite`, zero CGO), Unix socket HTTP API, and auto-start capability.
- 🎯 **Smart Scheduler & Runtime Pool**: Priority-based GPU matching (e.g. `--gpu L4,T4`), runtime instance reuse, and idle runtime auto-expiry.
- 🖥️ **OpenSSH Multiplexing & Native PTY**: OpenSSH `ControlMaster` + Colab `ProxyCommand` with direct terminal PTY attachment for interactive shells (`cbox exec -it`).
- 🎼 **Multi-Container Compose**: Declarative multi-service orchestration via `cbox-compose.yaml`.
- 🩺 **Built-in Diagnostics & Setup**: `cbox doctor` for end-to-end environment validation and `cbox setup` for automated dependency setup.

---

## Architecture

```text
                         Developer / User
                                │
                                ▼
                     ┌────────────────────┐
                     │    cbox (CLI)      │
                     │    Cobra Client    │
                     └──────────┬─────────┘
                                │
                       Unix HTTP REST API
                                │
                                ▼
                     ┌────────────────────┐
                     │   cboxd (Daemon)   │
                     │    Engine Core     │
                     └──────────┬─────────┘
                                │
        ┌───────────────────────┼───────────────────────┐
        │                       │                       │
        ▼                       ▼                       ▼
 ┌──────────────┐        ┌──────────────┐        ┌──────────────┐
 │  Container   │        │    Image     │        │    Volume    │
 │   Service    │        │   Service    │        │   Service    │
 └──────┬───────┘        └──────────────┘        └──────────────┘
        │
        ▼
 ┌──────────────┐
 │  Scheduler   │  (GPU preference matching & pool allocation)
 └──────┬───────┘
        │
        ▼
 ┌──────────────┐
 │   Runtime    │  (Session lifecycle, keep-alive, idle reaper)
 │   Service    │
 └──────┬───────┘
        │
        ▼
 ┌──────────────┐
 │   Provider   │  (Pluggable Provider Interface)
 └──────┬───────┘
        │
        ▼
 ┌──────────────┐
 │    Colab     │  (google-colab-cli wrapper + OAuth context)
 │   Provider   │
 └──────┬───────┘
        │
        ▼
 ┌──────────────┐
 │ Colab VM GPU │  (Ephemeral GPU instance connected via SSH Proxy)
 └──────────────┘
```

---

## Core Concepts

### 1. Images and `Cboxfile`

`Cboxfile` defines reproducible environments on top of cloud runtimes. Similar to Dockerfiles, it specifies base configurations and setup steps:

```dockerfile
FROM colab/python:3

# Remote system packages installed via apt-get
APT git rsync libgl1 tmux

# Remote Python dependencies installed via pip
PIP torch torchvision torchaudio
PIP mmengine mmcv mmsegmentation

# Environment and working directory
ENV PYTHONPATH=/workspace
WORKDIR /workspace

CMD ["bash"]
```

When building an image (`cbox build -t my-task:latest .`), CBox hashes the manifest and caches installation steps on the remote runtime to prevent redundant downloads.

### 2. Remote Synchronized Volumes

CBox uses `rsync` over SSH tunnels to manage files between local workspaces and remote runtimes. Four volume modes are supported:

| Mode | Direction | Behavior | Common Use Cases |
| :--- | :--- | :--- | :--- |
| `ro` | Local $\to$ Remote | Synced before container starts. Remote files are not synced back. | Datasets, pretrained weights, configs |
| `rw` | Local $\leftrightarrow$ Remote | Two-way synchronization before start and after exit. | Collaborative workspaces, interactive debugging |
| `output` | Remote $\to$ Local | Periodically synced to local disk during execution and finalized at process exit. | Checkpoints, training logs, evaluation artifacts |
| `cache` | Remote-only | Preserved on remote runtime for reuse across container invocations. | Pip wheels, Hugging Face / Torch hub cache |

### 3. GPU Preference & Runtime Pool

Specify GPU preferences using comma-separated fallbacks:

```bash
cbox run --gpu L4,T4 ...
```

The scheduler inspects the local runtime pool:
1. Reuses an existing idle runtime matching the requested GPU tier.
2. If none is available, provisions a new instance from the provider.
3. Keeps runtimes alive after container completion for a configurable timeout (default `30m`), drastically cutting cold-start latency for sequential workflows.

---

## Prerequisites & Installation

### Requirements

- **Linux** (amd64)
- **Go 1.26.5** or newer
- **OpenSSH client** (`ssh`) & **rsync**
- **uv** (fast Python package manager)
- **google-colab-cli** (tested against `v0.7.2` as of 2026-09-30)

### Quick Automated Setup

Run the included install script to install system dependencies, build CBox, configure the systemd user service, and set up the default profile:

```bash
bash scripts/install.sh
```

- When run as a standard user, binaries are installed to `~/.local/bin/`.
- When run as `root`, binaries are installed to `/usr/local/bin/`.

Ensure `~/.local/bin` is in your `PATH`:

```bash
export PATH="${HOME}/.local/bin:${PATH}"
```

### Manual Compilation

```bash
# 1. Build local binaries to ./bin/ (cbox and cboxd)
make build

# 2. Build static standalone binaries and install to ~/.local/bin
make install

# 3. Automatic installation of Colab CLI dependencies and trigger OAuth
cbox setup

# 4. Verify system environment
cbox doctor
```

To update an existing installation and reload the daemon:

```bash
cbox daemon stop
make install
cbox daemon start
```

### Environment Health Check

Run `cbox doctor` to inspect the local environment:

```bash
cbox doctor
```

Example diagnostic report:
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

## Colab Authentication & Profiles

Google Colab initiates OAuth authentication when credentials are needed.

- **Default Profile**: Reuses user credentials stored in `~/.config/colab-cli/token.json`. Session state is maintained independently in `<data_dir>/profiles/default/config.json`.
- **Default Data Directory**: Defaults to `~/.local/share/cbox` (or respects `XDG_DATA_HOME`).
- **Manual Runtime Inspection / Termination**: Colab lifecycle commands and generated SSH proxies share this session state. To query or stop CBox sessions manually, pass `--config` before the subcommand:

```bash
# Query active CBox Colab sessions
colab --config ~/.local/share/cbox/profiles/default/config.json sessions

# Terminate a session explicitly
colab --config ~/.local/share/cbox/profiles/default/config.json stop -s <SESSION_ID>
```

- **Named Profiles**: Store isolated credentials under `<data_dir>/profiles/<name>/home`. An optional `oauth.json` client configuration can be supplied via `--client-oauth-config`.

---

## Quick Start

### Step 1: Start the Background Engine

```bash
cbox daemon start
```

> **Note**: Client commands automatically start `cboxd` in the background if it is not running. To run in the foreground with verbose logs, use `cbox daemon run --debug`.

### Step 2: Create a Blueprint Image

Create a directory with a minimal `Cboxfile`:

```bash
mkdir -p /tmp/cbox-gpu-demo
cat << 'EOF' > /tmp/cbox-gpu-demo/Cboxfile
FROM colab/python:3
APT tmux rsync
PIP numpy
EOF

cbox build -t gpu-demo:latest /tmp/cbox-gpu-demo
cbox images
```

### Step 3: Run a Detached GPU Container

Run a container in the background requesting a `T4` GPU (or fallback `L4,T4`), mounting local folders for datasets and outputs:

```bash
cbox run -d \
  --name gpu-demo \
  --gpu T4 \
  --workdir /content \
  -e LD_LIBRARY_PATH=/usr/lib64-nvidia \
  gpu-demo:latest -- bash -c 'echo "CBox GPU worker started"; nvidia-smi; sleep 600'
```

> **Important**: The Colab remote SSH environment requires `LD_LIBRARY_PATH=/usr/lib64-nvidia` to locate NVIDIA CUDA runtime and driver libraries. Set this via `-e` for container commands and noninteractive `cbox exec`.

### Step 4: Monitor, Inspect, and Interact

```bash
# List running containers
cbox ps

# Stream container stdout / stderr
cbox logs -f gpu-demo

# Execute an ad-hoc GPU command inside the running container
cbox exec gpu-demo -- nvidia-smi --query-gpu=name,memory.total --format=csv,noheader

# Check PyTorch CUDA availability
cbox exec gpu-demo -- python3 -c \
  'import torch; print("CUDA available:", torch.cuda.is_available()); print("Device:", torch.cuda.get_device_name(0))'

# Attach an interactive terminal (SSH PTY)
cbox exec -it gpu-demo -- bash

# Inspect detailed JSON state
cbox inspect gpu-demo
```

### Step 5: Stop and Clean Up

```bash
# Stop and remove the container
cbox stop gpu-demo
cbox rm gpu-demo
```

When a container is removed, the remote Colab runtime remains in the idle pool for 30 minutes, allowing future containers to start instantly without re-provisioning.

---

## Multi-Service Compose

CBox supports multi-service orchestration with `cbox-compose.yaml`.

### Example `cbox-compose.yaml`

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
      - 'import torch, time; print("Training on:", torch.cuda.get_device_name(0)); time.sleep(60)'
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

### Compose Commands

```bash
# Launch all compose services in background
cbox compose up -d

# Check service status
cbox compose ps

# View service logs
cbox compose logs

# Stop and clean up all services
cbox compose down
```

---

## CLI Reference

### Container Management

| Command | Description | Example Flags |
| :--- | :--- | :--- |
| `cbox run` | Create and start a container | `-d`, `--name`, `--gpu`, `-v`, `-e`, `-w`, `--restart` |
| `cbox create` | Create a container without starting | `--name`, `--gpu`, `-v`, `-e`, `-w` |
| `cbox start` | Start stopped container(s) | `-a, --attach` |
| `cbox stop` | Gracefully stop running container(s) | `-t, --time <seconds>` |
| `cbox restart` | Restart container(s) | `-t, --time <seconds>` |
| `cbox rm` | Remove container record | `-f, --force` |
| `cbox ps` | List containers | `-a, --all`, `--format json` |
| `cbox inspect` | Inspect low-level JSON details | `cbox inspect <name>` |
| `cbox logs` | View container execution logs | `-f, --follow` |
| `cbox exec` | Execute commands in running container | `-i`, `-t` (direct SSH PTY attachment) |
| `cbox stats` | View container resource utilization | `cbox stats <name>` |

### Blueprint Image Management

| Command | Description | Example Flags |
| :--- | :--- | :--- |
| `cbox build` | Build an image from a `Cboxfile` | `-t, --tag <name:tag>`, `-f, --file` |
| `cbox images` | List local blueprint images | `--format json` |
| `cbox rmi` | Remove an image tag | `cbox rmi <image>` |

### Storage & Volumes

| Command | Description | Example Flags |
| :--- | :--- | :--- |
| `cbox volume create` | Create a named volume mapping | `--source <path>`, `--mode <ro\|rw\|output\|cache>` |
| `cbox volume ls` | List configured volumes | `--format json` |
| `cbox volume inspect`| Inspect volume metadata | `cbox volume inspect <name>` |
| `cbox volume rm` | Delete a volume configuration | `cbox volume rm <name>` |

### Multi-Container Orchestration

| Command | Description | Example Flags |
| :--- | :--- | :--- |
| `cbox compose up` | Create and launch all compose services | `-f <file>`, `-d, --detach` |
| `cbox compose down` | Stop and remove all compose services | `-f <file>` |
| `cbox compose ps` | List status of compose services | `-f <file>` |
| `cbox compose logs` | View logs from compose services | `-f <file>` |

### Contexts & Daemon Engine

| Command | Description |
| :--- | :--- |
| `cbox context create` | Create an execution context (`--provider`, `--profile`) |
| `cbox context ls` | List available contexts |
| `cbox context use` | Switch active context |
| `cbox daemon start` | Start `cboxd` background engine |
| `cbox daemon stop` | Stop `cboxd` background engine |
| `cbox daemon restart` | Restart `cboxd` background engine |
| `cbox daemon status` | Show daemon PID, socket, and health status |
| `cbox daemon run` | Run engine in foreground (`--debug`, `--tcp`) |

### System & Diagnostics

| Command | Description |
| :--- | :--- |
| `cbox doctor` | Comprehensive diagnostic check of tools, network, and credentials |
| `cbox setup` | Automated installation of Colab CLI and OAuth bootstrap |
| `cbox events` | Real-time stream of engine events (`-n <limit>`) |
| `cbox version` | Display client and daemon version info |

---

## Implementation Notes & Best Practices

1. **GPU Driver Path**:
   Colab's SSH environment places NVIDIA driver user-space libraries in `/usr/lib64-nvidia`. Always pass `-e LD_LIBRARY_PATH=/usr/lib64-nvidia` or set it in your interactive shell (`export LD_LIBRARY_PATH=/usr/lib64-nvidia`) before executing GPU programs.
2. **Command Flag Boundary (`--`)**:
   When invoking commands that accept their own flags, place `--` before the command (e.g. `cbox run ... image -- bash -c '...'`). Arguments after `--` are forwarded verbatim to the remote process.
3. **Session Re-use & Teardown**:
   When a container finishes or is removed, the Colab VM remains alive in the pool for 30 minutes to facilitate instant restart for subsequent runs. If you want to release cloud GPU quota immediately, inspect the `runtime_id` using `cbox inspect <name>` and terminate the session with:
   ```bash
   colab --config ~/.local/share/cbox/profiles/default/config.json stop -s <RUNTIME_ID>
   ```
4. **Volume Sync Mechanics**:
   - `ro` volumes are synchronized upon container creation. Note that filesystem-level write restrictions are not enforced on the remote side; changes made remotely are simply discarded.
   - `output` volumes sync remote results back to the local host periodically (default interval `10m`) and perform a final synchronous pull when the container process terminates.
5. **Interactive TTY Attachment**:
   Using `cbox exec -it <container> -- bash` connects directly through OpenSSH multiplexed sockets with a pseudo-terminal allocated (`ssh -tt`), preserving full ANSI colors, terminal dimensions, and key bindings.

---

## Roadmap

- [ ] **Multi-Cloud Providers**: First-class support for RunPod, GCP Compute Engine, custom SSH bastions, and local GPU workstations.
- [ ] **Real-time Telemetry**: Streaming GPU memory utilization and compute load charts in `cbox stats`.
- [ ] **Dynamic Distributed Scaling**: Support for multi-node distributed PyTorch / DeepSpeed clusters across multiple ephemeral runtimes.
- [ ] **Snapshot Checkpoint Sync**: Automated remote snapshotting and fast-resume integration.

---

## License

This project is licensed under the [MIT License](LICENSE).
