# CBox: Docker-like Ephemeral GPU Runtime Engine (Go)

CBox is an ephemeral runtime engine written in Go. It manages Google Colab runtimes and remote processes through Docker-style commands, blueprint images, synchronized volumes, detached execution, and compose orchestration. Google Colab is the currently implemented cloud provider.

CBox containers run as processes on the remote runtime and share its filesystem and installed packages.

## Features

- **Docker-like CLI**: `cbox run`, `cbox ps`, `cbox logs`, `cbox exec`, `cbox stop`, `cbox start`, `cbox restart`, `cbox rm`, `cbox stats`
- **Blueprint Images (`Cboxfile`)**: Manifest hashing, remote dependency installation with `APT` / `PIP` / `RUN`, file copying, and runtime image caching; see the implementation notes below for image defaults
- **Synchronized Remote Volumes**: `ro`, `rw`, `output`, and `cache` modes, with periodic output sync and final sync on process exit
- **Detached Lifecycle Engine (`cboxd`)**: Go daemon with SQLite (`modernc.org/sqlite`, no CGO), a Unix socket HTTP API, and remote process monitoring
- **Scheduler & Runtime Pool**: GPU preference matching, runtime reuse, and automatic expiry of idle runtimes
- **OpenSSH Multiplexing**: OpenSSH `ControlMaster` + Colab `ProxyCommand` with direct PTY attachment for interactive exec
- **Compose**: Multi-service `cbox-compose.yaml` orchestration

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

The current `go.mod` requires Go **1.26.5 or newer**. Local dependencies include OpenSSH, rsync, and `google-colab-cli`; setup uses uv and git to install the Colab CLI with `jupyter-kernel-client`. The Colab integration was tested with CLI **0.7.2** on 2026-09-30.

```bash
# Build binaries (bin/cbox and bin/cboxd)
make build

# Build static binaries and install to ~/.local/bin
make install

# Install Colab CLI dependencies and authenticate
cbox setup

# Check dependencies and cached credentials
cbox doctor

# Run the test suite
make test
```

Ensure `~/.local/bin` is on your `PATH`. Alternatively, `bash scripts/install.sh` installs dependencies, builds CBox, creates the default configuration and a user systemd service, and invokes Colab authentication. That script installs CBox into `~/.local/bin` for a regular user and `/usr/local/bin` when run as root.

To reinstall an updated checkout and reload the background engine:

```bash
cbox daemon stop
make install
cbox daemon start
```

## Colab Authentication and Profiles

Colab starts OAuth when an API command needs credentials. Use these commands:

```bash
colab version
colab sessions
```

In the tested CLI, version information comes from the `version` subcommand, and authentication is triggered by `sessions`. CBox uses these commands during diagnosis and setup. `cbox doctor` checks for cached credentials; their presence does not verify current API access.

The default CBox profile reuses the user's credentials in `~/.config/colab-cli/token.json`. Its session state is stored separately in `<data_dir>/profiles/default/config.json`. With the default installation, `<data_dir>` is `~/.local/share/cbox`. An explicit `engine.data_dir` overrides this location; when it is unset, `XDG_DATA_HOME` is honored.

All Colab lifecycle commands and generated SSH proxies use the same session state file. When querying or releasing CBox runtimes manually, pass that file as a **global option before the subcommand**:

```bash
colab --config ~/.local/share/cbox/profiles/default/config.json sessions
```

Named provider profiles use `<data_dir>/profiles/<name>/home` to isolate credentials. Their optional `oauth.json` is a **client OAuth configuration**, supplied via `--client-oauth-config`; it is separate from the cached login token. Current context selection does not propagate a named profile to `cbox run` or `cbox create`.

## Quick Start

1. Start the background engine:

```bash
cbox daemon start
```

For foreground operation, use `cboxd --debug`. Client commands also automatically start the engine when it is unavailable.

2. Create a minimal blueprint and build its manifest:

