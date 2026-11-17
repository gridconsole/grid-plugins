package machinefolder

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

// realPath makes path absolute and resolves the symlinks in its longest existing
// prefix, as gridhome.Real does, so two spellings of one place compare equal. It is
// here because this package cannot import gridhome: gridhome reads gridconfig, and
// gridconfig names the OS standard place through DefaultPath.
func realPath(path string) string {
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
	}
	rest := ""
	for p := path; ; {
		if real, err := filepath.EvalSymlinks(p); err == nil {
			return filepath.Join(real, rest)
		}
		parent := filepath.Dir(p)
		if parent == p {
			return path
		}
		rest = filepath.Join(filepath.Base(p), rest)
		p = parent
	}
}

// Finding is one row for grid doctor: a name, whether it holds, one line of
// detail, and the command a person runs to fix it. Grid never runs the fix.
type Finding struct {
	Name   string
	OK     bool
	Detail string
	Fix    string
}

// Row names. The group rows exist in shared mode only.
const (
	RowFolder   = "machine folder"
	RowGroup    = "machine folder group"
	RowMode     = "machine folder mode"
	RowLogin    = "machine folder login"
	RowLeftover = "machine folder leftover"
)

// CheckOptions are the seams of Check. A nil function reads the real system; tests
// supply their own so none depends on the groups of the machine they run on.
type CheckOptions struct {
	Env         Env
	Login       string                       // "" is CurrentLogin
	LookupGroup func(string) (string, error) // group name to gid
	GroupName   func(string) (string, error) // gid to group name
	Groups      func() ([]string, error)     // gids of the running login
}

func (o CheckOptions) lookupGroup(name string) (string, error) {
	if o.LookupGroup != nil {
		return o.LookupGroup(name)
	}
	g, err := user.LookupGroup(name)
	if err != nil {
		return "", err
	}
	return g.Gid, nil
}

func (o CheckOptions) groupName(gid string) string {
	var name string
	var err error
	if o.GroupName != nil {
		name, err = o.GroupName(gid)
	} else {
		var g *user.Group
		if g, err = user.LookupGroupId(gid); err == nil {
			name = g.Name
		}
	}
	if err != nil || name == "" {
		return "gid " + gid
	}
	return name
}

func (o CheckOptions) groups() ([]string, error) {
	if o.Groups != nil {
		return o.Groups()
	}
	u, err := user.Current()
	if err != nil {
		return nil, err
	}
	return u.GroupIds()
}

func (o CheckOptions) login() string {
	if o.Login != "" {
		return o.Login
	}
	if l := CurrentLogin(o.Env.Getenv); l != "" {
		return l
	}
	return "<login>"
}

// Check returns the doctor rows for the folder. It reads and changes nothing but
// a short-lived temp file it makes to test that the login can write. A wrong
// group, a missing setgid bit, a folder that is not there and a private folder
// left behind are findings with a fix, never a refusal of any command.
func Check(f Folder, o CheckOptions) []Finding {
	var out []Finding
	add := func(name string, ok bool, detail, fix string) {
		if ok {
			fix = ""
		}
		out = append(out, Finding{Name: name, OK: ok, Detail: detail, Fix: fix})
	}
	login := o.login()
	switch {
	case f.Problem != "" && f.Path == "":
		add(RowFolder, false, f.Problem, "Fix or remove "+KeyMachineFolder+" in the Grid config, or set "+EnvDir+".")
		return out
	case f.Path == "":
		add(RowFolder, false, "the machine folder cannot be placed: the login has no home folder", "Set "+EnvDir+" to an absolute folder.")
		return out
	}
	known := folderRow(f, o, login, add)
	if f.Problem != "" {
		add(RowFolder, false, f.Problem, "Fix or remove "+KeyMachineFolder+" in the Grid config.")
	}
	if f.Shared() && known != nil {
		groupRows(f, o, login, known, add)
	}
	leftoverRow(f, o, f.SharedGroup, add)
	return out
}

// folderRow says whether the folder is there and the login can write in it. It
// returns the folder's FileInfo when there is one.
func folderRow(f Folder, o CheckOptions, login string, add func(string, bool, string, string)) os.FileInfo {
	setup := strings.Join(SharedCommands(f.Path, f.SharedGroup, login, o.groupKnown(f.SharedGroup)), " && ")
	fix := setup
	if !f.Shared() {
		fix = "Give your login write permission on " + f.Path + ", or point " + KeyMachineFolder + " at a folder it can write."
	}
	st, err := os.Stat(f.Path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if f.Shared() {
			add(RowFolder, false, f.Path+" does not exist; setup does not create a shared folder", setup)
		} else {
			add(RowFolder, true, f.Path+" (not made yet)", "")
		}
		return nil
	case err != nil:
		add(RowFolder, false, "cannot read "+f.Path, fix)
		return nil
	case !st.IsDir():
		add(RowFolder, false, f.Path+" is not a folder", "Move the file away, or point "+KeyMachineFolder+" at a folder.")
		return nil
	case !writable(f.Path):
		add(RowFolder, false, "cannot write in "+f.Path, fix)
		return st
	}
	detail := f.Path
	if f.Shared() {
		detail += " (shared with group " + f.SharedGroup + ")"
	}
	add(RowFolder, true, detail, "")
	return st
}

func (o CheckOptions) groupKnown(name string) bool {
	if name == "" {
		return true
	}
	_, err := o.lookupGroup(name)
	return err == nil
}

