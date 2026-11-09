package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Rows for the L135 rework: redirect operators, substitutions, brackets,
// known-variable expansion, cd options and ordinary commands that must pass.
func TestHostWriteTargetsRework(t *testing.T) {
	f := setup(t)
	project := filepath.Join(f.projects, "example")
	policy := filepath.Join(project, "workflows.md")
	f.write(policy, "policy")
	f.link(filepath.Join(project, "alias"), policy)
	if err := os.MkdirAll(filepath.Join(project, "a=b"), 0o755); err != nil {
		t.Fatal(err)
	}
	prefix := f.home + "/.gr[[:alpha:]]d/projects/example/workflows.md"
	for _, tt := range []struct {
		name, cwd, command string
		deny               bool
	}{
		// Redirect operators read as one write operator.
		{"clobber", project, "echo x >| alias", true},
		{"clobber_glob", project, "echo x >| workflow?.md", true},
		{"dup_both", project, "echo x >& alias", true},
		{"dup_both_glued", project, "echo x >&alias", true},
		{"read_write_fd", project, "echo x 1<> alias", true},
		{"read_write", project, "echo x <> alias", true},
		{"amp_redirect", project, "echo x &> alias", true},
		{"amp_append", project, "echo x &>> alias", true},
		{"fd_redirect", project, "echo x 2> alias", true},
		{"fd_clobber", project, "echo x 2>| alias", true},
		{"clobber_absolute", f.repo, "echo x >| " + filepath.Join(project, "alias"), true},
		{"dup_fd", project, "echo x >&2", false},
		{"dup_close", project, "echo x >&-", false},
		{"dup_stderr", project, "make -j4 > build.log 2>&1", false},
		{"dup_fd_move", project, "make 2>&1- > build.log", false},
		{"read_alias_redirect", project, "cat < alias", false},
		{"read_write_outside", project, "echo x <> notes.md", false},

		// The inside of every substitution is a command of its own.
		{"subst_tee", project, "echo $(tee alias </dev/null)", true},
		{"subst_assignment_cd", f.repo, "x=$(cd " + project + " && tee alias </dev/null)", true},
		{"subst_assignment_glob", project, "x=$(tee workflow?.md </dev/null)", true},
		{"subst_backticks", project, "echo `cp /dev/null workflow?.md`", true},
		{"subst_nested", project, "echo $(echo $(tee alias </dev/null))", true},
		{"subst_quoted", project, `echo "$(tee alias </dev/null)"`, true},
		{"subst_process_read", project, "cat <(tee alias </dev/null)", true},
		{"subst_process_write", project, "echo x | tee >(tee alias >/dev/null)", true},
		{"subst_parameter_default", project, "echo ${E:-$(tee alias </dev/null)}", true},
		{"subst_after_cd", f.repo, "cd " + project + " && echo $(tee alias </dev/null)", true},
		{"parse_quote", project, `echo x > "alias`, true},
		{"parse_backticks", project, "echo `tee alias", true},
		{"parse_parameter", project, "echo ${E:-$(tee alias", true},
		{"parse_too_deep", project, strings.Repeat("echo $(", 20) + "date" + strings.Repeat(")", 20), true},
		{"subst_read", project, "echo $(cat alias)", false},
		{"subst_assignment_read", project, "x=$(cat workflows.md)", false},
		{"subst_date", project, "echo $(date)", false},
		{"subst_outside", f.repo, "echo $(tee out.txt </dev/null)", false},

		// A key=value word is checked whole and by its value.
		{"equals_in_path", f.repo, "cp /dev/null " + project + "/a=b/../alias", true},
		{"equals_value", f.repo, "dd of=" + filepath.Join(project, "alias"), true},
		{"equals_uncertain_absolute", f.repo, "cd ${DEST}; dd of=" + filepath.Join(f.repo, "out.txt"), false},

		// Shell brackets that Go reads differently.
		{"bracket_negation", project, "echo x > workflow[!x].md", true},
		{"bracket_class", project, "echo x > workflow[[:alpha:]].md", true},
		{"bracket_equivalence", project, "tee workflow[=s=].md </dev/null", true},
		{"bracket_unfinished", project, "echo x > workflow[s.md", true},
		{"bracket_alias", project, "echo x > ali[!x]s", true},
		{"bracket_root_prefix", f.repo, "echo x > " + prefix, true},
		{"bracket_operand_class", project, "tee workflow[[:alpha:]].md </dev/null", true},
		{"bracket_operand_sed", f.task, "sed -i 's/[[:space:]]*$//' notes.md", false},
		{"bracket_operand_slice", f.task, `python3 -c "print(a[:3])"`, false},
		{"bracket_other_file", project, "echo x > notes[!x].txt", false},
		{"bracket_range", project, "echo x > notes[1].md", false},
		{"bracket_class_outside", f.repo, "echo x > out[[:alpha:]].txt", false},
		{"bracket_class_other_root", f.repo, "echo x > " + filepath.Join(f.root, "out[[:alpha:]].txt"), false},

		// Values the guard holds are filled in before the check.
		{"home", f.repo, "echo x > $HOME/.grid/projects/example/alias", true},
		{"home_braces", f.repo, "echo x > ${HOME}/.grid/projects/example/alias", true},
		{"projects_braces", f.repo, "echo x > ${GRID_PROJECTS}/example/alias", true},
		{"grid_home_glob", f.repo, "echo x > $GRID_HOME/projects/example/workflow?.md", true},
		{"grid_home_quoted", f.repo, `echo x > "$GRID_HOME"/projects/example/workflows.md`, true},
		{"operand_home", f.repo, "tee $HOME/.grid/projects/example/alias </dev/null", true},
		{"operand_value", f.repo, "dd of=$GRID_PROJECTS/example/alias", true},
		{"pwd", project, "echo x > $PWD/alias", true},
		{"pwd_after_cd", f.repo, "cd " + project + " && echo x > ${PWD}/alias", true},
		{"plus_tilde", project, "echo x > ~+/alias", true},
		{"plus_tilde_after_cd", f.repo, "cd " + project + " && echo x > ~+/alias", true},
		{"cd_known_variable", f.repo, "cd $GRID_HOME/projects/example && echo x > alias", true},
		{"home_outside", f.repo, "echo x > $HOME/notes.md", false},
		{"pwd_outside", f.repo, "echo x > $PWD/out.txt", false},
		{"unknown_variable_outside", f.repo, "echo x > $OUT", false},
		{"unknown_variable_prefix_outside", f.repo, "echo x > " + filepath.Join(f.repo, "$OUT"), false},

		// cd options are not the directory.
		{"cd_physical", f.repo, "cd -P " + project + " && echo x > alias", true},
		{"cd_logical_dashes", f.repo, "cd -L -- " + project + "; dd of=alias", true},
		{"cd_options", f.repo, "cd -e -P " + project + " && echo x > alias", true},
		{"cd_previous", project, "cd - && echo x > alias", true},
		{"cd_home", project, "cd && echo x > alias", true},
		{"cd_physical_outside", project, "cd -P " + f.repo + " && echo x > a*.txt", false},

		// A $ outside a write target is not a reason to refuse.
		{"operand_plugin_root", f.task, `node "$CLAUDE_PLUGIN_ROOT/scripts/proof-add.mjs" item-1 shot.png`, false},
		{"operand_message", f.task, `git commit -m "$MSG"`, false},
		{"operand_message_text", f.task, `git commit -m "fix $(date)"`, false},
		{"operand_caption", f.task, `node "$CLAUDE_PLUGIN_ROOT/scripts/proof-add.mjs" --caption "ran $N tests" item-1`, false},
		{"operand_source", f.task, "cp $SRC artifacts/out.png", false},
		{"operand_directory", project, "tee $DIR/notes.md </dev/null", false},
		{"operand_literal_prefix", project, "tee workflow${E}s.md </dev/null", true},
		{"operand_unknown_directory", project, "tee $DIR/workflows.md </dev/null", true},

		// Ordinary commands outside the Grid folders.
		{"ordinary_rm_glob", f.repo, "rm *.o", false},
		{"ordinary_echo_home", f.repo, "echo $HOME", false},
		{"ordinary_build_log", f.repo, "make -j4 > build.log 2>&1", false},
		{"ordinary_go_build", f.repo, "go build -o bin/x ./...", false},
		{"ordinary_cat_glob", f.repo, "cat a*.txt > out.txt", false},
		{"ordinary_export", f.repo, "export PATH=$PATH:/x", false},
		{"ordinary_commit", f.repo, `git commit -m "fix $(date)"`, false},
		{"ordinary_cd_rm", f.repo, "cd /tmp && rm -f *.o", false},
		{"ordinary_cd_variable", f.repo, "cd $TMPDIR && make", false},
		{"ordinary_for_loop", f.repo, "for f in *.go; do gofmt -w $f; done", false},
		{"ordinary_xargs", f.repo, "ls | xargs rm", false},
		{"ordinary_home_file", f.repo, "echo x > ~/notes.md", false},
		{"ordinary_sed", f.repo, "sed -i 's/$x/y/' file.go", false},
		{"ordinary_subdir_bracket", f.repo, "cd sub && echo x > out[1].txt", false},
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
