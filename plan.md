# CBox：Go 版本开发设计

## 1. 项目定位

CBox 是一个：

> Docker-like Ephemeral GPU Runtime Engine

第一阶段 Provider：

```text
Google Colab
```

未来：

```text
Colab
RunPod
GCP
SSH Server
Local GPU
```

用户体验：

```bash
cbox build -t mmseg:latest .

cbox run -d \
  --name full \
  --gpu L4,T4 \
  -v /data/WWTP:/data:ro \
  -v ./runs/full:/output:output \
  mmseg:latest \
  python tools/train.py configs/full.py

cbox ps
cbox logs -f full
cbox exec -it full bash
cbox stats full

cbox stop full
cbox start full
cbox rm full
```

核心目标：

```text
Docker-like UX
+
Remote GPU
+
Disposable Runtime
+
Persistent State
+
Automatic Recovery
```

---

# 2. 核心设计原则

## 2.1 不把 Colab 暴露给用户

用户不应该直接处理：

```text
colab new
colab ssh
colab status
ProxyCommand
sessions.json
rsync
tmux
```

这些全部属于：

```text
ColabProvider
```

内部实现。

---

## 2.2 用户模型只有五个主要概念

```text
Image
Container
Volume
Context
Compose
```

内部才存在：

```text
Runtime
Worker
Provider
Session
Account
Scheduler
```

---

## 2.3 Server 是永久状态源

```text
Remote Linux Server
       │
       ├── CBox DB
       ├── Image manifests
       ├── Volume metadata
       ├── Dataset
       ├── Source code
       ├── Checkpoint
       └── Logs

Colab VM
       │
       └── Disposable Cache
```

原则：

> Colab 随时可以全部消失，而不导致实验永久状态丢失。

---

# 3. Go 技术栈

推荐：

```text
Go >= 1.24
```

核心依赖尽量少。

CLI：

```text
github.com/spf13/cobra
```

配置：

```text
gopkg.in/yaml.v3
```

SQLite：

```text
database/sql
modernc.org/sqlite
```

UUID：

```text
github.com/google/uuid
```

系统日志：

```text
log/slog
```

HTTP：

```text
net/http
```

Unix Socket：

```text
net
```

进程：

```text
os/exec
```

并发：

```text
context
sync
sync/atomic
channels
```

测试：

```text
testing
httptest
```

尽量不要引入：

```text
Gin
Echo
Fiber
GORM
Redis
Celery-like queue
```

---

# 4. 二进制设计

生成两个核心程序：

```text
cbox
cboxd
```

## cbox

CLI Client。

职责：

```text
CLI parsing
调用 cboxd API
输出格式化
interactive attach
stdin/stdout forwarding
```

---

## cboxd

长期运行的 Engine。

职责：

```text
Container lifecycle
Runtime lifecycle
Scheduler
Volume sync
Image materialization
Health monitor
Recovery
Logs
Metrics
Provider management
```

---

# 5. 总体架构

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
 Colab Runtime
       │
       ▼
 SSH Proxy
       │
       ▼
 Remote process
```

---

# 6. Go Repository 结构

推荐：

```text
cbox/
├── cmd/
│   ├── cbox/
│   │   └── main.go
│   │
│   └── cboxd/
│       └── main.go
│
├── internal/
│
│   ├── api/
│   │   ├── server.go
│   │   ├── middleware.go
│   │   ├── containers.go
│   │   ├── images.go
│   │   ├── volumes.go
│   │   ├── contexts.go
│   │   └── system.go
│
│   ├── client/
│   │   ├── client.go
│   │   └── transport.go
│
│   ├── container/
│   │   ├── model.go
│   │   ├── service.go
│   │   ├── state.go
│   │   ├── runner.go
│   │   └── recovery.go
│
│   ├── image/
│   │   ├── model.go
│   │   ├── parser.go
│   │   ├── builder.go
│   │   └── materializer.go
│
│   ├── volume/
│   │   ├── model.go
│   │   ├── service.go
│   │   ├── manifest.go
│   │   └── sync.go
│
│   ├── runtime/
│   │   ├── model.go
│   │   ├── service.go
│   │   ├── monitor.go
│   │   └── pool.go
│
│   ├── scheduler/
│   │   ├── scheduler.go
│   │   └── scoring.go
│
│   ├── provider/
│   │   ├── provider.go
│   │   └── colab/
│   │       ├── provider.go
│   │       ├── cli.go
│   │       ├── parser.go
│   │       └── profile.go
│
│   ├── transport/
│   │   ├── transport.go
│   │   ├── ssh.go
│   │   ├── rsync.go
│   │   └── tar.go
│
│   ├── worker/
│   │   ├── bootstrap.go
│   │   ├── executor.go
│   │   ├── health.go
│   │   └── metrics.go
│
│   ├── store/
│   │   ├── db.go
│   │   ├── migrations.go
│   │   ├── container_repo.go
│   │   ├── image_repo.go
│   │   ├── volume_repo.go
│   │   ├── runtime_repo.go
│   │   └── event_repo.go
│
│   ├── context/
│   │   ├── model.go
│   │   └── service.go
│
│   ├── compose/
│   │   ├── model.go
│   │   ├── parser.go
│   │   └── service.go
│
│   ├── config/
│   │   ├── config.go
│   │   └── paths.go
│
│   └── errors/
│       └── errors.go
│
├── pkg/
│   └── api/
│       └── types.go
│
├── scripts/
│   └── install.sh
│
├── worker/
│   ├── bootstrap.sh
│   ├── entrypoint.sh
│   └── health.sh
│
├── examples/
│   ├── Cboxfile
│   └── cbox-compose.yaml
│
├── migrations/
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

