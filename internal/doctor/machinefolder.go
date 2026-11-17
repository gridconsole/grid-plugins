package doctor

import "github.com/gridconsole/grid-plugins/internal/machinefolder"

// AddMachineFolder appends the rows of the machine folder to rep, after the rows Run
// made, and returns it. The rows show only when the folder is configured: the Grid
// config names a machine folder or a shared group, the environment override is
// set, or one of those was not usable. With none of them it adds nothing, so the
// default list is the one the README names. Each failing row carries the command a person runs to
// fix it; nothing here changes the folder, and a failing row blocks no command.
//
// Run does not call this: it resolves nothing from the machine itself. The caller
// passes the Env and the check options (a zero CheckOptions reads the real groups).
func AddMachineFolder(rep Report, e machinefolder.Env, o machinefolder.CheckOptions) Report {
	f := machinefolder.Resolve(e)
	if !f.Configured() {
		return rep
	}
	o.Env = e
	for _, row := range machinefolder.Check(f, o) {
		rep.Checks = append(rep.Checks, Check{Name: row.Name, OK: row.OK, Detail: row.Detail, Fix: row.Fix})
		if !row.OK {
			rep.OK = false
		}
	}
	return rep
}
