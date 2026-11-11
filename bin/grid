#!/bin/sh
# Generated as bin/grid by make generate; edit internal/packaging/grid.sh.
set -eu

# Resolve shell links without readlink -f (which is absent on older macOS).
self=$0
case $self in
*/*) ;;
*) self=$(command -v "$self") ;;
esac
links=0
while [ -L "$self" ]; do
	links=$((links + 1))
	if [ "$links" -gt 40 ]; then
		echo 'grid: launcher symlink loop' >&2
		exit 1
	fi
	link=$(readlink "$self")
	case $link in
	/*) self=$link ;;
	*) self=$(dirname "$self")/$link ;;
	esac
done
root=$(CDPATH= cd -P "$(dirname "$self")/.." && pwd -P)
os=$(uname -s)
arch=$(uname -m)
case $os in
Darwin) platform=darwin ;;
Linux) platform=linux ;;
MINGW*|MSYS*|CYGWIN*)
	# Git Bash, MSYS2 or Cygwin on Windows: run the shipped Windows binary when this tree has one.
	if [ -f "$root/bin/grid-windows-amd64.exe" ]; then
		exec "$root/bin/grid-windows-amd64.exe" "$@"
	fi
	echo "grid: unsupported operating system: $os (supported: Darwin, Linux, Windows with a release binary)" >&2; exit 1 ;;
*) echo "grid: unsupported operating system: $os (supported: Darwin, Linux)" >&2; exit 1 ;;
esac
case $arch in
arm64|aarch64) platform=$platform-arm64 ;;
x86_64|amd64) platform=$platform-amd64 ;;
*) echo "grid: unsupported architecture: $arch on $os (supported: arm64, amd64)" >&2; exit 1 ;;
esac
binary="$root/bin/grid-$platform"
if [ ! -x "$binary" ] && [ -f "$root/go.mod" ]; then
	# Source checkout: use a fresh dist build, else build natively as the old dev launcher did.
	stale() {
		[ ! -x "$1" ] || [ -n "$(find "$root" -path "$root/dist" -prune -o \( -name '*.go' -o -name go.mod -o -name go.sum \) -newer "$1" -print -quit)" ]
	}
	binary="$root/dist/plugin/bin/grid-$platform"
	if stale "$binary"; then
		binary="$root/dist/grid"
		if stale "$binary"; then
			if ! command -v go >/dev/null 2>&1 || ! mkdir -p "$root/dist" || ! go build -C "$root" -trimpath -buildvcs=false -o "$binary" ./cmd/grid </dev/null >&2; then
				# A hook never fails the session; other commands say why.
				[ "${1:-}" = hook ] && exit 0
				echo 'grid: cannot build grid from this source checkout (needs go on PATH)' >&2
				exit 1
			fi
		fi
	fi
fi
if [ ! -x "$binary" ]; then
	echo "grid: missing executable for $platform; install a complete release or run make dist in the source checkout" >&2
	exit 1
fi
exec "$binary" "$@"
