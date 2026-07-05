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
.PHONY: dist-arm64

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
dist-arm64:
	docker build -t kiwix-arm64 -f Dockerfile.arm64 .
	@mkdir -p dist
	docker create --name kiwix-extract kiwix-arm64 >/dev/null 2>&1
	docker cp kiwix-extract:/dist/kiwix-sdl dist/
	docker rm kiwix-extract >/dev/null 2>&1
	@echo "=== Done: dist/kiwix-sdl/ ==="
	@ls -lh dist/kiwix-sdl/
