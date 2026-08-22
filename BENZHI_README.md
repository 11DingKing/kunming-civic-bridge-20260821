# BENZHI_README

## 项目说明

- 项目：11DingKing/kunming-civic-bridge-20260821
- 项目用途：CivicBridge 服务于昆明市人民建议专场征集。市民可通过线上入口或覆盖各县区、街道的线下征集点提交建议；市委社会工作部门维护专题活动和点位，特邀建议人开展专业评议，承办部门形成办理方案并推动状态流转，最终将可复制的成果发布并向建议人反馈。系统覆盖“征集、研判、办理、转化、反馈”全链条，而不是简单 CRUD。
- Go 工具链：`golang:1.26`
- 前端工具链：无

## 标准构建、运行和测试命令

进入容器后执行：

```bash
# 编译
cd '/app' && GOTOOLCHAIN=local go build ./...

# 启动
cd '/app' && GOTOOLCHAIN=local go run ./cmd/civicbridge
cd '/app' && GOTOOLCHAIN=local go run ./cmd/civicctl

# 测试
cd '/app' && GOTOOLCHAIN=local go test ./...
```

## Docker 构建和进入容器

```bash
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-task-26-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-task-26-arm64 linux/arm64
docker run -it benzhi-task-26-amd64:latest
docker run -it --platform linux/arm64 benzhi-task-26-arm64:latest
```

## 题目验证命令

1. 预期退出码 1：`go test ./internal/worker -run '^TestCompletedSuggestionKeepsItsHistoricalRouting$' -count=1`
