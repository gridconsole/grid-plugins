package judge

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"math/big"
)

// Post is one post of a judge verb: a job or control record on the task's Feed.
type Post struct {
	Task   string
	Type   string
	Job    string
	Body   map[string]any
	Wake   string
	Person string
}

// ID is the record id of the post. The Feed answers a repeat of a post it already wrote (the same
// id and the same record) with the record it holds, and refuses a Job id it holds to any other
// request, so a verb that runs again after a crash between the write and its own exit makes the
// same post again: the id comes only from what the post says, the task, the type, the Job id, the
// body bytes, the wake and the person. Never from memory, a clock or a lease epoch. The 128 bits
// are the head of the SHA-256 of the length-prefixed parts (the app's requestID).
func (p Post) ID() string {
	body, err := json.Marshal(p.Body)
	if err != nil {
		body = nil
	}
	return requestID(p.Task, p.Type, p.Job, body, p.Wake, p.Person)
}

func requestID(task, typ, job string, body []byte, wake, person string) string {
	h := sha256.New()
	for _, part := range []string{task, typ, job, string(body), wake, person} {
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], uint64(len(part)))
		h.Write(n[:])
		h.Write([]byte(part))
	}
	var id [16]byte
	copy(id[:], h.Sum(nil))
	return ulidString(id)
}

// ulidString writes 128 bits as the 26 Crockford base-32 digits of a ULID.
func ulidString(b [16]byte) string {
	n := new(big.Int).SetBytes(b[:])
	base := big.NewInt(32)
	rem := new(big.Int)
	var chars [26]byte
	for i := 25; i >= 0; i-- {
		n.QuoRem(n, base, rem)
		chars[i] = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"[rem.Int64()]
	}
	return string(chars[:])
}

// Control is the control post that sets a Job's status, with extra fields laid over it.
func Control(task, job, status, reason, wake string, extra map[string]any) Post {
	body := map[string]any{"target": job, "status": status, "reason": reason}
	for k, v := range extra {
		body[k] = v
	}
	return Post{Task: task, Type: "control", Job: job, Body: body, Wake: wake}
}
