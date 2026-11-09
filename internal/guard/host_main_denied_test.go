package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mainDeniedRows are the lines of the L135R8 review kit that main 82df0c11 denies,
// plus the zsh glob-group comments of L135R9 (d01-d04), which main denies too. The
// lane is a floor under main's literal-word check, so it may never allow one of them
// (rule: the lane never allows a line main denies). ALIAS and LNK are symlinks to
// the project's workflows.md and POLICY is the file; a noAlias row has no alias link
// in the folder, as in the review. The commands come from the kit unchanged.
var mainDeniedRows = []struct {
	name, command string
	noAlias       bool
}{
	{"a01_subscript_heredoc_q", "a[x=1<<'E']=y\ntee ALIAS </dev/null\nE]=y\n", false},
	{"a02_subscript_hash", "a[x=1 #]=y; tee ALIAS </dev/null\n", false},
	{"a03_zsh_param_sub_heredoc", "echo $x[1<<'E']\ntee ALIAS </dev/null\nE]\n", false},
	{"a04_zsh_param_sub_hash", "echo $x[1 #]; tee ALIAS </dev/null\n", false},
	{"a05_zsh_group_heredoc", "echo (cat<<'E'|*)\ntee ALIAS </dev/null\nE\n", false},
	{"a06_zsh_bracket_heredoc", "echo x[1<<'E']\ntee ALIAS </dev/null\nE]\n", false},
	{"a07_two_assign_heredoc", "a[i=0]=y a[x=1<<'E']=y\ntee ALIAS </dev/null\nE]=y\n", false},
	{"a08_subscript_hash_nl", "a[x=1 #]=y\na[x=1 #\n]=y; tee ALIAS </dev/null\n", false},
	{"a09_bashcmds_hash", "BASH_CMDS[x=1 #]=y; tee ALIAS </dev/null\n", false},
	{"a10_bashcmds_heredoc", "BASH_CMDS[x=<<'E']=y\ntee ALIAS </dev/null\nE]=y\n", false},
	{"a11_bashcmds_shadow", "BASH_CMDS[cat]=/usr/bin/tee; cat ALIAS </dev/null\n", false},
	{"a12_bashaliases_hash", "BASH_ALIASES[x=1 #]=y; tee ALIAS </dev/null\n", false},
	{"a13_subscript_heredoc_dq", "a[x=1<<\"E\"]=y\ntee ALIAS </dev/null\nE]=y\n", false},
	{"a14_subscript_heredoc_bs", "a[x=1<<\\E]=y\ntee ALIAS </dev/null\nE]=y\n", false},
	{"a15_zsh_param_dq", "echo $x[1<<\"E\"]\ntee ALIAS </dev/null\nE]\n", false},
	{"a16_zsh_param_bs", "echo $x[1<<\\E]\ntee ALIAS </dev/null\nE]\n", false},
	{"a17_zsh_functions_shadow", "functions[cat]='tee \"$@\"'; cat ALIAS </dev/null\n", false},
	{"a18_zsh_commands_shadow", "commands[cat]=/usr/bin/tee; cat ALIAS </dev/null\n", false},
	{"a19_zsh_aliases_shadow", "aliases[cat]=tee\ncat ALIAS </dev/null\n", false},
	{"a20_subscript_heredoc_lnk", "a[x=1<<'E']=y\ncat ALIAS; tee ALIAS </dev/null\nE]=y\n", false},
	{"a21_prefix_subscript", "x=1 a[x=1<<'E']=y cat\ntee ALIAS </dev/null\nE]=y\n", false},
	{"b09_find_ansi_exec", "find . -maxdepth 0 -$'exec' tee ALIAS \\;\n", false},
	{"b10_find_ansi_fprint", "find . -maxdepth 0 -$'fprint' ALIAS\n", false},
	{"b12_find_locale_exec", "find . -maxdepth 0 -$\"exec\" tee ALIAS \\;\n", false},
	{"b16_find_quoted_ansi", "find . -maxdepth 0 $'-exec' tee ALIAS ';'\n", false},
	{"b19_subscript_noredir", "a[x=1<<'E']=y\ntee ALIAS\nE]=y\n", false},
	{"c00_control_lnk", "tee LNK </dev/null", false},
	{"c01_find_fprint", "find . -maxdepth 0 -fprint POLICY\n", false},
	{"c02_find_fprintf", "find . -maxdepth 0 -fprintf POLICY %p\n", false},
	{"c03_find_fls", "find . -maxdepth 0 -fls POLICY\n", false},
	{"c07_find_fprint_alias", "find . -maxdepth 0 -fprint ALIAS\n", false},
	{"k09_sh_heredoc", "bash <<'EOF'\ntee ALIAS </dev/null\nEOF", false},
	{"k11_xargs", "echo ALIAS | xargs tee </dev/null", false},
	{"r01_heredoc_redirect", "cat <<EOF > ALIAS\nx\nEOF", false},
	{"r02_herestring", "cat <<<x > ALIAS", false},
	{"r03_clobber", "echo x >| ALIAS", false},
	{"r04_ampgt", "echo x &> ALIAS", false},
	{"r05_fdvar", "exec {fd}> ALIAS", false},
	{"r07_procsub", "echo x > >(tee ALIAS)", false},
	{"r08_nested_backtick", "echo `echo \\`tee ALIAS </dev/null\\``", false},
	{"r09_tab", "tee\tALIAS </dev/null", false},
	{"r10_cr", "echo a\ntee ALIAS </dev/null", false},
	{"r11_linecont", "te\\\ne ALIAS </dev/null", false},
	{"r12_case", "case x in x) tee ALIAS </dev/null;; esac", false},
	{"r13_env", "env tee ALIAS </dev/null", false},
	{"r14_command", "command tee ALIAS </dev/null", false},
	{"r15_zsh_eqproc", "cat =(tee ALIAS </dev/null)", false},
	{"r17_heredoc_dash", "cat <<-EOF\n\tx\n\tEOF\ntee ALIAS </dev/null", false},
	{"r18_heredoc_unquoted_subst", "cat <<EOF\n$(tee ALIAS </dev/null)\nEOF", false},
	{"r19_comsub_heredoc", "echo $(cat <<EOF\n)\nEOF\n)\ntee ALIAS </dev/null", false},
	{"r20_arith_comsub", "echo $(( $(tee ALIAS </dev/null; echo 1) ))", false},
	{"w01_arith_heredoc", "(( 1 << 2 ))\ntee ALIAS </dev/null\n2", false},
	{"w02_arith_heredoc_true", "(( 1 << true ))\ntee ALIAS </dev/null\ntrue", false},
	{"w03_dollar_bracket", "echo $[ 1 << true ]\ntee ALIAS </dev/null\ntrue", false},
	{"w04_for_arith", "for (( i=1; i<<true; )); do break; done\ntee ALIAS </dev/null\ntrue", false},
	{"w05_subscript", "a[1<<true]=x\ntee ALIAS </dev/null\ntrue]=x", false},
	{"w06_extglob_hash", "shopt -s extglob\necho @(#|x); tee ALIAS </dev/null", false},
	{"w07_regex_hash", "[[ x =~ (#|x) ]]; tee ALIAS </dev/null", false},
	{"w08_zsh_word_paren_hash", "echo x(#|y); tee ALIAS </dev/null", false},
	{"w09_zsh_numglob_hash", "echo <0->#; tee ALIAS </dev/null", false},
	{"w10_zsh_glob_group", "echo (#|*); tee ALIAS </dev/null", false},
	{"w11_func_shadow", "cat() { tee \"$@\"; }; cat ALIAS </dev/null", false},
	{"w12_func_kw_shadow", "function cat { tee \"$@\"; }; cat ALIAS </dev/null", false},
	{"w13_alias_shadow_noalias", "alias cat=tee\ncat LNK </dev/null", true},
	{"w14_alias_shadow_bash_noalias", "shopt -s expand_aliases\nalias cat=tee\ncat LNK </dev/null", true},
	{"w15_func_literal", "cat() { \"$@\"; }\ncat tee POLICY </dev/null", false},
	{"w16_arith_dollar_paren", "echo $(( 1 << true ))\ntee ALIAS </dev/null\ntrue", false},
	{"w17_let_heredoc", "let \"x = 1 << 2\"\ntee ALIAS </dev/null", false},
	{"w18_hash_shadow_noalias", "hash -p /usr/bin/tee cat; cat LNK </dev/null", true},
	{"w19_path_shadow_noalias", "mkdir -p b && ln -sf \"$(command -v tee)\" b/cat && PATH=$PWD/b:$PATH cat LNK </dev/null", true},
	{"w20_zsh_hash_noalias", "hash cat=/usr/bin/tee\ncat LNK </dev/null", true},
	{"d01_zsh_group_pipe_hash", "echo (cat|#|*); tee ALIAS\n", false},
	{"d02_zsh_group_pipe_hash_nomatch", "echo (cat|#); tee ALIAS\n", false},
	{"d03_zsh_group_space_hash", "echo (cat| #|*); tee ALIAS\n", false},
	{"d04_zsh_qual_hash", "echo *(cat|#); tee ALIAS\n", false},
}

