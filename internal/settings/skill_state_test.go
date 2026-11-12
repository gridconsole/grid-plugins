package settings

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadSkillsOffFollowsEffective(t *testing.T) {
	personal, proj := fixture(t)
	must(t, personal.SetSkill("research", false))
	must(t, personal.SetSkill("plan-jobs", false))
	must(t, proj.SetSkill("plan-jobs", true))
	for _, tc := range []struct {
		name, project string
		want          []string
	}{
		{"personal", "", []string{"plan-jobs", "research"}},
		{"project name", "example", []string{"research"}},
		{"project id", "project-id", []string{"research"}},
		{"no record reads the personal level", "no-such-project", []string{"plan-jobs", "research"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ReadSkillsOff(personal.Home, tc.project)
			must(t, err)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ReadSkillsOff = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestReadSkillsOffReadsOnly(t *testing.T) {
	personal, _ := fixture(t)
	if got, err := ReadSkillsOff(personal.Home, ""); err != nil || len(got) != 0 {
		t.Fatalf("no settings file: %v, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(personal.Home, "settings.json")); !os.IsNotExist(err) {
		t.Fatalf("the reader wrote settings.json: %v", err)
	}
	if _, err := ReadSkillsOff("", ""); err == nil {
		t.Fatal("an empty home must be an error")
	}
	must(t, os.WriteFile(filepath.Join(personal.Home, "settings.json"), []byte("{not json"), 0o600))
	if _, err := ReadSkillsOff(personal.Home, ""); err == nil {
		t.Fatal("a broken settings.json must be an error, not an empty answer")
	}
}
