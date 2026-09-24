# CBox：Docker-like Colab GPU Runtime Engine

## 1. 项目目标

项目暂定名称：

```text
CBox
```

定位：

> 将 Google Colab Runtime 抽象为类似 Docker Container 的临时远程 GPU 执行环境。

目标用户不需要理解：

```text
colab new
colab ssh
ProxyCommand
rsync
tmux
runtime session
checkpoint sync
```

而只需要理解：

```text
Image
Container
Volume
Context
Compose
```

最终用户体验：

```bash
cbox build -t mmseg:latest .

cbox run -d \
    --name experiment-01 \
    --gpu L4 \
    -v /data/Potsdam:/data:ro \
    -v ./runs/exp01:/output:output \
    mmseg:latest \
    python tools/train.py configs/model.py

cbox ps

cbox logs -f experiment-01

cbox exec -it experiment-01 bash

cbox cp experiment-01:/output/best.pth ./

cbox stop experiment-01

cbox rm experiment-01
```

---

# 2. 核心原则

CBox 在交互模型上模仿 Docker，但绝不能假装 Colab 真的是 Docker。

三个最重要的语义差异：

| Docker                          | CBox                                          |
| ------------------------------- | --------------------------------------------- |
| Image 是 filesystem snapshot     | Image 是声明式环境 Blueprint                        |
| Volume 可以是真正 bind mount         | Volume 是服务器与 Colab 之间的同步副本                    |
| stop 后 container filesystem 仍存在 | Colab runtime 释放后 filesystem 消失               |
| start 恢复原 container             | CBox start 重建 runtime 并恢复状态                   |
| Container 与本机 kernel 隔离         | CBox Container 实际对应远程 Colab Runtime + Process |

因此 CBox 的设计原则是：

```text
Docker-like UX
+
Colab-native semantics
```

---

# 3. 总体系统架构

完整结构：

```text
                       User
                        │
                        ▼
                 ┌─────────────┐
                 │  cbox CLI   │
                 └──────┬──────┘
                        │
                   Unix Socket
                        │
                        ▼
                ┌───────────────┐
                │     cboxd     │
                │    Engine     │
                └──────┬────────┘
                       │
        ┌──────────────┼──────────────┐
        │              │              │
        ▼              ▼              ▼
 Container        Image Manager   Volume Manager
 Manager
        │
        ▼
 Scheduler
        │
        ▼
 Runtime Manager
        │
        ▼
 Provider Interface
        │
        ├─────────────┐
        ▼             ▼
 ColabProvider    Future Providers
        │         RunPod / GCP / Local
        ▼
 google-colab-cli
        │
        ▼
 Colab Runtime
        │
        ▼
 SSH over WebSocket
        │
        ▼
 Remote Worker Agent
        │
        ├── command
        ├── tmux
        ├── logs
        ├── GPU metrics
        └── filesystem
```

---

# 4. Client / Daemon 架构

采用：

```text
cbox
+
cboxd
```

类似：

```text
docker
+
dockerd
```

## cbox

负责：

```text
命令行解析
请求 daemon
格式化输出
attach stdin/stdout
```

不负责：

```text
Colab 生命周期
后台 checkpoint
健康检查
恢复
定时同步
```

---

## cboxd

长期后台运行。

负责：

```text
Container 生命周期
Runtime 生命周期
Colab Provider
SSH
rsync
Image materialization
Volume sync
日志
Runtime keepalive
状态监控
自动恢复
多账户管理
```

Linux：

```bash
systemctl --user enable --now cboxd
```

通信：

```text
Unix Domain Socket
```

例如：

```text
$XDG_RUNTIME_DIR/cbox/cbox.sock
```

推荐：

```text
HTTP + JSON over Unix Socket
```

第一版使用：

```text
FastAPI
+
Uvicorn
```

即可。

---

# 5. 软件分层

内部建议采用：

```text
CLI
 │
 ▼
API Client
 │
 ▼
Engine API
 │
 ▼
Service Layer
 │
 ├── ContainerService
 ├── ImageService
 ├── VolumeService
 ├── RuntimeService
 └── ContextService
 │
 ▼
Domain Layer
 │
 ├── Container
 ├── Image
 ├── Volume
 ├── Runtime
 └── Provider
 │
 ▼
Infrastructure
 │
 ├── SQLite
 ├── Colab CLI
 ├── SSH
 ├── rsync
 └── filesystem
```

这样以后替换：

```text
ColabProvider
```

不会影响：

```text
ContainerService
```

---

# 6. 技术栈

推荐：

| 组件             | 技术               |
| -------------- | ---------------- |
| Language       | Python 3         |
| CLI            | Typer            |
| Models         | Pydantic         |
| Async          | asyncio          |
| Daemon API     | FastAPI          |
| Unix Socket    | Uvicorn          |
| Database       | SQLite           |
| ORM            | SQLAlchemy 2     |
| Migration      | Alembic          |
| Serialization  | JSON             |
| Config         | YAML             |
| SSH            | OpenSSH          |
| Sync           | rsync            |
| Worker process | tmux             |
| Colab          | google-colab-cli |
| Logging        | structlog        |
| Tests          | pytest           |
| Packaging      | uv / hatchling   |