```bash
mkdir -p /tmp/cbox-gpu-demo
cat > /tmp/cbox-gpu-demo/Cboxfile <<'EOF'
FROM colab/python:3
APT tmux rsync
EOF
cbox build -t gpu-demo:latest /tmp/cbox-gpu-demo
```

3. Start a container on a T4 runtime:

```bash
cbox run -d \
  --name gpu-demo \
  --gpu T4 \
  --workdir /content \
  -e LD_LIBRARY_PATH=/usr/lib64-nvidia \
  gpu-demo:latest -- bash -c 'echo CBOX_GPU_DEMO_READY; sleep 600'
```

The tested Colab SSH environment requires `LD_LIBRARY_PATH=/usr/lib64-nvidia` to locate GPU driver libraries. Set it with `-e` for the container process and noninteractive `cbox exec` calls. `--gpu L4,T4` tries those GPUs in order; omitting `--gpu` requests a CPU runtime.

Use `--` before the remote command when it contains flags. Arguments after `--` belong to the remote command, and quoted arguments keep their boundaries when passed through SSH.

4. Check the running container and execute GPU commands:

```bash
cbox ps
cbox exec gpu-demo -- nvidia-smi --query-gpu=name,memory.total --format=csv,noheader

# Requires PyTorch in the remote runtime; verified during the T4 live test
cbox exec gpu-demo -- python3 -c \
  'import torch; print(torch.cuda.is_available()); print(torch.cuda.get_device_name(0)); print((torch.arange(4, device="cuda") ** 2).tolist())'

cbox logs gpu-demo
cbox --format json inspect gpu-demo
```

The live test returned `True`, `Tesla T4`, and `[0, 1, 4, 9]`. A separate CUDA container also completed with exit code `0` and its output was available through `cbox logs`.

For an interactive SSH shell, use `cbox exec -it gpu-demo -- bash`. This path uses the remote shell's environment and working directory; set `export LD_LIBRARY_PATH=/usr/lib64-nvidia` inside that shell before using GPU tools.

5. Stop the container and release the runtime:

```bash
# Note runtime_id in the output before removing the container
cbox --format json inspect gpu-demo
cbox stop gpu-demo
cbox rm gpu-demo

# Replace SESSION with the runtime_id printed above
colab --config ~/.local/share/cbox/profiles/default/config.json stop -s SESSION
colab --config ~/.local/share/cbox/profiles/default/config.json sessions
```

`cbox stop` stops the container process and clears its desired running state, including after a failed start. `cbox rm` removes the container record. The Colab runtime remains available for reuse until idle expiry (30 minutes by default); the explicit `colab stop` command above releases it immediately. Remove all other containers using the same runtime before releasing it.

## Current Implementation Notes

- `Cboxfile` parses `FROM`, `ENV`, `WORKDIR`, and `CMD`, but container creation currently uses the command, environment, and work directory supplied at run time. Provide the command explicitly and use `-e` / `--workdir`; `FROM` is stored as blueprint metadata.
- Relative `COPY` sources are currently resolved against the daemon's working directory during materialization. Bind volumes can be used to synchronize project code.
- `cbox stats` currently populates the GPU name. Numeric GPU and system memory/utilization fields are not populated and display zero.
- Volumes use rsync and remote symlinks. The `ro` mode describes synchronization behavior; remote filesystem permissions do not enforce read-only access.
- Restart policy and resume command fields are stored, but the reconciler does not yet apply their full semantics. Stop a failed container to cancel further recovery attempts.

## Compose

Create `cbox-compose.yaml` using an image built with `cbox build`. For example, the following service uses the `gpu-demo:latest` image from the quick start:

```yaml
services:
  gpu-check:
    image: gpu-demo:latest
    gpu:
      preference: [T4]
    env:
      LD_LIBRARY_PATH: /usr/lib64-nvidia
    workdir: /content
    command: [nvidia-smi]
```

Run it with:

```bash
cbox compose up -d
cbox compose ps
cbox compose logs
cbox compose down
```

`compose down` stops and removes the service containers. Their Colab runtimes follow the same reuse and release behavior described above.
