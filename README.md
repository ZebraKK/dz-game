# dz-game

德州扑克 / 牛仔百家乐游戏服务 — Go 语言重写版。

原工程为 C++ 动态插件（`game.so`），由上层大厅服务通过 `dlopen` 加载。本工程将其重写为独立 Go 进程，通过 HTTP JSON API 接受上层调用，内部保持 Actor 模型（每张桌一个 goroutine）。

---

## 目录

- [架构概述](#架构概述)
- [目录结构](#目录结构)
- [快速开始](#快速开始)
- [HTTP API](#http-api)
- [配置说明](#配置说明)
- [测试](#测试)
- [可观测性](#可观测性)
- [设计决策](#设计决策)
- [C++ 对应关系](#c-对应关系)

---

## 架构概述

```
上层服务（大厅 / 匹配 / 计费）
        │  HTTP JSON  POST /v1/...
        ▼
  ┌─────────────────────────────────┐
  │       gamesvr（本工程）          │
  │  ┌──────────┐  ┌─────────────┐ │
  │  │ Registry │  │   Handler   │ │
  │  └────┬─────┘  └──────┬──────┘ │
  │       │               │Msg     │
  │  ┌────▼───────────────▼──────┐ │
  │  │  Table Actor (goroutine)  │ │  ← 每桌一个，无锁
  │  │  ┌──────────┐ ┌────────┐  │ │
  │  │  │texas.Game│ │ Robot  │  │ │
  │  │  └──────────┘ └────────┘  │ │
  │  └───────────────────────────┘ │
  └─────────────────────────────────┘
        │  TableHost 接口
        ▼
  内存 Stub（开发/测试）
  或 HTTP 适配器（生产，待实现）
```

**核心原则**：

- **Actor 模型**：每张桌一个 goroutine，所有状态变更单线程，无需 mutex。
- **事件驱动**：游戏逻辑（`texas.Game`、`cowboy.Game`）不持有任何 I/O 引用，方法调用返回 `[]Event`，Table Actor 统一消费。
- **零额外依赖**：服务层只用标准库（`net/http` + `encoding/json`），配置用 `gopkg.in/yaml.v3`。

---

## 目录结构

```
dz-game/
├── api/
│   └── types.go              # HTTP JSON 请求/响应结构体（23 个端点）
├── cmd/
│   └── gamesvr/
│       └── main.go           # 二进制入口：加载配置、启动桌子、HTTP 服务
├── config/
│   └── default.yaml          # 默认配置（经典场 + SNG 盲注表 + 牛仔赔率）
└── internal/
    ├── cards/                # 牌引擎（纯函数）
    │   ├── card.go           #   Card 结构体、花色/点数枚举
    │   ├── deck.go           #   52 张牌组、Fisher-Yates 洗牌
    │   └── handeval.go       #   最优 5 张牌评估、手牌比较
    ├── config/               # 配置层
    │   ├── types.go          #   SceneConfig 等所有配置结构体
    │   └── loader.go         #   YAML 加载、atomic 热重载、SIGHUP 监听
    ├── player/
    │   └── player.go         # 玩家数据模型、所有枚举（与 C++ 数值完全一致）
    ├── texas/                # 德州扑克状态机（纯逻辑，无 I/O）
    │   ├── state.go          #   GameStatus / TimerID / EventKind / Event 类型
    │   ├── game.go           #   手牌生命周期：StartHand → PlayerAction → settle
    │   ├── pot.go            #   主池 + 边池计算（computePotsClassic）
    │   └── robot.go          #   规则型机器人 AI
    ├── cowboy/               # 牛仔百家乐状态机
    │   ├── state.go          #   GameStatus / BetPosition 枚举
    │   ├── game.go           #   Countdown → BetBegan → Result 循环
    │   └── odds.go           #   EvaluateResult / PayoutForBet
    ├── table/                # 桌子 Actor
    │   ├── host.go           #   TableHost 接口（对应 C++ ISoTableManager）
    │   ├── messages.go       #   MsgKind / Msg 值类型 / cmd 常量
    │   ├── table.go          #   Run() 主循环、事件消费、机器人调度
    │   ├── bill.go           #   财务结算：服务费计算 + BillData JSON 序列化
    │   └── timer.go          #   有序切片轻量定时器（无 goroutine-per-timer）
    ├── server/               # HTTP JSON 服务层
    │   ├── registry.go       #   TableRegistry（RWMutex 保护）
    │   └── handler.go        #   23 个 HTTP 端点 → Msg → Table.inbox
    ├── metrics/
    │   └── metrics.go        #   expvar 计数器（活跃桌、手数、行动、结算）
    └── stub/
        └── table_stub.go     #   TableHost 内存桩（测试用）
```

---

## 快速开始

**前置条件**：Go 1.21+

```bash
# 克隆并进入工程目录
cd /Users/xwill/dz-game

# 编译
go build ./cmd/gamesvr

# 运行（默认监听 :8080，加载 config/default.yaml）
./gamesvr

# 自定义参数
./gamesvr -addr :9090 -config /path/to/config.yaml
```

**冒烟测试**（服务启动后）：

```bash
# 玩家入桌
curl -s -X POST http://localhost:8080/v1/player/enter \
  -H 'Content-Type: application/json' \
  -d '{"table_id":1,"player":{"user_id":1001,"nick_name":"Alice","game_money":5000,"chair":255}}'
# → {"code":0,"status":1}

# 健康检查
curl -s http://localhost:8080/healthz
# → 200 OK

# 查看指标
curl -s http://localhost:8080/debug/vars | python3 -m json.tool | grep gamesvr
```

---

## HTTP API

所有端点均为 `POST`，`Content-Type: application/json`。
响应 `{"code":0}` 表示成功，非零 `code` 附带 `message` 错误描述。

### 玩家生命周期

| 端点 | 对应 C++ | 说明 |
|---|---|---|
| `POST /v1/player/enter` | `OnPlayerEnter` | 玩家入桌 |
| `POST /v1/player/exit` | `OnPlayerExit` | 玩家离桌 |
| `POST /v1/player/offline` | `OnPlayerOffline` | 玩家断线 |
| `POST /v1/player/reconnect` | `OnGameUserReplay` | 玩家重连 |
| `POST /v1/player/ready` | `OnGameUserReady` | 玩家准备 |
| `POST /v1/player/sit` | `OnGameUserSit` | 玩家坐下（指定座位） |
| `POST /v1/player/stand` | `OnGameUserStand` | 玩家站起 |
| `POST /v1/player/discard` | `OnPlayerDiscard` | 玩家弃权 |

### 游戏消息

| 端点 | 对应 C++ | 说明 |
|---|---|---|
| `POST /v1/game/message` | `OnGameMsg` | 路由客户端协议消息（fold/call/raise…） |
| `POST /v1/game/launch` | `LaunchSOGame` | 桌子启动钩子 |
| `POST /v1/game/close` | `CloseSOGame` | 桌子关闭钩子 |

### 桌子查询

| 端点 | 返回 | 说明 |
|---|---|---|
| `POST /v1/table/can-enter` | `CanEnterResp` | 准入检查 |
| `POST /v1/table/can-sit` | `CanSitDownResp` | 坐下检查 |
| `POST /v1/table/can-change` | `CanChangeTableResp` | 换桌检查 |
| `POST /v1/table/ranked-players` | `[]user_id` | 筹码排名（SNG 用，同步查询） |
| `POST /v1/table/capacity` | `capacity` | 桌子容量（同步查询） |

### 桌子控制

| 端点 | 对应 C++ | 说明 |
|---|---|---|
| `POST /v1/table/force-end` | `OnGameSetGameEnd` | 强制结束当前手 |
| `POST /v1/table/reload-config` | `ReloadCfg` | 热重载配置 |
| `POST /v1/table/blind-level` | `OnGameBlindLevel` | 更新 SNG 盲注级别 |
| `POST /v1/table/friend-cfg` | `SetFriendTableCfg` | 好友桌配置 |

### 赛事 / 结算

| 端点 | 对应 C++ | 说明 |
|---|---|---|
| `POST /v1/match/wait-buy` | `OnMatchWaitBuy` | 重购窗口通知 |
| `POST /v1/match/buy-finish` | `OnMatchBuyFinish` | 重购完成 |
| `POST /v1/jackpot/lucky-dogs` | `LuckyDogsBonusNotify` | Jackpot 奖励通知 |

---

## 配置说明

配置文件为 YAML，运行时通过 `SIGHUP` 触发热重载（对应 C++ `ReloadCfg`）：

```bash
kill -HUP <pid>
```

主要配置块（`config/default.yaml`）：

```yaml
scene:
  seatCount: 9
  serviceCharge: 5      # 抽水百分比（5 = 5%）
  smallBlind: 50
  bigBlind: 100
  actionTimeSec: 30     # 玩家行动超时（秒）
  minBuyIn: 2000
  maxBuyIn: 20000

blinds:                 # SNG 盲注递增表
  - level: 1
    smallBlind: 25
    bigBlind: 50
    durationSec: 600
  # ...

cowboy:                 # 牛仔百家乐配置
  betTimeSec: 15
  tableBetLimit: 1000000
```

结构体定义见 `internal/config/types.go`，字段与 C++ `_tagSceneConfig` 逐一对应。

---

## 测试

```bash
# 全量测试（含竞态检测）
go test -race ./...

# 100 桌机器人负载测试（2 秒并发）
go test -race -run TestRobotTableLoad ./internal/table/

# 性能基准
go test -bench=BenchmarkRobotTable -benchtime=3s ./internal/table/

# 单包测试
go test -v ./internal/texas/    # 德州：边池计算 + 机器人 AI
go test -v ./internal/cowboy/   # 牛仔：赔率结算 + 游戏流程
go test -v ./internal/server/   # HTTP 端点：14 个场景
```

测试覆盖：

| 包 | 测试数 | 覆盖点 |
|---|---|---|
| `internal/cards` | 23 | 所有手牌类型、7张最优选择、手牌比较 |
| `internal/texas` | 8+4 | 边池分配（8 场景含资金守恒）、机器人决策 |
| `internal/cowboy` | 10 | 赔率计算、边注、完整游戏流程 |
| `internal/table` | 4+1 | 集成一局（入座→发牌→结算）、100 桌负载 |
| `internal/server` | 14 | HTTP 200/404/400、同步查询、消息投递 |

---

## 可观测性

服务启动后以下端点与游戏 API 共用同一端口：

| 端点 | 说明 |
|---|---|
| `GET /healthz` | 负载均衡健康探测，始终返回 200 |
| `GET /debug/vars` | expvar JSON 指标快照 |
| `GET /debug/pprof/` | Go 运行时 profiling（CPU、heap、goroutine） |

**expvar 指标**：

```
gamesvr_active_tables       当前运行中的桌子 goroutine 数
gamesvr_hands_total         已完成手牌局数（含弃牌结束）
gamesvr_actions_total       已处理玩家行动总数
gamesvr_robot_actions_total 机器人自动行动数
gamesvr_bills_total         SetGameBill 调用次数
gamesvr_http_requests_total HTTP 请求总数
```

**日志**（`log/slog`，结构化 JSON 可对接任何日志收集器）：

```
INFO  player entered   tableID=1 userID=1001 seat=0 chips=5000
INFO  hand started     tableID=1
INFO  hand ended       tableID=1 hands_total=42
DEBUG bill settled     tableID=1 userID=1001 win=380 fee=20
```

---

## 设计决策

### 为什么不用 gRPC？

当前环境未安装 `protoc`，改用标准库 `net/http` + `encoding/json` 实现相同接口，零额外依赖。接口形状与原 `api/gamesvr.proto` 定义完全一致，后续可平滑切换。

### 为什么每桌一个 goroutine？

与 C++ 的单线程 `game.so` 插件模型语义一致：桌内所有状态变更严格串行，彻底避免加锁。外部调用只通过 `Msg` 值类型经 channel 投递，inbox 满时立即返回 503（背压信号）。

### 服务费计算

```
fee     = won × ServiceCharge / 100
actualWon = won - fee
```

费率从 `SceneConfig.ServiceCharge`（整数百分比）读取，在 `texas.Game.settle()` 中计算，通过 `EventSettleBill.ServiceFee` 字段携带到 Table Actor，最终序列化进 `BillResult.BillData` JSON 上报给计费服务。

### 同步查询（reply-channel 模式）

需要返回值的查询（`GetRankedPlayers`、`GetCapacity`）通过 `Msg.Reply chan any` 实现：HTTP handler 投入带 reply channel 的 Msg，桌子 goroutine 处理后写入 channel，handler 阻塞等待（2 s 超时）。全程无锁。

---

## C++ 对应关系

| Go 包 / 文件 | C++ 参考 |
|---|---|
| `internal/cards/` | `Source/CardAssistant/` |
| `internal/texas/game.go` | `Source/SoGameLogic/SoGameLogic.cpp` |
| `internal/texas/pot.go` | `CSoGameLogic::GetPoolsInfo` |
| `internal/texas/state.go` | `Source/SoGameLogic/SoGameCommon.h` |
| `internal/cowboy/` | `Source/SoCowboyGameLogic/` |
| `internal/table/host.go` | `Source/Manager/SoITableManager.h` |
| `internal/config/types.go` | `Source/SoGameLogic/SoConfig.h` |
| `internal/player/player.go` | `Source/SoGameLogic/SoUser.h` |
| `api/types.go` | `Deps/Include/IGame.h`（IGame 方法签名） |

所有枚举数值（`GameStatus`、`player.Action`、`cards.HandType`、`BetPosition` 等）与 C++ 完全一致，保证客户端协议兼容。
