package guard

import (
	"path/filepath"
	"strings"
	"testing"
)

// readsBackDeniedRows are lines a real shell runs as a write (bash 5.3.9 on the VM, zsh 5.9 on the
// laptop: the L135R10 review, rows r01-r24), that e3eb0da2 denies and the L135R10 floor allowed. LNK is a
// symlink to workflows.md. Each stays denied.
var readsBackDeniedRows = []struct{ name, command string }{
	// zsh ties cdpath to CDPATH, fignore to FIGNORE, psvar to PSVAR, mailpath to MAILPATH
	{"r01_zsh_cdpath", `cdpath=-exec; find . -maxdepth 0 "$CDPATH" tee LNK \;`},
	{"r02_zsh_cdpath_subscript", `cdpath[1]=-exec; find . -maxdepth 0 "$CDPATH" tee LNK \;`},
	{"r03_zsh_psvar_rg", `psvar[1]=--pre=tee; rg -n x "$PSVAR" LNK`},
	{"r04_zsh_fignore", `fignore=-exec; find . -maxdepth 0 "$FIGNORE" tee LNK \;`},
	{"r18_zsh_mailpath_subscript", `mailpath[1]=-exec; find . -maxdepth 0 "$MAILPATH" tee LNK \;`},
	{"r23_zsh_cdpath_tee_brace", `cdpath=-exec; find LNK -maxdepth 0 "$CDPATH" tee {} \;`},
	// a variable assigned through the name another variable holds: bash ${!n:=w}, zsh ${(P)n::=w}
	{"r07_bash_indirect_find", `n=FO; n+=O; cat ${!n:=-exec}; find . -maxdepth 0 "$FOO" tee LNK \;`},
	{"r08_bash_indirect_rg", `n=PA; n+=T; cat ${!n:=--pre=tee}; rg -n x "$PAT" LNK`},
	{"r22_bash_indirect_subscript", `n=FO; n+=O; x[0]=${!n:=-exec}; find . -maxdepth 0 "$FOO" tee LNK \;`},
	{"s01_zsh_indirect_P", `n=FO; n+=O; cat ${(P)n::=-exec}; find . -maxdepth 0 "$FOO" tee LNK \;`},
	{"s02_zsh_indirect_P_pwd", `n=PW; n+=D; cat ${(P)n::=-exec}; find . -maxdepth 0 "$PWD" tee LNK \;`},
	// text the shell decodes after the guard read it: $'..' hides $(tee ..) from flatten
	{"r15_bash_random_subscript", `a=$'x[\x24\x28tee\x20LNK\x29]'; RANDOM[0]=a; cat LNK`},
	{"r05_zsh_galiases_name", "galiases[1]=$'x\\x3btee\\x20LNK'\ncat 1 WF"},
	{"r06_zsh_galiases_link", "galiases[1]=$'x\\x3btee\\x20LNK'\ncat 1 LNK"},
	// an integer variable evaluates what it is given: a holds code, RANDOM=a runs it (the older hole)
	{"r14_bash_random_ansi", `a=$'x[\x24\x28tee\x20LNK\x29]'; RANDOM=a; cat LNK`},
	{"t01_random_plain_quote", `a='x[$(tee LNK)]'; RANDOM=a; cat LNK`},
	{"t02_random_append", `a='x[$(tee LNK)]'; RANDOM+=a; cat LNK`},
	{"t03_srandom", `a='x[$(tee LNK)]'; SRANDOM=a; cat LNK`},
	{"t04_random_subscript_plain", `a='x[$(tee LNK)]'; RANDOM[0]=a; cat LNK`},
}

// readsBackRefusedRows are lines the L135R10 floor let through (o04, r09, r10) and the lane refuses
// again: a quoted "$NAME" argument of find or rg for a name the line does not set. The environment or a
// startup file an earlier command wrote may hold an option there (PAT=--pre=tee, F=-exec), and a real
// shell then writes FILE (bash 5.3.9 and zsh 5.9, L135R10 review r09, r10). So it fails closed. The shell
// sets "$PWD" itself; o05 in ordinaryReadRows stays allowed. FILE is the protected file.
var readsBackRefusedRows = []struct{ name, command string }{
	{"o04_rg_var_pattern", `rg -n "$PAT" FILE`},
	{"r09_find_var_option", `find . -maxdepth 0 "$F" tee FILE \;`},
	{"r10_rg_var_option", `rg -n "$PAT" FILE`},
	{"u01_find_var_dir", `find "$ROOT" -maxdepth 1 -name FILE`},
	{"u02_braced", `rg -n "${PAT}" FILE`},
}

