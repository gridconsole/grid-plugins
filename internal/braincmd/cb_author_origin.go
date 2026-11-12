package braincmd

import "github.com/gridconsole/grid-plugins/internal/brain"

// cbAuProposeOrigin names the author a proposal is stamped with, from the
// caller's Env and never from a flag or the note text. In order:
//
//  1. a Job: Env.Job is a plain name and Env.Task (an id or a folder path, as
//     GRID_TASK is elsewhere) resolves to a task whose id is a plain name;
//     the origin is kind "job", id "<task id>/<job>";
//  2. a session: Env.Session is a plain name; kind "session", that id;
//  3. brain.SessionOrigin(), a random session id.
//
// A value that is not a plain name (empty, a "/", a newline, a space) is never
// written; the next source is used. A person never goes into a proposal's
// front matter, which allows session, job, server and import only; persons
// appear in audit lines, through hyDrOrigin.
func cbAuProposeOrigin(env Env) (brain.Origin, error) {
	if brain.ValidName(env.Job) && env.Task != "" && env.ResolveTask != nil {
		if t, err := env.ResolveTask(env.Task); err == nil && brain.ValidName(t.ID) {
			return brain.Origin{Kind: "job", ID: t.ID + "/" + env.Job}, nil
		}
	}
	if brain.ValidName(env.Session) {
		return brain.Origin{Kind: "session", ID: env.Session}, nil
	}
	return brain.SessionOrigin()
}
