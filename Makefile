APP      := zimlite
SRC      := ./cmd/zimlite
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
LDFLAGS      := -X 'github.com/dendec/zimlite/internal/storage.Version=$(VERSION)'

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
		$(GO) build -ldflags "$(LDFLAGS)" -o zimlite-amd64 ./cmd/zimlite

build-linux-arm64:
	$(GOFLAGS) CGO_ENABLED=1 GOOS=linux GOARCH=arm64 \
		CC=aarch64-linux-gnu-gcc CXX=aarch64-linux-gnu-g++ \
		CGO_CXXFLAGS="-std=c++17 -Iinternal/zim -Ilib/libzim_linux-aarch64-$(ZIM_VER)/include" \
		CGO_LDFLAGS="-Llib/libzim_linux-aarch64-$(ZIM_VER)/lib/aarch64-linux-gnu -lzim" \
		$(GO) build -ldflags "$(LDFLAGS)" -o zimlite-arm64 ./cmd/zimlite

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
DOCKER_IMAGE := zimlite-arm64
DOCKER_FILE  := Dockerfile.arm64
DEVICE_DIR   := /userdata/roms/ports/zimlite
PORTS_DIR    := /userdata/roms/ports
PORT_SCRIPT  := Zimlite.sh
PM_AUTOINSTALL := /userdata/system/.local/share/PortMaster/autoinstall

dist-arm64:
	docker build -t $(DOCKER_IMAGE) -f $(DOCKER_FILE) .
	@mkdir -p dist
	@docker rm -f zimlite-extract >/dev/null 2>&1 || true
	docker create --name zimlite-extract $(DOCKER_IMAGE) >/dev/null 2>&1
	docker cp zimlite-extract:/dist/zimlite/. dist/zimlite/
	docker rm zimlite-extract >/dev/null 2>&1
	@echo "=== dist/zimlite/ ==="
	@ls -lh dist/zimlite/

dist-windows:
	docker build -t zimlite-windows -f Dockerfile.windows .
	@mkdir -p dist/windows
	@docker rm -f zimlite-win-extract >/dev/null 2>&1 || true
	docker create --name zimlite-win-extract zimlite-windows >/dev/null 2>&1
	docker cp zimlite-win-extract:/dist/zimlite/. dist/windows/
	docker rm zimlite-win-extract >/dev/null 2>&1
	cd dist/windows && zip -r ../zimlite-windows-amd64.zip .
	@echo "=== dist/windows/ ==="
	@ls -lh dist/windows/

dist-amd64: build
	@mkdir -p dist/linux-amd64/lib
	cp zimlite dist/linux-amd64/
	cp $(ZIM_LIB)/libzim.so.9 dist/linux-amd64/lib/
	cd dist/linux-amd64 && zip -r ../zimlite-linux-amd64.zip .
	@echo "=== Generated dist/zimlite-linux-amd64.zip ==="

deploy: dist-arm64
	adb shell "mkdir -p $(DEVICE_DIR)/lib"
	adb push dist/zimlite/zimlite $(DEVICE_DIR)/
	adb push dist/zimlite/lib/libzim.so.9.7.0 $(DEVICE_DIR)/lib/
	adb push dist/zimlite/lib/liblzma.so $(DEVICE_DIR)/lib/
	adb push dist/zimlite/lib/libzstd.so.1.4.8 $(DEVICE_DIR)/lib/
	adb shell "cd $(DEVICE_DIR)/lib && ln -sf libzim.so.9.7.0 libzim.so.9 && ln -sf libzim.so.9.7.0 libzim.so && ln -sf libzstd.so.1.4.8 libzstd.so.1 && ln -sf liblzma.so liblzma.so.5"
	adb push scripts/zimlite.sh '$(PORTS_DIR)/$(PORT_SCRIPT)'
	adb shell "chmod +x '$(PORTS_DIR)/$(PORT_SCRIPT)' && killall -9 zimlite 2>/dev/null; true"
	@echo "=== Deployed ==="

dist-portmaster: dist-arm64
	@rm -rf dist/portmaster_build
	@mkdir -p dist/portmaster_build/zimlite/lib
	cp "portmaster/Zimlite.sh" dist/portmaster_build/
	cp "portmaster/port.json" dist/portmaster_build/
	cp "portmaster/screenshot.png" dist/portmaster_build/
	cp "portmaster/screenshot.png" dist/portmaster_build/zimlite/cover.png
	cp "portmaster/README.md" dist/portmaster_build/zimlite/
	cp -r "portmaster/licenses" dist/portmaster_build/zimlite/
	cp dist/zimlite/zimlite dist/portmaster_build/zimlite/
	cp dist/zimlite/lib/* dist/portmaster_build/zimlite/lib/
	@RELEASE_DATE=$$(date +%Y%m%d)T000000; \
	printf '<gameList>\n    <game>\n        <path>./Zimlite.sh</path>\n        <name>Zimlite</name>\n        <desc>Zimlite is a lightweight offline reader for ZIM archives, the format used by Kiwix and Wikipedia. Browse articles, search content, and read without an internet connection on your handheld device.</desc>\n        <image>./zimlite/cover.png</image>\n        <developer>dendec</developer>\n        <publisher>dendec</publisher>\n        <releasedate>%s</releasedate>\n        <genre>Reference</genre>\n    </game>\n</gameList>\n' "$$RELEASE_DATE" > dist/portmaster_build/zimlite/gameinfo.xml
	cd dist/portmaster_build && zip -r ../zimlite.zip "Zimlite.sh" port.json screenshot.png zimlite
	@echo "=== Generated dist/zimlite.zip ==="
	@ls -lh dist/zimlite.zip

deploy-portmaster: dist-portmaster
	adb push dist/zimlite.zip $(PM_AUTOINSTALL)/
	@echo "=== Zip deployed to autoinstall ==="

