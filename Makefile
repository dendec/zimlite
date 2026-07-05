APP      := kiwix-sdl
SRC      := ./cmd/kiwix-sdl
GO       := go
GOFLAGS  := CGO_ENABLED=1

ZIM_VER  := 9.7.0
ZIM_URL  := https://download.openzim.org/release/libzim

# Default: use system-installed libzim (dev).
CGO_CXXFLAGS := -std=c++17 -Iinternal/zim -I/usr/include
CGO_LDFLAGS  := -L/usr/lib/x86_64-linux-gnu -lzim

.PHONY: build test vet clean run info
.PHONY: deps deps-all build-linux-arm64 build-linux-armv8 build-linux-amd64

build:
	$(GOFLAGS) CGO_CXXFLAGS="$(CGO_CXXFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS)" \
		$(GO) build -o $(APP) $(SRC)

# Cross-builds require downloaded libzim (make deps first).
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

deps:
	@mkdir -p lib
	@for tag in x86_64 aarch64 armv8; do \
		dir=lib/libzim_linux-$$tag-$(ZIM_VER); \
		if [ ! -f $$dir/include/zim/archive.h ]; then \
			echo "Downloading libzim $$tag..."; \
			wget -q "$(ZIM_URL)/libzim_linux-$$tag-$(ZIM_VER).tar.gz" -O /tmp/libzim-$$tag.tar.gz; \
			tar xzf /tmp/libzim-$$tag.tar.gz -C lib/; \
			rm -f /tmp/libzim-$$tag.tar.gz; \
		fi; \
	done

deps-all: deps

test:
	$(GO) test ./...

vet:
	$(GOFLAGS) $(GO) vet ./...

clean:
	rm -f $(APP) $(APP)-*

run: build
	./$(APP) /tmp/test.md

info:
	@echo "CGO_CXXFLAGS=$(CGO_CXXFLAGS)"
	@echo "CGO_LDFLAGS=$(CGO_LDFLAGS)"
