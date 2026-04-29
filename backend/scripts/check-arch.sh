#!/usr/bin/env bash
# check-arch.sh — 架构约束验证脚本（R7+）
#
# 规则 1 (domain-pure): domain/ 只能 import stdlib，不得 import 任何项目内部包。
# 规则 2 (model-isolation): 生产代码中 model/ 的引用需控制。
#
# 用法:
#   ./scripts/check-arch.sh          # 全部检查
#   ./scripts/check-arch.sh --strict # 严格模式:任一规则失败则 exit 1

set -uo pipefail
cd "$(dirname "$0")/.."

STRICT=false
if [[ "${1:-}" == "--strict" ]]; then
    STRICT=true
fi

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

PASS=0
FAIL=0

# ── 规则 1: domain-pure ──
echo "━━━ 规则 1: domain/ 纯净性 ━━━"
violations=$(grep -rn '"github.com/serverhub/serverhub/' domain/ --include='*.go' 2>/dev/null || true)

if [[ -z "$violations" ]]; then
    echo -e "${GREEN}PASS${NC} domain/ 未引用任何项目内部包"
    PASS=$((PASS + 1))
else
    echo -e "${RED}FAIL${NC} domain/ 包含内部包引用:"
    echo "$violations"
    FAIL=$((FAIL + 1))
fi

# ── 规则 2: usecase/ 不得引用 model/ (R7 退出标准) ──
echo ""
echo "━━━ 规则 2: usecase/ 不引用 model/ ━━━"
uc_violations=$(grep -rl '"github.com/serverhub/serverhub/model"' usecase/*.go 2>/dev/null | grep -v _test.go || true)

if [[ -z "$uc_violations" ]]; then
    echo -e "${GREEN}PASS${NC} usecase/ 未引用 model/"
    PASS=$((PASS + 1))
else
    echo -e "${RED}FAIL${NC} usecase/ 仍然引用 model/:"
    echo "$uc_violations"
    FAIL=$((FAIL + 1))
fi

# ── 结果 ──
echo ""
echo "━━━━━━━━━━━━━━━━━━━━"
echo -e "通过: ${GREEN}$PASS${NC}  失败: ${RED}$FAIL${NC}"
if [[ $FAIL -gt 0 ]]; then
    if $STRICT; then
        exit 1
    else
        echo -e "${YELLOW}提示: 使用 --strict 使失败时退出码非零${NC}"
    fi
fi
