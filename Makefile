# ========================================================
# Rdp Sensei - Build & Packaging Makefile
# Pure Go + WebAssembly (Zero external dependencies)
# ========================================================

APP_NAME     = rdp-sensei
APP_BUNDLE   = "Rdp Sensei.app"
WASM_OUT     = static/main.wasm
BUILD_DIR    = build
GOROOT      := $(shell go env GOROOT)

.PHONY: all wasm build build-all app clean run run-web help

all: wasm build app

help:
	@echo "Rdp Sensei - Kullanılabilir Komutlar:"
	@echo "  make wasm        - Pure Go WebAssembly RDP motorunu derler (static/main.wasm)"
	@echo "  make build       - Yerel işletim sistemi için rdp-sensei binary derler"
	@echo "  make app         - macOS için 'Rdp Sensei.app' paketini üretir"
	@echo "  make build-all   - macOS .app, Windows (.exe) ve Linux için derler"
	@echo "  make run         - Masaüstü uygulama modunda başlatır"
	@echo "  make run-web     - Ağ üzerinden erişilebilir Web Sunucusu modunda başlatır (:8080)"
	@echo "  make clean       - Derleme çıktılarını temizler"

wasm:
	@echo "⚙️  WebAssembly RDP motoru derleniyor..."
	@cd wasm && GOOS=js GOARCH=wasm go build -o ../$(WASM_OUT) .
	@echo "✅ WASM motoru hazır: $(WASM_OUT)"

touchbar:
	@if [ "$$(uname -s)" = "Darwin" ]; then \
		echo "🍎 macOS Physical Touch Bar agent derleniyor..."; \
		mkdir -p native; \
		swiftc -O native/touchbar_agent.swift -o native/touchbar_agent; \
		chmod +x native/touchbar_agent; \
	fi

build: wasm touchbar
	@echo "⚙️  Yerel binary derleniyor..."
	@go build -ldflags="-s -w" -o $(APP_NAME) .
	@echo "✅ Derleme tamamlandı: ./$(APP_NAME)"

app: build
	@echo "🍏 macOS 'Rdp Sensei.app' paketi oluşturuluyor..."
	@mkdir -p $(BUILD_DIR)/$(APP_BUNDLE)/Contents/MacOS
	@mkdir -p $(BUILD_DIR)/$(APP_BUNDLE)/Contents/Resources
	@cp $(APP_NAME) $(BUILD_DIR)/$(APP_BUNDLE)/Contents/MacOS/"Rdp Sensei"
	@if [ -f native/touchbar_agent ]; then \
		cp native/touchbar_agent $(BUILD_DIR)/$(APP_BUNDLE)/Contents/MacOS/touchbar_agent; \
		chmod +x $(BUILD_DIR)/$(APP_BUNDLE)/Contents/MacOS/touchbar_agent; \
	fi
	@cp resources/Info.plist $(BUILD_DIR)/$(APP_BUNDLE)/Contents/Info.plist
	@cp resources/AppIcon.icns $(BUILD_DIR)/$(APP_BUNDLE)/Contents/Resources/AppIcon.icns
	@chmod +x $(BUILD_DIR)/$(APP_BUNDLE)/Contents/MacOS/"Rdp Sensei"
	@rm -rf $(APP_BUNDLE)
	@cp -R $(BUILD_DIR)/$(APP_BUNDLE) ./
	@echo "✅ 'Rdp Sensei.app' başarıyla oluşturuldu! Doğrudan çift tıklayarak açabilirsiniz."

build-all: wasm app
	@echo "⚙️  Tüm platformlar için derleniyor..."
	@mkdir -p $(BUILD_DIR)
	# macOS Apple Silicon
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(APP_NAME)-darwin-arm64 .
	# macOS Intel
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(APP_NAME)-darwin-amd64 .
	# Windows 64-bit
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(APP_NAME)-windows-amd64.exe .
	# Linux 64-bit
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(APP_NAME)-linux-amd64 .
	@echo "✅ Çapraz derleme tamamlandı. Çıktılar: $(BUILD_DIR)/"

run: build
	./$(APP_NAME)

run-web: build
	./$(APP_NAME) -web -port 8080

clean:
	rm -f $(APP_NAME) $(WASM_OUT)
	rm -rf $(BUILD_DIR) $(APP_BUNDLE)
