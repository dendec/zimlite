APP      := kiwix-sdl
SRC      := ./cmd/kiwix-sdl
GO       := go
GOFLAGS  := CGO_ENABLED=1

ZIM_VER      := 9.7.0
ZIM_TAG      := x86_64
ZIM_LIBDIR   := lib/libzim_linux-$(ZIM_TAG)-$(ZIM_VER)
ZIM_INC      := $(ZIM_LIBDIR)/include
ZIM_LIB      := $(ZIM_LIBDIR)/lib/x86_64-linux-gnu
ZIM_URL      := https://download.openzim.org/release/libzim

CGO_CXXFLAGS := -std=c++17 -I$(shell pwd)/internal/zim -I$(shell pwd)/$(ZIM_INC)
CGO_LDFLAGS  := -L$(shell pwd)/$(ZIM_LIB) -lzim -Wl,-rpath,\$$ORIGIN/$(ZIM_LIB) -Wl,--disable-new-dtags

VERSION      ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1")
LDFLAGS      := -X 'github.com/kiwix-sdl/kiwix-sdl/internal/storage.Version=$(VERSION)'

.PHONY: build test vet lint clean run info fmt
.PHONY: deps build-linux-arm64 build-linux-amd64 build-windows-amd64
.PHONY: dist-arm64 dist-windows dist-amd64 deploy dist-portmaster deploy-portmaster

fmt:
	gofmt -s -w .

build: fmt $(ZIM_LIB)/libzim.so
	$(GOFLAGS) CGO_CXXFLAGS="$(CGO_CXXFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS)" \
		$(GO) build -ldflags "$(LDFLAGS)" -o $(APP) $(SRC)


$(ZIM_LIB)/libzim.so:
	@mkdir -p $(ZIM_LIBDIR)
	wget -q "$(ZIM_URL)/libzim_linux-$(ZIM_TAG)-$(ZIM_VER).tar.gz" -O /tmp/libzim.tar.gz
	tar xzf /tmp/libzim.tar.gz -C lib/
	@rm -f /tmp/libzim.tar.gz

# Cross-build targets.
build-linux-amd64:
	$(GOFLAGS) CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
		CC=x86_64-linux-gnu-gcc CXX=x86_64-linux-gnu-g++ \
		CGO_CXXFLAGS="-std=c++17 -Iinternal/zim -Ilib/libzim_linux-x86_64-$(ZIM_VER)/include" \
		CGO_LDFLAGS="-Llib/libzim_linux-x86_64-$(ZIM_VER)/lib/x86_64-linux-gnu -lzim" \
		$(GO) build -ldflags "$(LDFLAGS)" -o kiwix-sdl-amd64 ./cmd/kiwix-sdl

build-linux-arm64:
	$(GOFLAGS) CGO_ENABLED=1 GOOS=linux GOARCH=arm64 \
		CC=aarch64-linux-gnu-gcc CXX=aarch64-linux-gnu-g++ \
		CGO_CXXFLAGS="-std=c++17 -Iinternal/zim -Ilib/libzim_linux-aarch64-$(ZIM_VER)/include" \
		CGO_LDFLAGS="-Llib/libzim_linux-aarch64-$(ZIM_VER)/lib/aarch64-linux-gnu -lzim" \
		$(GO) build -ldflags "$(LDFLAGS)" -o kiwix-sdl-arm64 ./cmd/kiwix-sdl

build-windows-amd64: dist-windows

test:
	$(GOFLAGS) CGO_CXXFLAGS="$(CGO_CXXFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS)" $(GO) test ./...

vet:
	$(GOFLAGS) CGO_CXXFLAGS="$(CGO_CXXFLAGS)" $(GO) vet ./...

lint:
	@which golangci-lint >/dev/null 2>&1 || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	golangci-lint run ./cmd/... ./internal/...

clean:
	rm -f $(APP) $(APP)-*
	rm -rf dist

run: build
	LD_LIBRARY_PATH=$(ZIM_LIB):$$LD_LIBRARY_PATH ./$(APP) /tmp/test.md

info:
	@echo "ZIM_LIB=$(ZIM_LIB)"

# ARM64 PortMaster distribution via Docker.
DOCKER_IMAGE := kiwix-arm64
DOCKER_FILE  := Dockerfile.arm64
DEVICE_DIR   := /mnt/SDCARD/Data/ports/kiwix-sdl
PORTS_DIR    := /mnt/SDCARD/Roms/PORTS
PORT_SCRIPT  := Kiwix SDL.sh

