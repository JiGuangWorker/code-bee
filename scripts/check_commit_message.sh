#!/usr/bin/env bash
# 核心功能:
# 1. 校验 Git commit message 或 PR 标题是否符合仓库约定的 Conventional Commits 格式
# 2. 作为本地 commit-msg hook、Makefile 检查入口和 CI 校验脚本复用，避免规则散落
#
# 开发维护: AI Assistant
# 创建时间: 2026-07-06
# 更新时间: 2026-07-06

set -euo pipefail

readonly COMMIT_MESSAGE_PATTERN='^(feat|fix|refactor|perf|test|docs|chore|style)(\([a-z0-9-]+\))?: .{1,50}$'

print_usage() {
  cat <<'EOF'
用法:
  ./scripts/check_commit_message.sh --file <commit-message-file>
  ./scripts/check_commit_message.sh --message "<commit-or-pr-title>"
  ./scripts/check_commit_message.sh --head
EOF
}

validate_message() {
  local message="$1"

  if [[ ! "$message" =~ $COMMIT_MESSAGE_PATTERN ]]; then
    echo "提交信息格式不合规: $message" >&2
    echo "正确格式: <type>(<scope>): <subject>" >&2
    echo "示例: feat(ci): add release workflow" >&2
    exit 1
  fi
}

if [[ $# -eq 0 ]]; then
  print_usage
  exit 1
fi

case "${1:-}" in
  --file)
    if [[ $# -ne 2 ]]; then
      print_usage
      exit 1
    fi
    validate_message "$(head -n 1 "$2" | tr -d '\r')"
    ;;
  --message)
    if [[ $# -ne 2 ]]; then
      print_usage
      exit 1
    fi
    validate_message "$2"
    ;;
  --head)
    if [[ $# -ne 1 ]]; then
      print_usage
      exit 1
    fi
    validate_message "$(git log -1 --pretty=%s)"
    ;;
  *)
    print_usage
    exit 1
    ;;
esac
