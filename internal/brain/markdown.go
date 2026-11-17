package brain

import (
	"fmt"
	"slices"
	"strings"

	"go.yaml.in/yaml/v4"

	"github.com/gridconsole/grid-plugins/internal/mdfile"
)

// Encode writes a note as Markdown with YAML front matter. The same library
// reads it back, so whatever Validate accepts, including control characters,
// round-trips. A proposal is written whole, with its origin and links as front
// matter, because it is Grid's own file, and so is an accepted proposal, which
// still carries the origin of its author. A filed note (accepted, with no origin)
// is a note a person also reads and edits in Obsidian: flat front matter (the
// author is the flat source), and its links as a ## Links section in the body.
// EncodeOver writes a filed note over the file that exists.
func Encode(n Note) ([]byte, error) {
	if n.Links == nil {
		n.Links = []Link{}
	}
	if n.Status == "accepted" && n.Origin == (Origin{}) {
		return encodeFiled(n)
	}
	return EncodeConfig(n, n.Body)
}

// Parse reads a note's front matter strictly: an unknown field, a duplicate key,
// an anchor, an alias or an explicit tag is an error rather than a silent change.
// The one exception is a filed (accepted) note, which a person edits too: a key
// Grid does not know is the person's and is skipped, not an error. The ## Links
// section Grid writes is read into Links and is not part of Body.
func Parse(data []byte) (Note, error) {
	front, body, err := mdfile.Split(data)
	if err != nil {
		return Note{}, err
	}
	var n Note
	if err := mdfile.Load(front, &n, true); err != nil {
		var filed Note
		if mdfile.Load(front, &filed, false) != nil || filed.Status != "accepted" {
			return Note{}, err
		}
		n = filed
	}
	// An origin key that is there must name an origin: a filed note may leave the key out, not leave it empty.
	var probe struct {
		Origin yaml.Node `yaml:"origin"`
	}
	if mdfile.Load(front, &probe, false) == nil && probe.Origin.Kind != 0 && n.Origin == (Origin{}) {
		return Note{}, fmt.Errorf("origin must name a kind and an id, or the key must be left out")
	}
	n.Body = strings.ReplaceAll(body, "\r\n", "\n")
	if plain, links := splitLinks(n.Body, n.Level); len(links) > 0 {
		n.Body = plain
		for _, l := range links {
			if !slices.ContainsFunc(n.Links, func(h Link) bool { return h.To == l.To && h.Rel == l.Rel }) {
				n.Links = append(n.Links, l)
			}
		}
	}
	if n.Links == nil {
		n.Links = []Link{}
	}
	return n, nil
}