不建议第一版引入：

```text
Redis
Celery
RabbitMQ
Kubernetes
Docker-in-Docker
```

---

# 7. Repository 结构

```text
cbox/
│
├── pyproject.toml
├── README.md
├── LICENSE
│
├── src/
│   └── cbox/
│
│       ├── cli/
│       │   ├── main.py
│       │   ├── container.py
│       │   ├── image.py
│       │   ├── volume.py
│       │   ├── context.py
│       │   └── compose.py
│       │
│       ├── client/
│       │   └── api.py
│       │
│       ├── daemon/
│       │   ├── main.py
│       │   ├── app.py
│       │   └── lifespan.py
│       │
│       ├── api/
│       │   ├── containers.py
│       │   ├── images.py
│       │   ├── volumes.py
│       │   ├── contexts.py
│       │   └── system.py
│       │
│       ├── domain/
│       │   ├── container.py
│       │   ├── image.py
│       │   ├── volume.py
│       │   ├── runtime.py
│       │   ├── context.py
│       │   └── enums.py
│       │
│       ├── services/
│       │   ├── container_service.py
│       │   ├── image_service.py
│       │   ├── volume_service.py
│       │   ├── runtime_service.py
│       │   ├── scheduler.py
│       │   └── recovery_service.py
│       │
│       ├── providers/
│       │   ├── base.py
│       │   └── colab/
│       │       ├── provider.py
│       │       ├── cli.py
│       │       ├── auth.py
│       │       └── session.py
│       │
│       ├── transport/
│       │   ├── ssh.py
│       │   ├── rsync.py
│       │   └── proxy.py
│       │
│       ├── worker/
│       │   ├── bootstrap.py
│       │   ├── executor.py
│       │   ├── metrics.py
│       │   └── health.py
│       │
│       ├── storage/
│       │   ├── database.py
│       │   ├── models.py
│       │   └── repositories.py
│       │
│       ├── compose/
│       │   ├── parser.py
│       │   └── service.py
│       │
│       └── utils/
│           ├── shell.py
│           ├── hash.py
│           ├── paths.py
│           └── errors.py
│
├── worker/
│   ├── bootstrap.sh
│   ├── entrypoint.sh
│   ├── exec.sh
│   └── health.sh
│
├── examples/
│   ├── Cboxfile
│   └── cbox-compose.yaml
│
└── tests/
    ├── unit/
    ├── integration/
    └── e2e/
```

---

# 8. 用户层核心对象

CBox 用户只需要主要理解四个对象：

```text
Image
Container
Volume
Context
```

Compose 是组合层。

内部的：

```text
Runtime
Worker
Session
Account
Provider
Process
```

不作为主要用户对象。

---

# 9. Image 设计

## 9.1 定义

CBox Image 是：

```text
Environment Blueprint
```

包含：

```text
基础环境要求
APT 包
Python 包
环境变量
初始化脚本
工作目录
默认命令
```

它不是 filesystem image。

---

# 10. Cboxfile

设计类似 Dockerfile。

例如：

```dockerfile
FROM colab/python:3

APT git
APT rsync
APT libgl1

PIP torch
PIP torchvision
PIP mmengine
PIP mmcv
PIP mmsegmentation

ENV PYTHONPATH=/workspace

WORKDIR /workspace

COPY requirements.txt /tmp/requirements.txt

RUN pip install -r /tmp/requirements.txt

CMD ["bash"]
```

第一版只需要实现：

```text
FROM
APT
PIP
ENV
WORKDIR
COPY
RUN
CMD
```

不要一开始实现完整 Dockerfile syntax。

---

# 11. `cbox build`

执行：

```bash
cbox build -t mmseg:latest .
```

内部不是启动 Runtime，而是：

```text
parse Cboxfile
       ↓
resolve dependencies
       ↓
normalize manifest
       ↓
calculate content hash
       ↓
create bootstrap plan
       ↓
save image metadata
```

例如：

```text
~/.local/share/cbox/images/
└── sha256-abc123/
    ├── manifest.json
    ├── bootstrap.sh
    ├── requirements.txt
    ├── context/
    └── metadata.json
```

Image ID：

```text
SHA256(
 Cboxfile
 + referenced files
 + normalized settings
)
```

这样环境不变时：

```text
Image ID 不变
```

---

# 12. Image Manifest

例如：

```json
{
  "id": "sha256:918cc3...",
  "tags": [
    "mmseg:latest"
  ],
  "base": {
    "provider": "colab",
    "python": "3"
  },
  "apt": [
    "git",
    "rsync",
    "libgl1"
  ],
  "pip": [
    "torch",
    "mmengine",
    "mmcv",
    "mmsegmentation"
  ],
  "environment": {
    "PYTHONPATH": "/workspace"
  },
  "workdir": "/workspace",
  "cmd": [
    "bash"
  ]
}
```

---

# 13. Image materialization

真正 `cbox run` 时：

```text
Runtime
   ↓
bootstrap CBox Worker
   ↓
检查 Image cache
   ↓
Image 已安装？
 ├─ YES → skip
 └─ NO
      ↓
     apt
      ↓
     pip
      ↓
     COPY
      ↓
     RUN
      ↓
写入 image marker
```