# 7. Domain Model

核心 domain 不允许依赖：

```text
Colab
SSH
SQLite
Cobra
```

这是关键。

---

# 8. Container

```go
type Container struct {
    ID       string
    Name     string
    ImageID  string

    Command  []string
    Env      map[string]string
    WorkDir  string

    Resource ResourceSpec
    Mounts   []Mount

    State    ContainerState

    RuntimeID string

    RestartPolicy RestartPolicy

    ResumeCommand []string

    ExitCode *int

    CreatedAt time.Time
    StartedAt *time.Time
    FinishedAt *time.Time
}
```

---

# 9. ContainerState

```go
type ContainerState string

const (
    ContainerCreated      ContainerState = "created"
    ContainerProvisioning ContainerState = "provisioning"
    ContainerPreparing    ContainerState = "preparing"
    ContainerStarting     ContainerState = "starting"
    ContainerRunning      ContainerState = "running"

    ContainerExited       ContainerState = "exited"

    ContainerStopping     ContainerState = "stopping"
    ContainerStopped      ContainerState = "stopped"

    ContainerInterrupted  ContainerState = "interrupted"
    ContainerRecovering   ContainerState = "recovering"

    ContainerFailed       ContainerState = "failed"
)
```

所有状态转换必须统一走：

```go
Transition(...)
```

不要：

```go
container.State = ...
```

散落在代码中。

---

# 10. State Machine

允许：

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
```

正常完成：

```text
RUNNING
 ↓
EXITED
```

主动停止：

```text
RUNNING
 ↓
STOPPING
 ↓
STOPPED
```

异常：

```text
RUNNING
 ↓
INTERRUPTED
 ↓
RECOVERING
 ↓
PROVISIONING
```

---

# 11. Runtime

```go
type Runtime struct {
    ID string

    Provider string
    Session  string
    Profile  string

    RequestedGPU string
    ActualGPU    string

    State RuntimeState

    CreatedAt time.Time
    LastSeen  time.Time

    Cache RuntimeCache
}
```

---

# 12. RuntimeState

```go
type RuntimeState string

const (
    RuntimeProvisioning RuntimeState = "provisioning"
    RuntimeConnecting   RuntimeState = "connecting"
    RuntimeReady        RuntimeState = "ready"
    RuntimeBusy         RuntimeState = "busy"
    RuntimeIdle         RuntimeState = "idle"
    RuntimeStopping     RuntimeState = "stopping"
    RuntimeStopped      RuntimeState = "stopped"
    RuntimeLost         RuntimeState = "lost"
)
```

---

# 13. ResourceSpec

```go
type ResourceSpec struct {
    GPUPreference []string

    HighMemory bool

    Profile string
}
```

例如：

```go
ResourceSpec{
    GPUPreference: []string{
        "L4",
        "T4",
    },
}
```

表示：

```text
优先 L4
L4 不可用则 T4
```

---

# 14. Provider Interface

这是整个架构最重要的接口。

```go
type Provider interface {
    Name() string

    CreateRuntime(
        ctx context.Context,
        req RuntimeRequest,
    ) (*RuntimeHandle, error)

    GetRuntime(
        ctx context.Context,
        runtime *Runtime,
    ) (*RuntimeStatus, error)

    StopRuntime(
        ctx context.Context,
        runtime *Runtime,
    ) error

    ListRuntimes(
        ctx context.Context,
    ) ([]RuntimeStatus, error)

    OpenTransport(
        ctx context.Context,
        runtime *Runtime,
    ) (transport.Transport, error)
}
```

`ContainerService` 永远不允许直接：

```go
exec.Command("colab", ...)
```

必须：

```text
ContainerService
→ RuntimeService
→ Provider
```

---

# 15. ColabProvider

```go
type ColabProvider struct {
    binary string

    profiles ProfileStore

    logger *slog.Logger
}
```

实际调用：

```text
google-colab-cli
```

例如：

```go
func (p *ColabProvider) CreateRuntime(
    ctx context.Context,
    req RuntimeRequest,
) (*RuntimeHandle, error) {

    args := []string{
        "new",
        "-s", req.Session,
    }

    if req.GPU != "" {
        args = append(
            args,
            "--gpu",
            req.GPU,
        )
    }

    if req.HighMemory {
        args = append(
            args,
            "--high-mem",
        )
    }

    cmd := exec.CommandContext(
        ctx,
        p.binary,
        args...,
    )

    ...
}
```

---

# 16. 禁止 import Colab CLI 私有代码

不要：

```go
// 不存在直接 Go import，但同理不要依赖 Python private module
```

也不要直接请求：

```text
Colab backend private endpoints
```

CBox 与 Colab 的唯一稳定边界：

```text
google-colab-cli executable
```

---

# 17. CLI Wrapper

单独写：

```go
type CLI struct {
    Binary string

    Profile Profile
}
```

方法：

```go
func (c *CLI) New(...)
func (c *CLI) Status(...)
func (c *CLI) Sessions(...)
func (c *CLI) Stop(...)
```

Provider 不直接大量拼命令。

---

# 18. Account Profile

```go
type Profile struct {
    Name string

    HomeDir    string
    ConfigFile string
    OAuthFile  string
}
```

运行：

```go
cmd.Env = append(
    os.Environ(),
    "HOME="+profile.HomeDir,
)
```

通过 profile 隔离：

```text
Account A
Account B
```

---

# 19. Transport Interface

```go
type Transport interface {
    Exec(
        ctx context.Context,
        command []string,
        opts ExecOptions,
    ) (*ExecResult, error)

    CopyTo(
        ctx context.Context,
        src string,
        dst string,
    ) error

    CopyFrom(
        ctx context.Context,
        src string,
        dst string,
    ) error

    SyncTo(
        ctx context.Context,
        src string,
        dst string,
        opts SyncOptions,
    ) error

    SyncFrom(
        ctx context.Context,
        src string,
        dst string,
        opts SyncOptions,
    ) error
}
```

---

# 20. SSHTransport

MVP 不要自己实现 SSH。

直接调用：

```text
OpenSSH
```

理由：

```text
PTY
terminal resize
ProxyCommand
ControlMaster
scp/rsync compatibility
signal handling
```

OpenSSH 已经解决。

---

# 21. SSH Config

Engine 动态生成：

```text
~/.local/share/cbox/ssh/
```

例如：

```text
Host cbox-ca84bd
    User root
    IdentityFile ~/.local/share/cbox/keys/worker

    ProxyCommand /usr/bin/colab \
        --config PROFILE_CONFIG \
        ssh \
        --proxy-mode \
        -s cbox-ca84bd \
        -i ~/.local/share/cbox/keys/worker

    ControlMaster auto
    ControlPersist 600
    ControlPath ~/.cache/cbox/ssh/%C

    StrictHostKeyChecking no
    UserKnownHostsFile /dev/null
