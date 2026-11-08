package doctor

import "testing"

func TestRedact(t *testing.T) {
	tests := []struct{ name, input, home, want string }{
		{"bearer", "Authorization: Bearer example-secret==", "", "Authorization: [REDACTED]"},
		{"bearer case", "authorization: bEaReR\ttoken-value", "", "authorization: [REDACTED]"},
		{"key", "key=sk-abcdefghijklmnop_123", "", "key=[REDACTED]"},
		{"short key", "sk-short", "", "sk-short"},
		{"jws", "token=eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJleGFtcGxlIn0.c2lnbmF0dXJl", "", "token=[REDACTED]"},
		{"jws looking", "abc.def.ghi", "", "[REDACTED]"},
		{"detached jws", "eyJhbGciOiJIUzI1NiJ9..c2lnbmF0dXJl", "", "[REDACTED]"},
		{"versions", "grid 0.1.0 contract 1 app_min 10.12.30", "", "grid 0.1.0 contract 1 app_min 10.12.30"},
		{"pem", "before\n-----BEGIN PRIVATE KEY-----\nsecret\n-----END PRIVATE KEY-----\nafter", "", "before\n[REDACTED]\nafter"},
		{"pem blocks", "-----BEGIN CERTIFICATE-----\none\n-----END CERTIFICATE-----\n-----BEGIN RSA PRIVATE KEY-----\ntwo\n-----END RSA PRIVATE KEY-----", "", "[REDACTED]\n[REDACTED]"},
		{"unterminated pem", "before\n-----BEGIN PRIVATE KEY-----\nsecret", "", "before\n[REDACTED]"},
		{"home", "/home/example/project /home/example/.grid", "/home/example", "~/project ~/.grid"},
		{"windows home", `C:\Users\example\logs C:/Users/example/logs`, `C:\Users\example`, `~\logs ~/logs`},
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