例如：

```text
/content/.cbox/cache/images/
└── sha256-918cc3.ready
```

这样同一个 Runtime 连续创建多个相同 Image 的 Container：

```text
第二次无需重新装 mmcv
```

---

# 14. Container 定义

Container 是：

```text
ContainerSpec
+
Provider Runtime Binding
+
Remote Process
+
Persistent Metadata
```

例如：

```json
{
  "id": "ca84bd...",
  "name": "rpgv-full",

  "image": "mmseg:latest",

  "command": [
    "python",
    "tools/train.py",
    "configs/full.py"
  ],

  "gpu": [
    "L4",
    "T4"
  ],

  "state": "RUNNING"
}
```

---

# 15. Container 生命周期

定义状态：

```text
CREATED
   ↓
PROVISIONING
   ↓
PREPARING
   ↓
STARTING
   ↓
RUNNING
   ↓
EXITED
```

异常：

```text
RUNNING
   ↓
INTERRUPTED
   ↓
RECOVERING
   ↓
RUNNING
```

用户主动：

```text
RUNNING
 ↓
STOPPING
 ↓
STOPPED
```

删除：

```text
STOPPED
 ↓
REMOVED
```

失败：

```text
FAILED
```

---

# 16. `cbox create`

与 Docker 一样，可以支持：

```bash
cbox create \
    --name exp \
    --gpu T4 \
    mmseg \
    python train.py
```

此时只创建：

```text
Container metadata
```

不启动 Runtime。

状态：

```text
CREATED
```

然后：

```bash
cbox start exp
```

才真正 provision。

---

# 17. `cbox run`

等价于：

```text
create
+
start
+
attach
```

例如：

```bash
cbox run \
    --name full \
    --gpu L4,T4 \
    mmseg \
    python train.py
```

内部：

```text
Create ContainerSpec
        ↓
Find available Runtime
        ↓
没有？
        ↓
Provision Runtime
        ↓
SSH Ready
        ↓
Bootstrap Worker
        ↓
Materialize Image
        ↓
Materialize Volumes
        ↓
Start command
        ↓
Attach logs
```

---

# 18. Detached mode

```bash
cbox run -d ...
```

立即返回：

```text
ca84bd93582a
```

后台：

```text
cboxd
```

继续管理。

---

# 19. Worker process

不要让训练程序直接绑定 SSH shell。

在 Colab：

```text
/content/.cbox/
```

建立：

```text
containers/<ID>/
├── config.json
├── stdout.log
├── stderr.log
├── exit_code
├── pid
└── state
```

启动：

```bash
tmux new-session -d \
    -s cbox-ca84bd \
    "/content/.cbox/bin/entrypoint.sh ca84bd"
```

---

# 20. Entrypoint

逻辑：

```bash
#!/usr/bin/env bash

set -o pipefail

cd "$WORKDIR"

"${COMMAND[@]}" \
    > >(tee -a "$STDOUT") \
    2> >(tee -a "$STDERR" >&2)

EXIT=$?

echo "$EXIT" > "$EXIT_FILE"

exit "$EXIT"
```

---

# 21. Volume 抽象

这是系统最核心的部分之一。

用户：

```bash
-v /data/Potsdam:/data:ro
```

用户认为：

```text
/data/Potsdam
```

被挂载到了：

```text
/data
```

但实际：

```text
Server
/data/Potsdam
      │
      │ rsync
      ▼
Colab
/content/.cbox/volumes/<ID>
      │
      │ symlink
      ▼
/data
```

因此 CBox Volume 本质是：

```text
Synchronized Remote Volume
```

---

# 22. Volume mode

定义四种：

```text
ro
rw
output
cache
```

### ro

启动前：

```text
Server → Colab
```

运行过程中不自动回传。

适合：

```text
dataset
pretrained weights
```

---

### rw

启动前：

```text
Server → Colab
```

停止时：

```text
Colab → Server
```

适合一般工作目录。

---

### output

重点模式。

启动前可恢复：

```text
Server → Colab
```

运行过程中：

```text
Colab → Server
```

周期同步。

适合：

```text
checkpoint
logs
prediction
```

---

### cache

只作为 Worker cache：

```text
Runtime 活着就复用
Runtime 死亡可以重新生成
```

---

# 23. Named Volume

支持：

```bash
cbox volume create \
    --source /data/Potsdam \
    potsdam
```

然后：

```bash
cbox run \
    --mount source=potsdam,target=/data,mode=ro \
    ...
```

查看：

```bash
cbox volume ls
```

---

# 24. Volume manifest

每个 Volume 生成：

```json
{
  "name": "potsdam",
  "source": "/data/Potsdam",
  "hash": "85df...",
  "mode": "ro",
  "version": 3
}
```

Runtime 内：

```text
/content/.cbox/volumes/
```

保存相同 manifest。

如果：

```text
source hash == worker cache hash
```

：

```text
CACHE HIT
```

无需传输。

---

# 25. Hash 策略

大型数据不能每次计算所有文件 SHA256。

推荐分两种：

快速模式：

```text
relative path
size
mtime
```

