// Package feed owns the local, append-only task Feed and its deterministic fold.
package feed

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

const MaxLine = 32 * 1024

var ulidPattern = regexp.MustCompile(`^[0-7][0-9A-HJKMNP-TV-Z]{25}$`)
var jobPattern = regexp.MustCompile(`^J-[0-9]+(?:\.[0-9]+)*$`)
var memberPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.@+\-]{0,199}$`)

// Author is stamped from the caller's session, never from its body: the
// contract's {id, role}, where id is the member part of the writer id.
type Author = contract.Author

// Record contains every envelope field, including explicit nulls and an empty
// after array. At is display information and is never used to decide state.
type Record struct {
	ID      string             `json:"id"`
	Task    string             `json:"task"`
	Type    string             `json:"type"`
	TypeV   int                `json:"type_v"`
	Author  Author             `json:"author"`
	Tier    contract.TrustTier `json:"tier"`
	Worker  string             `json:"worker"`
	Lease   *string            `json:"lease"`
	Gen     int64              `json:"gen"`
	Fence   int64              `json:"fence"`
	Seq     int64              `json:"seq"`
	At      string             `json:"at"`
	Re      *string            `json:"re"`
	After   []string           `json:"after"`
	Job     *string            `json:"job"`
	Attempt *int               `json:"attempt"`
	Wake    *string            `json:"wake"`
	Body    map[string]any     `json:"body"`
}

// Input describes a post. Set ID to reuse an id across retries.
type Input struct {
	ID, Type      string
	Body          map[string]any
	Re, Job, Wake string
	After         []string
	Attempt       int
}

// Writer names a session and, optionally, the lease tenure it holds.
type Writer struct {
	Author        Author
	Worker, Lease string
	Gen, Fence    int64
}

// NewID creates a ULID using the clock and 80 cryptographically random bits.
func NewID() (string, error) {
	var raw [16]byte
	ms := uint64(time.Now().UnixMilli())
	for i := 5; i >= 0; i-- {
		raw[i] = byte(ms)
		ms >>= 8
	}
	if _, err := rand.Read(raw[6:]); err != nil {
		return "", err
	}
	n := new(big.Int).SetBytes(raw[:])
	alphabet := "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	var out [26]byte
	for i := 25; i >= 0; i-- {
		out[i] = alphabet[new(big.Int).And(n, big.NewInt(31)).Int64()]
		n.Rsh(n, 5)
	}
	return string(out[:]), nil
}

func IsID(id string) bool { return ulidPattern.MatchString(id) }
func ptr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
func value(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
func number(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int:
		return int64(n)
	case int64:
		return n
	case json.Number:
		i, _ := n.Int64()
		return i
	}
	return 0
}
func str(v any) string                             { s, _ := v.(string); return s }
func object(v any) map[string]any                  { m, _ := v.(map[string]any); return m }
func writerFile(worker string, fence int64) string { return fmt.Sprintf("%s.%d.jsonl", worker, fence) }
func validWorker(worker string) bool {
	i := strings.LastIndexByte(worker, '@')
	return i > 0 && memberPattern.MatchString(worker[:i]) && IsID(worker[i+1:])
}

// Data reports whether the body is untrusted text, shown only in a data fence.
// The tier follows the role; a body marked external is data for any author.
func (r Record) Data() bool {
	return r.Tier == contract.PersonGuest || r.Tier == contract.External || r.Body["external"] == true
}

// dataTier is the fence label: external when the body says so, else the tier.
func (r Record) dataTier() string {
	if r.Body["external"] == true && r.Tier != contract.PersonGuest {
		return string(contract.External)
	}
	return string(r.Tier)
}
