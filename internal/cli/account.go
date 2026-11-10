package cli

import (
	"fmt"
	"io"
	"time"

	"github.com/gridconsole/grid-plugins/internal/account"
)

const accountHelp = `usage: grid account <command> [flags]

  list [--json]                        the registered accounts; writes nothing
  status [--json]                      scan the folders, register a login email not seen before
                                       (once, with a folder of its own), and answer every account
  scan [--json]                        the config folders with their login email
  folder --kind K --id ID [--json]     make ~/.<id> and link the person's config into it
  windows [--id ID] [--json]           5-hour and weekly use; "no data" when no source has any
  note [--dir D]                       read status line JSON on stdin and keep its rate_limits
  sign-in (--id ID | --dir D --kind K) run the provider's login with that folder's environment

Credentials stay where the provider keeps them. Grid files hold emails, folders and use numbers only.
`

func accountCoreEnv(env Env) account.Env {
	return account.Env{Home: env.Home, GridHome: env.GridHome, Getenv: env.getenv}
}

func accountFail(env Env, op string, err error) int {
	fmt.Fprintf(env.Stderr, "grid account %s: %v\n", op, err)
	return 1
}

func runAccount(args []string, env Env) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(env.Stdout, accountHelp)
		return 0
	}
	switch args[0] {
	case "list", "status":
		return accountStatus(args[0], args[1:], env)
	case "scan":
		return accountScan(args[1:], env)
	case "folder":
		return accountFolder(args[1:], env)
	case "windows":
		return accountWindows(args[1:], env)
	case "note":
		return accountNote(args[1:], env)
	case "sign-in":
		return accountSignIn(args[1:], env)
	}
	fmt.Fprintf(env.Stderr, "account: unknown command %q\n", args[0])
	return 2
}

func signedInWord(b *bool) string {
	switch {
	case b == nil:
		return "unknown"
	case *b:
		return "signed in"
	}
	return "not signed in"
}

func accountStatus(op string, args []string, env Env) int {
	fs := newFlags("account "+op, env)
	asJSON := fs.Bool("json", false, "print JSON")
	if code, stop := parse(fs, args, env); stop {
		return code
	}
	accounts, err := account.Status(accountCoreEnv(env), op == "status")
	if err != nil {
		return accountFail(env, op, err)
	}
	if *asJSON {
		return writeJSON(env.Stdout, struct {
			Accounts []account.Account `json:"accounts"`
		}{accounts})
	}
	if len(accounts) == 0 {
		fmt.Fprintln(env.Stdout, "account: none registered")
	}
	for _, a := range accounts {
		tag := ""
		if a.Default {
			tag = " (default)"
		}
		fmt.Fprintf(env.Stdout, "account %s%s: %s, %s, %s\n", a.ID, tag, a.Email, signedInWord(a.SignedIn), a.Dir)
	}
	return 0
}

func accountScan(args []string, env Env) int {
	fs := newFlags("account scan", env)
	asJSON := fs.Bool("json", false, "print JSON")
	if code, stop := parse(fs, args, env); stop {
		return code
	}
	folders := account.Scan(accountCoreEnv(env))
	if *asJSON {
		return writeJSON(env.Stdout, struct {
			Folders []account.Folder `json:"folders"`
		}{folders})
	}
	for _, f := range folders {
		email := f.Email
		if email == "" {
			email = "no email"
		}
		fmt.Fprintf(env.Stdout, "%s %s: %s, %s\n", f.Kind, f.Dir, email, signedInWord(f.SignedIn))
	}
	return 0
}

func accountFolder(args []string, env Env) int {
	fs := newFlags("account folder", env)
	kind := fs.String("kind", "", "claude or codex")
	id := fs.String("id", "", "account id, <kind>-<name>")
	asJSON := fs.Bool("json", false, "print JSON")
	if code, stop := parse(fs, args, env); stop {
		return code
	}
	if *kind == "" || *id == "" {
		fmt.Fprintln(env.Stderr, "account folder: --kind and --id are required")
		return 2
	}
	res, err := account.MakeFolder(accountCoreEnv(env), *kind, *id)
	if err != nil {
		return accountFail(env, "folder", err)
	}
	if *asJSON {
		return writeJSON(env.Stdout, res)
	}
	state := "exists"
	if res.Created {
		state = "created"
	}
	fmt.Fprintf(env.Stdout, "account folder %s: %s\n", res.Dir, state)
	return 0
}