生成 manifest hash。

严格模式：

```bash
cbox volume create --checksum ...
```

使用：

```text
SHA256
```

科研数据版本建议支持：

```text
--immutable
```

意味着源目录变化后：

```text
创建新 volume version
```

---

# 26. 文件传输

默认：

```text
rsync over OpenSSH
```

SSH 底层通过官方：

```bash
colab ssh --proxy-mode
```

官方明确支持将它作为 OpenSSH ProxyCommand，用于 IDE 或其他 SSH 工具，因此这是 CBox 最关键的底层能力之一。

SSH config 动态生成：

```text
Host cbox-ca84bd
    User root

    IdentityFile ~/.local/share/cbox/keys/worker

    ProxyCommand colab \
        --config ... \
        ssh \
        --proxy-mode \
        -s cbox-ca84bd \
        -i ~/.local/share/cbox/keys/worker
```

然后：

```bash
rsync ... cbox-ca84bd:/content/...
```

---

# 27. 大量小文件优化

遥感数据经常包含：

```text
几十万 PNG
mask
patch
```

首次传输支持：

```text
tar stream
```

模式：

```text
Server
   │ tar
   ▼
SSH stream
   │
   ▼
Worker extract
```

之后更新使用：

```text
rsync
```

可以提供：

```bash
--transfer auto
```

规则：

```text
文件数少 → rsync

文件数非常多且首次同步
→ tar-stream
```

---

# 28. `cbox cp`

语义与 Docker 一致：

```bash
cbox cp local.file container:/workspace/
```

以及：

```bash
cbox cp container:/output/model.pth .
```

内部：

```text
rsync
```

---

# 29. `cbox exec`

例如：

```bash
cbox exec experiment nvidia-smi
```

内部：

```text
SSH → exec command
```

交互式：

```bash
cbox exec -it experiment bash
```

直接：

```text
ssh -tt
```

进入 Runtime。

---

# 30. `cbox logs`

```bash
cbox logs exp
```

读取：

```text
stdout.log
stderr.log
```

实时：

```bash
cbox logs -f exp
```

实现：

```text
tail -F
```

通过 SSH 转发。

支持：

```bash
--tail 100
--since 10m
-f
```

---

# 31. `cbox ps`

输出：

```text
CONTAINER ID  IMAGE          GPU   STATUS       NAMES
ca84bd9358    mmseg:latest   L4    Up 2h        full
702b11aa9e    mmseg:latest   T4    Up 31m       no-freq
```

全部：

```bash
cbox ps -a
```

---

# 32. `cbox inspect`

```bash
cbox inspect full
```

输出：

```json
{
  "Id": "ca84bd...",
  "Name": "full",

  "Image": "mmseg:latest",

  "State": {
    "Status": "running",
    "ExitCode": null
  },

  "Resource": {
    "GPURequested": [
      "L4",
      "T4"
    ],
    "GPUAssigned": "L4"
  },

  "Runtime": {
    "Provider": "colab",
    "Session": "cbox-ca84bd",
    "Account": "colab-a"
  },

  "Mounts": [
    {
      "Source": "/data/Potsdam",
      "Destination": "/data",
      "Mode": "ro"
    }
  ]
}
```

---

# 33. `cbox stats`

例如：

```bash
cbox stats
```

返回：

```text
NAME       GPU  GPU MEM       GPU UTIL  CPU   RAM
full       L4   19.2/24 GB    97%       81%   18 GB
no-freq    T4   12.3/15 GB    89%       67%   14 GB
```

Worker 端执行：

```bash
nvidia-smi
ps
free
```

解析为 JSON。

---

# 34. stop 语义

```bash
cbox stop full
```

流程：

```text
SIGTERM process
       ↓
等待 timeout
       ↓
必要时 SIGKILL
       ↓
同步 output Volume
       ↓
记录 Exit State
       ↓
释放 Runtime
```

Runtime filesystem 随后不可依赖。

---

# 35. start 语义

因为 Colab VM 已经不存在：

```bash
cbox start full
```

实际：

```text
读取 ContainerSpec
       ↓
Provision 新 Runtime
       ↓
Materialize Image
       ↓
Restore Volumes
       ↓
Restore output/checkpoint
       ↓
重新执行 Command
```

所以：

```text
start != resume old VM
```

而是：

```text
reconstruct container
```

---

# 36. Restart

```bash
cbox restart full
```

执行：

```text
stop
+
start
```

---

# 37. 自动恢复

提供：

```bash
--restart unless-stopped
```

类似 Docker。

策略：

```text
no
on-failure
unless-stopped
always
```

Colab Runtime 意外消失：

```text
Runtime LOST
      ↓
Container INTERRUPTED
      ↓
output 已定期同步
      ↓
Provision new Runtime
      ↓
Restore
      ↓
run command
```

---

# 38. 深度学习 checkpoint 恢复

CBox 不应该假设：

```text
任意程序都能自动 resume
```

因此提供 Hook：

```yaml
recovery:
  resume_command: >
    python tools/train.py
    configs/full.py
    --resume
```

Cboxfile 或 run 参数：

```bash
--resume-cmd "python tools/train.py ... --resume"
```

初次运行：

