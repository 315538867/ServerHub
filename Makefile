.PHONY: build build-frontend dev-backend dev-frontend clean test test-race bench arch-lint e2e-smoke baseline

BINARY := serverhub
BACKEND_DIR := backend
FRONTEND_DIR := frontend

# Build frontend then backend (single binary with embedded frontend)
build: build-frontend
	cd $(BACKEND_DIR) && CGO_ENABLED=1 go build -ldflags="-s -w" -o $(BINARY) .
	@echo "Binary: $(BACKEND_DIR)/$(BINARY)"

# Build frontend only (output goes to backend/web/dist/)
build-frontend:
	cd $(FRONTEND_DIR) && npm install && npm run build

# Start backend in dev mode (reads frontend from Vite dev server via proxy)
dev-backend:
	cd $(BACKEND_DIR) && go run . --dev

# Start frontend Vite dev server
dev-frontend:
	cd $(FRONTEND_DIR) && npm run dev

clean:
	rm -f $(BACKEND_DIR)/$(BINARY)
	rm -rf $(BACKEND_DIR)/web/dist/assets $(BACKEND_DIR)/web/dist/index.html

# === v2 重构期门禁 (docs/architecture/v2/06-quality-gates.md) ===

# Race + count=1 全包测试
test-race:
	cd $(BACKEND_DIR) && CGO_ENABLED=1 go test -race -count=1 ./...

# 不带 race 的快速回归
test:
	cd $(BACKEND_DIR) && CGO_ENABLED=1 go test -count=1 ./...

# 性能基线对比(R0 baseline 已落 baseline/bench-v1.txt)
bench:
	cd $(BACKEND_DIR) && CGO_ENABLED=1 go test -bench=. -benchmem -run='^$$' \
	  ./pkg/svcstatus/... ./pkg/nginxrender/... ./derive/... ./repo/... 2>&1 \
	  | tee /tmp/bench-v2.txt

# arch-lint(R1 起逐步启用规则,占位先用 go vet)
arch-lint:
	cd $(BACKEND_DIR) && go vet ./...
	@echo "[TODO] arch-lint full rules will be enabled from R1 (see arch-lint.yml)"

# e2e 烟雾测试(R0 占位,R2/R4/R5 阶段陆续填实)
# 5 大场景:
#   1. server add → discover → takeover docker → service 列表
#   2. release create → apply → reconcile → synced
#   3. release modify → apply 失败 → autoRollback
#   4. application ingress apply → nginx -t → reload
#   5. login + MFA + JWT
e2e-smoke:
	@echo "[TODO] e2e-smoke 5 scenarios — placeholder until R2-R5"
	@echo "  scenario 1: discover+takeover  — R4 implements"
	@echo "  scenario 2: apply release       — R2 implements"
	@echo "  scenario 3: rollback            — R2 implements"
	@echo "  scenario 4: ingress apply       — R5 implements"
	@echo "  scenario 5: auth                — R6 wires up"

# 重新生成 baseline(慎用 — 会覆盖 R0 的对比基线)
baseline:
	@echo "WARN: this overwrites baseline/. Press Ctrl-C in 5s to abort."
	@sleep 5
	cd $(BACKEND_DIR) && CGO_ENABLED=1 go test -count=1 -coverprofile=/tmp/cover.out ./...
	cd $(BACKEND_DIR) && go tool cover -func=/tmp/cover.out > ../baseline/cover-v1.txt
	cd $(BACKEND_DIR) && CGO_ENABLED=1 go test -bench=. -benchmem -run='^$$' \
	  ./pkg/svcstatus/... ./pkg/nginxrender/... > ../baseline/bench-v1.txt
	@echo "baseline regenerated."
