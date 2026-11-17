package migrate

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// A worktree backlink or a plugin session pointer lives outside the Grid home, so
// both versions must be journaled. Changes are applied only after the complete new
// home has been published, and reversed by a rollback.
type pointerChange struct {
	Path    string `json:"path"`
	Before  string `json:"before"`
	After   string `json:"after"`
	Mode    uint32 `json:"mode"`
	Session bool   `json:"session,omitempty"` // a plugin session pointer, not a worktree backlink
}

func (p *Plan) rewriteWorktrees(paths map[string]string) {
	for _, a := range p.Actions {
		if a.Kind != "copy" || path.Base(a.From) != ".git" {
			continue
		}
		e := p.entryOf(a.To)
		oldFile := e.Src
		newFile, moved := mapPath(oldFile, paths)
		if !moved || newFile == oldFile {
			continue
		}
		oldTree := filepath.Dir(oldFile)
		refuse := func(reason, common string) {
			if common == "" {
				// Not a linked worktree that git can remove: git refuses a main working tree.
				p.Unmapped = append(p.Unmapped, a.From+": "+reason+"; stop its writers, then move "+shellQuote(oldTree)+" outside the Grid home and run grid migrate again")
				return
			}
			p.Unmapped = append(p.Unmapped, a.From+": "+reason+"; stop its writers, check it for untracked or modified files (git asks for --force over them), then remove the worktree with: git --git-dir "+shellQuote(common)+" worktree remove -- "+shellQuote(oldTree))
		}
		if !e.Mode.IsRegular() {
			refuse("embedded git metadata cannot be relocated automatically; move an embedded main repository outside the home", "")
			continue
		}
		raw, err := readSmall(oldFile, e.Size)
		if err != nil || !strings.HasPrefix(string(raw), "gitdir: ") {
			refuse("invalid gitdir pointer", "")
			continue
		}
		gitdir := strings.TrimSpace(strings.TrimPrefix(string(raw), "gitdir: "))
		if !filepath.IsAbs(gitdir) {
			gitdir = filepath.Join(oldTree, gitdir)
		}
		gitdir = gridhome.Real(gitdir)
		commonData, err := os.ReadFile(filepath.Join(gitdir, "commondir"))
		if err != nil {
			refuse("cannot read worktree commondir", "")
			continue
		}
		common := strings.TrimSpace(string(commonData))
		if !filepath.IsAbs(common) {
			common = filepath.Join(gitdir, common)
		}
		common = gridhome.Real(common)
		if within(gitdir, p.From) || within(gitdir, p.Home) || within(common, p.From) || within(common, p.Home) {
			refuse("repository metadata is inside the home being moved", common)
			continue
		}
		backlink := filepath.Join(gitdir, "gitdir")
		info, err := os.Lstat(backlink)
		if err != nil || !info.Mode().IsRegular() {
			refuse("missing or non-regular gitdir backlink", common)
			continue
		}
		before, err := readSmall(backlink, info.Size())
		if err != nil || gridhome.Real(strings.TrimSpace(string(before))) != oldFile {
			refuse("gitdir backlink does not name this worktree", common)
			continue
		}
		p.pointers = append(p.pointers, pointerChange{Path: backlink, Before: string(before), After: newFile + "\n", Mode: uint32(info.Mode().Perm())})
		e.Data = []byte("gitdir: " + gitdir + "\n")
		e.Size = int64(len(e.Data))
		p.setEntry(a.To, e)
		p.Notes = append(p.Notes, a.From+": gitdir and repository backlink will be rewritten")
	}
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }

func checkPointers(changes []pointerChange, allowAfter bool) error {
	for _, c := range changes {
		info, err := os.Lstat(c.Path)
		if err != nil {
			return err
		}
		what := "worktree backlink"
		if c.Session {
			what = "plugin session pointer"
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file: %s", what, c.Path)
		}
		b, err := readSmall(c.Path, info.Size())
		if err != nil {
			return err
		}
		if string(b) != c.Before && (!allowAfter || string(b) != c.After) {
			return fmt.Errorf("%s changed: %s; stop writers and resolve it before retrying", what, c.Path)
		}
	}
	return nil
}

func applyPointers(changes []pointerChange) error {
	if err := checkPointers(changes, true); err != nil {
		return err
	}
	for _, c := range changes {
		// Already completed changes are safe to repeat after a crash.
		if err := atomicfile.Write(c.Path, []byte(c.After), os.FileMode(c.Mode)); err != nil {
			return err
		}
		reached("worktree-rewritten")
	}
	return nil
}