```

---

# 22. Exec 实现

```go
func (t *SSHTransport) Exec(
    ctx context.Context,
    command []string,
) error {

    args := []string{
        "-F",
        t.configFile,
        t.host,
        "--",
    }

    args = append(args, command...)

    cmd := exec.CommandContext(
        ctx,
        "ssh",
        args...,
    )

    return cmd.Run()
}
```

---

# 23. Interactive Exec

```bash
cbox exec -it full bash
```

这里 CLI 不应该经过普通 JSON stdout。

流程：

```text
cbox
 ↓
向 daemon 请求 container transport info
 ↓
CLI 自己 exec ssh -tt
 ↓
直接接管当前 terminal
```

原因：

PTY 不适合简单 HTTP JSON 转发。

---

# 24. cbox CLI 与 cboxd 通信

使用：

```text
HTTP/1.1
over
Unix Domain Socket
```

Socket：

```text
$XDG_RUNTIME_DIR/cbox/cbox.sock
```

Go：

```go
listener, err := net.Listen(
    "unix",
    socketPath,
)
```

---

# 25. Daemon HTTP Server

直接：

```go
mux := http.NewServeMux()
```

Go 新版 ServeMux 足够。

例如：

```go
mux.HandleFunc(
    "GET /v1/containers",
    handler.ListContainers,
)

mux.HandleFunc(
    "POST /v1/containers",
    handler.CreateContainer,
)

mux.HandleFunc(
    "POST /v1/containers/{id}/start",
    handler.StartContainer,
)
```

不需要 Gin。

---

# 26. Client

```go
type Client struct {
    HTTP *http.Client
}
```

自定义 transport：

```go
DialContext:
    net.Dial("unix", socket)
```

CLI：

```text
Cobra
↓
Client
↓
Unix socket
↓
cboxd
```

---

# 27. API 设计

## Container

```text
POST   /v1/containers
GET    /v1/containers
GET    /v1/containers/{id}

POST   /v1/containers/{id}/start
POST   /v1/containers/{id}/stop
POST   /v1/containers/{id}/restart

DELETE /v1/containers/{id}

POST   /v1/containers/{id}/exec

GET    /v1/containers/{id}/logs
GET    /v1/containers/{id}/stats
```

---

# 28. Image API

```text
POST   /v1/images/build
GET    /v1/images
GET    /v1/images/{id}
DELETE /v1/images/{id}
```

---

# 29. Volume API

```text
POST   /v1/volumes
GET    /v1/volumes
GET    /v1/volumes/{name}
DELETE /v1/volumes/{name}
```

---

# 30. Image

```go
type Image struct {
    ID string

    Tags []string

    Manifest ImageManifest

    CreatedAt time.Time
}
```

---

# 31. Cboxfile

建议第一版不要真的复刻 Dockerfile parser。

采用简单 DSL：

```dockerfile
FROM colab/python

APT git rsync libgl1

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

---

# 32. Parser

定义：

```go
type Instruction interface {
    Type() InstructionType
}
```

实现：

```text
FromInstruction
AptInstruction
PipInstruction
EnvInstruction
WorkdirInstruction
CopyInstruction
RunInstruction
CmdInstruction
```

不要一开始支持：

```text
ARG
ONBUILD
ENTRYPOINT
USER
SHELL
ADD
multi-stage build
```

---

# 33. Image Build

`cbox build` 并不真正创建 Colab Runtime。

逻辑：

```text
Parse
↓
Normalize
↓
Validate
↓
Collect referenced files
↓
Hash
↓
Create manifest
↓
Create bootstrap plan
↓
Persist
```

---

# 34. Image Hash

建议：

```text
SHA256(
 normalized Cboxfile
 +
 referenced file contents
)
```

而不是：

```text
mtime
```

确保 reproducibility。

---

# 35. ImageManifest

