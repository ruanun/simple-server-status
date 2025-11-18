# Simple Server Status Makefile
# 作者: ruan

.PHONY: help lint test build clean race coverage fmt vet install-tools

# 默认目标
.DEFAULT_GOAL := help

# 颜色定义
GREEN  := \033[0;32m
YELLOW := \033[0;33m
RED    := \033[0;31m
NC     := \033[0m

# 变量定义
BINARY_AGENT := sss-agent
BINARY_DASHBOARD := sss-dashboard
BIN_DIR := bin
DIST_DIR := dist
COVERAGE_FILE := coverage.out

help: ## 显示帮助信息
	@echo "$(GREEN)Simple Server Status - 可用命令:$(NC)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(YELLOW)%-15s$(NC) %s\n", $$1, $$2}'

install-tools: ## 安装开发工具
	@echo "$(GREEN)安装开发工具...$(NC)"
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	go install golang.org/x/tools/cmd/goimports@latest
	@echo "$(GREEN)✓ 工具安装完成$(NC)"

lint: ## 运行代码检查
	@echo "$(GREEN)运行 golangci-lint...$(NC)"
	golangci-lint run ./...

lint-fix: ## 运行代码检查并自动修复
	@echo "$(GREEN)运行 golangci-lint (自动修复)...$(NC)"
	golangci-lint run --fix ./...

fmt: ## 格式化代码
	@echo "$(GREEN)格式化代码...$(NC)"
	gofmt -s -w .
	goimports -w .

vet: ## 运行 go vet
	@echo "$(GREEN)运行 go vet...$(NC)"
	go vet ./...

test: ## 运行测试
	@echo "$(GREEN)运行测试...$(NC)"
	go test -v ./...

test-coverage: ## 运行测试并生成覆盖率报告
	@echo "$(GREEN)运行测试并生成覆盖率...$(NC)"
	go test -v -coverprofile=$(COVERAGE_FILE) ./...
	go tool cover -html=$(COVERAGE_FILE) -o coverage.html
	@echo "$(GREEN)✓ 覆盖率报告: coverage.html$(NC)"

race: ## 运行竞态检测
	@echo "$(GREEN)运行竞态检测...$(NC)"
	go test -race ./...

build-web: ## 构建前端项目
	@echo "$(GREEN)构建前端项目...$(NC)"
	@bash scripts/build-web.sh

build-agent: ## 构建 Agent
	@echo "$(GREEN)构建 Agent...$(NC)"
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY_AGENT) ./cmd/agent
	@echo "$(GREEN)✓ Agent 构建完成: $(BIN_DIR)/$(BINARY_AGENT)$(NC)"

build-dashboard: build-web ## 构建 Dashboard（包含前端）
	@echo "$(GREEN)构建 Dashboard...$(NC)"
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY_DASHBOARD) ./cmd/dashboard
	@echo "$(GREEN)✓ Dashboard 构建完成: $(BIN_DIR)/$(BINARY_DASHBOARD)$(NC)"

build-dashboard-only: ## 仅构建 Dashboard（不构建前端）
	@echo "$(GREEN)构建 Dashboard（跳过前端）...$(NC)"
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY_DASHBOARD) ./cmd/dashboard
	@echo "$(GREEN)✓ Dashboard 构建完成: $(BIN_DIR)/$(BINARY_DASHBOARD)$(NC)"

build: build-agent build-dashboard ## 构建所有二进制文件

run-agent: build-agent ## 运行 Agent
	@echo "$(GREEN)运行 Agent...$(NC)"
	./$(BIN_DIR)/$(BINARY_AGENT)

run-dashboard: build-dashboard ## 运行 Dashboard
	@echo "$(GREEN)运行 Dashboard...$(NC)"
	./$(BIN_DIR)/$(BINARY_DASHBOARD)

dev-web: ## 启动前端开发服务器
	@echo "$(GREEN)启动前端开发服务器...$(NC)"
	cd web && pnpm run dev

clean: ## 清理构建产物
	@echo "$(GREEN)清理构建产物...$(NC)"
	rm -rf $(BIN_DIR) $(DIST_DIR) $(COVERAGE_FILE) coverage.html
	rm -rf web/dist web/node_modules
	find internal/dashboard/public/dist -mindepth 1 ! -name '.gitkeep' ! -name 'README.md' -delete 2>/dev/null || true
	@echo "$(GREEN)✓ 清理完成$(NC)"

clean-web: ## 清理前端构建产物
	@echo "$(GREEN)清理前端产物...$(NC)"
	rm -rf web/dist
	find internal/dashboard/public/dist -mindepth 1 ! -name '.gitkeep' ! -name 'README.md' -delete 2>/dev/null || true
	@echo "$(GREEN)✓ 前端清理完成$(NC)"

tidy: ## 整理依赖
	@echo "$(GREEN)整理依赖...$(NC)"
	go mod tidy
	@echo "$(GREEN)✓ 依赖整理完成$(NC)"

check: fmt vet lint test ## 运行所有检查（格式、审查、Lint、测试）

gosec: ## 运行安全检查
	@echo "$(GREEN)运行安全检查...$(NC)"
	gosec -fmt=text ./...

all: clean check build ## 清理、检查、构建全流程

release: ## 使用 goreleaser 构建发布版本
	@echo "$(GREEN)使用 goreleaser 构建...$(NC)"
	goreleaser release --snapshot --clean

pre-commit: fmt vet lint ## Git 提交前检查
	@echo "$(GREEN)✓ 提交前检查完成$(NC)"

docker-build: ## 构建 Docker 镜像（本地测试）
	@echo "$(GREEN)构建 Docker 镜像...$(NC)"
	@bash scripts/build-docker.sh

docker-build-multi: ## 构建多架构 Docker 镜像
	@echo "$(GREEN)构建多架构 Docker 镜像...$(NC)"
	@bash scripts/build-docker.sh --multi-arch

docker-run: ## 运行 Docker 容器（使用示例配置）
	@echo "$(GREEN)运行 Docker 容器...$(NC)"
	docker run --rm -p 8900:8900 -v $$(pwd)/configs/sss-dashboard.yaml.example:/app/sss-dashboard.yaml sssd:dev

docker-clean: ## 清理 Docker 镜像
	@echo "$(GREEN)清理 Docker 镜像...$(NC)"
	docker rmi sssd:dev 2>/dev/null || true
	@echo "$(GREEN)✓ Docker 镜像清理完成$(NC)"

