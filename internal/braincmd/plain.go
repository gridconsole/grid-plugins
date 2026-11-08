package braincmd

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

func facts(w io.Writer, prefix string, value any) error {
	switch v := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			name := k
			if prefix != "" {
				name = prefix + "." + k
			}
			if err := facts(w, name, v[k]); err != nil {
				return err
			}
		}
	case []any:
		if len(v) == 0 {
			_, err := fmt.Fprintf(w, "%s: []\n", prefix)
			return err
		}
		for i, item := range v {
			if err := facts(w, prefix+"["+strconv.Itoa(i)+"]", item); err != nil {
				return err
			}
		}
	case string:
		s := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(v, "\\", "\\\\"), "\n", "\\n"), "\r", "\\r")
		_, err := fmt.Fprintf(w, "%s: %s\n", prefix, s)
		return err
	default:
		_, err := fmt.Fprintf(w, "%s: %v\n", prefix, v)
		return err
	}
	return nil
}
