APP      := kiwix-sdl
SRC      := ./cmd/kiwix-sdl
GO       := go
GOFLAGS  := CGO_ENABLED=1

ZIM_VER  := 9.7.0
ZIM_URL  := https://download.openzim.org/release/libzim

# ---- Host arch detection → download tag ----
HOST_ARCH := $(shell uname -m)
ifeq ($(HOST_ARCH),x86_64)
  ZIM_TAG        := x86_64
  ZIM_LIB_SUBDIR := x86_64-linux-gnu
else ifeq ($(HOST_ARCH),aarch64)
  ZIM_TAG        := aarch64
  ZIM_LIB_SUBDIR := aarch64-linux-gnu
else ifneq ($(findstring armv,$(HOST_ARCH)),)
  ZIM_TAG        := armv8
  ZIM_LIB_SUBDIR := armv8-linux-gnueabihf
else
  ZIM_TAG        := x86_64
  ZIM_LIB_SUBDIR := x86_64-linux-gnu
endif

ZIM_DIR     := lib/libzim_linux-$(ZIM_TAG)-$(ZIM_VER)
ZIM_INCLUDE := $(ZIM_DIR)/include
ZIM_LIB     := $(ZIM_DIR)/lib/$(ZIM_LIB_SUBDIR)

CGO_CXXFLAGS := -std=c++17 -Iinternal/zim -I$(ZIM_INCLUDE)
CGO_LDFLAGS  := -L$(ZIM_LIB) -lzim -Wl,-rpath,'$$ORIGIN/$(ZIM_LIB)'

.PHONY: build test vet clean run info
.PHONY: deps deps-all build-linux-arm64 build-linux-armv8 build-linux-amd64

# ---- Default build for host platform ----
build: deps
	$(GOFLAGS) CGO_CXXFLAGS="$(CGO_CXXFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS)" \
		$(GO) build -o $(APP) $(SRC)

# ---- Cross-builds ----
build-linux-amd64:
	$(MAKE) _cross TAG=x86_64       LIBSUB=x86_64-linux-gnu       GOARCH=amd64          CC=x86_64-linux-gnu-gcc      CXX=x86_64-linux-gnu-g++

build-linux-arm64:
	$(MAKE) _cross TAG=aarch64      LIBSUB=aarch64-linux-gnu      GOARCH=arm64          CC=aarch64-linux-gnu-gcc     CXX=aarch64-linux-gnu-g++

build-linux-armv8:
	$(MAKE) _cross TAG=armv8        LIBSUB=armv8-linux-gnueabihf  GOARCH=arm GOARM=7    CC=arm-linux-gnueabihf-gcc   CXX=arm-linux-gnueabihf-g++

_cross:
	@test -n "$(CC)" || (echo "No cross-compiler. Install gcc-aarch64-linux-gnu or gcc-arm-linux-gnueabihf"; exit 1)
	@$(MAKE) deps ZIM_TAG=$(TAG) ZIM_LIB_SUBDIR=$(LIBSUB)
	$(GOFLAGS) CGO_ENABLED=1 GOOS=linux GOARCH=$(GOARCH) GOARM=$(GOARM) \
		CC=$(CC) CXX=$(CXX) \
		CGO_CXXFLAGS="-std=c++17 -Iinternal/zim -Ilib/libzim_linux-$(TAG)-$(ZIM_VER)/include" \
		CGO_LDFLAGS="-Llib/libzim_linux-$(TAG)-$(ZIM_VER)/lib/$(LIBSUB) -lzim" \
		$(GO) build -o $(APP)-$(GOARCH) $(SRC)

# ---- Dependencies ----
deps: $(ZIM_LIB)/libzim.so

$(ZIM_LIB)/libzim.so:
	@echo "Downloading libzim linux-$(ZIM_TAG) $(ZIM_VER)..."
	@mkdir -p $(ZIM_DIR)
	wget -q "$(ZIM_URL)/libzim_linux-$(ZIM_TAG)-$(ZIM_VER).tar.gz" -O /tmp/libzim.tar.gz
	tar xzf /tmp/libzim.tar.gz -C lib/
	@rm -f /tmp/libzim.tar.gz

deps-all:
	@$(MAKE) deps ZIM_TAG=x86_64  ZIM_LIB_SUBDIR=x86_64-linux-gnu
	@$(MAKE) deps ZIM_TAG=aarch64 ZIM_LIB_SUBDIR=aarch64-linux-gnu
	@$(MAKE) deps ZIM_TAG=armv8   ZIM_LIB_SUBDIR=armv8-linux-gnueabihf

# ---- Other ----
test:
	$(GO) test ./...

vet:
	$(GOFLAGS) $(GO) vet ./...

clean:
	rm -f $(APP) $(APP)-*

run: build
	LD_LIBRARY_PATH=$(ZIM_LIB):$$LD_LIBRARY_PATH ./$(APP) /tmp/test.md

info:
	@echo "Host:    $(HOST_ARCH)"
	@echo "ZIM tag: $(ZIM_TAG)  libsub: $(ZIM_LIB_SUBDIR)"
	@echo "Lib dir: $(ZIM_LIB)"
