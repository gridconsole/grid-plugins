package guard

import (
	"path/filepath"
	"strings"
	"testing"
)

// quoteGroupRows are tried from the project folder (alias) and from the repo folder
// (the absolute path of the alias). A deny row is a command that would write the
// policy file or that the guard refuses because it cannot tell what the shell runs.
var quoteGroupRows = []struct {
	name, command string
	deny          bool
}{
	{"sq1", `echo $(echo '\' ' ) ' ; tee ALIAS </dev/null ) #'`, true},
	{"sq2", `echo $(echo '\' ' ) ' ; tee ALIAS </dev/null )`, true},
	{"sq3", `cat <(echo '\' ' ) ' ; tee ALIAS </dev/null ) #'`, true},
	{"sq4", `echo "$(echo '\' ' ) ' ; tee ALIAS </dev/null )" #'`, true},
	{"sq5", "echo $(echo '\\' ' ) '\ntee ALIAS </dev/null\n) #'", true},
	{"dq1", `echo $(echo "'" ' " ) ' ; tee ALIAS </dev/null ) #'`, true},
	{"dq2", `cat <(echo "'" ' " ) ' ; tee ALIAS </dev/null ) #'`, true},
	// A quote in a comment opens nothing, so the next line is a command.
	{"comment_quote", "echo a # '\ntee ALIAS </dev/null\n#'", true},
	{"comment_quote_group", "echo $(echo a # '\ntee ALIAS </dev/null\n) #'", true},
	{"comment_quote_group_tail", "echo \"$(echo a # '\n)\" ; tee ALIAS </dev/null ; echo \"'\"", true},
	{"comment_after_semicolon", "echo a;# '\ntee ALIAS </dev/null\n#'", true},
	// A # inside a word or quotes is no comment: what follows still runs.
	{"hash_quoted", "echo '#'; tee ALIAS </dev/null", true},
	{"hash_double_quoted", `echo "a #"; tee ALIAS </dev/null`, true},
	{"hash_in_word", "echo a#b; tee ALIAS </dev/null", true},
	{"hash_after_quotes", "echo ''#x; tee ALIAS </dev/null", true},
	{"hash_escaped_blank", `echo \ #; tee ALIAS </dev/null`, true},
	{"hash_dollar", "echo $#; tee ALIAS </dev/null", true},
	{"hash_in_group_word", "echo \"$(echo a#b)\" ; tee ALIAS </dev/null", true},
	{"hash_after_group", "echo $(echo a)#b; tee ALIAS </dev/null", true},
	// $'..' is the one single-quoted form where a backslash escapes.
	{"ansi_quote_group", `echo $(echo $'\'' ' ) ' ; tee ALIAS </dev/null )`, true},
	{"ansi_quote_top", `echo $'\'' ; tee ALIAS </dev/null ; echo $'\''`, true},
	{"ansi_plain", `echo $'\'' x`, false},
	// A comment ends at its newline even after a backslash: tee runs.
	{"comment_continuation", "echo a # c \\\ntee ALIAS </dev/null", true},
	{"comment_continuation_group", "echo $(echo a # c \\\ntee ALIAS </dev/null)", true},
	// An unquoted heredoc runs a substitution on a # line.
	{"heredoc_comment_substitution", "cat <<EOF\n# $(tee ALIAS </dev/null)\nEOF", true},
	// A # inside quotes is no comment, so a continuation after it still joins.
	{"continuation_after_quoted_hash", "echo \"fix #12\" \\\n b", false},
	{"continuation_after_quoted_hash_single", "echo ' #x' \\\n b", false},
	{"continuation_after_quoted_hash_then_case", "echo \"a #\" ; echo $(ca\\\nse x in x) tee ALIAS </dev/null;; esac)", true},
	// With a substitution in the text every # at the start of a word counts: a
	// continuation after it is refused, quoted or not.
	{"quoted_hash_continuation_with_substitution", "echo \"fix #12\" $(echo a) \\\n b", true},
	{"comment_continuation_after_quote", "echo a # it's\necho \"b\" # c \\\ntee ALIAS </dev/null", true},
	// Real comments and plain # words run nothing protected.
	{"comment_text", "echo a # tee ALIAS </dev/null", false},
	{"comment_lone_quote", "echo a # it's\necho b", false},
	{"hash_word", "echo a#b", false},
	{"hash_quote_only", "echo '#'", false},
	{"hash_count", "echo $#", false},
	{"continuation_before_comment", "echo a \\\n b # c", false},
	// A substitution in a comment is scanned but is not an argument of the command.
	{"comment_substitution_not_an_argument", "rm notes.md # $(echo a)", false},
	{"continuation_after_hash_in_word", "echo a#b \\\n b", false},
	{"continuation_after_comment_line", "echo a # c\necho b \\\n c", false},
	// In a ( ) group a # inside a word is no comment: the ) after it still closes the
	// group, so the words after it are echo arguments, not a command (foo would write).
	{"group_hash_mid_word", "echo $(echo a#b) foo ALIAS", false},
	{"group_hash_after_substitution", "echo $(echo $(echo a)#b) foo ALIAS", false},
	{"group_hash_after_process", "echo $(echo <(echo a)#b) foo ALIAS", false},
	{"group_hash_after_quote", "echo $(echo 'a'#b) foo ALIAS", false},
	{"group_hash_after_escape", "echo $(echo a\\ #b) foo ALIAS", false},
	{"group_hash_after_brace", "echo $(echo ${x}#b) foo ALIAS", false},
	{"group_hash_after_ansi", "echo $(echo $'a'#b) foo ALIAS", false},
}

// Rows for the L135R4 rework: the quote rules inside $( ) and <( ) groups follow
// the shells (a backslash is literal in '..', only \ " ` $( and ${ are special in
// "..") and a # at the start of a word begins a comment that ends at the newline.
// Each hidden write truncates the policy file under bash and zsh.
func TestHostWriteTargetsQuoteGroups(t *testing.T) {
	f := setup(t)
	project := filepath.Join(f.projects, "example")
	policy := filepath.Join(project, "workflows.md")
	f.write(policy, "policy")
	f.link(filepath.Join(project, "alias"), policy)
	target := filepath.Join(project, "alias")

	// Each row is tried from the project folder (alias) and from the repo folder
	// (the absolute path of the alias).
	for _, tt := range quoteGroupRows {
		for _, where := range []struct{ suffix, cwd, path string }{{"", project, "alias"}, {"_absolute", f.repo, target}} {
			t.Run(tt.name+where.suffix, func(t *testing.T) {
				command := strings.ReplaceAll(tt.command, "ALIAS", where.path)
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

// A refusal reaches both scans: the project policy check and the protected-write
// check each end on it, so removing either leaves the other to deny.
func TestHostRefuseReachesBothScans(t *testing.T) {
	f := setup(t)
	project := filepath.Join(f.projects, "example")
	command := "echo \"fix #12\" $(echo a) \\\n b"
	in := f.input("Bash", command)
	in.Cwd = project
	c := newContext(in, f.root, f.env)
	if !c.writesProjectWorkflow(command) {
		t.Fatal("writesProjectWorkflow did not refuse")
	}
	if c.protectedWriteReason(command) == "" {
		t.Fatal("protectedWriteReason did not refuse")
	}
}