dist-arm64:
	docker build -t $(DOCKER_IMAGE) -f $(DOCKER_FILE) .
	@mkdir -p dist
	@docker rm -f kiwix-extract >/dev/null 2>&1 || true
	docker create --name kiwix-extract $(DOCKER_IMAGE) >/dev/null 2>&1
	docker cp kiwix-extract:/dist/kiwix-sdl/. dist/kiwix-sdl/
	docker rm kiwix-extract >/dev/null 2>&1
	@echo "=== dist/kiwix-sdl/ ==="
	@ls -lh dist/kiwix-sdl/

dist-windows:
	docker build -t kiwix-windows -f Dockerfile.windows .
	@mkdir -p dist/windows
	@docker rm -f kiwix-win-extract >/dev/null 2>&1 || true
	docker create --name kiwix-win-extract kiwix-windows >/dev/null 2>&1
	docker cp kiwix-win-extract:/dist/kiwix-sdl/. dist/windows/
	docker rm kiwix-win-extract >/dev/null 2>&1
	cd dist/windows && zip -r ../kiwix-sdl-windows-amd64.zip .
	@echo "=== dist/windows/ ==="
	@ls -lh dist/windows/

dist-amd64: build
	@mkdir -p dist/linux-amd64/lib
	cp kiwix-sdl dist/linux-amd64/
	cp $(ZIM_LIB)/libzim.so.9 dist/linux-amd64/lib/
	cd dist/linux-amd64 && zip -r ../kiwix-sdl-linux-amd64.zip .
	@echo "=== Generated dist/kiwix-sdl-linux-amd64.zip ==="

deploy: dist-arm64
	adb shell "mkdir -p $(DEVICE_DIR)/lib"
	adb push dist/kiwix-sdl/kiwix-sdl $(DEVICE_DIR)/
	adb push dist/kiwix-sdl/lib/libzim.so.9.7.0 $(DEVICE_DIR)/lib/
	adb push dist/kiwix-sdl/lib/liblzma.so $(DEVICE_DIR)/lib/
	adb push dist/kiwix-sdl/lib/libzstd.so.1.4.8 $(DEVICE_DIR)/lib/
	adb shell "cd $(DEVICE_DIR)/lib && cp libzim.so.9.7.0 libzim.so.9 && cp libzim.so.9.7.0 libzim.so && cp libzstd.so.1.4.8 libzstd.so.1 && cp liblzma.so liblzma.so.5"
	adb push scripts/kiwix-sdl.sh '$(PORTS_DIR)/$(PORT_SCRIPT)'
	adb shell "chmod +x '$(PORTS_DIR)/$(PORT_SCRIPT)' && rm -f $(PORTS_DIR)/PORTS_cache7.db && killall -9 kiwix-sdl 2>/dev/null; true"
	@echo "=== Deployed ==="

dist-portmaster: dist-arm64
	@rm -rf dist/portmaster_build
	@mkdir -p dist/portmaster_build/kiwix-sdl/lib
	cp "portmaster/Kiwix SDL.sh" dist/portmaster_build/
	cp "portmaster/port.json" dist/portmaster_build/
	cp "portmaster/screenshot.png" dist/portmaster_build/
	cp "portmaster/screenshot.png" dist/portmaster_build/kiwix-sdl/cover.png
	cp "portmaster/gameinfo.xml" dist/portmaster_build/kiwix-sdl/
	cp "portmaster/README.md" dist/portmaster_build/kiwix-sdl/
	cp -r "portmaster/licenses" dist/portmaster_build/kiwix-sdl/
	cp dist/kiwix-sdl/kiwix-sdl dist/portmaster_build/kiwix-sdl/
	cp dist/kiwix-sdl/lib/* dist/portmaster_build/kiwix-sdl/lib/
	cd dist/portmaster_build && zip -r ../kiwix-sdl-portmaster.zip "Kiwix SDL.sh" port.json screenshot.png kiwix-sdl
	@echo "=== Generated dist/kiwix-sdl-portmaster.zip ==="
	@ls -lh dist/kiwix-sdl-portmaster.zip

deploy-portmaster: dist-portmaster
	adb shell "rm -rf $(DEVICE_DIR) '$(PORTS_DIR)/$(PORT_SCRIPT)'"
	adb shell "mkdir -p /mnt/SDCARD/Apps/PortMaster/PortMaster/autoinstall"
	adb push dist/kiwix-sdl-portmaster.zip /mnt/SDCARD/Apps/PortMaster/PortMaster/autoinstall/
	adb shell "mkdir -p /mnt/SDCARD/Imgs/PORTS"
	adb push portmaster/screenshot.png '/mnt/SDCARD/Imgs/PORTS/Kiwix SDL.png'
	@echo "=== Zip deployed to autoinstall ==="