```text
command
```

恢复：

```text
resume_command
```

对于你的 MMSegmentation：

```text
--resume
```

非常适合。

---

# 39. Healthcheck

Cboxfile：

```dockerfile
HEALTHCHECK nvidia-smi
```

或：

```yaml
healthcheck:
  command: python health.py
  interval: 30
  retries: 3
```

状态：

```text
healthy
unhealthy
unknown
```

---

# 40. Runtime reuse

与 Docker 最大不同之一。

创建 Colab VM 有成本。

所以 Engine 可以：

```text
Runtime Pool
```

一个 Runtime 的主 Container 完成后：

```text
不立即销毁
```

而是：

```text
IDLE
```

等待：

```text
idle_timeout = 30 min
```

如果下一 Container：

```text
同 account
兼容 GPU
```

直接复用。

这让：

```text
数据缓存
pip 环境
image cache
```

保留下来。

---

# 41. Runtime 与 Container 的映射

MVP：

```text
1 Runtime
=
1 Running Container
```

最简单、最安全。

未来可以允许：

```text
1 Runtime
=
多个 sequential containers
```

但不要同时跑多个训练 Container。

---

# 42. Scheduler

用户虽然看不到 Scheduler，但内部必须有。

调度优先级：

```text
GPU compatibility
      ↓
existing idle Runtime
      ↓
image cache hit
      ↓
volume cache hit
      ↓
account availability
      ↓
new Runtime
```

可以定义 score：

```python
score = (
    gpu_match * 100
    + image_cache * 30
    + volume_cache * 50
    + idle_runtime * 100
)
```

---

# 43. GPU preference

用户：

```bash
--gpu L4,T4
```

表示：

```text
prefer L4
fallback T4
```

不是同时请求两张卡。

内部：

```text
L4 allocation
 ↓ fail
T4 allocation
```

官方当前 CLI 支持指定 T4、L4、G4、A100、H100 等 accelerator，但最终能否得到目标 GPU 取决于账户资格和后端资源。

---

# 44. Context

模仿：

```bash
docker context
```

例如：

```bash
cbox context ls
```

：

```text
NAME          PROVIDER   PROFILE
colab-main *  colab      account-a
colab-alt     colab      account-b
colab-pool    colab      auto
```

切换：

```bash
cbox context use colab-alt
```

---

# 45. Colab Profile

每个账户使用完全独立：

```text
HOME
OAuth config
sessions.json
```

例如：

```text
~/.local/share/cbox/profiles/
├── account-a/
│   ├── home/
│   └── sessions.json
└── account-b/
```

启动 CLI：

```text
HOME=<profile-home>
colab --config <sessions> ...
```

避免两个账户：

```text
token
session
OAuth
```

互相污染。

---

# 46. Compose

文件：

```text
cbox-compose.yaml
```

例如：

```yaml
version: "1"

services:

  full:
    image: wwtp:mmseg

    gpu:
      preference:
        - L4
        - T4

    volumes:
      - source: /data/WWTP
        target: /data
        mode: ro

      - source: ./runs/full
        target: /output
        mode: output

    command:
      - python
      - tools/train.py
      - configs/full.py

    restart: unless-stopped


  no_frequency:
    image: wwtp:mmseg

    gpu:
      preference:
        - T4

    volumes:
      - source: /data/WWTP
        target: /data
        mode: ro

      - source: ./runs/no_frequency
        target: /output
        mode: output

    command:
      - python
      - tools/train.py
      - configs/no_frequency.py
```

使用：

```bash
cbox compose up -d
```

查看：

```bash
cbox compose ps
```

停止：

```bash
cbox compose down
```

---

# 47. Compose 调度

例如两个账户：

```text
Account A → full
Account B → no_frequency
```

其余：

```text
QUEUED
```

当其中一个结束：

```text
Runtime 如果仍然可用
↓
下一个 service
```

并复用 dataset cache。

---

# 48. Engine REST API

Unix Socket 上暴露：

```text
GET    /version

POST   /images/build
GET    /images
DELETE /images/{id}

POST   /containers/create
POST   /containers/{id}/start
POST   /containers/{id}/stop
POST   /containers/{id}/restart
DELETE /containers/{id}

GET    /containers
GET    /containers/{id}

POST   /containers/{id}/exec

GET    /containers/{id}/logs
GET    /containers/{id}/stats

POST   /volumes
GET    /volumes
DELETE /volumes/{name}

GET    /contexts
POST   /contexts/{name}/use
```

---

# 49. Database

SQLite：

```text
~/.local/share/cbox/cbox.db
```

核心表：

## images

```text
id
created_at
manifest
```

## image_tags

```text
repository
tag
image_id
```

## containers

```text
id
name
image_id
state
command
workdir
restart_policy
exit_code
created_at
started_at
finished_at
runtime_id
```

## volumes

```text
id
name
source
hash
mode
created_at
```

## container_mounts

```text
container_id
volume_id
target
mode
```

## runtimes

```text
id
provider
provider_session
profile
accelerator
state
created_at
last_seen
```

## runtime_cache

```text
runtime_id
object_type
object_id
hash
```

## events

```text
id
object_type
object_id
event
timestamp
payload
```

