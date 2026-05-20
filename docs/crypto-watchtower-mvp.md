# CryptoWatchtower MVP 开发输入文档

> Git 仓库建议名：`crypto-watchtower`
>
> 项目定位：面向 Web3 / 币圈用户的实时市场异常监控机器人与微 SaaS。第一阶段不做自动交易，不承诺收益，只提供实时数据监控、风险提醒和市场情报摘要。

---

## 1. 项目名称

### 产品名

**CryptoWatchtower**

含义：像瞭望塔一样持续监控市场异动、爆仓、Funding Rate、异常成交和风险事件。

### Git 仓库名

```text
crypto-watchtower
```

### Go Module 建议名

```text
github.com/renfei198727/crypto-watchtower
```

如果后续准备商业化，也可以改为：

```text
github.com/your-org/crypto-watchtower
```

---

## 2. 一句话定位

**CryptoWatchtower 是一个基于 Go 的实时币圈异常监控平台，通过交易所 WebSocket/API 采集行情、爆仓、Funding 等数据，并通过 Telegram / Discord 向用户推送实时风险提醒和市场摘要。**

---

## 3. MVP 核心目标

第一版只做一件事：

> **实时发现市场异常，并通过 Telegram 推送给用户。**

MVP 不做自动交易、不做托管资金、不做收益承诺、不做复杂策略市场。

---

## 4. MVP 功能范围

### 4.1 必做功能

#### 1. Binance 行情采集

优先支持：

- BTCUSDT
- ETHUSDT
- SOLUSDT

数据源：

- aggTrade stream：成交数据
- liquidation / forceOrder stream：爆仓数据
- Funding Rate REST API：资金费率

#### 2. 大单异动监控

规则示例：

```text
单笔成交额 >= 100,000 USDT
或 1 分钟内累计主动买入 / 卖出 >= 500,000 USDT
```

推送示例：

```text
🚨 BTCUSDT 大额主动买入

价格: 103421.5
成交额: 1.2M USDT
时间: 2026-05-20 14:21:33
方向: Aggressive Buy

过去 1 分钟成交额: 3.6M USDT
```

#### 3. 爆仓监控

规则示例：

```text
单笔爆仓金额 >= 100,000 USDT
```

推送示例：

```text
💥 ETHUSDT 大额爆仓

方向: Long Liquidation
金额: 2.1M USDT
价格: 3821.2
时间: 2026-05-20 14:23:01
```

#### 4. Funding Rate 异常监控

规则示例：

```text
funding rate >= 0.08%
或 funding rate <= -0.08%
```

推送示例：

```text
⚠️ Funding Rate 异常

交易所: Binance
币种: ETHUSDT
当前 Funding: 0.12%

可能含义:
- 多头过热
- 短期 squeeze 风险上升
```

#### 5. Telegram Bot 推送

支持：

- 单个用户绑定 Telegram Chat ID
- 推送到 Telegram 群组 / 频道
- Markdown 消息格式
- 基础限流，防止刷屏

#### 6. 基础管理 API

第一版可以不做复杂后台，但需要提供 HTTP API：

- 健康检查
- 查看已启用 symbols
- 查看规则配置
- 手动触发测试告警
- Telegram webhook 或 polling 管理

---

### 4.2 可选功能

MVP 后半段可做：

- Discord Bot 推送
- 简单 Web Dashboard
- AI 15 分钟市场摘要
- 多交易所扩展：OKX / Bybit
- 用户自定义告警规则
- SaaS 订阅和支付

---

### 4.3 暂不做功能

第一版明确不做：

- 自动交易
- 策略回测
- 带单信号
- 跟单系统
- 钱包托管
- 收益率展示
- 高频套利执行
- 复杂 K 线分析
- 移动 App

---

## 5. 技术架构

### 5.1 架构原则

由于计划使用 Go 开发，MVP 推荐采用：

> **Go 模块化单体 + Redis + PostgreSQL + Telegram Bot**

不要一开始就拆微服务。先保证快速上线、稳定采集、稳定推送。

---

## 6. MVP 架构图