```go
type ImageManifest struct {
    Base string

    Apt []string
    Pip []string

    Env map[string]string

    WorkDir string

    Copies []CopySpec

    Runs []string

    Command []string
}
```

---

# 36. Image Materialization

Container 启动后：

```text
Worker
 ↓
check image cache
 ↓
MISS
 ↓
APT
 ↓
PIP
 ↓
COPY
 ↓
RUN
 ↓
ready marker
```

例如：

```text
/content/.cbox/images/<imageID>/ready
```

---

# 37. Image cache

Runtime：

```go
type RuntimeCache struct {
    Images  map[string]bool
    Volumes map[string]string
}
```

用于 Scheduler。

---

# 38. Volume

```go
type Volume struct {
    ID string

    Name string

    Source string

    Mode VolumeMode

    Immutable bool

    ManifestHash string

    CreatedAt time.Time
}
```

---

# 39. VolumeMode

```go
type VolumeMode string

const (
    VolumeReadOnly VolumeMode = "ro"

    VolumeReadWrite VolumeMode = "rw"

    VolumeOutput VolumeMode = "output"

    VolumeCache VolumeMode = "cache"
)
```

---

# 40. Mount

```go
type Mount struct {
    VolumeID string

    Source string

    Target string

    Mode VolumeMode
}
```

---

# 41. Volume 实际语义

例如：

```bash
-v /data/Potsdam:/data:ro
```

内部：

```text
/data/Potsdam
      ↓
rsync
      ↓
/content/.cbox/volumes/<hash>
      ↓
symlink
      ↓
/data
```

不是真实网络 mount。

---

# 42. `output` Volume

例如：

```bash
-v ./runs/full:/output:output
```

行为：

启动：

```text
server → runtime
```

训练：

```text
runtime → server
```

周期同步：

```text
every 300/600 sec
```

停止：

```text
final sync
```

---

# 43. SyncService

```go
type SyncService struct {
    interval time.Duration

    transportFactory TransportFactory

    logger *slog.Logger
}
```

每个 running Container：

```go
func (s *SyncService) Start(
    ctx context.Context,
    c *Container,
) {
    ticker := time.NewTicker(s.interval)

    go func() {
        defer ticker.Stop()

        for {
            select {
            case <-ctx.Done():
                return

            case <-ticker.C:
                s.syncOutputs(...)
            }
        }
    }()
}
```

---

# 44. 为什么 Go 特别适合这里

一个 Container 可以对应：

```text
goroutine:
 runtime monitor

goroutine:
 output sync

goroutine:
 process monitor

goroutine:
 metrics

goroutine:
 recovery controller
```

全部绑定：

```go
containerCtx
```

当：

```bash
cbox stop
```

：

```go
cancel()
```

整个 Container 的后台协程统一退出。

---

# 45. Context hierarchy

推荐：

```text
daemonCtx
   │
   ├── runtimeCtx
   │      │
   │      └── containerCtx
   │
   └── schedulerCtx
```

关闭 daemon：

```go
daemonCancel()
```

逐层取消。

---

# 46. Container Runtime Controller

核心：

```go
type Controller struct {
    container *Container

    cancel context.CancelFunc

    wg sync.WaitGroup
}
```

每个运行中的 Container 一个 controller。

---

# 47. Scheduler

接口：

```go
type Scheduler interface {
    AcquireRuntime(
        ctx context.Context,
        container *Container,
    ) (*Runtime, error)
}
```

策略：

```text
1 GPU match
2 idle runtime
3 volume cache
4 image cache
5 account availability
6 allocate new runtime
```

---

# 48. Scheduler scoring

例如：

```go
func score(
    runtime Runtime,
    container Container,
) int {

    score := 0

    if runtime.State == RuntimeIdle {
        score += 100
    }

    if runtime.Cache.Images[
        container.ImageID
    ] {
        score += 30
    }

    for _, mount := range container.Mounts {
        if runtime.Cache.Volumes[
            mount.VolumeID
        ] != "" {
            score += 50
        }
    }

    return score
}
```

---

# 49. MVP 调度原则

第一版严格：

```text
一个 Runtime
同时最多一个 Container
```

不要做：

```text
一个 L4 同时跑三个训练任务
```

Runtime 可以：

```text
sequentially reuse
```

不能：

```text
parallel reuse
```

---

# 50. Runtime Pool

Container 完成后：

```text
Runtime
RUNNING
 ↓
IDLE
```

不要立即：

```text
stop
```

默认：

```text
idleTimeout = 30 min
```

方便下一个实验复用：

```text
Dataset
Image
Python packages
```

---

# 51. Runtime pool goroutine

```go
func (p *Pool) Reaper(
    ctx context.Context,
) {
    ticker := time.NewTicker(
        time.Minute,
    )

    for {
        select {
        case <-ctx.Done():
            return

        case <-ticker.C:
            p.stopExpired()
        }
    }
}
```

---

# 52. Worker bootstrap

Runtime Ready 后：

```text
/content/.cbox/
├── bin/
├── containers/
├── images/
├── volumes/
├── cache/
└── worker.json
```

bootstrap：

```text
apt install:
rsync
tmux
```

尽量不要在 Worker 上跑完整 CBox Go daemon。

---

# 53. Remote execution

第一版继续用：

```text
tmux
```

因为成熟稳定。

Container 启动：

```bash
tmux new-session -d \
    -s cbox-ca84bd \
    "/content/.cbox/bin/entrypoint ca84bd"
```

---

# 54. Remote metadata