func TestHostReadsBackStayDenied(t *testing.T) {
	f := setup(t)
	project := filepath.Join(f.projects, "example")
	policy := filepath.Join(project, "workflows.md")
	f.write(policy, "policy")
	f.link(filepath.Join(project, "lnk"), policy)
	for _, tt := range readsBackDeniedRows {
		for _, where := range []struct{ suffix, cwd, lnk, wf string }{{"", project, "lnk", "workflows.md"}, {"_absolute", f.repo, filepath.Join(project, "lnk"), policy}} {
			t.Run(tt.name+where.suffix, func(t *testing.T) {
				command := strings.ReplaceAll(strings.ReplaceAll(tt.command, "LNK", where.lnk), "WF", where.wf)
				in := f.input("Bash", command)
				in.Cwd = where.cwd
				if got := EvaluateHost(in, f.root, f.env); got.Behavior != "deny" {
					t.Fatalf("behavior = %q, want deny for %q", got.Behavior, command)
				}
			})
		}
	}
}

func TestHostReadsBackRefusedVariable(t *testing.T) {
	f := setup(t)
	project := filepath.Join(f.projects, "example")
	policy := filepath.Join(project, "workflows.md")
	f.write(policy, "policy")
	f.link(filepath.Join(project, "lnk"), policy)
	f.write(filepath.Join(project, "settings.json"), "{}")
	files := []struct{ name, dir, base, abs string }{
		{"workflows", project, "workflows.md", policy},
		{"lnk", project, "lnk", filepath.Join(project, "lnk")},
		{"task", f.task, "task.md", filepath.Join(f.task, "task.md")},
		{"settings", project, "settings.json", filepath.Join(project, "settings.json")},
	}
	for _, file := range files {
		for _, where := range []struct{ suffix, cwd, file string }{{"", file.dir, file.base}, {"_absolute", f.repo, file.abs}} {
			for _, tt := range readsBackRefusedRows {
				t.Run(tt.name+"_"+file.name+where.suffix, func(t *testing.T) {
					command := strings.ReplaceAll(tt.command, "FILE", where.file)
					in := f.input("Bash", command)
					in.Cwd = where.cwd
					if got := EvaluateHost(in, f.root, f.env); got.Behavior != "deny" {
						t.Fatalf("behavior = %q, want deny for %q in %s", got.Behavior, command, where.cwd)
					}
				})
			}
		}
	}
}

// hostPlainVar on its own, so each of its rules shows even where the caller refuses the name first.
func TestHostPlainVarRules(t *testing.T) {
	for _, tt := range []struct {
		name, word, whole string
		want              bool
	}{
		{"control_only_read", `$PWD`, `find "$PWD" -name x`, true},
		{"control_other_name", `$PAT`, `rg -n "$PAT" x`, true},
		{"control_braced", `${PWD}`, `find "${PWD}" -name x`, true},
		{"assigned", `$PAT`, `PAT=a; rg -n "$PAT" x`, false},
		{"tied_cdpath", `$CDPATH`, `cdpath=-exec; find . "$CDPATH" x`, false},
		{"tied_cdpath_subscript", `$CDPATH`, `cdpath[1]=-exec; find . "$CDPATH" x`, false},
		{"tied_fignore", `$FIGNORE`, `fignore=-exec; find . "$FIGNORE" x`, false},
		{"tied_psvar", `$PSVAR`, `psvar[1]=a; rg -n x "$PSVAR" y`, false},
		{"tied_mailpath", `$MAILPATH`, `mailpath[1]=a; find . "$MAILPATH" y`, false},
		{"lower_use_is_not_a_set", `$PAT`, `rg -n "$PAT" x; echo $pat`, true},
		{"indirect_bash", `$FOO`, `n=FO; n+=O; cat ${!n:=-exec}; find . "$FOO" x`, false},
		{"indirect_bash_read_only", `$FOO`, `n=FO; cat ${!n}; find . "$FOO" x`, false},
		{"indirect_zsh_P", `$FOO`, `n=FO; cat ${(P)n::=-exec}; find . "$FOO" x`, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := hostPlainVar(tt.word, tt.whole); got != tt.want {
				t.Fatalf("hostPlainVar(%q, %q) = %v, want %v", tt.word, tt.whole, got, tt.want)
			}
		})
	}
}

