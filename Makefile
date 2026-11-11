PKG     := github.com/gridconsole/grid-plugins
VERSION ?= $(shell go run ./internal/packaging/cmd -describe)
# What make dist stamps into the manifests and the binaries: 0.1.<commit count>, so claude plugin update sees it.
RELEASE ?= $(shell go run ./internal/packaging/cmd -release)
LDFLAGS := -buildid= -X $(PKG)/internal/version.Version=$(VERSION)

.PHONY: build dist generate test test-clean lint cross golden eval-check clean

build:
	CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags "$(LDFLAGS)" -o dist/grid ./cmd/grid

dist:
	go run ./internal/packaging/cmd -dist -version "$(RELEASE)"

generate:
	go run ./internal/packaging/cmd
	UPDATE_GOLDEN=1 go test ./internal/cli -run '^TestDoctorGolden$$' -count=1
	UPDATE_CONTRACT_COMMANDS=1 go test ./internal/cli -run '^TestContractCommandsParserInventory$$' -count=1

# Pre-push set: make test lint eval-check (eval-check stays out of test to keep it fast).
test: test-clean

# Keep the caller's default TMPDIR and compare entries around an uncached run.
test-clean:
	go run ./internal/testclean

lint:
	go vet ./...
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

# Vets and compiles every package and test for the release OSes, so a per-OS file that breaks is found here and not on a Mac or a Windows machine.
# -exec /bin/true compiles each test binary and does not run it (the binaries are not for the VM's OS).
cross:
	for target in darwin/arm64 windows/amd64; do \
		os="$${target%/*}"; arch="$${target#*/}"; \
		echo "cross: $$os/$$arch vet"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go vet ./... || exit 1; \
		echo "cross: $$os/$$arch test compile"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go test -run XXX_NONE -count=1 -exec /bin/true ./... || exit 1; \
	done

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
