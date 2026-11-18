package cli

import (
	"errors"
	"flag"
	"fmt"
	"slices"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/settings"
)

const projectSettingsUsage = "usage: grid project settings record <project> --answers <file|-> [--json]\n       grid project settings show <project> [--json]\n       grid project settings approve <project> --hash <hash> [--json]"

// projectSettings records a project's setup answers for an agent: grid setup <project> also
// has a Claude Code form, so only this verb runs outside an agent's sandbox. The keys that
// name a command wait in setup-pending.json until a person approves their hash. It is not
// named run*: the contract inventory follows run* callees, and the verbs are one route.
func projectSettings(args []string, env Env) int {
	fs := newFlags("project settings", env)
	asJSON := fs.Bool("json", false, "print JSON")
	source := fs.String("answers", "", "answers JSON file relative to the current folder, or - for stdin")
	hash := fs.String("hash", "", "hash of the held setup commands, as show prints it")
	if err := parsePositionals(fs, args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	given := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { given[f.Name] = true })
	operands := fs.Args()
	if len(operands) == 0 {
		fmt.Fprintln(env.Stderr, projectSettingsUsage)
		return 2
	}
	verb := operands[0]
	if verb != "record" && verb != "show" && verb != "approve" {
		fmt.Fprintf(env.Stderr, "grid project settings: unknown command %q\n", verb)
		return 2
	}
	if len(operands) != 2 {
		fmt.Fprintf(env.Stderr, "grid project settings %s: wrong number of arguments\n%s\n", verb, projectSettingsUsage)
		return 2
	}
	usage := func(format string, a ...any) int {
		fmt.Fprintf(env.Stderr, "grid project settings %s: %s\n", verb, fmt.Sprintf(format, a...))
		return 2
	}
	if verb == "record" && *source == "" {
		return usage("--answers is required")
	}
	if verb == "approve" && *hash == "" {
		return usage("--hash is required")
	}
	if given["answers"] && verb != "record" {
		return usage("--answers does not apply")
	}
	if given["hash"] && verb != "approve" {
		return usage("--hash does not apply")
	}
	if verb != "show" && resolver(env).Caller.IsGuest() {
		return failure(env, ilGuestRefused)
	}
	name := "grid project settings " + verb
	store, err := settingsStore(env, operands[1], verb == "show")
	if err != nil {
		return commandError(env, name, err)
	}
	switch verb {
	case "record":
		data, err := readSetupAnswers(env, *source)
		if err != nil {
			return commandError(env, name, err)
		}
		got, err := store.RecordAgentSetup(data)
		if err != nil {
			return commandError(env, name, err)
		}
		if *asJSON {
			return writeJSON(env.Stdout, struct {
				Project  string                `json:"project"`
				Recorded settings.SetupAnswers `json:"recorded"`
				Held     settings.SetupAnswers `json:"held"`
				Hash     string                `json:"hash,omitempty"`
			}{store.Project, got.Recorded, got.Held, got.Hash})
		}
		fmt.Fprintf(env.Stdout, "Setup answers recorded for %s: %s\n", store.Project, answerNames(got.Recorded))
		if got.Hash != "" {
			fmt.Fprintf(env.Stdout, "Held for a person to approve: %s\nA person approves them with: grid project settings approve %s --hash %s\n", answerNames(got.Held), store.Project, got.Hash)
		}
		return 0
	case "show":
		answers, pending, sum, err := store.PendingSetup()
		if err != nil {
			return commandError(env, name, err)
		}
		if *asJSON {
			return writeJSON(env.Stdout, struct {
				Project string                `json:"project"`
				Answers settings.SetupAnswers `json:"answers"`
				Pending settings.SetupAnswers `json:"pending"`
				Hash    string                `json:"hash,omitempty"`
			}{store.Project, answers, pending, sum})
		}
		fmt.Fprintf(env.Stdout, "project: %s\n", store.Project)
		printAnswers(env, "", answers)
		if sum != "" {
			fmt.Fprintf(env.Stdout, "waiting for approval (hash %s):\n", sum)
			printAnswers(env, "  ", pending)
		}
		return 0
	}
	role := contract.NormalizeRole(env.getenv("GRID_ROLE"))
	switch role {
	case "":
		role = "member"
	case "cli":
		role = "owner"
	}
	if env.getenv("GRID_MEMBER") == "" || contract.TierFor(role) != contract.PersonTeam {
		return commandError(env, name, errors.New("approving setup commands is for a person of the team: set GRID_MEMBER"))
	}
	approved, answers, err := store.ApproveSetupCommands(*hash)
	if err != nil {
		return commandError(env, name, err)
	}
	if *asJSON {
		return writeJSON(env.Stdout, struct {
			Project  string                `json:"project"`
			Approved settings.SetupAnswers `json:"approved"`
			Answers  settings.SetupAnswers `json:"answers"`
		}{store.Project, approved, answers})
	}
	fmt.Fprintf(env.Stdout, "Setup commands approved for %s: %s\n", store.Project, answerNames(approved))
	return 0
}

func answerNames(answers settings.SetupAnswers) string {
	if len(answers) == 0 {
		return "none"
	}
	keys := make([]string, 0, len(answers))
	for key := range answers {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return strings.Join(keys, ", ")
}

func printAnswers(env Env, indent string, answers settings.SetupAnswers) {
	keys := strings.Split(answerNames(answers), ", ")
	for _, key := range keys {
		if value, ok := answers[key]; ok {
			fmt.Fprintf(env.Stdout, "%s%s: %s\n", indent, key, value)
		}
	}
}
