package cli

import (
	"testing"

	"github.com/gridconsole/grid-plugins/internal/account"
)

func yaRoomAccount(kind, source string, used ...float64) account.AccountWindows {
	a := account.AccountWindows{ID: kind + "-1", Kind: kind, Source: source, Windows: []account.Window{}}
	for i, u := range used {
		a.Windows = append(a.Windows, account.Window{Label: []string{"5h", "7d"}[i%2], Used: u})
	}
	return a
}

func TestYaRoomReadsTheWindows(t *testing.T) {
	for _, tc := range []struct {
		name        string
		list        []account.AccountWindows
		room, known bool
	}{
		{"one claude account, 5h window used 40", []account.AccountWindows{yaRoomAccount("claude", "statusline", 40)}, true, true},
		{"every window used 100", []account.AccountWindows{yaRoomAccount("claude", "statusline", 100, 100)}, false, true},
		{"one of two windows used 100", []account.AccountWindows{yaRoomAccount("claude", "statusline", 40, 100)}, false, true},
		{"a second account with room", []account.AccountWindows{yaRoomAccount("claude", "statusline", 100), yaRoomAccount("claude", "stream", 10, 20)}, true, true},
		{"every account source none", []account.AccountWindows{yaRoomAccount("claude", "none"), yaRoomAccount("claude", "none")}, false, false},
		{"a codex account only", []account.AccountWindows{yaRoomAccount("codex", "none")}, false, false},
		{"a codex account with numbers is not read", []account.AccountWindows{yaRoomAccount("codex", "codex", 10)}, false, false},
		{"full claude account beside an unread one", []account.AccountWindows{yaRoomAccount("claude", "statusline", 100), yaRoomAccount("claude", "none")}, false, true},
		{"no accounts", nil, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			room, known := yaAccountRoom(tc.list)
			if room != tc.room || known != tc.known {
				t.Fatalf("room %v known %v, want room %v known %v", room, known, tc.room, tc.known)
			}
		})
	}
}
