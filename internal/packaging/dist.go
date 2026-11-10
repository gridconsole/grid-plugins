package packaging

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Targets are the release platforms; every one ships a prebuilt binary under bin/.
var targets = [][2]string{{"darwin", "arm64"}, {"darwin", "amd64"}, {"linux", "amd64"}, {"linux", "arm64"}, {"windows", "amd64"}}

// binaryName is the file the launchers look for in bin/: grid-<os>-<arch>, with .exe on Windows.
func binaryName(goos, goarch string) string {
	name := "grid-" + goos + "-" + goarch
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

var releaseVersion = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*$`)

// Build writes a complete offline plugin under dist/plugin and its SHA256SUMS.
// The caller supplies git describe's version. Build never changes source metadata.
func Build(root, version string, log io.Writer) error {
	if !releaseVersion.MatchString(version) {
		return fmt.Errorf("invalid release version %q", version)
	}
	dist := filepath.Join(root, "dist")
	plugin := filepath.Join(dist, "plugin")
	if err := os.MkdirAll(dist, 0o755); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(dist, "SHA256SUMS")); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.RemoveAll(plugin); err != nil {
		return err
	}
	if err := stage(root, plugin, version); err != nil {
		return err
	}
	for _, target := range targets {
		if err := buildBinary(root, filepath.Join(plugin, "bin", binaryName(target[0], target[1])), version, target[0], target[1], log); err != nil {
			return err
		}
	}
	// The Windows binary above covers cmd/grid; this compiles every other package for Windows too.
	if err := buildCheck(root, "windows", log); err != nil {
		return err
	}
	return writeChecksums(dist, plugin)
}

// shippedDirs are the only source folders copied into the public plugin. Evals,
// the legacy corpus, recordings, fixtures and golden cases stay in the source
// repo and are never listed here.
var shippedDirs = []string{"skills", "agents", "agent-templates"}

// stage writes everything in the plugin except the binaries.
func stage(root, plugin, version string) error {
	if err := Write(plugin, version); err != nil {
		return err
	}
	for _, dir := range shippedDirs {
		if err := copyTree(filepath.Join(root, dir), filepath.Join(plugin, dir)); err != nil {
			return err
		}
	}
	license, err := os.ReadFile(filepath.Join(root, "LICENSE"))
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(plugin, "LICENSE"), license, 0o644)
}

func buildBinary(root, output, version, goos, goarch string, log io.Writer) error {
	fmt.Fprintf(log, "Building grid %s for %s/%s\n", version, goos, goarch)
	cmd := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-ldflags",
		"-buildid= -X github.com/gridconsole/grid-plugins/internal/version.Version="+version,
		"-o", output, "./cmd/grid")
	cmd.Dir, cmd.Env = root, buildEnv(goos, goarch)
	cmd.Stdout, cmd.Stderr = log, log
	return cmd.Run()
}

func buildCheck(root, goos string, log io.Writer) error {
	cmd := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-buildid=", "./...")
	cmd.Dir, cmd.Env = root, buildEnv(goos, "amd64")
	cmd.Stdout, cmd.Stderr = log, log
	return cmd.Run()
}

func buildEnv(goos, goarch string) []string {
	// Pin architecture baselines and prevent developer flags/workspaces from
	// changing release bytes. The Go toolchain version must also match.
	overrides := map[string]string{
		"GOOS": goos, "GOARCH": goarch, "CGO_ENABLED": "0",
		"GOAMD64": "v1", "GOARM64": "v8.0", "GOEXPERIMENT": "",
		"GOFLAGS": "", "GOWORK": "off", "GOENV": "off",
	}
	var env []string
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if _, replaced := overrides[key]; !replaced {
			env = append(env, entry)
		}
	}
	for key, value := range overrides {
		env = append(env, key+"="+value)
	}
	return env
}

func copyTree(source, dest string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		to := filepath.Join(dest, rel)
		if entry.IsDir() {
			return os.MkdirAll(to, 0o755)
		}
		if entry.Name() == ".gitkeep" {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("package source %s must be a regular file", rel)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		mode := fs.FileMode(0o644)
		if info.Mode()&0o111 != 0 {
			mode = 0o755
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(to, b, mode)
	})
}

func writeChecksums(dist, plugin string) error {
	var paths []string
	if err := filepath.WalkDir(plugin, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() {
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		return err
	}
	sort.Strings(paths)
	var sums strings.Builder
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		h := sha256.New()
		_, err = io.Copy(h, f)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		rel, err := filepath.Rel(dist, path)
		if err != nil {
			return err
		}
		fmt.Fprintf(&sums, "%x  %s\n", h.Sum(nil), filepath.ToSlash(rel))
	}
	return os.WriteFile(filepath.Join(dist, "SHA256SUMS"), []byte(sums.String()), 0o644)
}
