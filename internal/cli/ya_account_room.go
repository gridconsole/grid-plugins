package cli

import "github.com/gridconsole/grid-plugins/internal/account"

// yaAccountRoom reads the account_windows answer as the scheduler needs it. Room:
// some Claude account has numbers and every window of it is used below 100.
// Known: some Claude account has numbers at all. A source of "none" is no number,
// never a zero, and Codex use is not read, so neither makes anything known.
func yaAccountRoom(list []account.AccountWindows) (room, known bool) {
	for _, a := range list {
		if a.Kind != account.Claude || a.Source == "none" {
			continue
		}
		known = true
		open := true
		for _, w := range a.Windows {
			if w.Used >= 100 {
				open = false
			}
		}
		if open {
			room = true
		}
	}
	return room, known
}

// yaAccountRoomFor is the Store's AccountRoom: it reads the windows each time it
// is asked, and an unreadable registry answers not known.
func yaAccountRoomFor(env Env) func() (room, known bool) {
	return func() (bool, bool) {
		list, err := account.Windows(accountCoreEnv(env), "")
		if err != nil {
			return false, false
		}
		return yaAccountRoom(list)
	}
}