---

# 50. Event system

必须实现内部事件：

```text
container.create
container.start
container.running
container.exit

runtime.provision
runtime.ready
runtime.lost
runtime.stop

image.materialize.start
image.materialize.done

volume.sync.start
volume.sync.done

container.recover.start
container.recover.done
```

所有事件写入：

```text
events
```

方便故障排查。

---

# 51. 本地目录布局

遵循 XDG：

```text
~/.config/cbox/
├── config.yaml
└── contexts.yaml
```

数据：

```text
~/.local/share/cbox/
├── cbox.db
├── images/
├── volumes/
├── containers/
├── profiles/
├── keys/
└── logs/
```

runtime：

```text
$XDG_RUNTIME_DIR/cbox/
└── cbox.sock
```

---

# 52. Worker filesystem

Colab：

```text
/content/.cbox/
├── bin/
├── containers/
├── images/
├── volumes/
├── cache/
└── worker.json
```

项目：

```text
/workspace
```

可以软链接：

```text
/workspace
→
/content/.cbox/containers/<ID>/workspace
```

---

# 53. Worker Agent

第一版不需要真正常驻 Python daemon。

因为：

```text
SSH + shell
```

已经够用。

只需 bootstrap 一组脚本：

```text
cbox-worker
cbox-exec
cbox-inspect
cbox-stats
```

第二阶段再考虑：

```text
轻量 Python Worker Agent
```

---

# 54. Colab Provider Interface

```python
class Provider(ABC):

    async def create_runtime(
        self,
        request: RuntimeRequest
    ) -> Runtime:
        ...

    async def stop_runtime(
        self,
        runtime: Runtime
    ) -> None:
        ...

    async def runtime_status(
        self,
        runtime: Runtime
    ) -> RuntimeStatus:
        ...

    async def exec(
        self,
        runtime: Runtime,
        command: list[str]
    ) -> ExecResult:
        ...

    async def open_transport(
        self,
        runtime: Runtime
    ) -> Transport:
        ...
```

---

# 55. ColabProvider

内部 wrapper：

```python
class ColabCLI:

    async def new(...):
        ...

    async def sessions(...):
        ...

    async def status(...):
        ...

    async def stop(...):
        ...
```

执行：

```python
proc = await asyncio.create_subprocess_exec(
    "colab",
    "new",
    "-s",
    session,
    "--gpu",
    accelerator,
)
```

不要 import 官方 CLI 私有模块。

只把：

```text
CLI
```

作为稳定边界。

---

# 56. Transport interface

```python
class Transport:

    async def exec(...):
        ...

    async def upload(...):
        ...

    async def download(...):
        ...

    async def sync_to(...):
        ...

    async def sync_from(...):
        ...
```

实现：

```text
SSHTransport
```

未来可：

```text
LocalTransport
```

---

# 57. SSH multiplexing

大量：

```text
logs
stats
rsync
healthcheck
```

如果每次重新建立 Colab WebSocket SSH 成本较高。

建议使用 OpenSSH：

```text
ControlMaster auto
ControlPersist 600
ControlPath ~/.cache/cbox/ssh/%C
```

能显著降低重复连接开销。

---

# 58. Runtime health monitor

cboxd：

```text
每 30 秒
```

检查：

```text
colab status
+
SSH echo
```

例如：

```bash
echo CBOX_HEALTHY
```

状态：

```text
READY
BUSY
UNREACHABLE
LOST
```

---

# 59. 容器进程检查

执行：

```bash
tmux has-session -t cbox-ID
```

同时检查：

```text
exit_code
```

情况：

```text
tmux存在
→ RUNNING

tmux不存在 + exit_code=0
→ EXITED success

tmux不存在 + exit_code!=0
→ EXITED failure
```

---

# 60. Output 自动同步

Daemon：

```text
每 N 分钟
```

针对：

```text
mode=output
```

执行：

```text
rsync Worker → Server
```

建议默认：

```text
300 秒
```

训练场景可以：

```text
600 秒
```

使用：

```text
--partial
--append-verify
```

时要注意正在写入 checkpoint 的竞争。

更安全：

```text
只同步已经 rename 完成的 checkpoint
```

建议训练程序：

```text
tmp.pth
↓
atomic rename
↓
epoch_10.pth
```

---

# 61. 日志持久化

Daemon 同时可以：

```text
tail offset
```

持续拉日志。

服务器最终：

```text
~/.local/share/cbox/containers/<ID>/
├── stdout.log
└── stderr.log
```

即使 Runtime 死亡：

```bash
cbox logs old-container
```

仍能查看最后日志。

---

# 62. Error taxonomy

统一定义：

```text
ProviderError
AllocationError
AuthenticationError
TransportError
ImageBuildError
ImageMaterializationError
VolumeSyncError
ContainerStartError
ContainerLostError
RecoveryError
```

用户输出例如：

```text
Error: GPU allocation failed.

Requested:
  L4

Profile:
  colab-a

Fallback:
  T4

Trying fallback T4...
```

---

# 63. Exit code

CLI exit code：

```text
0 success
1 generic
2 usage/config
10 provider
11 allocation
12 transport
20 image
30 volume
40 container
```

