# CivicBridge - 昆明人民建议全链条协同服务

CivicBridge 服务于昆明市人民建议专场征集。市民可通过线上入口或覆盖各县区、街道的线下征集点提交建议；市委社会工作部门维护专题活动和点位，特邀建议人开展专业评议，承办部门形成办理方案并推动状态流转，最终将可复制的成果发布并向建议人反馈。系统覆盖“征集、研判、办理、转化、反馈”全链条，而不是简单 CRUD。

- Go 1.26，纯后端 HTTP 服务
- 默认端口 `49660`
- SQLite 关系索引与按日期分片的 JSONL 原始记录
- 服务端可撤销会话，支持 `citizen`、`reviewer`、`handler`、`admin` 角色

## 结构

```text
cmd/civicbridge/  HTTP 服务、信号处理与优雅关闭
cmd/civicctl/     初始化、导入导出、核对、索引重建和诊断工具
internal/auth/    bcrypt 账号与服务端会话生命周期
internal/domain/  建议、活动、点位、评议、办理、转化和反馈状态机
internal/service/ 跨实体业务编排、事务、幂等和错误链
internal/store/   持久化接口与事务边界
internal/repo/    JSONL 分片与 SQLite 索引的复合存储
internal/index/   SQL migration、约束、查询和乐观更新
internal/httpapi/ 路由、统一错误、请求 ID、鉴权和角色控制
internal/worker/  规则重评估、取消、重试和停止
internal/scheduler/ 超期升级、核对巡检、退避和永久失败
migrations/      可从空库顺序执行的版本化迁移
```

依赖方向保持为 `httpapi -> service -> store -> repo -> index/shard`。领域层不依赖 HTTP 或具体数据库，HTTP handler 不直接拼接 SQL。

## 数据与一致性

主要关系表包括：

- `items`：人民建议主记录，`external_ref` 唯一。
- `campaigns`：专题征集活动及其公开窗口和生命周期。
- `collection_points`：活动线下点位，带辖区、主题、状态和每日容量。
- `invited_advisors`：特邀建议人及专业领域、覆盖区域和待评配额。
- `suggestion_intakes`：建议与活动、点位的幂等绑定。
- `professional_reviews`：专业评议分配、结论和证据。
- `handling_plans`：承办部门的办理承诺与状态流转。
- `conversion_outcomes`：办理成果转化、复核和发布。
- `feedback_receipts`：向建议人反馈的租约、重试和永久失败状态。
- `outbox_events`：与反馈记录同事务创建的可靠投递事件。
- `rules`、`assignments`、`escalations`、`audit_entries`、`permanent_failures`、`import_batches`：分办、升级、审计与运行恢复支撑。

迁移包含外键、唯一约束、业务索引、状态约束和并发容量触发器。建议登记使用幂等键；点位容量与建议人待评配额由数据库约束兜底；状态更新采用版本号避免静默覆盖。反馈记录和 outbox 事件在同一事务中落库，失败时整体回滚。服务重启后从 SQLite 和 JSONL 恢复状态，损坏分片会在核对与重建报告中显式列出。

## 配置与启动

配置可从 YAML 读取，并由 `CIVICBRIDGE_` 前缀的环境变量覆盖。完整示例见 `config.example.yaml` 和 `.env.example`。

首次启动需要注入至少一个账号，口令仅用于生成 bcrypt 哈希，不应写入仓库：

```bash
go run ./cmd/civicctl init -data-dir ./data

CIVICBRIDGE_AUTH_BOOTSTRAP_USERS='[{"id":"u-admin","username":"admin","password":"<strong-password>","role":"admin"}]' \
  go run ./cmd/civicbridge config.example.yaml
```

存活检查为 `GET /healthz`。就绪检查为 `GET /readyz`，它会验证数据库、数据目录、调度器和重评估 worker。

## 核心业务路径

活动与征集点：

```text
POST /api/campaigns
POST /api/campaigns/{id}/collection-points
POST /api/collection-points/{id}/activate
POST /api/campaigns/{id}/transition
GET  /api/campaigns/{id}/collection-points
```

建议征集与办理：

```text
POST /api/suggestions
POST /api/suggestions/{id}/intake
POST /api/suggestions/{id}/reviews
POST /api/reviews/{id}/decision
POST /api/suggestions/{id}/handling-plans
POST /api/handling-plans/{id}/transition
POST /api/suggestions/{id}/conversion
POST /api/suggestions/{id}/conversion/verify
POST /api/suggestions/{id}/feedback
```

查询与运行维护：

```text
GET  /api/suggestions
GET  /api/suggestions/{id}
GET  /api/stats/backlog
GET  /api/audit
GET  /api/failures
POST /api/failures/{id}/retry
POST /api/batches/import
GET  /api/batches/export
```

除健康检查和登录外，默认要求 Bearer 会话。管理员维护活动与点位，特邀建议人执行评议，承办人员推进办理方案，市民可提交并跟踪建议。退出会撤销服务端会话，过期令牌会返回稳定的鉴权错误码。

## 示例流程

登录后使用返回的 token：

```bash
curl -s -X POST http://localhost:49660/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"<strong-password>"}'
```

管理员创建七夕专题活动：

```bash
curl -s -X POST http://localhost:49660/api/campaigns \
  -H 'Authorization: Bearer <token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "code":"KM-2026-QIXI",
    "title":"昆明市人民建议专场征集",
    "theme":"承滇风精神 抒云岭民意",
    "opens_at":"2026-08-19T00:00:00+08:00",
    "closes_at":"2026-09-02T00:00:00+08:00",
    "actor":"市委社会工作部"
  }'
```

市民登记建议后，通过 intake 接口绑定专题活动、线下点位和幂等键。重复请求返回同一绑定记录，不会占用第二份点位容量。

## 构建与验证

```bash
go mod tidy
go fmt ./...
go vet ./...
go build ./...
go test -timeout=300s -count=1 ./...
go test -race -timeout=420s -count=1 ./...
docker build -t civicbridge:local .
```

测试覆盖状态机非法转换、跨实体前置条件、事务回滚、幂等、并发配额、乐观冲突、会话撤销与过期、分页过滤、重启恢复、worker 重试取消和 HTTP 错误契约。测试使用临时真实 SQLite 数据库和可注入时钟，不依赖在线服务或随机等待。

## 运维工具

```bash
go run ./cmd/civicctl init -data-dir ./data
go run ./cmd/civicctl import -file suggestions.json
go run ./cmd/civicctl export -out suggestions.jsonl
go run ./cmd/civicctl reconcile
go run ./cmd/civicctl rebuild-index
go run ./cmd/civicctl diagnose
```

`reconcile` 比较 JSONL 分片与 SQLite 索引，`rebuild-index` 从可读取分片恢复索引，`diagnose` 汇总存储可写性、建议与规则数量、超期项、schema 版本和损坏分片。
