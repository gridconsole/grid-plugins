PKG     := github.com/gridconsole/grid-plugins
VERSION ?=
LDFLAGS := $(if $(VERSION),-X $(PKG)/internal/version.Version=$(VERSION))

.PHONY: build test lint golden clean

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o dist/grid ./cmd/grid

test:
	go test ./...

lint:
	go vet ./...
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

# Runs the golden-file tests; UPDATE_GOLDEN=1 make golden rewrites testdata/.
golden:
	go test ./... -run Golden

clean:
	rm -rf dist