func accountWindows(args []string, env Env) int {
	fs := newFlags("account windows", env)
	id := fs.String("id", "", "only this account")
	asJSON := fs.Bool("json", false, "print JSON")
	if code, stop := parse(fs, args, env); stop {
		return code
	}
	list, err := account.Windows(accountCoreEnv(env), *id)
	if err != nil {
		return accountFail(env, "windows", err)
	}
	if *asJSON {
		return writeJSON(env.Stdout, struct {
			Accounts []account.AccountWindows `json:"accounts"`
		}{list})
	}
	for _, a := range list {
		if len(a.Windows) == 0 {
			fmt.Fprintf(env.Stdout, "account %s: no data\n", a.ID)
		}
		for _, w := range a.Windows {
			resets := ""
			if w.ResetsAt != 0 {
				resets = ", resets " + time.Unix(w.ResetsAt, 0).UTC().Format(time.RFC3339)
			}
			fmt.Fprintf(env.Stdout, "account %s %s: %.0f%% used%s\n", a.ID, w.Label, w.Used, resets)
		}
	}
	return 0
}

func accountNote(args []string, env Env) int {
	fs := newFlags("account note", env)
	dir := fs.String("dir", "", "the config folder the numbers belong to; defaults to CLAUDE_CONFIG_DIR, then ~/.claude")
	fs.Bool("json", false, "accepted for uniformity; note prints nothing")
	if code, stop := parse(fs, args, env); stop {
		return code
	}
	core := accountCoreEnv(env)
	if *dir == "" {
		*dir = account.ConfigDir(core)
	}
	var in io.Reader = env.Stdin
	if in == nil {
		in = emptyReader{}
	}
	if err := account.Note(core, in, *dir); err != nil {
		return accountFail(env, "note", err)
	}
	return 0
}

type emptyReader struct{}

func (emptyReader) Read([]byte) (int, error) { return 0, io.EOF }

func accountSignIn(args []string, env Env) int {
	fs := newFlags("account sign-in", env)
	id := fs.String("id", "", "a registered account id")
	dir := fs.String("dir", "", "the config folder to sign in on")
	kind := fs.String("kind", "", "claude or codex")
	asJSON := fs.Bool("json", false, "print JSON; the provider's output then goes to stderr")
	if code, stop := parse(fs, args, env); stop {
		return code
	}
	core := accountCoreEnv(env)
	if *id != "" {
		list, err := account.Status(core, false)
		if err != nil {
			return accountFail(env, "sign-in", err)
		}
		for _, a := range list {
			if a.ID == *id {
				*dir, *kind = a.Dir, a.Kind
			}
		}
		if *dir == "" {
			return accountFail(env, "sign-in", fmt.Errorf("no account %q: %w", *id, account.ErrBadFolder))
		}
	}
	if *dir == "" || *kind == "" {
		fmt.Fprintln(env.Stderr, "account sign-in: give --id, or --dir and --kind")
		return 2
	}
	out := env.Stdout
	if *asJSON {
		out = env.Stderr
	}
	var in io.Reader = env.Stdin
	if in == nil {
		in = emptyReader{}
	}
	code, err := account.SignIn(core, *kind, *dir, in, out, env.Stderr)
	if err != nil {
		return accountFail(env, "sign-in", err)
	}
	if *asJSON {
		writeJSON(env.Stdout, struct {
			Dir  string `json:"dir"`
			Kind string `json:"kind"`
			Exit int    `json:"exit"`
		}{*dir, *kind, code})
	}
	if code != 0 {
		return 1
	}
	return 0
}