// groupRows are the shared-mode rows: the owning group, the mode and the login.
func groupRows(f Folder, o CheckOptions, login string, st os.FileInfo, add func(string, bool, string, string)) {
	want, err := o.lookupGroup(f.SharedGroup)
	if err != nil {
		add(RowGroup, false, "group "+f.SharedGroup+" does not exist on this machine",
			"sudo groupadd "+quote(f.SharedGroup)+" && sudo chgrp -R "+quote(f.SharedGroup)+" "+quote(f.Path))
		want = ""
	}
	have, haveOK := groupOf(st)
	switch {
	case want == "":
	case !haveOK:
		add(RowGroup, true, "skipped: the owning group is not read on this system", "")
	case have != want:
		add(RowGroup, false, fmt.Sprintf("%s belongs to group %s, not %s", f.Path, o.groupName(have), f.SharedGroup),
			"sudo chgrp -R "+quote(f.SharedGroup)+" "+quote(f.Path))
	default:
		add(RowGroup, true, f.Path+" belongs to group "+f.SharedGroup, "")
	}

	mode := st.Mode()
	switch {
	case mode.Perm()&0o070 != 0o070:
		add(RowMode, false, fmt.Sprintf("%s has mode %s; the group cannot read, write and enter it", f.Path, modeText(mode)),
			"sudo chmod 2770 "+quote(f.Path))
	case o.Env.goos() == "linux" && mode&os.ModeSetgid == 0:
		add(RowMode, false, fmt.Sprintf("%s has mode %s; the setgid bit is missing, so new files take their maker's own group", f.Path, modeText(mode)),
			"sudo chmod 2770 "+quote(f.Path))
	default:
		add(RowMode, true, f.Path+" has mode "+modeText(mode), "")
	}

	if want == "" {
		return
	}
	gids, err := o.groups()
	switch {
	case err != nil:
		add(RowLogin, true, "skipped: the groups of "+login+" were not read", "")
	case !contains(gids, want):
		add(RowLogin, false, "login "+login+" is not in group "+f.SharedGroup,
			"sudo usermod -aG "+quote(f.SharedGroup)+" "+quote(login)+" (then log in again)")
	default:
		add(RowLogin, true, "login "+login+" is in group "+f.SharedGroup, "")
	}
}

// leftoverRow reports a private machine folder at the OS standard place that holds
// something while the folder in use is another. Nothing moves it: the move is a
// person's step.
func leftoverRow(f Folder, o CheckOptions, group string, add func(string, bool, string, string)) {
	old := DefaultPath(o.Env)
	if old == "" || f.Path == "" || realPath(old) == realPath(f.Path) {
		add(RowLeftover, true, "no private machine folder left behind", "")
		return
	}
	entries, err := os.ReadDir(old)
	if err != nil || len(entries) == 0 {
		add(RowLeftover, true, "no private machine folder left behind", "")
		return
	}
	fix := "Stop the launchers, then: cp -a " + quote(old+"/.") + " " + quote(f.Path+"/")
	if group != "" {
		fix += " && sudo chgrp -R " + quote(group) + " " + quote(f.Path) + " && sudo chmod -R g+rwX " + quote(f.Path) +
			" && sudo find " + quote(f.Path) + " -type d -exec chmod 2770 {} +"
	}
	fix += "; when the copy is checked, remove " + quote(old)
	add(RowLeftover, false, "a private machine folder from before is still at "+old+"; Grid left it in place and does not move it", fix)
}

// SharedCommands are the commands a person runs to make a shared folder: the
// group when it does not exist yet, the folder, its group, setgid mode 2770, and
// the login in the group. Grid prints them and never runs them.
func SharedCommands(path, group, login string, groupExists bool) []string {
	var c []string
	if !groupExists {
		c = append(c, "sudo groupadd "+quote(group))
	}
	return append(c,
		"sudo mkdir -p "+quote(path),
		"sudo chgrp "+quote(group)+" "+quote(path),
		"sudo chmod 2770 "+quote(path),
		"sudo usermod -aG "+quote(group)+" "+quote(login))
}

// SharedPlan is what grid setup --shared-group finds before it writes anything.
type SharedPlan struct {
	Path     string
	Group    string
	Ready    bool     // the folder exists and the login can write in it
	Why      string   // when not Ready
	Commands []string // when not Ready: what a person runs
}

// PlanShared looks at the folder the config would use for a shared group. It does
// not make, chmod or chgrp anything.
func PlanShared(e Env, group string, o CheckOptions) SharedPlan {
	f := Resolve(e)
	p := SharedPlan{Path: f.Path, Group: group}
	o.Env = e
	login := o.login()
	if f.Path == "" {
		p.Why = "the machine folder cannot be placed: the login has no home folder"
		return p
	}
	p.Commands = SharedCommands(f.Path, group, login, o.groupKnown(group))
	st, err := os.Stat(f.Path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		p.Why = f.Path + " does not exist"
	case err != nil:
		p.Why = "cannot read " + f.Path
	case !st.IsDir():
		p.Why = f.Path + " is not a folder"
	case !writable(f.Path):
		p.Why = "cannot write in " + f.Path
	default:
		p.Ready, p.Commands = true, nil
	}
	return p
}

// writable reports whether the login can make a file in dir, with a temp file it
// removes at once. It chmods nothing.
func writable(dir string) bool {
	f, err := os.CreateTemp(dir, ".doctor-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	return os.Remove(name) == nil
}

// modeText spells a folder's mode in octal with the setgid bit, as chmod does:
// 2770.
func modeText(m os.FileMode) string {
	n := int(m.Perm())
	if m&os.ModeSetuid != 0 {
		n |= 0o4000
	}
	if m&os.ModeSetgid != 0 {
		n |= 0o2000
	}
	if m&os.ModeSticky != 0 {
		n |= 0o1000
	}
	return fmt.Sprintf("%04o", n)
}

// quote spells s for a shell when it holds anything but plain path characters.
func quote(s string) string {
	plain := s != ""
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("/._-+=:@%,", r)) {
			plain = false
			break
		}
	}
	if plain {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
