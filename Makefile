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

# Replays the legacy corpus in-process and prints every case plus totals; make test runs it too.
golden:
	go test -count=1 -v ./internal/golden -run '^TestGoldenCorpus$$'

clean:
	rm -rf dist