```text
/content/.cbox/containers/<id>/
├── config.json
├── stdout.log
├── stderr.log
├── pid
├── exit_code
└── state
```

---

# 55. Process Monitor

Daemon：

```text
每 10~30s
```

检查：

```text
Runtime alive?
tmux session alive?
exit_code exists?
```

逻辑：

```text
tmux exists
→ RUNNING

tmux absent + exit_code = 0
→ EXITED

tmux absent + exit_code != 0
→ FAILED

runtime inaccessible
→ INTERRUPTED
```

---

# 56. Recovery

这是 V2 的核心。

Runtime 丢失：

```text
RUNNING
 ↓
INTERRUPTED
```

检查：

```go
container.RestartPolicy
```

如果允许：

```text
INTERRUPTED
 ↓
RECOVERING
 ↓
PROVISIONING
```

---

# 57. RestartPolicy

```go
type RestartPolicy string

const (
    RestartNo RestartPolicy = "no"

    RestartOnFailure RestartPolicy =
        "on-failure"

    RestartUnlessStopped RestartPolicy =
        "unless-stopped"

    RestartAlways RestartPolicy =
        "always"
)
```

---

# 58. ResumeCommand

普通程序无法凭空恢复。

所以：

```go
type Container struct {
    ...
    ResumeCommand []string
}
```

第一次：

```text
Command
```

恢复：

```text
ResumeCommand
```

例如：

```bash
--resume-command \
  "python tools/train.py configs/a.py --resume"
```

---

# 59. Deep Learning Recovery

例如：

```text
/output/latest.pth
```

通过 output volume：

```text
Colab
↓
Server
```

新 Runtime：

```text
Server
↓
Colab
```

然后：

```bash
python tools/train.py ... --resume
```

---

# 60. SQLite

推荐：

```text
modernc.org/sqlite
```

这样可以：

```text
pure Go
no CGO
cross compile easier
```

---

# 61. DB tables

至少：

```text
images
image_tags

containers

volumes
container_mounts

runtimes

runtime_cache

contexts

events
```

---

# 62. Container SQL

```sql
CREATE TABLE containers (
    id TEXT PRIMARY KEY,

    name TEXT UNIQUE NOT NULL,

    image_id TEXT NOT NULL,

    state TEXT NOT NULL,

    command_json TEXT NOT NULL,

    env_json TEXT,

    resource_json TEXT,

    restart_policy TEXT NOT NULL,

    resume_command_json TEXT,

    runtime_id TEXT,

    exit_code INTEGER,

    created_at DATETIME NOT NULL,

    started_at DATETIME,

    finished_at DATETIME
);
```

---

# 63. Event store

所有重要操作：

```text
container.created
container.started
container.exited

runtime.created
runtime.ready
runtime.lost

volume.sync.started
volume.sync.completed

recovery.started
recovery.completed
```

写：

```text
events
```

表。

---

# 64. Event struct

```go
type Event struct {
    ID string

    ObjectType string
    ObjectID string

    Type string

    Timestamp time.Time

    Payload json.RawMessage
}
```

后续：

```bash
cbox events full
```

可以排错。

---

# 65. Repository pattern

例如：

```go
type ContainerRepository interface {
    Create(
        ctx context.Context,
        c *Container,
    ) error

    Get(
        ctx context.Context,
        id string,
    ) (*Container, error)

    UpdateState(
        ctx context.Context,
        id string,
        state ContainerState,
    ) error

    List(
        ctx context.Context,
    ) ([]Container, error)
}
```

业务逻辑不要直接写 SQL。

---

# 66. Transaction

状态切换：

```text
Container state
Runtime binding
Event
```

应该：

```text
同一 SQLite transaction
```

避免 daemon crash 后：

```text
container RUNNING
但 runtime_id 没写
```

---

# 67. Daemon recovery on startup

`cboxd` 启动：

```text
load DB
↓
找：
 RUNNING
 PREPARING
 STARTING
 RECOVERING
↓
查询 runtime
↓
reconcile
```

类似 Kubernetes reconciliation。

---

# 68. Reconciliation Loop

这是长期来看非常关键的设计。

不要把系统写成：

```text
执行一次命令
假设成功
```

而要：

```text
Desired State
vs
Observed State
```

例如：

```go
Container{
    DesiredState: Running,
}
```

实际：

```text
Runtime missing
```

Controller：

```text
重新 provision
```

---

# 69. 可以考虑增加 DesiredState

例如：

```go
type DesiredState string

const (
    DesiredRunning DesiredState = "running"

    DesiredStopped DesiredState = "stopped"
)
```

这是比只保存 `state` 更稳的设计。

---

# 70. Controller pattern

核心思想：

```text
用户：
cbox start

只是写：
DesiredState = RUNNING

Controller：
负责最终让实际状态达到 RUNNING
```

以后自动恢复会非常自然。

---

# 71. CLI

推荐 Cobra：

```text
cmd/cbox/root.go

cmd/cbox/run.go
cmd/cbox/ps.go
cmd/cbox/logs.go
cmd/cbox/exec.go
...
```

---

# 72. CLI hierarchy

```text
cbox

├── build
├── images
├── rmi
│
├── create
├── run
├── start
├── stop
├── restart
├── rm
│
├── ps
├── inspect
├── logs
├── exec
├── cp
├── stats
│
├── volume
│   ├── create
│   ├── ls
│   ├── inspect
│   └── rm
│
├── context
│   ├── create
│   ├── ls
│   ├── use
│   └── rm
│
└── compose
    ├── up
    ├── ps
    ├── logs
    └── down
```

