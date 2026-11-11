package contract

import (
	"strings"
	"testing"
)

// The run-job skill is what a Job agent reads, also when it is started without
// --agent. One paragraph of it tells the agent to post to the task's Feed with
// the feed_post tool while the result still goes with `grid job result`. These
// words are pinned, not the sentence, so a later wording fix stays green.

const imFeedSkill = "skills/run-job/SKILL.md"

var imFeedWords = []string{"feed_post", "Feed", "feed_types", "`grid job result`"}

// imFeedMissing returns the words of imFeedWords that paragraph lacks.
func imFeedMissing(paragraph string) []string {
	var missing []string
	for _, w := range imFeedWords {
		if !strings.Contains(paragraph, w) {
			missing = append(missing, w)
		}
	}
	return missing
}

func TestImFeedRunJobNamesFeedPost(t *testing.T) {
	text := strings.ReplaceAll(readPluginFile(t, imFeedSkill), "\r\n", "\n")
	var closest []string
	for _, paragraph := range strings.Split(text, "\n\n") {
		missing := imFeedMissing(paragraph)
		if len(missing) == 0 {
			return
		}
		if closest == nil || len(missing) < len(closest) {
			closest = missing
		}
	}
	t.Fatalf("run-job (%s): no paragraph names all of %q; the closest paragraph lacks %q", imFeedSkill, imFeedWords, closest)
}
