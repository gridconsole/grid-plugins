package jobgraph

import (
	"github.com/gridconsole/grid-plugins/internal/job"
	"strings"
	"testing"
)

func TestVerifyCommands(t *testing.T) {
	for _, tc := range []struct {
		command string
		valid   bool
	}{
		{"go test ./...", true}, {"make test", true}, {"go build ./... && go test ./...", true},
		{"node --test tests/*.test.mjs", true}, {"npm run test:unit", true}, {"npm test", true},
		{"python3 -m pytest", true}, {"pytest tests", true}, {"cargo test", true}, {"dotnet test", true},
		{"./scripts/run-tests.sh", true}, {"node 'tests/example.test.mjs'", true}, {"env CI=1 go test ./...", true},
		{"npx vitest run", true}, {"python3 -m unittest", true}, {"make check", false},
		{"go build ./...", false}, {"go build ./tests", false}, {"go vet ./...", false}, {"true", false},
		{"echo test", false}, {"printf 'go test'", false}, {"echo 'hello; go test ./...'", false},
		{"echo \"hello && go test\"", false}, {"go build ./... # go test ./...", false},
		{"node -e 'console.log(\"test\")'", false}, {"test -f tests/foo", false}, {"npm run latest", false},
		{"make contest", false}, {"make build TEST_NAME=test", false}, {"npm run build --description test", false},
		{"make \"test", false}, {"", false},
	} {
		t.Run(tc.command, func(t *testing.T) {
			err := ValidateVerify(job.Job{Verify: job.Verify{Command: tc.command, MinTests: 1}})
			if (err == nil) != tc.valid {
				t.Fatalf("got %v, valid %v", err, tc.valid)
			}
		})
	}
	for _, n := range []int{0, -1} {
		if err := ValidateVerify(job.Job{Verify: job.Verify{Command: "go test ./...", MinTests: n}}); err == nil || !strings.Contains(err.Error(), "min_tests") {
			t.Fatalf("got %v", err)
		}
	}
	for _, tc := range []struct {
		kind   string
		verify job.Verify
		valid  bool
	}{
		{"non-code", job.Verify{Kind: "file-exists"}, true}, {"research", job.Verify{Kind: "reviewer-check"}, true},
		{"code", job.Verify{Kind: "file-exists"}, false}, {"non-code", job.Verify{Kind: "unknown"}, false},
		{"non-code", job.Verify{Kind: "file-exists", Command: "go test", MinTests: 1}, false},
	} {
		if err := ValidateVerify(job.Job{Kind: tc.kind, Verify: tc.verify}); (err == nil) != tc.valid {
			t.Fatalf("%+v: %v", tc, err)
		}
	}
}

// The strict test stays exact for lowering; the fold test is the raising one.
func TestScopeOverlapFold(t *testing.T) {
	for _, tc := range []struct {
		a, b         string
		strict, fold bool
	}{
		{"src/auth/**", "Src/Auth/x.go", false, true},
		{"infra/**", "INFRA/a.yaml", false, true},
		{"src/auth/**", "src/auth/x.go", true, true},
		{"repo:src/**", "Repo:SRC/x.go", false, true},
		{"repo:src/**", "other:src/x.go", false, false},
		{"src/*.go", "src/*.md", false, false},
		{"a/[A-C].go", "a/b.go", false, true},
		{"a/[^a-c].go", "a/b.go", false, true},
		{"a/[^a-c].go", "a/d.go", true, true},
		{"a/k.go", "a/\u212a.go", false, true},
		{"a/?.go", "a/B.go", true, true},
		{"**/Pages/**", "app/pages/view.ts", false, true},
	} {
		t.Run(tc.a+" vs "+tc.b, func(t *testing.T) {
			for _, p := range []string{tc.a, tc.b} {
				if err := validatePattern(p); err != nil {
					t.Fatal(err)
				}
			}
			for _, pair := range [][2]string{{tc.a, tc.b}, {tc.b, tc.a}} {
				if got := scopeOverlap(pair[0], pair[1]); got != tc.strict {
					t.Fatalf("strict %q vs %q: %v, want %v", pair[0], pair[1], got, tc.strict)
				}
				if got := overlap(pair[0], pair[1], true); got != tc.fold {
					t.Fatalf("fold %q vs %q: %v, want %v", pair[0], pair[1], got, tc.fold)
				}
			}
		})
	}
}

func TestScopeOverlap(t *testing.T) {
	for _, tc := range []struct {
		a, b    string
		overlap bool
	}{
		{"src/**", "src/auth/**", true}, {"src/*.go", "src/*_test.go", true}, {"src/*.go", "src/*.md", false},
		{"**/pages/**", "app/pages/view.ts", true}, {"**/pages/**", "pages/view.ts", true},
		{"src/**/auth/*", "src/auth/token.go", true}, {"src/*/auth/*", "src/auth/token.go", false},
		{"repo:src/**", "src/**", false}, {"repo:src/**", "repo:src/auth/*", true},
		{"**/*.go", "repo:src/main.go", false}, {"repo:**/*.go", "repo:src/main.go", true},
		{"a/[ab].go", "a/[bc].go", true}, {"a/[a-c].go", "a/[d-f].go", false},
		{"a/[^a-c].go", "a/b.go", false}, {"a/[^a-c].go", "a/d.go", true},
		{"a/?.go", "a/*.go", true}, {"a/?.go", "a/ab.go", false}, {"a/*x", "a/y*", true},
	} {
		t.Run(tc.a+" vs "+tc.b, func(t *testing.T) {
			for _, p := range []string{tc.a, tc.b} {
				if err := validatePattern(p); err != nil {
					t.Fatal(err)
				}
			}
			if got := scopeOverlap(tc.a, tc.b); got != tc.overlap {
				t.Fatalf("overlap %v, want %v", got, tc.overlap)
			}
			if scopeOverlap(tc.b, tc.a) != tc.overlap {
				t.Fatal("asymmetric overlap")
			}
		})
	}
	for _, pattern := range []string{"", "../src", "/src/**", `src\foo`, "src//foo", "src/a**", "src/[", "src/[]", "src/./foo", "repo:/src", ":src", "repo:", "*:src/**", "repo:src:bad"} {
		if err := validatePattern(pattern); err == nil {
			t.Fatalf("accepted %q", pattern)
		}
	}
}