---

# 73. `cbox run`

流程：

```text
Parse CLI
↓
CreateContainer API
↓
StartContainer API
↓
如果 -d
    return ID
否则
    attach logs
```

---

# 74. cbox ps

Server 返回 JSON：

```json
[
  {
    "id": "abc123",
    "name": "full",
    "image": "mmseg:latest",
    "gpu": "L4",
    "state": "running"
  }
]
```

Client 自己格式化表格。

---

# 75. 不要让 daemon 输出表格

Daemon 永远：

```text
structured JSON
```

CLI：

```text
human-readable output
```

Agent：

```text
--format json
```

可以直接调用。

---

# 76. 支持 `--format json`

例如：

```bash
cbox ps --format json
```

对于以后 Agent/Codex 调用很重要。

---

# 77. cbox logs

非 follow：

```text
HTTP API
↓
返回持久日志
```

follow：

推荐：

```text
HTTP streaming
```

或者：

```text
CLI 直接 SSH tail
```

MVP 推荐后者。

---

# 78. cbox stats

调用：

```text
SSH
↓
nvidia-smi --query...
↓
parse CSV
```

定义：

```go
type Stats struct {
    GPUName string

    GPUMemoryUsedMB int
    GPUMemoryTotalMB int

    GPUUtilization float64

    MemoryUsedMB int64
    MemoryTotalMB int64
}
```

---

# 79. Context

Context 定义：

```go
type Context struct {
    Name string

    Provider string

    Profile string

    AutoSchedule bool
}
```

例如：

```text
colab-a
colab-b
colab-pool
```

---

# 80. 多账户

推荐：

```text
~/.local/share/cbox/profiles/
├── colab-a/
│   └── home/
└── colab-b/
    └── home/
```

每次：

```go
cmd.Env = append(
    os.Environ(),
    "HOME="+profile.Home,
)
```

隔离官方 CLI 状态。

---

# 81. Compose

建议 V3 做。

```yaml
services:

  full:
    image: mmseg:latest

    gpu:
      - L4
      - T4

    volumes:
      - /data/WWTP:/data:ro
      - ./runs/full:/output:output

    command:
      - python
      - tools/train.py
      - configs/full.py

    restart: unless-stopped


  no_freq:
    image: mmseg:latest

    gpu:
      - T4

    volumes:
      - /data/WWTP:/data:ro
      - ./runs/no_freq:/output:output

    command:
      - python
      - tools/train.py
      - configs/no_freq.py
```

---

# 82. Compose 不需要单独 Job 系统

Compose 的 Service 最终：

```text
Service
 ↓
ContainerSpec
 ↓
Container
```

避免：

```text
Container
Job
Task
Service
```

概念爆炸。

---

# 83. Configuration

```text
~/.config/cbox/config.yaml
```

例如：

```yaml
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
```

---

# 84. Config struct

```go
type Config struct {
    Engine EngineConfig `yaml:"engine"`

    Runtime RuntimeConfig `yaml:"runtime"`

    Sync SyncConfig `yaml:"sync"`

    SSH SSHConfig `yaml:"ssh"`

    Provider ProviderConfig `yaml:"provider"`

    Recovery RecoveryConfig `yaml:"recovery"`
}
```

---

# 85. Duration parsing

推荐使用：

```go
time.ParseDuration
```

所以配置：

```text
10m
30m
1h
```

自然支持。

---

# 86. Logging

使用：

```go
log/slog
```

例如：

```go
logger.Info(
    "runtime created",
    "runtime_id", runtime.ID,
    "gpu", runtime.ActualGPU,
)
```

daemon 日志：

```text
~/.local/share/cbox/logs/cboxd.log
```

systemd 下：

```bash
journalctl --user -u cboxd
```

---

# 87. Structured logging

调试时：

```json
{
  "level": "INFO",
  "event": "runtime.created",
  "runtime": "abc",
  "gpu": "T4"
}
```

以后很好做 UI。

---

# 88. Errors

定义 error types。

```go
type AllocationError struct {
    GPU string
    Err error
}

func (e *AllocationError) Error() string {
    ...
}
```

分类：

```text
ProviderError

AllocationError

TransportError

AuthenticationError

ImageError

VolumeError

ContainerError

RecoveryError
```

---

# 89. errors.Is / errors.As

必须使用 Go idiomatic error wrapping：

```go
return fmt.Errorf(
    "create runtime: %w",
    err,
)
```

不要依靠：

```text
string matching
```

作为业务错误机制。

---

# 90. 外部 CLI 错误解析

但 `colab` 本身属于 subprocess。

所以：

```go
type CommandError struct {
    ExitCode int
    Stdout string
    Stderr string
}
```

ColabProvider 再把它转换成：

```text
AllocationError
AuthenticationError
ProviderError
```

---

# 91. CommandRunner

建议把 subprocess 抽象出来：

```go
type CommandRunner interface {
    Run(
        ctx context.Context,
        name string,
        args []string,
        opts CommandOptions,
    ) (*CommandResult, error)
}
```

价值：

```text
单元测试可 mock
```

否则 Provider 非常难测试。

---

# 92. CommandResult

```go
type CommandResult struct {
    ExitCode int

    Stdout []byte
    Stderr []byte
}
```

---

# 93. Security

SSH：

```text
专用 worker key
```

路径：

```text
~/.local/share/cbox/keys/
```

权限：

```text
0600
```

---

