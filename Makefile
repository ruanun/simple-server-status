# Simple Server Status Makefile
# 作者: ruan

.PHONY: help lint lint-fix fmt vet test test-coverage race build-web test-web e2e \
	build-agent build-dashboard build-dashboard-only build run-agent run-dashboard dev-web \
	clean tidy check gosec release docker-build docker-run

.DEFAULT_GOAL := help

GREEN  := \033[0;32m
YELLOW := \033[0;33m
NC     := \033[0m

BIN_DIR := bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
ifeq ($(OS),Windows_NT)
EXE := .exe
endif

help: ## 显示帮助信息
	@echo "$(GREEN)Simple Server Status - 可用命令:$(NC)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(YELLOW)%-20s$(NC) %s\n", $$1, $$2}'

lint: ## Go 代码检查
	golangci-lint run --timeout=5m ./...

lint-fix: ## Go 代码检查并自动修复
	golangci-lint run --timeout=5m --fix ./...

fmt: ## 格式化 Go 代码
	gofmt -s -w .

vet: ## 运行 go vet
	go vet ./...

test: ## 运行 Go 测试
	go test ./...

test-coverage: ## 运行 Go 测试并生成覆盖率报告
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

race: ## 运行竞态检测
	go test -race ./...

build-web: ## 构建前端（输出到 internal/dashboard/web/dist）
	cd web && pnpm install --frozen-lockfile && pnpm build

test-web: ## 前端类型检查、Lint 与单元测试
	cd web && pnpm typecheck && pnpm lint && pnpm test && pnpm test:scripts

e2e: ## 前端端到端测试（会构建前端与 Dashboard）
	cd web && pnpm test:e2e

build-agent: ## 构建 Agent
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/sss-agent$(EXE) ./cmd/sss-agent

build-dashboard: build-web ## 构建 Dashboard（包含前端）
	$(MAKE) build-dashboard-only

build-dashboard-only: ## 仅构建 Dashboard（使用已有前端产物）
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/sss-dashboard$(EXE) ./cmd/sss-dashboard

build: build-agent build-dashboard ## 构建全部

run-agent: build-agent ## 运行 Agent（读取 ./sss-agent.yaml）
	./$(BIN_DIR)/sss-agent$(EXE)

run-dashboard: build-dashboard ## 运行 Dashboard
	./$(BIN_DIR)/sss-dashboard$(EXE)

dev-web: ## 启动前端开发服务器（代理到 127.0.0.1:8900）
	cd web && pnpm dev

clean: ## 清理构建产物
	rm -rf $(BIN_DIR) dist coverage.out coverage.html
	find internal/dashboard/web/dist -mindepth 1 ! -name '.gitkeep' -delete 2>/dev/null || true

tidy: ## 整理 Go 依赖
	go mod tidy

check: vet lint test test-web ## 运行全部检查

gosec: ## 安全扫描
	gosec ./...

release: ## 本地试打包（不发布）
	goreleaser release --snapshot --clean

docker-build: ## 构建 Docker 镜像 sssd:dev
	docker build -f deployments/docker/Dockerfile --build-arg VERSION=$(VERSION) -t sssd:dev .

docker-run: ## 运行 Docker 镜像（数据卷 sss-data）
	docker run --rm -p 8900:8900 -v sss-data:/app/data sssd:dev
