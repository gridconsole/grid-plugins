package land

import (
	"os"
	"path/filepath"

	"github.com/gridconsole/grid-plugins/internal/brain"
)

// brainFolder is where the project's deploy rule and workflow notes are read:
// the folder to open and the name of the folder inside a project level that holds
// the config topics. With the brain folder key set it is the brain folder and
// "grid" (projects/<p>/grid/deploy); with the key absent it is the Grid home and
// "knowledge" (projects/<p>/knowledge/deploy), as before.
func (s Store) brainFolder() (root, config string) {
	getenv := s.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	b := brain.Store{GridHome: s.Resolver.GridHome}.WithConfig(s.Resolver.Home, getenv)
	if b.Vault() {
		return b.Root(), "grid"
	}
	return b.Root(), "knowledge"
}

// projectTopic is the path below the folder brainFolder names of one config topic
// of a project.
func projectTopic(config, project, topic string) string {
	return filepath.Join("projects", project, config, topic)
}
