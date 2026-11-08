// Package intent records requests for a worker; it never launches an agent.
package intent

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	ClaimTimeout = 5 * time.Minute
	SendWindow   = 5 * time.Second
	alphabet     = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
)

var (
	ErrClaimed = errors.New("intent is already claimed")
	ErrDone    = errors.New("intent is already done")
	ErrClaim   = errors.New("a current claim token is required")
)

// Record is the immutable request in intents/<id>.json, not a Feed post.
type Record struct {
	ID             string            `json:"id"`
	Verb           string            `json:"verb"`
	Target         string            `json:"target"`
	Args           map[string]string `json:"args"`
	Who            string            `json:"who"`
	Created        time.Time         `json:"created"`
	IdempotencyKey string            `json:"idempotency_key,omitempty"`
}

type Claim struct {
	Token   string    `json:"token"`
	Who     string    `json:"who"`
	Created time.Time `json:"created"`
	Until   time.Time `json:"until"`
}

type Completion struct {
	Token   string    `json:"token"`
	Created time.Time `json:"created"`
	Error   string    `json:"error,omitempty"`
}

type View struct {
	Record
	State string      `json:"state"`
	Claim *Claim      `json:"claim,omitempty"`
	Done  *Completion `json:"done,omitempty"`
}

// Store uses a stable fslock lock for transitions and an O_EXCL claim marker.
// Shared filesystems must support exclusive creation and distributed flock.
// Now is a test clock. Production workers must keep their clocks synchronized.
type Store struct {
	TaskDir string
	Now     func() time.Time
	// Link replaces os.Link in tests of the no-hard-link fallback.
	Link func(oldname, newname string) error
}

func (s Store) link() func(oldname, newname string) error {
	if s.Link != nil {
		return s.Link
	}
	return os.Link
}

func (s Store) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC().Truncate(time.Millisecond)
	}
	return time.Now().UTC().Truncate(time.Millisecond)
}

func newID(now time.Time) (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[6:]); err != nil {
		return "", err
	}
	ms := now.UnixMilli()
	if ms < 0 || ms >= 1<<48 {
		return "", errors.New("time is outside ULID range")
	}
	for i := 5; i >= 0; i-- {
		raw[i] = byte(ms)
		ms >>= 8
	}
	n := new(big.Int).SetBytes(raw[:])
	var out [26]byte
	mask := big.NewInt(31)
	for i := 25; i >= 0; i-- {
		out[i] = alphabet[new(big.Int).And(n, mask).Int64()]
		n.Rsh(n, 5)
	}
	return string(out[:]), nil
}

func validID(id string) bool {
	if len(id) != 26 || id[0] > '7' {
		return false
	}
	for _, c := range id {
		if !strings.ContainsRune(alphabet, c) {
			return false
		}
	}
	return true
}

// ValidateRequest rejects unknown verbs and ambiguous payloads before any write.
func ValidateRequest(verb, target string, args map[string]string, who string) error {
	if strings.TrimSpace(target) == "" || strings.TrimSpace(who) == "" {
		return errors.New("target and who are required")
	}
	allowed := map[string]bool{}
	required := ""
	switch verb {
	case "start":
		allowed["agent"] = true
	case "stop", "pause", "resume", "handback":
	case "send":
		allowed["text"] = true
		required = "text"
	case "switch":
		allowed["agent"] = true
		required = "agent"
	case "move":
	default:
		return fmt.Errorf("unknown intent verb %q", verb)
	}
	for k, v := range args {
		if !allowed[k] || strings.TrimSpace(v) == "" {
			return fmt.Errorf("invalid %s argument %q", verb, k)
		}
	}
	if required != "" && strings.TrimSpace(args[required]) == "" {
		return fmt.Errorf("%s requires %s", verb, required)
	}
	return nil
}

// Enqueue coalesces matching sends, including already completed sends, for five
// seconds from the original creation. Other verbs always receive fresh IDs.
func (s Store) Enqueue(verb, target string, args map[string]string, who, key string) (Record, error) {
	var result Record
	if err := ValidateRequest(verb, target, args, who); err != nil {
		return result, err
	}
	if key != "" && verb != "send" {
		return result, errors.New("idempotency keys apply only to send")
	}
	copied := map[string]string{}
	for k, v := range args {
		copied[k] = v
	}
	err := s.locked(true, func(dir string) error {
		now := s.now()
		if verb == "send" {
			// Even explicit keys are scoped to sender and target, and stored as hashes.
			value := any([]any{who, target, copied})
			if key != "" {
				value = []string{who, target, key}
			}
			b, err := json.Marshal(value)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(b)
			key = hex.EncodeToString(sum[:])
			records, err := listRecords(dir)
			if err != nil {
				return err
			}
			for _, r := range records {
				age := now.Sub(r.Created)
				if r.IdempotencyKey == key && age >= 0 && age < SendWindow {
					if r.Args["text"] != copied["text"] {
						return errors.New("idempotency key reused for different send text")
					}
					result = r
					return nil
				}
			}
		}
		id, err := newID(now)
		if err != nil {
			return err
		}
		result = Record{id, verb, target, copied, who, now, key}
		return publish(filepath.Join(dir, id+".json"), result, s.link())
	})
	return result, err
}