// tildeDeniedRows are lines a real shell runs as a write (the L135R11 review: bash 5.3.9 on the VM, zsh 5.9
// on the laptop) that the lane let through: an unquoted word that starts with ~ and a +, - or digit expands
// to the zsh directory stack (~N, ~+N, ~-N), PWD (~+) or OLDPWD (~-), and find or rg reads the result as an
// option. LNK is a symlink to workflows.md. Each stays denied.
var tildeDeniedRows = []struct{ name, command string }{
	{"m04_zsh_dirstack1_tilde", `dirstack[1]=-exec; find . -maxdepth 0 ~1 tee LNK \;`},
	{"m05_zsh_dirstack1_tplus", `dirstack[1]=-exec; find . -maxdepth 0 ~+1 tee LNK \;`},
	{"m06_zsh_dirstack1_tminus", `dirstack[1]=-exec; find . -maxdepth 0 ~-0 tee LNK \;`},
	{"z05_zsh_dirstack2", `dirstack[2]=-exec; find . -maxdepth 0 ~2 tee LNK \;`},
	{"m32_zsh_dirstack1_rg", `dirstack[1]=--pre=tee; rg -n x ~1 LNK`},
	{"z04_zsh_dirstack1_rg_tplus", `dirstack[1]=--pre=tee; rg -n x ~+1 LNK`},
	{"m07_oldpwd_tminus", `OLDPWD=-exec; find . -maxdepth 0 ~- tee LNK \;`},
	{"m08_pwd_tplus", `PWD=-exec; find . -maxdepth 0 ~+ tee LNK \;`},
	{"n06_z_tilde_var", `x=1; dirstack[1]=-exec; find . -maxdepth 0 ~$x tee LNK \;`},
	{"n07_z_tilde_var_rg", `x=1; dirstack[1]=--pre=tee; rg -n x ~$x LNK`},
	{"n14_emptyq_t1", `dirstack[1]=-exec; find . -maxdepth 0 ''~1 tee LNK \;`},
	{"n15_emptydq_t1", `dirstack[1]=-exec; find . -maxdepth 0 ""~1 tee LNK \;`},
	{"p03_emptydq_tplus1", `dirstack[1]=-exec; find . -maxdepth 0 ""~+1 tee LNK \;`},
	{"p04_emptyq_tminus0", `dirstack[1]=-exec; find . -maxdepth 0 ''~-0 tee LNK \;`},
	{"p05_emptyq_rg", `dirstack[1]=--pre=tee; rg -n x ''~1 LNK`},
	{"p06_tvar_plus", `x=+1; dirstack[1]=-exec; find . -maxdepth 0 ~$x tee LNK \;`},
	{"p07_tvar_dq", `x=1; dirstack[1]=-exec; find . -maxdepth 0 ~"$x" tee LNK \;`},
	{"p08_tvar_optind", `dirstack[1]=-exec; find . -maxdepth 0 ~$OPTIND tee LNK \;`},
}

// tildeKeptRows keep the verdict they had before the ~ rule: a home (~, ~/x, ~user) is no option, and a quoted
// or escaped ~ is no tilde. The L135R11 lane allowed each.
var tildeKeptRows = []struct{ name, command string }{
	{"c01_tilde_plain", `find . -maxdepth 0 ~ tee LNK \;`},
	{"c02_tilde_slash", `find . -maxdepth 0 ~/x tee LNK \;`},
	{"c03_tilde_user", `find . -maxdepth 0 ~root tee LNK \;`},
	{"c04_quoted_tminus", `find . -maxdepth 0 '~-' tee LNK \;`},
	{"c05_dq_tilde_digit", `find . -maxdepth 0 "~1" tee LNK \;`},
	{"c06_escaped_tminus", `find . -maxdepth 0 \~- tee LNK \;`},
	{"c07_rg_tilde_user", `rg -n x ~root LNK`},
	{"c08_rg_tilde_slash", `rg -n x ~/x LNK`},
}

func TestHostTildeOpaque(t *testing.T) {
	f := setup(t)
	project := filepath.Join(f.projects, "example")
	policy := filepath.Join(project, "workflows.md")
	f.write(policy, "policy")
	f.link(filepath.Join(project, "lnk"), policy)
	for _, table := range []struct {
		name string
		rows []struct{ name, command string }
		want string
	}{{"denied", tildeDeniedRows, "deny"}, {"kept", tildeKeptRows, "allow"}} {
		for _, tt := range table.rows {
			for _, where := range []struct{ suffix, cwd, lnk string }{{"", project, "lnk"}, {"_absolute", f.repo, filepath.Join(project, "lnk")}} {
				t.Run(table.name+"/"+tt.name+where.suffix, func(t *testing.T) {
					command := strings.ReplaceAll(tt.command, "LNK", where.lnk)
					in := f.input("Bash", command)
					in.Cwd = where.cwd
					if got := EvaluateHost(in, f.root, f.env); (got.Behavior == "deny") != (table.want == "deny") {
						t.Fatalf("behavior = %q, want %s for %q", got.Behavior, table.want, command)
					}
				})
			}
		}
	}
}
