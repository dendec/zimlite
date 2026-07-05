.PHONY: build test vet clean run

APP     := kiwix-sdl
SRC     := ./cmd/kiwix-sdl
GO      := go
GOFLAGS := CGO_ENABLED=1

build:
	$(GOFLAGS) $(GO) build -o $(APP) $(SRC)

test:
	$(GO) test ./...

vet:
	$(GOFLAGS) $(GO) vet ./...

clean:
	rm -f $(APP)

run: build
	./$(APP) /tmp/test.md