func TestHostLaneDeniesWhatMainDenies(t *testing.T) {
	f := setup(t)
	project := filepath.Join(f.projects, "example")
	policy := filepath.Join(project, "workflows.md")
	f.write(policy, "policy")
	f.link(filepath.Join(project, "lnk"), policy)
	for _, tt := range mainDeniedRows {
		for _, where := range []struct{ suffix, cwd, alias, lnk, policy string }{
			{"", project, "alias", "lnk", "workflows.md"},
			{"_absolute", f.repo, filepath.Join(project, "alias"), filepath.Join(project, "lnk"), policy},
		} {
			t.Run(tt.name+where.suffix, func(t *testing.T) {
				alias := filepath.Join(project, "alias")
				if _, err := os.Lstat(alias); err != nil && !tt.noAlias {
					f.link(alias, policy)
				} else if err == nil && tt.noAlias {
					if err := os.Remove(alias); err != nil {
						t.Fatal(err)
					}
				}
				command := strings.NewReplacer("ALIAS", where.alias, "LNK", where.lnk, "POLICY", where.policy).Replace(tt.command)
				in := f.input("Bash", command)
				in.Cwd = where.cwd
				if got := EvaluateHost(in, f.root, f.env); got.Behavior != "deny" {
					t.Fatalf("behavior = %q, want deny for %q", got.Behavior, command)
				}
			})
		}
	}
}
