PKG     := github.com/gridconsole/grid-plugins
VERSION ?= $(shell go run ./internal/packaging/cmd -describe)
LDFLAGS := -buildid= -X $(PKG)/internal/version.Version=$(VERSION)

.PHONY: build dist generate test test-clean lint golden eval-check clean

build:
	CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags "$(LDFLAGS)" -o dist/grid ./cmd/grid

dist:
	go run ./internal/packaging/cmd -dist -version "$(VERSION)"

generate:
	go run ./internal/packaging/cmd
	UPDATE_CONTRACT_COMMANDS=1 go test ./internal/cli -run '^TestContractCommandsParserInventory$$' -count=1

# Pre-push set: make test lint eval-check (eval-check stays out of test to keep it fast).
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

# Parses every eval case and runs the sample tasks with the fake agent (results in the ignored .cache).
eval-check:
	go run ./cmd/grid eval list --json > /dev/null
	rm -rf .cache/eval-check
	for id in clean-csv fix-total release-notes; do \
		go run ./cmd/grid eval run $$id --results .cache/eval-check || exit 1; \
	done

clean:
	rm -rf dist
