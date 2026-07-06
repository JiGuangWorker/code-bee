#!/usr/bin/env bash
# 核心功能:
# 1. 将仓库内维护的 Git hooks 安装到当前本地仓库，统一提交前和推送前门禁
# 2. 避免开发者手工复制脚本导致本地检查行为不一致
#
# 开发维护: AI Assistant
# 创建时间: 2026-07-06
# 更新时间: 2026-07-06

set -euo pipefail

readonly GIT_HOOKS_DIR=".git/hooks"
readonly SOURCE_HOOKS_DIR="scripts/githooks"

mkdir -p "$GIT_HOOKS_DIR"

for hook_name in pre-commit commit-msg pre-push; do
  cp "$SOURCE_HOOKS_DIR/$hook_name" "$GIT_HOOKS_DIR/$hook_name"
  chmod +x "$GIT_HOOKS_DIR/$hook_name"
done

echo "Git hooks 安装完成"
