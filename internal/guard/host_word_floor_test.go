package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// wordFloorRows are the L135W attack lines, tried from the project folder (lnk) and
// from the repo folder (the absolute path of lnk); POLICY is workflows.md there. The
// fixture has no file named alias, so alias cat=tee resolves to nothing. A deny row
// writes the policy file under bash or zsh.
var wordFloorRows = []struct {
	name, command string
	deny          bool
}{
	// Arithmetic << read as a heredoc: the lines after it run.
	{"w01_arith_shift", "(( 1 << 2 ))\ntee LNK </dev/null\n2", true},
	{"w02_arith_shift_word", "(( 1 << true ))\ntee LNK </dev/null\ntrue", true},
	{"w03_dollar_bracket", "echo $[ 1 << true ]\ntee LNK </dev/null\ntrue", true},
	{"w04_for_arith", "for (( i=1; i<<true; )); do break; done\ntee LNK </dev/null\ntrue", true},
	{"w05_subscript", "a[1<<true]=x\ntee LNK </dev/null\ntrue]=x", true},
	{"dollar_bracket_quoted", "echo $[ 1 << 'x' ]\ntee LNK </dev/null\nx", true},
	{"arith_reader_quoted", "(( cat << 'x' ))\ntee LNK </dev/null\nx", true},
	// A # right after (, ), < or > is a pattern byte to bash extglob, [[ =~ ]] and zsh.
	{"w06_extglob_hash", "shopt -s extglob\necho @(#|x); tee LNK </dev/null", true},
	{"w07_regex_hash", "[[ x =~ (#|x) ]]; tee LNK </dev/null", true},
	{"w10_zsh_glob_group", "echo (#|*); tee LNK </dev/null", true},
	// A read-only name shadowed on the same line.
	{"w11_func_shadow", `cat() { tee "$@"; }; cat LNK </dev/null`, true},
	{"w12_func_keyword_shadow", `function cat { tee "$@"; }; cat LNK </dev/null`, true},
	{"w14_alias_shadow", "shopt -s expand_aliases\nalias cat=tee\ncat LNK </dev/null", true},
	{"w18_hash_shadow", "hash -p /usr/bin/tee cat; cat LNK </dev/null", true},
	{"w19_path_shadow", `mkdir -p b && ln -sf "$(command -v tee)" b/cat && PATH=$PWD/b:$PATH cat LNK </dev/null`, true},
	{"w20_zsh_hash_shadow", "hash cat=/usr/bin/tee\ncat LNK </dev/null", true},
	// Code fed to an interpreter.
	{"k09_bash_heredoc", "bash <<'EOF'\ntee LNK </dev/null\nEOF", true},
	{"k11_xargs", "echo LNK | xargs tee </dev/null", true},
	// A read-only command that writes or runs another.
	{"k05_find_fprint0", "find . -maxdepth 0 -fprint0 POLICY", true},
	{"k06_rg_pre", "rg --pre rm zzz POLICY", true},
	{"rg_pre_equals", "rg --pre=rm zzz POLICY", true},
	{"find_fprint", "find . -maxdepth 0 -fprint POLICY", true},
	{"find_fprintf", "find . -maxdepth 0 -fprintf POLICY %p", true},
	{"find_fls", "find . -maxdepth 0 -fls POLICY", true},
	// L135R9, subscript assignments: the shells read the text in [..] as an expression, so a
	// << there is a shift and an assignment to BASH_CMDS[..] or zsh functions[..] shadows a command.
	{"a01_subscript_heredoc_q", "a[x=1<<'E']=y\ntee LNK </dev/null\nE]=y\n", true},
	{"a07_two_assign_heredoc", "a[i=0]=y a[x=1<<'E']=y\ntee LNK </dev/null\nE]=y\n", true},
	{"a09_bashcmds_hash", "BASH_CMDS[x=1 #]=y; tee LNK </dev/null\n", true},
	{"a10_bashcmds_heredoc", "BASH_CMDS[x=<<'E']=y\ntee LNK </dev/null\nE]=y\n", true},
	{"a11_bashcmds_shadow", "BASH_CMDS[cat]=/usr/bin/tee; cat LNK </dev/null\n", true},
	{"a12_bashaliases_hash", "BASH_ALIASES[x=1 #]=y; tee LNK </dev/null\n", true},
	{"a13_subscript_heredoc_dq", "a[x=1<<\"E\"]=y\ntee LNK </dev/null\nE]=y\n", true},
	{"a14_subscript_heredoc_bs", "a[x=1<<\\E]=y\ntee LNK </dev/null\nE]=y\n", true},
	{"a17_zsh_functions_shadow", "functions[cat]='tee \"$@\"'; cat LNK </dev/null\n", true},
	{"a20_subscript_heredoc_lnk", "a[x=1<<'E']=y\ncat LNK; tee LNK </dev/null\nE]=y\n", true},
	{"a21_prefix_subscript", "x=1 a[x=1<<'E']=y cat\ntee LNK </dev/null\nE]=y\n", true},
	{"b18_bashcmds_noredir", "BASH_CMDS[cat]=/usr/bin/tee; cat LNK\n", true},
	// L135R9, option words the guard cannot decode: $'..', $"..", a variable, a substitution or a brace
	// expansion that the shell turns into -exec, -fprint or --pre.
	{"b09_find_ansi_exec", "find . -maxdepth 0 -$'exec' tee LNK \\;\n", true},
	{"b10_find_ansi_fprint", "find . -maxdepth 0 -$'fprint' LNK\n", true},
	{"b11_rg_ansi_pre", "rg --$'pre' tee x LNK\n", true},
	{"b12_find_locale_exec", "find . -maxdepth 0 -$\"exec\" tee LNK \\;\n", true},
	{"b15_rg_pre_ansi_eq", "rg $'--pre=tee' x LNK\n", true},
	{"b16_find_quoted_ansi", "find . -maxdepth 0 $'-exec' tee LNK ';'\n", true},
	{"find_variable_option", "x=-exec; find . -maxdepth 0 $x tee LNK \\;", true},
	{"find_substitution_option", "find . -maxdepth 0 $(echo -exec) tee LNK \\;", true},
	{"find_brace_option", "find . -maxdepth 0 {-exec,tee} LNK \\;", true},
	{"rg_variable_option", "x=--pre=tee; rg $x y LNK", true},
	// L135R9, a # that a zsh glob group reads as a pattern byte: the text after it runs.
	{"d01_zsh_group_pipe_hash", "echo (cat|#|*); tee LNK\n", true},
	{"d03_zsh_group_space_hash", "echo (cat| #|*); tee LNK\n", true},
	// Reads stay allowed.
	{"plain_assignment_read", "x=1; cat LNK", false},
	{"balanced_bracket_heredoc", "[ -f LNK ] && cat <<'EOF'\nx\nEOF", false},
	{"rg_dollar_anchor", "rg 'x$' POLICY", false},
	{"rg_flag_value", "rg -e foo -g '*.md' POLICY", false},
	{"read_lnk", "cat LNK", false},
	{"read_lnk_pipe", "grep -n x LNK | head -5", false},
	{"rg_pre_glob_alone", "rg --pre-glob '*.md' x POLICY", false},
	{"find_name", "find . -maxdepth 1 -name POLICY", false},
	{"quoted_heredoc_read", "cat <<'EOF'\nLNK\nEOF", false},
}

func TestHostWordFloor(t *testing.T) {
	f := setup(t)
	project := filepath.Join(f.projects, "example")
	policy := filepath.Join(project, "workflows.md")
	f.write(policy, "policy")
	f.link(filepath.Join(project, "lnk"), policy)
	if _, err := os.Lstat(filepath.Join(project, "alias")); err == nil {
		t.Fatal("the fixture must not hold a file named alias")
	}
	for _, tt := range wordFloorRows {
		for _, where := range []struct{ suffix, cwd, lnk, policy string }{
			{"", project, "lnk", "workflows.md"},
			{"_absolute", f.repo, filepath.Join(project, "lnk"), policy},
		} {
			t.Run(tt.name+where.suffix, func(t *testing.T) {
				command := strings.ReplaceAll(tt.command, "LNK", where.lnk)
				command = strings.ReplaceAll(command, "POLICY", where.policy)
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
					t.Fatalf("reason = %q, want the gate reason", got.Reason)
				}
			})
		}
	}
}
