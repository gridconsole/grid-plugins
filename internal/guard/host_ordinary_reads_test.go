package guard

import (
	"path/filepath"
	"strings"
	"testing"
)

// ordinaryReadRows are plain reads of a protected file that main allows and L135R9 refused:
// a comment after the read, a variable as the pattern or the folder, an option-like pattern in
// '..', and an assignment to a numbered array element. FILE is the protected file.
var ordinaryReadRows = []struct{ name, command string }{
	{"o01_cat_comment", "cat FILE # read the policy"},
	{"o03_grep_comment", "grep -n verb FILE  # find verbs"},
	{"o04_rg_var_pattern", `rg -n "$PAT" FILE`},
	{"o05_find_var_dir", `find "$PWD" -maxdepth 1 -name FILE`},
	{"o18_array_then_cat", "x[0]=1; cat FILE"},
	{"o22_rg_e_dash_dollar", `rg -e '-x$' FILE`},
	{"o28_rg_type_comment", "rg -n approve FILE # where approve is"},
	{"numbered_append", "x[12]+=a; grep v FILE"},
}

// ordinaryDeniedRows are lines that look like those but write the file under bash or zsh, or
// that main denies. LNK is a symlink to workflows.md. Each stays denied.
var ordinaryDeniedRows = []struct{ name, command string }{
	{"p01_last_argument_option", "echo --pre=tee; rg -n \"$_\" LNK"},
	{"p02_printf_v", `printf -v PAT %s --pre=tee; rg -n "$PAT" LNK`},
	{"p03_assigned_var", `PAT=--pre=tee; rg -n "$PAT" LNK`},
	{"p04_default_assign", `echo ${PAT:=--pre=tee}; rg -n "$PAT" LNK`},
	{"p05_read_var", `read PAT <<< --pre=tee; rg -n "$PAT" LNK`},
	{"p07_printf_v_dynamic", `n=PA; printf -v "$n"T %s --pre=tee; rg -n "$PAT" LNK`},
	{"p08_suffix_after_quote", `rg -n "$PAT"x LNK`},
	{"p09_unquoted_var", `rg -n $PAT LNK`},
	{"p10_pwd_assigned", `PWD=-exec; find . -maxdepth 0 "$PWD" tee LNK \;`},
	{"p11_braced_assigned", `x=-exec; find . -maxdepth 0 "${x}" tee LNK \;`},
	{"p12_reply", `read <<< --pre=tee; rg -n "$REPLY" LNK`},
	{"p13_last_argument_find", `echo -exec; find . -maxdepth 0 "$_" tee LNK \;`},
	{"p14_path_subscript", "PATH[0]=/tmp/b; cat LNK"},
	{"p15_zsh_path_subscript", "path[1]=/tmp/b; cat LNK"},
	{"p16_ifs_subscript", "IFS[0]=x; cat LNK"},
	{"p17_number_then_shadow", "x[0]=1; BASH_CMDS[cat]=/usr/bin/tee; cat LNK"},
	{"p18_subscript_substitution", "x[0]=$(tee LNK </dev/null); cat LNK"},
	{"p20_comment_next_line", "cat LNK # a\ntee LNK </dev/null"},
	{"p25_comment_blank_next_line", "cat LNK # a\n tee LNK </dev/null"},
	{"p26_comment_shadow", "cat() { tee \"$@\"; } # c\ncat LNK </dev/null"},
	{"p27_comment_bashcmds", "BASH_CMDS[cat]=/usr/bin/tee # c\ncat LNK"},
	{"p28_comment_path", "PATH=/tmp/b # c\ncat LNK"},
	{"p29_comment_then_variable_option", "x=--pre=tee; rg $x y LNK # c"},
	{"p30_brace_glob_quoted", `rg -g '{*.md,*.go}' approve LNK`},
	{"p31_braced_var", `rg -n "${PAT}" LNK`},
}

// The ordinary reads are allowed for each protected file the guard knows, from the folder the
// file sits in (its name) and from the repo folder (its absolute path). A control row per
// file shows that a write to it is denied there, so an allow is not a missing fixture.
func TestHostOrdinaryReadsAllowed(t *testing.T) {
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
			run := func(command string) string {
				in := f.input("Bash", strings.ReplaceAll(command, "FILE", where.file))
				in.Cwd = where.cwd
				return EvaluateHost(in, f.root, f.env).Behavior
			}
			t.Run("control_"+file.name+where.suffix, func(t *testing.T) {
				if got := run("echo x > FILE"); got != "deny" {
					t.Fatalf("behavior = %q, want deny: %s is not protected here", got, file.name)
				}
			})
			for _, tt := range ordinaryReadRows {
				t.Run(tt.name+"_"+file.name+where.suffix, func(t *testing.T) {
					if got := run(tt.command); got != "" {
						t.Fatalf("behavior = %q, want allow for %q in %s", got, strings.ReplaceAll(tt.command, "FILE", where.file), where.cwd)
					}
				})
			}
		}
	}
}

func TestHostOrdinaryReadsStayDenied(t *testing.T) {
	f := setup(t)
	project := filepath.Join(f.projects, "example")
	policy := filepath.Join(project, "workflows.md")
	f.write(policy, "policy")
	f.link(filepath.Join(project, "lnk"), policy)
	for _, tt := range ordinaryDeniedRows {
		for _, where := range []struct{ suffix, cwd, lnk string }{{"", project, "lnk"}, {"_absolute", f.repo, filepath.Join(project, "lnk")}} {
			t.Run(tt.name+where.suffix, func(t *testing.T) {
				command := strings.ReplaceAll(tt.command, "LNK", where.lnk)
				in := f.input("Bash", command)
				in.Cwd = where.cwd
				if got := EvaluateHost(in, f.root, f.env); got.Behavior != "deny" {
					t.Fatalf("behavior = %q, want deny for %q", got.Behavior, command)
				}
			})
		}
	}
}
