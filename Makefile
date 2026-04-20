.PHONY: build build-frontend dev-backend dev-frontend clean

BINARY := serverhub
BACKEND_DIR := backend
FRONTEND_DIR := frontend

# Build frontend then backend (single binary with embedded frontend)
build: build-frontend
	cd $(BACKEND_DIR) && CGO_ENABLED=1 go build -ldflags="-s -w" -o $(BINARY) .
	@echo "Binary: $(BACKEND_DIR)/$(BINARY)"

# Build frontend only (output goes to backend/web/dist/)
build-frontend:
	cd $(FRONTEND_DIR) && npm ci && npm run build

# Start backend in dev mode (reads frontend from Vite dev server via proxy)
dev-backend:
	cd $(BACKEND_DIR) && go run . --dev

# Start frontend Vite dev server
dev-frontend:
	cd $(FRONTEND_DIR) && npm run dev

clean:
	rm -f $(BACKEND_DIR)/$(BINARY)
	rm -rf $(BACKEND_DIR)/web/dist/assets $(BACKEND_DIR)/web/dist/index.html