# 94. Worker 不保存 server credential

传输永远：

```text
Server
主动 push/pull
Colab
```

而不是：

```text
Colab
SSH 回 server
```

这非常重要。

---

# 95. Secret management

MVP：

```bash
cbox run \
  --secret HF_TOKEN \
  ...
```

CBox 从当前环境：

```text
HF_TOKEN
```

读取。

传给远程：

```text
environment variable
```

Container 结束后清理。

---

# 96. Secret 禁止写 DB 明文

DB 只保存：

```text
secret names
```

例如：

```json
[
  "HF_TOKEN"
]
```

不保存：

```text
实际 token
```

---

# 97. systemd

安装：

```text
~/.config/systemd/user/cboxd.service
```

例如：

```ini
[Unit]
Description=CBox Runtime Engine
After=network-online.target

[Service]
ExecStart=/usr/local/bin/cboxd
Restart=on-failure

[Install]
WantedBy=default.target
```

---

# 98. cboxd 生命周期

启动：

```text
load config
↓
open DB
↓
migrate
↓
load providers
↓
reconcile state
↓
start monitor loops
↓
start HTTP Unix server
```

关闭：

```text
SIGTERM
↓
cancel daemon context
↓
stop background goroutines
↓
flush DB
↓
close socket
```

不应该：

```text
自动 stop 所有 Colab runtimes
```

否则 daemon restart 会杀实验。

---

# 99. Graceful Shutdown

```go
ctx, cancel := signal.NotifyContext(
    context.Background(),
    syscall.SIGINT,
    syscall.SIGTERM,
)
defer cancel()
```

所有 background loop 都监听：

```go
ctx.Done()
```

---

# 100. Testing Strategy

四层。

## Unit Tests

完全不调用：

```text
Colab
SSH
```

测试：

```text
state machine
scheduler
Cboxfile parser
hashing
volume manifest
config
API handlers
```

---

# 101. Mock Provider

```go
type FakeProvider struct {
    ...
}
```

可以：

```text
CreateRuntime成功
GPU allocation fail
Runtime lost
```

模拟。

---

# 102. Fake Transport

模拟：

```text
exec
copy
sync
```

用于测试：

```text
ContainerService
RecoveryService
```

---

# 103. Integration Tests

真实：

```text
sqlite
Unix socket
subprocess
```

但 mock Colab。

---

# 104. Provider Integration

单独标记：

```go
//go:build integration_colab
```

运行：

```bash
go test \
  -tags integration_colab \
  ./internal/provider/colab/...
```

会真实消费 Colab 资源。

不能默认 CI 执行。

---

# 105. E2E

最重要：

```text
cbox build
↓
cbox run T4
↓
copy 100MB test dataset
↓
execute PyTorch test
↓
logs
↓
stats
↓
output
↓
stop
```

---

# 106. Recovery E2E

训练：

```text
epoch 1
epoch 2
epoch 3
```

然后主动：

```bash
colab stop
```

预期：

```text
CBox detects LOST
↓
container interrupted
↓
new runtime
↓
restore
↓
resume
```

---

# 107. CI

GitHub Actions：

```text
go fmt
go vet
go test
go build
```

matrix：

```text
linux amd64
linux arm64
```

第一阶段实际运行平台：

```text
Linux
```

即可。

---

# 108. Cross Compile

```bash
CGO_ENABLED=0 \
GOOS=linux \
GOARCH=amd64 \
go build ./cmd/cbox
```

以及：

```bash
go build ./cmd/cboxd
```

因为：

```text
modernc sqlite
```

可避免 CGO。

---

# 109. Release

最终：

```text
cbox_linux_amd64.tar.gz

cbox
cboxd
```

安装：

```bash
sudo install cbox /usr/local/bin/
sudo install cboxd /usr/local/bin/
```

---

# 110. Makefile

建议：

```make
build:
	go build -o bin/cbox ./cmd/cbox
	go build -o bin/cboxd ./cmd/cboxd

test:
	go test ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

install:
	go install ./cmd/cbox
	go install ./cmd/cboxd
```

---

# 111. 开发阶段

## Phase 0 — Transport PoC

只写：

```text
internal/provider/colab
internal/transport
```

做到：

```bash
go run ./cmd/cbox-dev new --gpu T4

go run ./cmd/cbox-dev exec nvidia-smi

go run ./cmd/cbox-dev sync ./test /content/test

go run ./cmd/cbox-dev stop
```

先证明：

```text
Go
↓
colab CLI
↓
SSH Proxy
↓
rsync
```

整个链稳定。

---

# 112. Phase 1 — Container MVP

实现：

```text
Container model
Container service
Runtime service
ColabProvider
SSHTransport
tmux runner
```

用户可：

```bash
cbox run

cbox ps

cbox logs

cbox exec

cbox stop
```

这个阶段暂时：

```text
CLI 直接调用 Engine
```

甚至可以暂时没有 daemon。

---

# 113. Phase 2 — Daemon

加入：

```text
cboxd
Unix socket
REST API
SQLite
```

将：

```text
CLI
```

和：

```text
Engine
```

解耦。

这是架构正式成型阶段。

---

# 114. Phase 3 — Volume

实现：

```text
-v

ro
rw
output

rsync

volume manifest

cache
```

完成后才真正适合遥感训练。

---

# 115. Phase 4 — Image

实现：

```text
Cboxfile
build
images
materialize
cache
```

在这之前可以：

```text
直接 bootstrap command
```

不用阻塞前期。

