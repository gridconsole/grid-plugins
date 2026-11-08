package doctor

import "testing"

func TestRedact(t *testing.T) {
	tests := []struct{ name, input, home, want string }{
		{"bearer", "Authorization: Bearer example-secret==", "", "Authorization: [REDACTED]"},
		{"bearer case", "authorization: bEaReR\ttoken-value", "", "authorization: [REDACTED]"},
		{"key", "key=sk-abcdefghijklmnop_123", "", "key=[REDACTED]"},
		{"short key", "sk-short", "", "sk-short"},
		{"jws", "token=eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJleGFtcGxlIn0.c2lnbmF0dXJl", "", "token=[REDACTED]"},
		{"dotted text", "abc.def.ghi", "", "abc.def.ghi"},
		{"detached jws", "eyJhbGciOiJIUzI1NiJ9..c2lnbmF0dXJl", "", "[REDACTED]"},
		{"versions", "grid 0.1.0 contract 1 app_min 10.12.30", "", "grid 0.1.0 contract 1 app_min 10.12.30"},
		{"pem", "before\n-----BEGIN PRIVATE KEY-----\nsecret\n-----END PRIVATE KEY-----\nafter", "", "before\n[REDACTED]\nafter"},
		{"pem blocks", "-----BEGIN CERTIFICATE-----\none\n-----END CERTIFICATE-----\n-----BEGIN RSA PRIVATE KEY-----\ntwo\n-----END RSA PRIVATE KEY-----", "", "[REDACTED]\n[REDACTED]"},
		{"unterminated pem", "before\n-----BEGIN PRIVATE KEY-----\nsecret", "", "before\n[REDACTED]"},
		{"home", "/home/example/project /home/example/.grid", "/home/example", "~/project ~/.grid"},
		{"windows home", `C:\Users\example\logs C:/Users/example/logs`, `C:\Users\example`, `~\logs ~/logs`},
		{"windows JSON home", `{"path":"C:\\Users\\example\\logs"}`, `C:\Users\example`, `{"path":"~\\logs"}`},
		{"windows lower case", `c:\users\example\logs c:/users/example/logs`, `C:\Users\example`, `~\logs ~/logs`},
		{"windows lower JSON", `c:\\users\\example\\logs`, `C:\Users\example`, `~\\logs`},
		{"windows shell home", `/c/Users/example/logs /C/users/EXAMPLE/logs`, `C:\Users\example`, `~/logs ~/logs`},
		{"windows slash configured home", `C:\\Users\\example\\logs /c/Users/example/logs`, `C:/Users/example`, `~\\logs ~/logs`},
		{"home exact", "/home/example", "/home/example", "~"},
		{"home prefix boundary", "/home/example-two /home/example/logs", "/home/example", "/home/example-two ~/logs"},
		{"hostnames", "grid.example.com https://api.example.com/status", "", "grid.example.com https://api.example.com/status"},
		{"file names", "task.test.mjs run.test.log", "", "task.test.mjs run.test.log"},
		{"slug", "risk-assessment-for-example-q4", "", "risk-assessment-for-example-q4"},
		{"email", "contact person+support@example.com", "", "contact [REDACTED]"},
		{"JSON secrets", `{"key":"sk-abcdefghijklmnop","auth":"Bearer example-token"}`, "", `{"key":"[REDACTED]","auth":"[REDACTED]"}`},
		{"empty home", "ordinary diagnostic", "", "ordinary diagnostic"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Redact(tc.input, tc.home); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
