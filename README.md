# CBox: Docker-like Colab GPU Runtime Engine

CBox brings Docker-like ergonomics to Google Colab GPU runtimes. It abstracts remote ephemeral GPU instances into containers with declarative blueprint images, synchronized remote volumes, detached execution, automatic recovery, and multi-service compose orchestration.

## Features

- **Docker-like CLI**: `cbox run`, `cbox ps`, `cbox logs`, `cbox exec`, `cbox cp`, `cbox stop`, `cbox rm`
- **Declarative Image (`Cboxfile`)**: Specify `FROM`, `APT`, `PIP`, `ENV`, `WORKDIR`, `COPY`, `RUN`, `CMD`, `HEALTHCHECK`
- **Synchronized Remote Volumes**: Four modes (`ro`, `rw`, `output`, `cache`) with fast mtime/size or sha256 checksum hashing
- **Detached Lifecycle Daemon (`cboxd`)**: Background monitoring over Unix Domain Socket with SQLite persistence
- **OpenSSH Multiplexing & Colab Proxy**: Seamless low-latency command execution and file synchronization
- **Automatic Recovery**: Checkpoint detection, runtime loss detection, automatic resume hooks
- **Multi-Context & Compose**: Multi-account isolation and `cbox-compose.yaml` orchestration

## Quick Start

```bash
# Build an image
cbox build -t mmseg:latest .

# Run container with remote GPU and synchronized volumes
cbox run -d \
    --name experiment-01 \
    --gpu L4,T4 \
    -v /data/Potsdam:/data:ro \
    -v ./runs/exp01:/output:output \
    mmseg:latest \
    python tools/train.py configs/model.py

# Check status
cbox ps

# View logs
cbox logs -f experiment-01

# Run interactive bash
cbox exec -it experiment-01 bash

# Stop and clean up
cbox stop experiment-01
cbox rm experiment-01
```