```text
                ┌────────────────────┐
                │   Binance API / WS  │
                │                    │
                │ - aggTrade          │
                │ - liquidation       │
                │ - funding REST      │
                └─────────┬──────────┘
                          │
                          │ WebSocket / REST
                          ▼
              ┌────────────────────────┐
              │   Go Collector Module   │
              │                        │
              │ - reconnect             │
              │ - heartbeat             │
              │ - normalize event       │
              └─────────┬──────────────┘
                        │
                        ▼
              ┌────────────────────────┐
              │      Event Bus          │
              │                        │
              │ MVP: in-process channel │
              │ Later: Redis Stream     │
              └─────────┬──────────────┘
                        │
                        ▼
              ┌────────────────────────┐
              │    Rule Engine Module   │
              │                        │
              │ - large trade rule      │
              │ - liquidation rule      │
              │ - funding rule          │
              │ - rate limit / dedupe   │
              └─────────┬──────────────┘
                        │
                        ▼
              ┌────────────────────────┐
              │ Notification Module     │
              │                        │
              │ - Telegram Bot          │
              │ - Discord later         │
              │ - webhook later         │
              └─────────┬──────────────┘
                        │
                        ▼
              ┌────────────────────────┐
              │ Telegram User / Channel │
              └────────────────────────┘

              ┌────────────────────────┐
              │ PostgreSQL              │
              │                        │
              │ - users                 │
              │ - alert_rules           │
              │ - market_events         │
              │ - notification_logs     │
              └────────────────────────┘

              ┌────────────────────────┐
              │ Redis                   │
              │                        │
              │ - cache                 │
              │ - rate limit            │
              │ - stream later          │
              └────────────────────────┘
```

---

## 7. Go 项目目录结构

```text
crypto-watchtower/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── config/
│   │   └── config.go
│   ├── collector/
│   │   ├── binance_ws.go
│   │   ├── binance_rest.go
│   │   └── normalizer.go
│   ├── eventbus/
│   │   ├── bus.go
│   │   └── redis_stream.go
│   ├── rule/
│   │   ├── engine.go
│   │   ├── large_trade.go
│   │   ├── liquidation.go
│   │   └── funding.go
│   ├── notifier/
│   │   ├── telegram.go
│   │   ├── discord.go
│   │   └── formatter.go
│   ├── storage/
│   │   ├── postgres.go
│   │   ├── market_event_repo.go
│   │   └── alert_rule_repo.go
│   ├── api/
│   │   ├── router.go
│   │   ├── health.go
│   │   ├── rules.go
│   │   └── telegram.go
│   ├── model/
│   │   ├── market_event.go
│   │   ├── alert.go
│   │   └── user.go
│   └── scheduler/
│       └── funding_job.go
├── migrations/
│   └── 001_init.sql
├── deployments/
│   ├── docker-compose.yml
│   └── Dockerfile
├── configs/
│   └── config.example.yaml
├── scripts/
│   └── run-local.sh
├── README.md
└── go.mod
```

---

## 8. 核心模块说明

### 8.1 Collector 模块

职责：

- 连接 Binance WebSocket
- 自动重连
- 心跳检测
- 数据标准化
- 将原始事件转换为内部统一 MarketEvent

内部事件结构示例：

```go
type MarketEvent struct {
    ID          string
    Exchange    string
    Symbol      string
    EventType   string
    Side        string
    Price       float64
    Quantity    float64
    Notional    float64
    RawPayload  []byte
    EventTime   time.Time
    CreatedAt   time.Time
}
```

---

### 8.2 Rule Engine 模块

职责：

- 消费 MarketEvent
- 根据规则判断是否触发告警
- 去重
- 限流
- 生成 Alert

告警结构示例：

```go
type Alert struct {
    ID        string
    Symbol    string
    Type      string
    Severity  string
    Title     string
    Message   string
    EventID   string
    CreatedAt time.Time
}
```

---

### 8.3 Notification 模块

职责：

- 格式化告警
- 推送 Telegram
- 记录通知日志
- 失败重试
- 后续扩展 Discord / Webhook

---

### 8.4 API 模块

MVP HTTP API：

```text
GET  /health
GET  /api/v1/symbols
GET  /api/v1/rules
POST /api/v1/rules
POST /api/v1/telegram/test
POST /api/v1/alerts/test
```

---

## 9. 数据库设计

### 9.1 users

