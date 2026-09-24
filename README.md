# CBox: Docker-like Ephemeral GPU Runtime Engine (Go)

CBox is an Ephemeral GPU Runtime Engine written in Go. It abstracts remote disposable GPU instances (such as Google Colab, and in the future RunPod, GCP, SSH Server, Local GPU) into Docker-like containers with declarative blueprint images, synchronized volumes, detached execution, automatic recovery, and compose orchestration.

## Features

- **Docker-like CLI**: `cbox run`, `cbox ps`, `cbox logs`, `cbox exec`, `cbox stop`, `cbox start`, `cbox restart`, `cbox rm`, `cbox stats`
- **Declarative Images (`Cboxfile`)**: DSL supporting `FROM`, `APT`, `PIP`, `ENV`, `WORKDIR`, `COPY`, `RUN`, `CMD` with reproducible SHA256 hashing and remote caching
- **Synchronized Remote Volumes**: `ro` (read-only), `rw` (read-write), `output` (periodic background sync + final exit sync), `cache` (content-hashed cache reuse)
- **Detached Lifecycle Engine (`cboxd`)**: High-performance pure Go daemon with SQLite (`modernc.org/sqlite` pure Go, no CGO) and Unix Domain Socket HTTP API
- **Scheduler & Runtime Pool**: Intelligent runtime reuse, GPU preference matching, idle runtime pool management with automatic timeout reaper
- **OpenSSH Multiplexing**: OpenSSH `ControlMaster` + Colab `ProxyCommand` with direct PTY attachment for interactive exec
- **Multi-Context & Compose**: Multi-account profile isolation and multi-service `cbox-compose.yaml` orchestration

## Architecture

```text
                         User
                           │
                           ▼
                    ┌────────────┐
                    │    cbox    │
                    │ Cobra CLI  │
                    └─────┬──────┘
                          │
                   Unix HTTP API
                          │
                          ▼
                  ┌──────────────┐
                  │    cboxd     │
                  │    Engine    │
                  └──────┬───────┘
                         │
       ┌─────────────────┼─────────────────┐
       │                 │                 │
       ▼                 ▼                 ▼
 ContainerService   ImageService     VolumeService
       │
       ▼
 Scheduler
       │
       ▼
 RuntimeService
       │
       ▼
 Provider Interface
       │
       ▼
 ColabProvider
       │
       ▼
 google-colab-cli
       │
       ▼
 Colab Runtime (GPU VM)
```

## Building and Installation

Requires Go >= 1.24.

```bash
# Build binaries (bin/cbox and bin/cboxd)
make build

# Run test suite
make test

# Install to ~/.local/bin or /usr/local/bin
bash scripts/install.sh
```

## Quick Start

1. Start the CBox Engine Daemon:
```bash
cboxd
# Or run with debug logging:
cboxd --debug
```

2. Build an image:
```bash
cbox build -t mmseg:latest .
```

3. Run a container:
```bash
cbox run -d \
  --name full \
  --gpu L4,T4 \
  -v /data/WWTP:/data:ro \
  -v ./runs/full:/output:output \
  mmseg:latest \
  python tools/train.py configs/full.py
```

4. Monitor and interact:
```bash
cbox ps
cbox logs -f full
cbox exec -it full bash
cbox stats full
```

5. Stop and cleanup:
```bash
cbox stop full
cbox rm full
```

## Compose

Run multi-experiment workflows via `cbox compose`:
```bash
cbox compose up -d
cbox compose ps
cbox compose logs
cbox compose down
```
