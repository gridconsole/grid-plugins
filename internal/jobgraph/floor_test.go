package jobgraph

import (
	"github.com/gridconsole/grid-plugins/internal/job"
	"strings"
	"testing"
)

func TestReviewFloor(t *testing.T) {
	policy := Policy{EnvironmentPaths: []string{"infra/**", ".github/workflows/**"}, RiskPaths: []string{"src/auth/**", "other:transport/**"}, GeneratedPaths: []string{"generated/**"}, TaskPaths: []string{"task/**"}, UIPaths: []string{"web/**"}}
	for _, tc := range []struct {
		name              string
		write, risk       []string
		kind, visible     string
		irreversible      bool
		floor             string
		classify, capture bool
	}{
		{"ordinary", []string{"src/service.go"}, nil, "", "", false, "L1", false, false},
		{"empty scope", nil, nil, "", "", false, "L1", false, false},
		{"environment first", []string{"infra/**"}, []string{"security"}, "generated", "big", false, "L2", true, false},
		{"irreversible", []string{"docs/readme.md"}, nil, "", "", true, "L2", true, false},
		{"risk", []string{"tests/**"}, []string{"concurrency"}, "", "", false, "L2", false, false},
		{"risk overlap", []string{"src/**"}, nil, "", "", false, "L2", false, false},
		{"risk exact", []string{"src/auth/session.go"}, nil, "", "", false, "L2", false, false},
		{"other repo", []string{"other:transport/client.go"}, nil, "", "", false, "L2", false, false},
		{"primary repo only", []string{"other:infra/config.yaml"}, nil, "", "", false, "L1", false, false},
		{"visible", []string{"docs/readme.md"}, nil, "", "big", false, "L1", false, true},
		{"UI before tests", []string{"app/pages/page.test.ts"}, nil, "", "", false, "L1", false, true},
		{"UI extension", []string{"src/widget.tsx"}, nil, "", "", false, "L1", false, true},
		{"UI policy", []string{"web/index.html"}, nil, "", "", false, "L1", false, true},
		{"docs", []string{"docs/**", "README.md"}, nil, "", "", false, "L0", false, false},
		{"tests", []string{"internal/jobgraph/*_test.go", "tests/**"}, nil, "", "", false, "L0", false, false},
		{"generated tag", []string{"generated/**/*.go"}, nil, "", "", false, "L0", false, false},
		{"generated kind", []string{"api/client.go"}, nil, "generated", "", false, "L0", false, false},
		{"rename", []string{"src/renamed.go"}, nil, "rename", "", false, "L0", false, false},
		{"mixed docs and code", []string{"docs/**", "src/service.go"}, nil, "", "", false, "L1", false, false},
		{"research task only", []string{"task/research.txt"}, nil, "research", "", false, "L0", false, false},
		{"scout mixed", []string{"task/notes.txt", "src/service.go"}, nil, "scout", "", false, "L1", false, false},
		{"glob can touch environment", []string{"*/docs/**", "src/service.go"}, nil, "", "", false, "L2", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item := job.Job{ID: "J-1", Write: tc.write, Risk: tc.risk, Kind: tc.kind, Visible: tc.visible, Irreversible: tc.irreversible, Review: "L3", Verify: job.Verify{Command: "go test ./...", MinTests: 1}}
			got, err := ReviewFloor(item, policy)
			if err != nil || got.Floor != tc.floor || got.Classify != tc.classify || got.CaptureAtL3 != tc.capture || len(got.Errors) > 0 {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	}
}

func TestReviewFloorCaseAndPrimaryRepo(t *testing.T) {
	policy := Policy{Repo: "main", EnvironmentPaths: []string{"infra/**"}, RiskPaths: []string{"src/auth/**", "main:src/billing/**", "other:transport/**"}, UIPaths: []string{"web/**"}, GeneratedPaths: []string{"generated/**"}, TaskPaths: []string{"task/**"}}
	for _, tc := range []struct {
		name         string
		write        []string
		kind         string
		floor        string
		classify, ui bool
	}{
		{"risk path in other case", []string{"Src/Auth/x.go"}, "", "L2", false, false},
		{"environment path in other case", []string{"INFRA/a.yaml"}, "", "L2", true, false},
		{"UI tag in other case", []string{"Web/index.html"}, "", "L1", false, true},
		{"built-in UI directory in other case", []string{"app/Pages/page.test.ts"}, "", "L1", false, true},
		{"built-in UI extension in other case", []string{"src/Widget.TSX"}, "", "L1", false, true},
		{"glob in other case", []string{"SRC/**"}, "", "L2", false, false},
		{"primary prefix on an unprefixed tag", []string{"main:src/auth/x.go"}, "", "L2", false, false},
		{"primary prefix in other case", []string{"MAIN:src/auth/x.go"}, "", "L2", false, false},
		{"unprefixed scope on a primary-prefixed tag", []string{"src/billing/x.go"}, "", "L2", false, false},
		{"primary prefix on an environment tag", []string{"main:infra/a.yaml"}, "", "L2", true, false},
		{"other repo stays apart", []string{"other:src/auth/x.go"}, "", "L1", false, false},
		{"other repo with its own tag", []string{"OTHER:transport/x.go"}, "", "L2", false, false},
		{"docs in other case do not lower", []string{"Docs/guide.txt"}, "", "L1", false, false},
		{"generated in other case does not lower", []string{"Generated/x.go"}, "", "L1", false, false},
		{"task path in other case does not lower", []string{"Task/notes.txt"}, "research", "L1", false, false},
		{"generated under the primary prefix lowers", []string{"main:generated/x.go"}, "", "L0", false, false},
		{"generated under another case of the prefix does not", []string{"MAIN:generated/x.go"}, "", "L1", false, false},
		{"task path under the primary prefix lowers", []string{"main:task/notes.txt"}, "research", "L0", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item := job.Job{ID: "J-1", Write: tc.write, Kind: tc.kind, Review: "L3", Verify: job.Verify{Command: "go test ./...", MinTests: 1}}
			got, err := ReviewFloor(item, policy)
			if err != nil || got.Floor != tc.floor || got.Classify != tc.classify || got.CaptureAtL3 != tc.ui || len(got.Errors) > 0 {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	}
	// Without a primary repo name every prefix is another repo, as before.
	item := job.Job{ID: "J-1", Write: []string{"main:src/auth/x.go"}, Review: "L3", Verify: job.Verify{Command: "go test ./...", MinTests: 1}}
	if got, err := ReviewFloor(item, Policy{RiskPaths: []string{"src/auth/**"}}); err != nil || got.Floor != "L1" {
		t.Fatalf("got %+v, %v", got, err)
	}
	for _, repo := range []string{"a:b", "a/b", "a*", "a\\b"} {
		if _, err := ReviewFloor(item, Policy{Repo: repo}); err == nil {
			t.Fatalf("accepted policy repo %q", repo)
		}
	}
}

func TestFloorRefusalsAndOverride(t *testing.T) {
	for _, tc := range []struct {
		name, review, reason string
		wantErrors           int
		override             bool
	}{
		{"missing", "", "", 0, false}, {"invalid", "L4", "", 1, false}, {"too low", "L0", "", 1, false},
		{"blank reason", "L0", "  ", 1, false}, {"recorded exception", "L0", "Explicit exception for this small change", 0, true},
		{"floor", "L2", "", 0, false}, {"higher", "L3", "", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ReviewFloor(job.Job{ID: "J-1", Risk: []string{"security"}, Review: tc.review, ReviewReason: tc.reason, Verify: job.Verify{Command: "make test", MinTests: 2}}, Policy{})
			if err != nil || len(got.Errors) != tc.wantErrors || got.Override != tc.override || got.Floor != "L2" {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	}
	for _, item := range []job.Job{{Risk: []string{"unknown"}}, {Kind: "unknown"}, {Visible: "unknown"}, {Write: []string{"src/["}}} {
		if _, err := ReviewFloor(item, Policy{}); err == nil {
			t.Fatalf("accepted %+v", item)
		}
	}
	if _, err := ReviewFloor(job.Job{}, Policy{RiskPaths: []string{"../secret"}}); err == nil {
		t.Fatal("accepted invalid policy")
	}
}

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