```sql
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(255),
    telegram_chat_id VARCHAR(128),
    plan VARCHAR(32) DEFAULT 'free',
    status VARCHAR(32) DEFAULT 'active',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

### 9.2 alert_rules

```sql
CREATE TABLE alert_rules (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id),
    symbol VARCHAR(32) NOT NULL,
    rule_type VARCHAR(64) NOT NULL,
    threshold NUMERIC(24, 8) NOT NULL,
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

### 9.3 market_events

```sql
CREATE TABLE market_events (
    id BIGSERIAL PRIMARY KEY,
    exchange VARCHAR(32) NOT NULL,
    symbol VARCHAR(32) NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    side VARCHAR(16),
    price NUMERIC(24, 8),
    quantity NUMERIC(24, 8),
    notional NUMERIC(24, 8),
    raw_payload JSONB,
    event_time TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);
```

### 9.4 notification_logs

```sql
CREATE TABLE notification_logs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id),
    alert_type VARCHAR(64),
    channel VARCHAR(32),
    target VARCHAR(255),
    status VARCHAR(32),
    error_message TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);
```

---

## 10. Telegram 对接方案

### 10.1 Bot 创建

1. 在 Telegram 搜索 `@BotFather`
2. 执行 `/newbot`
3. 设置 bot 名称，例如：`CryptoWatchtower Bot`
4. 设置 username，例如：`crypto_watchtower_bot`
5. 获取 Bot Token

配置：

```yaml
telegram:
  bot_token: "YOUR_TELEGRAM_BOT_TOKEN"
  default_chat_id: "YOUR_CHAT_ID_OR_CHANNEL_ID"
  parse_mode: "Markdown"
```

---

### 10.2 获取 Chat ID

个人用户：

1. 用户向 bot 发送 `/start`
2. 服务端通过 Telegram update 获取 `chat.id`
3. 存入 `users.telegram_chat_id`

群组或频道：

1. 将 bot 添加到群组或频道
2. 给 bot 管理员权限
3. 群里发送测试消息
4. 通过 update 获取 chat id

---

### 10.3 Telegram 命令设计

```text
/start     绑定用户并显示欢迎信息
/help      查看帮助
/status    查看服务状态
/rules     查看当前告警规则
/test      发送测试告警
/upgrade   查看 Pro 版本说明
```

---

### 10.4 Telegram 消息格式规范

大单提醒：

```text
🚨 {{symbol}} 大额{{side}}

价格: {{price}}
成交额: {{notional}} USDT
交易所: {{exchange}}
时间: {{event_time}}

规则: 单笔成交额超过 {{threshold}} USDT
```

爆仓提醒：

```text
💥 {{symbol}} 大额爆仓

方向: {{side}}
金额: {{notional}} USDT
价格: {{price}}
时间: {{event_time}}
```

Funding 提醒：

```text
⚠️ {{symbol}} Funding 异常

当前 Funding: {{funding_rate}}
阈值: {{threshold}}
交易所: {{exchange}}
```

---

### 10.5 防刷屏策略

必须实现基础限流：

```text
同一个 symbol + rule_type：60 秒内最多推送 1 次
同一个 chat_id：每分钟最多 20 条
全局：每秒最多 5 条 Telegram 请求
```

建议使用 Redis：

```text
rate_limit:alert:{symbol}:{rule_type}
rate_limit:telegram:{chat_id}
```

---

## 11. Discord 对接方案

MVP 可以先不做 Discord，但架构上预留接口。

推荐方式：

- 第一阶段：Discord Webhook
- 第二阶段：Discord Bot

Webhook 配置：

```yaml
discord:
  enabled: false
  webhook_url: ""
```

Notifier 接口建议：

```go
type Notifier interface {
    Send(ctx context.Context, alert Alert) error
}
```

后续实现：

```text
TelegramNotifier
DiscordNotifier
WebhookNotifier
```

---

## 12. AI 摘要方案

MVP 可选。

建议第一版先做规则提醒，后续添加 AI Summary。

### AI Summary 逻辑

每 15 分钟聚合：

- 大单事件
- 爆仓事件
- Funding 异常
- 价格变化

输出：

```text
🧠 过去 15 分钟市场摘要

- BTC 出现连续主动买入
- ETH 多头爆仓金额明显增加
- SOL Funding Rate 快速升高

风险提示：短线波动可能放大，请注意仓位风险。
```

注意：

- 不输出买卖建议
- 不预测收益
- 不使用“稳赚”“高胜率”等词

---

## 13. SaaS 收费模型

