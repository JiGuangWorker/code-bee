# code-bee · 项目统一操作入口
#
# 核心功能:
# 1. 提供 backend / frontend / deploy 分层后的统一命令入口，避免开发者记忆各子目录的具体命令
# 2. 将“提交前检查、CI 校验、发布前校验”收口为可复用目标，保证本地与流水线执行口径一致
#
# 开发维护: AI Assistant
# 创建时间: 2026-07-04
# 更新时间: 2026-07-06

.PHONY: \
	help \
	build build-backend \
	test test-backend-unit test-backend-race test-ci \
	lint lint-backend \
	check-structure check-commits check-all \
	install-hooks release-check release-smoke \
	clean

# 默认目标
help:
	@echo "code-bee · Issue-driven AI coding scheduler"
	@echo ""
	@echo "  make build                 编译后端二进制"
	@echo "  make test                  运行后端 race 测试"
	@echo "  make lint                  运行后端静态分析"
	@echo "  make test-backend-unit     运行后端单元测试"
	@echo "  make test-backend-race     运行后端 race 测试"
	@echo "  make check-structure       项目目录结构校验"
	@echo "  make check-commits         当前提交信息格式校验"
	@echo "  make check-all             提交前静态门禁"
	@echo "  make test-ci               CI 级测试入口"
	@echo "  make install-hooks         安装本地 Git hooks"
	@echo "  make release-check         发布前总校验"
	@echo "  make release-smoke         发布前最小冒烟校验"
	@echo "  make clean                 清理编译产物"
	@echo ""

# ============================================================
# 编译
# ============================================================
build: build-backend

build-backend:
	@echo "→ 编译 code-bee..."
	cd backend && go build -o ../bin/code-bee ./cmd/worker/

# ============================================================
# 测试
# ============================================================
test: test-backend-race

test-backend-unit:
	@echo "→ 运行后端单元测试..."
	cd backend && go test -v ./...

test-backend-race:
	@echo "→ 运行测试..."
	cd backend && go test -race -v ./...

test-ci: test-backend-unit test-backend-race
	@echo ""
	@echo "✅ CI 级测试通过"

# ============================================================
# 静态分析
# ============================================================
lint: lint-backend

lint-backend:
	@echo "→ golangci-lint 代码检查..."
	cd backend && golangci-lint run -c ../.golangci.yml ./...

# ============================================================
# 目录结构校验
# ============================================================
check-structure:
	@echo "→ conform 目录结构校验..."
	conform enforce

check-commits:
	@echo "→ 校验当前提交信息格式..."
	./scripts/check_commit_message.sh --head

# ============================================================
# 全部校验
# ============================================================
check-all: lint-backend check-structure check-commits
	@echo ""
	@echo "✅ 全部校验通过"

# ============================================================
# Git hooks
# ============================================================
install-hooks:
	@echo "→ 安装本地 Git hooks..."
	./scripts/install_hooks.sh

# ============================================================
# 发布前校验
# ============================================================
release-check: check-all test-ci build-backend
	@echo ""
	@echo "✅ 发布前总校验通过"

release-smoke:
	@echo "→ 执行发布前最小冒烟校验..."
	test -f bin/code-bee
	# 使用 version 子命令做无副作用冒烟校验，避免帮助输出路径返回非零退出码。
	./bin/code-bee -version >/dev/null
	@echo "✅ 冒烟校验通过"

# ============================================================
# 清理
# ============================================================
clean:
	@echo "→ 清理编译产物..."
	rm -rf bin/