---

# 116. Phase 5 — Runtime Pool

加入：

```text
idle Runtime
reuse
cache affinity
idle timeout
```

显著降低：

```text
dataset transfer
pip install
```

次数。

---

# 117. Phase 6 — Recovery

实现：

```text
health monitor
runtime lost
restart policy
resume command
checkpoint restore
```

---

# 118. Phase 7 — Context / Multi-account

加入：

```text
colab-a
colab-b
colab-pool
```

自动调度。

---

# 119. Phase 8 — Compose

最后：

```bash
cbox compose up -d
```

支持大规模：

```text
baseline
ablation
public dataset experiments
```

---

# 120. MVP 第一版明确范围

第一版必须有：

```text
Go

ColabProvider

SSHTransport

rsync

Container

cbox run
cbox ps
cbox logs
cbox exec
cbox stop
cbox rm

SQLite

cboxd

output sync
```

暂时不要：

```text
Cboxfile完整实现

Compose

multi-account scheduler

auto recovery

Web UI
```

---

# 121. 第一版验收

必须完整完成：

```text
1 cboxd 启动

2 cbox run --gpu T4 ubuntu-style command

3 自动创建 Colab Runtime

4 SSH Ready

5 项目目录同步

6 训练进程启动

7 cbox 命令退出

8 远程训练继续

9 cbox ps 显示 RUNNING

10 cbox logs -f 正常

11 cbox exec -it bash 正常

12 cbox stats 正常

13 output 周期同步到服务器

14 训练退出

15 ExitCode 正确

16 Runtime 进入 idle

17 第二个 Container 复用 Runtime

18 cbox stop 正常

19 cbox rm 正常

20 daemon restart 后状态不丢失
```

完成这 20 项：

```text
CBox V0.1
```

就已经具备实际科研使用价值。

---

# 122. V0.2 验收

增加：

```text
Image
Volume cache
start
restart
```

---

# 123. V0.3 验收

增加：

```text
runtime failure recovery

resume

multi-account
```

---

# 124. V0.4

增加：

```text
Compose
```

到这里基本形成完整 Docker-like 产品。

---

# 125. 推荐开发顺序

不要先：

```text
Cboxfile parser
Compose
漂亮 CLI
```

真正应该：

```text
1 Provider

2 SSH Transport

3 Runtime

4 Remote Process

5 Container

6 Persistent State

7 Daemon

8 Volume

9 Image

10 Recovery

11 Scheduler

12 Compose
```

---

# 126. 第一批 Go 文件

真正开始写代码时建议第一批只创建：

```text
cmd/
├── cbox/
│   └── main.go
└── cboxd/
    └── main.go

internal/
├── provider/
│   ├── provider.go
│   └── colab/
│       ├── provider.go
│       └── cli.go
│
├── transport/
│   ├── transport.go
│   └── ssh.go
│
├── runtime/
│   ├── model.go
│   └── service.go
│
├── container/
│   ├── model.go
│   ├── state.go
│   └── service.go
│
└── errors/
    └── errors.go
```

先不要把 50 个文件全部空着创建出来。

---

# 127. 第一阶段核心接口

只需要先稳定四个：

```go
Provider

Transport

RuntimeService

ContainerService
```

关系：

```text
ContainerService
       │
       ▼
RuntimeService
       │
       ▼
Provider
       │
       ▼
Transport
```

---

# 128. 最重要的 Go 架构原则

整个工程要始终遵守：

```text
cmd
 ↓
service
 ↓
domain/interface
 ↓
infrastructure
```

不要出现：

```text
Cobra command
直接执行 colab CLI

API handler
直接执行 rsync

Container model
依赖 SQLite

Scheduler
解析 shell 输出
```

这些都会让项目后期迅速失控。

---

# 129. 最终推荐架构

```text
                  cbox
                   │
                   ▼
              Unix Socket
                   │
                   ▼
                 cboxd
                   │
          ┌────────┼────────┐
          ▼        ▼        ▼
     Container   Image    Volume
          │
          ▼
      Scheduler
          │
          ▼
    RuntimeService
          │
          ▼
       Provider
          │
          ▼
     ColabProvider
          │
          ▼
   google-colab-cli
          │
          ▼
     SSHTransport
          │
          ▼
       Colab VM
```

而永久状态：

```text
              Remote Server
                   │
        ┌──────────┼──────────┐
        ▼          ▼          ▼
      SQLite     Dataset     Runs
```

Colab 则始终：

```text
Disposable
Rebuildable
Cacheable
```

---

# 130. 项目最终目标

最终应该可以做到：

```bash
cbox build -t wwtp:mmseg .

cbox volume create \
  --source /data/WWTP \
  wwtp

cbox run -d \
  --name rpgv-full \
  --gpu L4,T4 \
  --mount source=wwtp,target=/data,mode=ro \
  -v ./runs/full:/output:output \
  --restart unless-stopped \
  --resume-command \
  "python tools/train.py configs/full.py --resume" \
  wwtp:mmseg \
  python tools/train.py configs/full.py
```

之后：

```bash
cbox ps
```

看到：

```text
CONTAINER    IMAGE         GPU   STATUS      NAME
7fe39bc2     wwtp:mmseg    L4    Up 3h       rpgv-full
```

而你完全不再关心：

```text
Colab 页面
Notebook
Runtime 创建
SSH Proxy
rsync
tmux
checkpoint 回传
```

这才是 CBox 作为 Docker-like 工具真正应该达到的抽象层级。
