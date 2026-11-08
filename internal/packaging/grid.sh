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
*) echo "grid: unsupported operating system: $os (supported: Darwin, Linux)" >&2; exit 1 ;;
esac
case $arch in
arm64|aarch64) platform=$platform-arm64 ;;
x86_64|amd64) platform=$platform-amd64 ;;
*) echo "grid: unsupported architecture: $arch on $os (supported: arm64, amd64)" >&2; exit 1 ;;
esac
binary="$root/bin/grid-$platform"
if [ ! -x "$binary" ] && [ -f "$root/go.mod" ]; then
	binary="$root/dist/plugin/bin/grid-$platform"
fi
if [ ! -x "$binary" ]; then
	echo "grid: missing executable for $platform; install a complete release or run make dist in the source checkout" >&2
	exit 1
fi
exec "$binary" "$@"