func listRecords(dir string) ([]Record, error) {
	records := []Record{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".done.json") {
			continue
		}
		id := strings.TrimSuffix(name, ".json")
		if !validID(id) {
			continue
		}
		r, err := readRecord(dir, id)
		if err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, nil
}

func readRecord(dir, id string) (Record, error) {
	var r Record
	if !validID(id) {
		return r, errors.New("intent id must be a canonical ULID")
	}
	if err := readJSON(filepath.Join(dir, id+".json"), &r); err != nil {
		return r, err
	}
	if r.ID != id || r.Created.IsZero() || r.Args == nil {
		return r, errors.New("invalid intent record")
	}
	return r, ValidateRequest(r.Verb, r.Target, r.Args, r.Who)
}

func state(dir string, r Record, now time.Time) (View, error) {
	v := View{Record: r, State: "pending"}
	var done Completion
	err := readJSON(filepath.Join(dir, r.ID+".done.json"), &done)
	if err == nil {
		v.State = "done"
		v.Done = &done
		return v, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return v, err
	}
	claim, active, err := readClaim(filepath.Join(dir, r.ID+".claim"), now)
	if err != nil {
		return v, err
	}
	if active {
		v.State = "claimed"
		v.Claim = claim
	}
	return v, nil
}

// List returns ULID-sorted requests; pending means available to claim now.
func (s Store) List(pending bool) ([]View, error) {
	result := []View{}
	err := s.locked(false, func(dir string) error {
		records, err := listRecords(dir)
		if err != nil {
			return err
		}
		now := s.now()
		for _, r := range records {
			v, err := state(dir, r, now)
			if err != nil {
				return err
			}
			if !pending || v.State == "pending" {
				result = append(result, v)
			}
		}
		return nil
	})
	return result, err
}

// Claim returns a new token only to the winner. Expired claims may be replaced.
func (s Store) Claim(id, who string) (Claim, error) {
	var result Claim
	if !validID(id) || strings.TrimSpace(who) == "" {
		return result, errors.New("valid intent id and who are required")
	}
	err := s.locked(false, func(dir string) error {
		r, err := readRecord(dir, id)
		if err != nil {
			return err
		}
		now := s.now()
		v, err := state(dir, r, now)
		if err != nil {
			return err
		}
		if v.State == "done" {
			return ErrDone
		}
		if v.State == "claimed" {
			return ErrClaimed
		}
		path := filepath.Join(dir, id+".claim")
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		token, err := newID(now)
		if err != nil {
			return err
		}
		result = Claim{token, who, now, now.Add(ClaimTimeout)}
		return exclusiveJSON(path, result)
	})
	if err == nil && result.Token == "" {
		err = os.ErrNotExist
	}
	return result, err
}

// Renew gives the holder of a live claim another ClaimTimeout from now. An
// expired, replaced or unknown token is refused with ErrClaim, so a worker that
// lost its claim cannot take it back from a second claimer.
func (s Store) Renew(id, token string) (Claim, error) {
	var result Claim
	if !validID(id) || !validID(token) {
		return result, ErrClaim
	}
	err := s.locked(false, func(dir string) error {
		r, err := readRecord(dir, id)
		if err != nil {
			return err
		}
		now := s.now()
		v, err := state(dir, r, now)
		if err != nil {
			return err
		}
		if v.State == "done" {
			return ErrDone
		}
		if v.Claim == nil || v.Claim.Token != token {
			return ErrClaim
		}
		result = *v.Claim
		if until := now.Add(ClaimTimeout); until.After(result.Until) {
			result.Until = until
		}
		return replaceJSON(filepath.Join(dir, id+".claim"), result)
	})
	if err == nil && result.Token == "" {
		err = os.ErrNotExist
	}
	return result, err
}

// Done requires the token from Claim. Expired workers cannot finish new claims.
func (s Store) Done(id, token, failure string) (Completion, error) {
	var result Completion
	if !validID(id) || !validID(token) {
		return result, ErrClaim
	}
	err := s.locked(false, func(dir string) error {
		r, err := readRecord(dir, id)
		if err != nil {
			return err
		}
		now := s.now()
		v, err := state(dir, r, now)
		if err != nil {
			return err
		}
		if v.Done != nil {
			if v.Done.Token != token || v.Done.Error != failure {
				return ErrDone
			}
			result = *v.Done
			return nil
		}
		if v.Claim == nil || v.Claim.Token != token {
			return ErrClaim
		}
		result = Completion{token, now, failure}
		return publish(filepath.Join(dir, id+".done.json"), result, s.link())
	})
	if err == nil && result.Token == "" {
		err = os.ErrNotExist
	}
	return result, err
}
