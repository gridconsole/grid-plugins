package guard

import (
	"fmt"
	"path/filepath"
	"testing"
)

// Rows for the L135RR rework: a case pattern's ) does not close a $( ) or <( )
// group, and the zsh clobber operators are write operators.
func TestHostWriteTargetsCaseGroup(t *testing.T) {
	f := setup(t)
	project := filepath.Join(f.projects, "example")
	policy := filepath.Join(project, "workflows.md")
	f.write(policy, "policy")
	f.link(filepath.Join(project, "alias"), policy)
	target := filepath.Join(project, "alias")
	for _, tt := range []struct {
		name, cwd, command string
		deny               bool
	}{
		// The ) of a case pattern stays inside the group.
		{"case_subst", project, "echo $(case x in x) tee alias </dev/null;; esac)", true},
		{"case_process", project, "cat <(case x in x) tee alias </dev/null;; esac)", true},
		{"case_subst_absolute", f.repo, "echo $(case x in x) tee " + target + " </dev/null;; esac)", true},
		{"case_process_absolute", f.repo, "cat <(case x in x) tee " + target + " </dev/null;; esac)", true},
		{"case_subst_quoted", project, `echo "$(case x in x) tee alias </dev/null;; esac)"`, true},
		{"case_subst_parameter", project, "echo ${E:-$(case x in x) tee alias </dev/null;; esac)}", true},
		{"case_subst_nested", project, "echo $(case x in x) case y in y) tee alias </dev/null;; esac;; esac)", true},
		{"case_subst_second_pattern", project, "echo $(case x in a) echo a;; x) tee alias </dev/null;; esac)", true},
		{"case_subst_leading_paren", project, "echo $(case x in (x) tee alias </dev/null;; esac)", true},
		{"case_subst_after_command", project, "echo $(echo a; case x in x) tee alias </dev/null;; esac)", true},
		{"case_subst_clobber", project, "echo $(case x in x) echo y >|alias;; esac)", true},
		{"case_subst_then", project, "echo $(if true; then case x in x) tee alias </dev/null;; esac; fi)", true},
		{"case_subst_default_pattern", project, "echo $(case x in *) tee alias </dev/null;; esac)", true},
		{"case_subst_cd", f.repo, "echo $(cd " + project + " && case x in x) tee alias </dev/null;; esac)", true},
		// A group that does not finish is never allowed for that reason.
		{"case_subst_no_esac", project, "echo $(case x in x) tee alias </dev/null;;)", true},
		{"case_subst_no_close", project, "echo $(case x in x) tee alias </dev/null;;", true},
		{"case_process_no_close", project, "cat <(case x in x) tee alias </dev/null", true},
		{"case_subst_read", project, "echo $(case x in x) cat alias;; esac)", false},
		{"case_subst_outside", f.repo, "echo $(case x in x) tee out.txt </dev/null;; esac)", false},
		{"case_subst_other_file", project, "echo $(case x in x) tee notes.md </dev/null;; esac)", false},
		{"case_word_argument", project, "echo $(echo case x) > notes.md", false},

		// zsh and bash clobber forms are write operators.
		{"clobber_glued", project, "echo x >|alias", true},
		{"clobber_bang", project, "echo x >!alias", true},
		{"append_bang", project, "echo x >>!alias", true},
		{"amp_bang", project, "echo x &>!alias", true},
		{"amp_append_bang", project, "echo x &>>!alias", true},
		{"amp_clobber", project, "echo x &>|alias", true},
		{"dup_bang", project, "echo x >&!alias", true},
		{"dup_clobber", project, "echo x >&|alias", true},
		{"append_dup", project, "echo x >>&alias", true},
		{"bang_spaced", project, "echo x >! alias", true},
		{"bang_fd", project, "echo x 2>!alias", true},
		{"bang_absolute", f.repo, "echo x >!" + target, true},
		{"bang_glob", project, "echo x >!workflow?.md", true},
		{"bang_outside", project, "echo x >!notes.md", false},
		{"bang_fd_dup", project, "echo x >&2", false},

		// A case outside any group, and outside the Grid folders, runs.
		{"case_plain", f.repo, "case $1 in a) echo a;; esac", false},
		{"case_plain_write", f.repo, "case $1 in a) echo a > out.txt;; esac", false},
		{"case_plain_grid_folder", project, "case $1 in a) echo a;; esac", false},
		{"case_plain_policy_write", project, "case $1 in a) tee alias </dev/null;; esac", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			in := f.input("Bash", tt.command)
			in.Cwd = tt.cwd
			got := EvaluateHost(in, f.root, f.env)
			want := ""
			if tt.deny {
				want = "deny"
			}
			if got.Behavior != want {
				t.Fatalf("behavior = %q, want %q for %s", got.Behavior, want, fmt.Sprint(tt.command))
			}
			if tt.deny && got.Reason != gateReason {
				t.Fatal("write did not use the direct protected-write reason")
			}
		})
	}
}
