PKG     := github.com/gridconsole/grid-plugins
VERSION ?= $(shell go run ./internal/packaging/cmd -describe)
LDFLAGS := -buildid= -X $(PKG)/internal/version.Version=$(VERSION)

.PHONY: build dist generate test test-clean lint golden clean

build:
	CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags "$(LDFLAGS)" -o dist/grid ./cmd/grid

dist:
	go run ./internal/packaging/cmd -dist -version "$(VERSION)"

generate:
	go run ./internal/packaging/cmd
	UPDATE_CONTRACT_COMMANDS=1 go test ./internal/cli -run '^TestContractCommandsParserInventory$$' -count=1

test: test-clean

# Keep the caller's default TMPDIR and compare entries around an uncached run.
test-clean:
	go run ./internal/testclean

lint:
	go vet ./...
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

# Replays the legacy corpus in-process and prints every case plus totals; make test runs it too.
golden:
	go test -count=1 -v ./internal/golden -run '^TestGoldenCorpus$$'

clean:
	rm -rf dist
