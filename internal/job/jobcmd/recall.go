package jobcmd

import (
	"strings"
	"unicode/utf8"

	"github.com/gridconsole/grid-plugins/internal/job"
)

// recallLimit is the number of hits asked of the brain search. It adds no limit
// of its own: job.Render shows at most 8 brain blocks, so a ninth hit could
// never be shown.
const recallLimit = 8

// recallWhy is the one fixed reason a recalled note carries in the Job.
const recallWhy = "recalled: matches the Job's title and intent words"

// recall returns the brain notes whose words match the Job's title and intent,
// as links for job.Render to rank after the planner's own links. The search runs
// here so that Render stays pure. A Job in the Personal project has no project
// brain, so the store searches the Grid home's brain only. A missing brain or a
// search error gives no recall; the render goes on without it and prints nothing.
func (w *world) recall(j job.Job) []job.Link {
	query := []string{j.Title}
	for _, p := range []string{j.Intent.Item, j.Intent.Goal} {
		if s, err := w.files.Resolve(p); err == nil {
			query = append(query, s.Text)
		}
	}
	hits, err := w.store.Search(strings.Join(query, " "), "", recallLimit)
	if err != nil {
		return nil
	}
	links := make([]job.Link, 0, len(hits))
	for _, h := range hits {
		// A hit Render could not show (no or overlong summary) is left out, so
		// recall never makes a render fail that would pass without it.
		s, err := w.files.Resolve(h.ID)
		if err != nil || (!s.PointerOnly && (strings.TrimSpace(s.Summary) == "" || utf8.RuneCountInString(s.Summary) > 300)) {
			continue
		}
		links = append(links, job.Link{To: h.ID, Rel: "uses", Why: recallWhy})
	}
	return links
}
