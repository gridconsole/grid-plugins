package account

import "errors"

// RegisterKept is Status with register true for the folders keep accepts only: a
// login email no record has is registered (once, under the registry lock) when keep
// says its folder is kept, and a folder keep refuses is left out of the registry.
// It answers every registered account, as Status does. grid init uses it for the
// accounts the person kept; a nil keep keeps every folder.
func RegisterKept(env Env, keep func(Folder) bool) ([]Account, error) {
	if env.GridHome == "" {
		return nil, errors.New("cannot find the Grid home; set GRID_HOME")
	}
	unlock, err := lockRegistry(env)
	if err != nil {
		return nil, err
	}
	defer unlock()
	recs, err := loadRecords(env)
	if err != nil {
		return nil, err
	}
	folders := Scan(env)
	kept := make([]Folder, 0, len(folders))
	for _, f := range folders {
		if keep == nil || keep(f) {
			kept = append(kept, f)
		}
	}
	recs, changed, err := registerNew(env, recs, kept)
	if err != nil {
		return nil, err
	}
	if changed {
		if err := saveRecords(env, recs); err != nil {
			return nil, err
		}
	}
	return describe(env, recs, folders), nil
}