方便：

```text
shell scripts
CI
Agent
```

判断。

---

# 64. Security

服务器保存：

```text
Colab OAuth credentials
SSH private key
```

Worker 不保存：

```text
服务器 private key
```

所有 Volume 操作由服务器主动：

```text
push
pull
```

SSH Key：

```text
专用 Ed25519
```

不是服务器日常登录 key。

权限：

```bash
chmod 600
```

---

# 65. Secret 支持

不要把：

```text
API_KEY
TOKEN
PASSWORD
```

写进 Cboxfile。

支持：

```bash
cbox run \
    --secret HF_TOKEN \
    ...
```

服务器：

```text
~/.config/cbox/secrets/
```

Worker 运行时注入：

```text
environment
```

Container 结束后清理。

---

# 66. CLI 完整设计

第一阶段主要命令：

```text
cbox build
cbox images
cbox rmi

cbox create
cbox run
cbox start
cbox stop
cbox restart
cbox rm

cbox ps
cbox inspect
cbox logs
cbox exec
cbox cp
cbox stats

cbox volume create
cbox volume ls
cbox volume inspect
cbox volume rm

cbox context create
cbox context ls
cbox context use
cbox context rm

cbox compose up
cbox compose ps
cbox compose logs
cbox compose down

cbox system info
cbox version
```

---

# 67. `cbox run` 参数

```text
--name
-d / --detach
-it / --interactive
--gpu
--high-mem
-v / --volume
--mount
-e / --env
--env-file
--secret
-w / --workdir
--restart
--resume-command
--context
--rm
```

示例：

```bash
cbox run -d \
    --name potsdam-mask2former \
    --gpu L4,T4 \
    --high-mem \
    -v /data/Potsdam:/data:ro \
    -v ./runs/mask2former:/output:output \
    -e DATA_ROOT=/data \
    --restart unless-stopped \
    --resume-command \
      "python tools/train.py configs/m2f.py --resume" \
    mmseg:latest \
    python tools/train.py configs/m2f.py
```

---

# 68. Config

```yaml
engine:
  state_dir: ~/.local/share/cbox

runtime:
  idle_timeout: 1800

sync:
  output_interval: 600
  transfer: auto

ssh:
  control_persist: 600

scheduler:
  gpu_fallback: true

recovery:
  enabled: true
  max_attempts: 3
```

---

# 69. 开发顺序

## Milestone 0：Colab Transport PoC

完成：

```text
new
ssh proxy
ssh command
rsync
stop
```

测试：

```bash
cbox-dev new --gpu T4

cbox-dev exec nvidia-smi

cbox-dev push ./test /content/test
```

目标：

```text
证明底层通信稳定
```

---

## Milestone 1：无 daemon 的 `cbox run`

实现：

```text
ImageSpec
ContainerSpec
ColabProvider
SSHTransport
rsync
tmux
```

支持：

```bash
cbox run
cbox ps
cbox logs
cbox exec
cbox stop
```

此时可直接投入个人使用。

---

## Milestone 2：cboxd

拆成：

```text
Client
Daemon
```

实现：

```text
Unix socket API
SQLite
background monitor
```

CLI 可以随时退出。

---

## Milestone 3：Volume

实现：

```text
ro
rw
output
cache
named volume
manifest
```

这是科研实际可用性的关键里程碑。

---

## Milestone 4：Image

实现：

```text
Cboxfile parser
build
image hash
runtime materialization
image cache
```

---

## Milestone 5：Recovery

实现：

```text
runtime lost detection
output checkpoint persistence
new runtime
restore
resume_command
```

---

## Milestone 6：多账户与 Context

实现：

```text
profile isolation
context
automatic fallback
```

---

## Milestone 7：Compose

支持：

```text
multiple experiments
queue
runtime reuse
dataset affinity
```

---

# 70. 第一版不要做的事情

不要：

```text
自己实现 Container runtime

真的运行 Docker daemon inside Colab

尝试 OCI filesystem layers

自己实现 SSH protocol

自己逆向 Colab backend API

先开发 Web UI

先开发 Kubernetes scheduler

先做复杂分布式数据库
```

CBox 的价值来自：

```text
抽象
生命周期管理
状态持久化
同步
恢复
```

而不是重新造底层。

---

# 71. Testing

需要四层测试。

## Unit

测试：

```text
Cboxfile parser
hash
state transition
CLI parser
scheduler
volume manifest
```

全部 mock。

---

## Provider Integration

真实执行：

```text
colab new
colab status
colab stop
```

但不跑训练。

---

## Transport Integration

真实：

```text
SSH
rsync
exec
cp
```

---

## End-to-End

使用小型 PyTorch：

```python
for epoch in range(20):
    train()
    save_checkpoint()
```

测试：

```text
run
↓
logs
↓
cp
↓
人工 stop Runtime
↓
recovery
↓
resume
↓
success
```

---

# 72. E2E 验收场景

V1 发布前必须成功：

