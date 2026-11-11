package lib

import "github.com/gridconsole/grid-plugins/internal/cli"

// InProcess reports whether Run may run args inside a host process, and why.
// False means the call belongs in the grid binary: it waits on a signal of the
// process, records the process id as an owner, serves or reads the host's own
// streams, opens the host's browser, starts a child with the host's
// environment, or writes a system schedule for the host's executable. The
// answer fails closed: an empty argv, an unknown verb, or a
// subverb grid does not name answers false. Run itself still runs any argv it is
// given; InProcess only answers the question.
func InProcess(args []string) (ok bool, why string) {
	return cli.InProcess(args)
}