### 13.1 Free

价格：免费

限制：

```text
- 延迟 5 分钟提醒
- 仅 BTC / ETH
- 每日最多 20 条提醒
- 仅 Telegram 免费群
```

目的：

- 获客
- 建立信任
- 给用户体验实时监控价值

---

### 13.2 Pro

价格建议：`$19 / 月`

权益：

```text
- 实时提醒
- 支持 BTC / ETH / SOL / 热门合约
- 自定义阈值
- Telegram 私聊提醒
- Discord Webhook
- Funding 异常提醒
```

---

### 13.3 VIP

价格建议：`$79 / 月`

权益：

```text
- 全币种监控
- 高频提醒
- AI 15 分钟市场摘要
- Webhook API
- 更多规则模板
- 更高推送频率
```

---

### 13.4 Team / Enterprise

后期提供：

```text
- 专属 API
- 白标 Telegram Bot
- 私有部署
- 自定义数据源
- SLA
```

---

## 14. 获客增长方案

### 14.1 增长核心

不要先做复杂官网。

第一阶段核心资产：

> **一个持续推送高质量市场异动的 Telegram 免费频道。**

---

### 14.2 Telegram 增长方案

#### 免费频道定位

频道名建议：

```text
CryptoWatchtower Free Alerts
```

频道内容：

- BTC / ETH 大单提醒
- 大额爆仓提醒
- Funding 异常提醒
- 每日 1~2 次市场摘要

免费频道限制：

- 延迟 3~5 分钟
- 仅主流币
- 不支持自定义规则

转化文案：

```text
升级 Pro 可解锁：
- 实时提醒
- 更多币种
- 私聊推送
- 自定义阈值
- Discord / Webhook
```

---

### 14.3 Discord 增长方案

Discord 更适合海外用户和长期社区。

频道设计：

```text
#announcements
#btc-alerts
#eth-alerts
#liquidation-alerts
#funding-alerts
#ai-summary
#support
```

增长方式：

- 在 Telegram 免费频道引导加入 Discord
- 在 Reddit / X 分享市场异动截图
- 提供 Discord 免费 webhook 示例

---

### 14.4 X / Twitter 内容策略

每天发 3 类内容：

1. 实时异常事件截图
2. Funding / 爆仓科普
3. 项目构建日志

示例：

```text
BTC just printed the largest aggressive buy flow in the last 6 hours.

Not a trading signal. Just real-time market telemetry.

Tracked by CryptoWatchtower.
```

---

### 14.5 Reddit / 社区获客

目标社区：

```text
r/algotrading
r/CryptoCurrency
r/ethfinance
r/binance
```

内容角度：

- “I built a realtime crypto anomaly alert bot in Go”
- “How I monitor liquidation spikes using Binance WebSocket”
- “Open-sourcing part of my crypto alert infrastructure”

避免：

- 直接广告
- 喊单
- 收益截图

---

### 14.6 GitHub 获客

可以开源部分模块：

```text
- Binance WebSocket collector
- Telegram alert formatter
- rule engine demo
```

保留商业部分：

```text
- SaaS 用户系统
- 付费规则
- 多交易所适配
- AI summary
```

README 里展示：

- 架构图
- Demo 截图
- Telegram 免费频道链接
- Pro 版等待名单

---

## 15. 30 天开发计划

### Week 1：核心数据链路跑通

目标：完成 Binance -> Go -> Rule -> Telegram。

#### Day 1

- 初始化 Go 项目
- 设计目录结构
- 配置文件加载
- Docker Compose 初始化 PostgreSQL / Redis

#### Day 2

- Binance aggTrade WebSocket 接入
- 实现自动重连
- 标准化 MarketEvent

#### Day 3

- 实现 in-process EventBus
- market_events 入库
- 健康检查 API

#### Day 4

- 实现大单规则
- 实现基础 Alert 结构
- 实现规则配置读取

#### Day 5

- Telegram Bot 接入
- 实现测试推送
- 实现大单提醒推送

#### Day 6

- Binance liquidation stream 接入
- 爆仓规则实现
- 爆仓推送模板

#### Day 7

- 第一轮部署到 VPS
- 完成日志、错误处理、重启策略

验收：

```text
BTC / ETH 出现大单或爆仓时，Telegram 能收到提醒。
```

---

### Week 2：规则体系和稳定性

