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

.PHONY: build test vet lint clean run info
.PHONY: deps build-linux-arm64 build-linux-armv8 build-linux-amd64
.PHONY: dist-arm64 deploy deploy-full

build: $(ZIM_LIB)/libzim.so
	$(GOFLAGS) CGO_CXXFLAGS="$(CGO_CXXFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS)" \
		$(GO) build -o $(APP) $(SRC)

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
		$(GO) build -o $(APP)-amd64 $(SRC)

build-linux-arm64:
	$(GOFLAGS) CGO_ENABLED=1 GOOS=linux GOARCH=arm64 \
		CC=aarch64-linux-gnu-gcc CXX=aarch64-linux-gnu-g++ \
		CGO_CXXFLAGS="-std=c++17 -Iinternal/zim -Ilib/libzim_linux-aarch64-$(ZIM_VER)/include" \
		CGO_LDFLAGS="-Llib/libzim_linux-aarch64-$(ZIM_VER)/lib/aarch64-linux-gnu -lzim" \
		$(GO) build -o $(APP)-arm64 $(SRC)

build-linux-armv8:
	$(GOFLAGS) CGO_ENABLED=1 GOOS=linux GOARCH=arm GOARM=7 \
		CC=arm-linux-gnueabihf-gcc CXX=arm-linux-gnueabihf-g++ \
		CGO_CXXFLAGS="-std=c++17 -Iinternal/zim -Ilib/libzim_linux-armv8-$(ZIM_VER)/include" \
		CGO_LDFLAGS="-Llib/libzim_linux-armv8-$(ZIM_VER)/lib/armv8-linux-gnueabihf -lzim" \
		$(GO) build -o $(APP)-armv8 $(SRC)

test:
	$(GOFLAGS) CGO_CXXFLAGS="$(CGO_CXXFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS)" $(GO) test ./...

vet:
	$(GOFLAGS) CGO_CXXFLAGS="$(CGO_CXXFLAGS)" $(GO) vet ./...

lint:
	@which golangci-lint >/dev/null 2>&1 || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	golangci-lint run

clean:
	rm -f $(APP) $(APP)-*

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
	docker create --name kiwix-extract $(DOCKER_IMAGE) >/dev/null 2>&1
	docker cp kiwix-extract:/dist/kiwix-sdl/. dist/kiwix-sdl/
	docker rm kiwix-extract >/dev/null 2>&1
	@echo "=== dist/kiwix-sdl/ ==="
	@ls -lh dist/kiwix-sdl/

deploy: dist-arm64
	adb shell "mkdir -p $(DEVICE_DIR)/lib"
	adb push dist/kiwix-sdl/kiwix-sdl $(DEVICE_DIR)/
	adb shell "cd $(DEVICE_DIR)/lib && if [ ! -f libzim.so.9.7.0 ]; then echo 'push libs...'; fi; cp libzim.so.9.7.0 libzim.so.9 2>/dev/null; cp libzim.so.9.7.0 libzim.so 2>/dev/null; cp libzstd.so.1.4.8 libzstd.so.1 2>/dev/null; cp liblzma.so liblzma.so.5 2>/dev/null; true"
	adb push scripts/kiwix-sdl.sh '$(PORTS_DIR)/$(PORT_SCRIPT)'
	adb shell "chmod +x '$(PORTS_DIR)/$(PORT_SCRIPT)' && rm -f $(PORTS_DIR)/PORTS_cache7.db && killall -9 kiwix-sdl 2>/dev/null; true"
	@echo "=== Deployed ==="

deploy-full: dist-arm64
	adb shell "mkdir -p $(DEVICE_DIR)/lib"
	adb push dist/kiwix-sdl/kiwix-sdl $(DEVICE_DIR)/
	adb push dist/kiwix-sdl/lib/libzim.so.9.7.0 $(DEVICE_DIR)/lib/
	adb push dist/kiwix-sdl/lib/liblzma.so $(DEVICE_DIR)/lib/
	adb push dist/kiwix-sdl/lib/libzstd.so.1.4.8 $(DEVICE_DIR)/lib/
	adb shell "cd $(DEVICE_DIR)/lib && cp libzim.so.9.7.0 libzim.so.9 && cp libzim.so.9.7.0 libzim.so && cp libzstd.so.1.4.8 libzstd.so.1 && cp liblzma.so liblzma.so.5"
	adb push scripts/kiwix-sdl.sh '$(PORTS_DIR)/$(PORT_SCRIPT)'
	adb shell "chmod +x '$(PORTS_DIR)/$(PORT_SCRIPT)' && rm -f $(PORTS_DIR)/PORTS_cache7.db && killall -9 kiwix-sdl 2>/dev/null; true"
	@echo "=== Full deployed ==="
