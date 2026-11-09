package guard

import (
	"path/filepath"
	"strings"
	"testing"
)

// Rows for the L135R3 rework: once a case word is seen inside a $( ) or <( )
// group no ) closes it, whatever esac, comment, heredoc or terminator follows,
// and a backslash-newline never hides the word case.
func TestHostWriteTargetsCaseMonotonic(t *testing.T) {
	f := setup(t)
	project := filepath.Join(f.projects, "example")
	policy := filepath.Join(project, "workflows.md")
	f.write(policy, "policy")
	f.link(filepath.Join(project, "alias"), policy)
	target := filepath.Join(project, "alias")

	// Each hidden write is tried from the project folder (alias) and from the
	// repo folder (the absolute path of the alias).
	hidden := []struct{ name, command string }{
		{"esac_argument", "echo $(case x in y) echo esac;; x) tee ALIAS </dev/null;; esac)"},
		{"esac_comment", "echo $(case x in # esac\nx) tee ALIAS </dev/null;; esac)"},
		{"esac_heredoc", "echo $(case x in y) cat <<Q\nesac\nQ\n;; x) tee ALIAS </dev/null;; esac)"},
		{"esac_fallthrough_terminator", "echo $(case x in y) echo esac;& x) tee ALIAS </dev/null;; esac)"},
		{"esac_process", "cat <(case x in y) echo esac;; x) tee ALIAS </dev/null;; esac)"},
		{"esac_subject", "echo $(case esac in x) tee ALIAS </dev/null;; esac)"},
		{"esac_quoted_group", `echo "$(case x in y) echo esac;; x) tee ALIAS </dev/null;; esac)"`},
		{"esac_nested", "echo $(case x in y) case z in z) echo esac;; esac;; x) tee ALIAS </dev/null;; esac)"},
		{"continuation_after_case", "echo $(case\\\n x in x) tee ALIAS </dev/null;; esac)"},
		{"continuation_inside_case", "echo $(ca\\\nse x in x) tee ALIAS </dev/null;; esac)"},
		{"continuation_process", "cat <(ca\\\nse x in x) tee ALIAS </dev/null;; esac)"},
		{"case_before_quote", `echo $(case"x" in x) tee ALIAS </dev/null;; esac)`},
		{"case_before_paren", "echo $(case(x) in x) tee ALIAS </dev/null;; esac)"},
		// An escaped backslash does not join the next line.
		{"escaped_backslash_newline", "echo x\\\\\ntee ALIAS </dev/null"},
	}
	type row struct {
		name, cwd, command string
		deny               bool
	}
	var rows []row
	for _, h := range hidden {
		rows = append(rows,
			row{h.name, project, strings.ReplaceAll(h.command, "ALIAS", "alias"), true},
			row{h.name + "_absolute", f.repo, strings.ReplaceAll(h.command, "ALIAS", target), true})
	}
	rows = append(rows,
		// Words after a substitution that holds case are scanned as commands: the
		// group stays open to the end of the text, so this is refused.
		row{"case_subst_words_after_policy", project, "echo $(case x in x) echo a;; esac) foo workflows.md", true},
		row{"case_subst_words_after_other", project, "echo $(case x in x) echo a;; esac) foo notes.md", false},
		// Plain case scripts keep running.
		row{"plain_patterns", f.repo, "case $1 in a) echo a;; b|c) echo b;; *) echo other;; esac", false},
		row{"plain_fallthrough", f.repo, "case $1 in a) echo a;& b) echo b;; esac", false},
		row{"plain_in_substitution", f.repo, "echo $(case $1 in a) echo a;; esac)", false},
		row{"plain_in_loop", f.repo, "for f in a b; do case $f in a) echo a;; esac; done", false},
		row{"plain_esac_argument", project, "case $1 in a) echo esac;; esac", false},
		row{"plain_in_then", project, "if true; then case $1 in a) echo a;; esac; fi", false},
		row{"plain_continuation", f.repo, "case\\\n $1 in a) echo a;; esac", false},
	)
	for _, tt := range rows {
		t.Run(tt.name, func(t *testing.T) {
			in := f.input("Bash", tt.command)
			in.Cwd = tt.cwd
			got := EvaluateHost(in, f.root, f.env)
			want := ""
			if tt.deny {
				want = "deny"
			}
			if got.Behavior != want {
				t.Fatalf("behavior = %q, want %q for %q", got.Behavior, want, tt.command)
			}
			if tt.deny && got.Reason != gateReason {
				t.Fatal("write did not use the direct protected-write reason")
			}
		})
	}
}
