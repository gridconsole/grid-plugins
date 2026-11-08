package jobgraph

import (
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
			item := Item{ID: "J-1", Write: tc.write, Risk: tc.risk, Kind: tc.kind, Visible: tc.visible, Irreversible: tc.irreversible, Review: "L3", Verify: Verify{Command: "go test ./...", MinTests: 1}}
			got, err := ReviewFloor(item, policy)
			if err != nil || got.Floor != tc.floor || got.Classify != tc.classify || got.CaptureAtL3 != tc.capture || len(got.Errors) > 0 {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	}
}

func TestFloorRefusalsAndOverride(t *testing.T) {
	for _, tc := range []struct {
		name, review, reason string
		wantErrors           int
		override             bool
	}{
		{"missing", "", "", 1, false}, {"invalid", "L4", "", 1, false}, {"too low", "L0", "", 1, false},
		{"blank reason", "L0", "  ", 1, false}, {"recorded exception", "L0", "Explicit exception for this small change", 0, true},
		{"floor", "L2", "", 0, false}, {"higher", "L3", "", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ReviewFloor(Item{ID: "J-1", Risk: []string{"security"}, Review: tc.review, ReviewReason: tc.reason, Verify: Verify{Command: "make test", MinTests: 2}}, Policy{})
			if err != nil || len(got.Errors) != tc.wantErrors || got.Override != tc.override || got.Floor != "L2" {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	}
	for _, item := range []Item{{Risk: []string{"unknown"}}, {Kind: "unknown"}, {Visible: "unknown"}, {Write: []string{"src/["}}} {
		if _, err := ReviewFloor(item, Policy{}); err == nil {
			t.Fatalf("accepted %+v", item)
		}
	}
	if _, err := ReviewFloor(Item{}, Policy{RiskPaths: []string{"../secret"}}); err == nil {
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
			err := ValidateVerify(Item{Verify: Verify{Command: tc.command, MinTests: 1}})
			if (err == nil) != tc.valid {
				t.Fatalf("got %v, valid %v", err, tc.valid)
			}
		})
	}
	for _, n := range []int{0, -1} {
		if err := ValidateVerify(Item{Verify: Verify{Command: "go test ./...", MinTests: n}}); err == nil || !strings.Contains(err.Error(), "min_tests") {
			t.Fatalf("got %v", err)
		}
	}
	for _, tc := range []struct {
		kind   string
		verify Verify
		valid  bool
	}{
		{"non-code", Verify{Kind: "file-exists"}, true}, {"research", Verify{Kind: "reviewer-check"}, true},
		{"code", Verify{Kind: "file-exists"}, false}, {"non-code", Verify{Kind: "unknown"}, false},
		{"non-code", Verify{Kind: "file-exists", Command: "go test", MinTests: 1}, false},
	} {
		if err := ValidateVerify(Item{Kind: tc.kind, Verify: tc.verify}); (err == nil) != tc.valid {
			t.Fatalf("%+v: %v", tc, err)
		}
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
