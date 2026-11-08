package brain

import "github.com/gridconsole/grid-plugins/internal/mdfile"

// Encode writes a note as Markdown with YAML front matter. The same library
// reads it back, so whatever Validate accepts, including control characters,
// round-trips.
func Encode(n Note) ([]byte, error) {
	if n.Links == nil {
		n.Links = []Link{}
	}
	return EncodeConfig(n, n.Body)
}

// Parse reads a note's front matter strictly: an unknown field, a duplicate key,
// an anchor, an alias or an explicit tag is an error rather than a silent change.
func Parse(data []byte) (Note, error) {
	front, body, err := mdfile.Split(data)
	if err != nil {
		return Note{}, err
	}
	n := Note{Body: body}
	if err := mdfile.Load(front, &n, true); err != nil {
		return Note{}, err
	}
	if n.Links == nil {
		n.Links = []Link{}
	}
	return n, nil
}