```text
01 build image

02 run T4 container

03 mount 1GB volume

04 command starts

05 CLI exits

06 daemon continues monitoring

07 logs -f works

08 exec bash works

09 stats works

10 output appears on server

11 forcibly terminate Colab Runtime

12 daemon detects LOST

13 new runtime allocated

14 image restored

15 volume restored

16 checkpoint restored

17 command resumes

18 training finishes

19 exit code recorded

20 output completely synchronized

21 stop/rm works

22 daemon restart does not lose container state
```

---

# 73. 性能指标

建议测试：

```text
Runtime provision latency

SSH setup latency

Image materialization latency

dataset transfer throughput

output sync latency

recovery time

daemon CPU/RAM
```

重点目标：

```text
cache hit 时启动新实验
尽可能控制在几十秒级
```

而不是每个实验重新：

```text
下载数据
pip install
```

---

# 74. 针对遥感科研的数据设计

你的场景典型：

```text
Dataset
  20~100GB

Code
  <1GB

Checkpoint
  0.5~5GB

Logs
  MB级
```

因此最优策略：

```text
Dataset
→ named immutable ro volume

Code
→ 每次 run 增量 rsync

Image
→ runtime cache

Checkpoint
→ output volume

Log
→ continuous persistence
```

例如：

```bash
cbox volume create \
    --source /data/Potsdam_512 \
    --immutable \
    potsdam
```

以后几十个实验共享。

---

# 75. 你的实际工作流

建立：

```text
WWTP/
├── Cboxfile
├── cbox-compose.yaml
├── configs/
├── mmseg/
└── scripts/
```

Cboxfile：

```dockerfile
FROM colab/python:3

APT git
APT rsync
APT libgl1

COPY requirements_colab.txt /tmp/

RUN pip install -r /tmp/requirements_colab.txt

WORKDIR /workspace
```

构建：

```bash
cbox build -t wwtp:mmseg .
```

数据：

```bash
cbox volume create \
    --source /data/WWTP_v2_512 \
    --immutable \
    wwtp-v2
```

训练：

```bash
cbox run -d \
    --name full \
    --gpu L4,T4 \
    --mount source=wwtp-v2,target=/data,mode=ro \
    -v ./runs/full:/output:output \
    wwtp:mmseg \
    python tools/train.py \
        configs/full.py \
        --work-dir /output
```

以后：

```bash
cbox ps
```

```bash
cbox logs -f full
```

```bash
cbox exec -it full bash
```

---

# 76. 双账户

最终：

```text
                 cboxd
                   │
            Internal Scheduler
             ┌─────┴─────┐
             ▼           ▼
         Context A   Context B
             │           │
         Colab A     Colab B
             │           │
         T4/L4       T4/L4
```

运行两个实验：

```bash
cbox compose up -d
```

CBox 自动调度。

---

# 77. Provider 可扩展设计

未来：

```text
CBox
 │
 ├── ColabProvider
 ├── SSHProvider
 ├── RunPodProvider
 ├── GCPProvider
 └── LocalProvider
```

甚至：

```bash
cbox run \
    --context runpod \
    mmseg \
    python train.py
```

用户 API 不变。

这也是为什么：

```text
Container
```

不能直接等价为：

```text
ColabSession
```

两者必须解耦。

---

# 78. 推荐的 MVP 边界

真正适合第一版发布的功能：

```text
cboxd

cbox build
cbox images

cbox run
cbox ps
cbox logs
cbox exec
cbox cp
cbox stop
cbox rm

cbox volume

单账户

T4/L4 GPU

SSH transport

rsync

tmux

SQLite
```

先不要实现：

```text
Compose
自动多账户
Web UI
A100/H100调度
复杂 recovery
provider marketplace
```

---

# 79. 第二版

加入：

```text
start
restart
restart policy

automatic recovery

named cache volumes

GPU fallback

multi-account contexts

runtime pool
```

---

# 80. 第三版

加入：

```text
Compose

experiment queue

multi-account scheduler

dataset affinity

image affinity

TUI / Web dashboard
```

---

# 81. 最终产品心智模型

用户永远只看到：

```text
Cboxfile
       ↓
    Image
       ↓
cbox run
       ↓
 Container
       │
       ├── Volume
       └── GPU
```

Engine 内部才是：

```text
Container
   ↓
Scheduler
   ↓
Runtime
   ↓
Colab Provider
   ↓
google-colab-cli
   ↓
WebSocket SSH
   ↓
Colab VM
```

这层隔离是整个项目最重要的架构决策。

---

# 82. 开发优先级

P0：

```text
ColabProvider
SSHTransport
Container
run
exec
logs
stop
```

P1：

```text
daemon
SQLite
Volume
cp
stats
Image
```

P2：

```text
restart
recovery
cache
context
multi-account
```

P3：

```text
compose
scheduler
provider extensions
dashboard
```

---

# 83. 核心判断

这个工具不应该被设计成：

```text
“Colab CLI Wrapper”
```

也不应该只是：

```text
“科研任务调度器”
```

正确定位应该是：

> **CBox 是一个面向 Ephemeral GPU Compute 的 Docker-like Runtime Engine，Colab 是第一个 Provider。**

这样既能解决你当前：

```text
远程服务器
+
Colab GPU
+
大量遥感数据
+
深度学习训练
+
消融实验
```

的问题，同时架构不会被 Colab 本身锁死。
