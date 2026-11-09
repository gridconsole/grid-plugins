package guard

import (
	"path/filepath"
	"strings"
	"testing"
)

// failClosedRows are tried from the project folder (alias) and from the repo folder
// (the absolute path of the alias); PROJECT stands for the project folder. A deny
// row is a command that would write the policy file under bash or zsh, or one the
// guard refuses because the shells could read it differently.
var failClosedRows = []struct {
	name, command string
	deny          bool
}{
	// A carriage return outside quotes is refused: it is a word byte to the shells.
	{"cr_hash", "echo a\r# ; tee ALIAS </dev/null", true},
	{"cr_hash_group", "echo $(echo a\r#) ; tee ALIAS </dev/null #\n)", true},
	{"cr_hash_procsub", "echo >(echo a\r#) ; tee ALIAS </dev/null #\n)", true},
	{"cr_in_double_quotes", "echo \"a\rb\"", false},
	{"cr_escape_text", `printf 'a\r\n'`, false},
	// $$ is one unit; a quote, ( or { right after it is read differently by the shells.
	{"dd_quote", `echo $$'\' ; tee ALIAS </dev/null #'`, true},
	{"dd_quote_group", "echo $(echo $$'\\' ; tee ALIAS </dev/null #'\n)", true},
	{"dd_quote_lines", "echo $$'\\'\ntee ALIAS </dev/null\necho '", true},
	{"dd_paren_double_quoted", `echo "$$(" ; tee ALIAS </dev/null ; echo ")"`, true},
	{"dd_ansi_zsh", `echo $$'\'' ; tee ALIAS </dev/null ; echo ''`, true},
	{"dd_plain", "echo $$", false},
	{"dd_double_quoted", `echo "/tmp/x.$$" "$$"`, false},
	// A write target holding $'..' or $".." is refused: the guard does not decode it.
	{"ansi_octal", `tee $'\141lias' </dev/null`, true},
	{"ansi_unicode", `tee $'\u0061lias' </dev/null`, true},
	{"ansi_hex_slash_abs", `tee PROJECT$'\x2f'workflows.md </dev/null`, true},
	{"ansi_octal_slash_abs", `tee PROJECT$'\057'alias </dev/null`, true},
	{"ansi_empty_abs", `tee PROJECT$''/alias </dev/null`, true},
	{"ansi_empty_rel", `tee .$''/alias </dev/null`, true},
	{"ansi_redirect", `: > .$''/alias`, true},
	{"locale_quote", `tee $"ALIAS" </dev/null`, true},
	{"ansi_echo", `echo $'a\tb'`, false},
	{"ansi_cut_option", `cut -d$'\t' -f1 notes.txt`, false},
	{"ansi_assignment", `IFS=$'\n' read -r line`, false},
	// Heredoc bodies are skipped up to the delimiter line; an unquoted body still runs
	// its substitutions.
	{"doc_lone_quote", "cat <<EOF\nit's\nEOF\ntee ALIAS </dev/null\n#'", true},
	{"doc_lone_quote_open", "cat <<EOF\nit's\nEOF\ntee ALIAS </dev/null", true},
	{"doc_quoted_double", "cat <<'EOF'\n\"\nEOF\ntee ALIAS </dev/null\n#\"", true},
	{"doc_hash_mid", "cat <<EOF\na#'\nEOF\ntee ALIAS </dev/null\n#'", true},
	{"doc_dash_tabs", "cat <<-EOF\n\tit's\n\tEOF\ntee ALIAS </dev/null\n#'", true},
	{"doc_in_group", "echo $(cat <<EOF\nit's\nEOF\ntee ALIAS </dev/null\n) #'", true},
	{"doc_in_double_quoted_group", "echo \"$(cat <<EOF\nit's\nEOF\n)\" ; tee ALIAS </dev/null ; echo \"'\"", true},
	{"doc_two", "cat <<A <<'B'\nit's\nA\n\"\nB\ntee ALIAS </dev/null\n#'", true},
	{"doc_backslash_quoted", "cat <<\\EOF\nit's\nEOF\ntee ALIAS </dev/null\n#'", true},
	{"doc_join_in_quoted_body", "cat <<'EOF'\nEO\\\nF\nit's\nEOF\ntee ALIAS </dev/null\n#'", true},
	{"doc_no_delimiter", "cat <<EOF\nit's\ntee ALIAS </dev/null", true},
	{"doc_unquoted_substitution", "cat <<EOF\nit's $(tee ALIAS </dev/null)\nEOF", true},
	{"doc_quoted_substitution", "cat <<'EOF'\n$(tee ALIAS </dev/null)\nEOF", false},
	{"doc_commit_message", "git commit -m \"$(cat <<'EOF'\nFix the guard's quote rules.\n\nIt's done.\nEOF\n)\"", false},
	{"doc_commit_file", "git commit -F - <<'EOF'\nit's here\nEOF", false},
	{"doc_pr_continuation", "gh pr create --title \"Fix it\" \\\n  --body \"$(cat <<'EOF'\nThe body's text.\nEOF\n)\"", false},
	{"doc_unquoted_date", "cat <<EOF > notes.md\nit's $(date)\nEOF", false},
	// Inside "..": in a backtick group bash ends at the first backtick, and in ${..}
	// zsh reads ' as a plain byte while bash reads it as a quote. Both readings count.
	{"dq_backtick_quote", "echo \"`echo '`\" ; tee ALIAS </dev/null ; echo \"`'`\"", true},
	{"dq_brace_quote", `echo "${x:-'}" ; tee ALIAS </dev/null ; echo "'}"`, true},
	{"dq_brace_quote_group", `echo $(echo "${x:-'}" ; tee ALIAS </dev/null ; echo "'}")`, true},
	{"dq_brace_quote_nested", `echo "$(echo "${x:-'}")" ; tee ALIAS </dev/null ; echo "'}"`, true},
	{"zsh_reading_after_cd", `echo "${x:-'}" ; tee ALIAS </dev/null ; echo "'}" ; cd /tmp`, true},
	{"dq_brace_default", `echo "${VAR:-'none'}"`, false},
	{"dq_text", `echo "it's a test" 'a "b"'`, false},
	{"grep_hash", `grep -rn "fix #12" . && grep -n '#' notes.md`, false},
}

// Rows for the L135R5 rework: where bash and zsh can read a line differently the
// guard refuses it, and heredoc bodies are skipped as the shells skip them.
func TestHostWriteTargetsFailClosed(t *testing.T) {
	f := setup(t)
	project := filepath.Join(f.projects, "example")
	policy := filepath.Join(project, "workflows.md")
	f.write(policy, "policy")
	f.link(filepath.Join(project, "alias"), policy)
	target := filepath.Join(project, "alias")

	for _, tt := range failClosedRows {
		for _, where := range []struct{ suffix, cwd, path string }{{"", project, "alias"}, {"_absolute", f.repo, target}} {
			t.Run(tt.name+where.suffix, func(t *testing.T) {
				command := strings.ReplaceAll(tt.command, "ALIAS", where.path)
				command = strings.ReplaceAll(command, "PROJECT", project)
				in := f.input("Bash", command)
				in.Cwd = where.cwd
				got := EvaluateHost(in, f.root, f.env)
				want := ""
				if tt.deny {
					want = "deny"
				}
				if got.Behavior != want {
					t.Fatalf("behavior = %q, want %q for %q", got.Behavior, want, command)
				}
				if tt.deny && got.Reason != gateReason {
					t.Fatal("write did not use the direct protected-write reason")
				}
			})
		}
	}
}