目标：让系统可以连续稳定运行。

#### Day 8

- Funding Rate REST job
- Funding 异常规则

#### Day 9

- Redis 限流
- Telegram 防刷屏

#### Day 10

- alert_rules 表
- 支持从数据库读取规则

#### Day 11

- `/rules` API
- `/alerts/test` API

#### Day 12

- Telegram `/start` `/status` `/test` 命令

#### Day 13

- notification_logs
- 推送失败记录
- 简单重试

#### Day 14

- 稳定性测试
- 断网重连测试
- Binance WS 断连恢复测试

验收：

```text
系统连续运行 24 小时不崩溃，断线后可自动恢复。
```

---

### Week 3：产品化和增长基础

目标：具备对外展示能力。

#### Day 15

- README 完善
- 项目介绍图
- Telegram 免费频道创建

#### Day 16

- 支持多 symbol 配置
- 添加 SOLUSDT

#### Day 17

- 免费频道推送模式
- 私聊推送模式

#### Day 18

- 简单 Landing Page
- 收集等待名单邮箱

#### Day 19

- Discord Webhook MVP

#### Day 20

- 基础 Dashboard 页面，可选
- 展示最近告警列表

#### Day 21

- 准备公开 Demo
- 录制短视频 / GIF

验收：

```text
陌生用户可以通过 README / Landing Page 理解项目并加入 Telegram 频道。
```

---

### Week 4：冷启动和商业化验证

目标：获取前 100 个真实用户。

#### Day 22

- X / Twitter 开始发布构建日志
- 发布 3 条市场异动内容

#### Day 23

- Reddit 发技术帖
- GitHub 开源 collector demo

#### Day 24

- Telegram 社群互推
- 加入 Web3 开发者社区

#### Day 25

- 增加 Pro 权益说明
- 增加 `/upgrade` 命令

#### Day 26

- 收集用户反馈
- 优化推送格式

#### Day 27

- 添加 AI Summary 原型，可选

#### Day 28

- 制作 Pro 订阅等待名单

#### Day 29

- 找 3~5 个种子用户试用
- 访谈他们最需要的提醒类型

#### Day 30

- 总结指标
- 决定下一阶段：继续 SaaS、接单、还是开源增长

验收：

```text
Telegram 免费频道达到 100 个真实用户，至少 5 个用户表达付费意愿。
```

---

## 16. MVP 验收标准

技术验收：

```text
- Go 服务可启动
- 可连接 Binance WebSocket
- 可自动重连
- 可采集 BTC / ETH / SOL 事件
- 可识别大单
- 可识别爆仓
- 可检测 Funding 异常
- 可推送 Telegram
- 可限流
- 可记录事件和通知日志
- 可 Docker Compose 本地运行
```

业务验收：

```text
- 有 Telegram 免费频道
- 有至少 3 种告警模板
- 有清晰的 Free / Pro / VIP 权益区分
- 有 README 和 Landing Page
- 有增长内容发布计划
```

---

## 17. 风险和边界

必须避免：

```text
- 不承诺收益
- 不提供投资建议
- 不做自动交易
- 不托管用户资金
- 不接触用户私钥
- 不使用“稳赚”“高胜率”“内幕”等营销词
```

推荐声明：

```text
CryptoWatchtower only provides real-time market telemetry and alerting.
It is not financial advice. Users are responsible for their own decisions.
```

中文：

```text
CryptoWatchtower 仅提供实时市场数据监控和风险提醒，不构成任何投资建议。用户需自行承担交易决策风险。
```

---

## 18. 下一阶段路线图

### V0.2

- OKX / Bybit 支持
- Discord Bot
- AI Summary
- 多用户规则配置

### V0.3

- Web Dashboard
- Stripe / LemonSqueezy 订阅
- Webhook API
- Pro 权限控制

### V1.0

- SaaS 正式版
- 多交易所聚合
- 团队版
- API 商业化
- 白标 Telegram Bot

---

## 19. 推荐优先级

开发优先级：

```text
1. Binance WebSocket Collector
2. MarketEvent 标准化
3. 大单规则
4. Telegram 推送
5. 爆仓规则
6. Funding 规则
7. 限流和日志
8. 免费频道
9. Landing Page
10. Pro 等待名单
```

第一目标不是功能完整，而是：

> **尽快让真实用户看到实时提醒。**

