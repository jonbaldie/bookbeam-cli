package integration_test

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestVersion(t *testing.T) {
	stamped := filepath.Join(t.TempDir(), "bookbeam.exe")
	build := exec.Command("go", "build", "-ldflags=-X github.com/jonbaldie/bookbeam-cli/cmd.Version=1.2.3", "-o", stamped, "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build stamped binary: %v\n%s", err, out)
	}

	for _, tc := range []struct {
		name, binary, version string
	}{
		{"default", binary, "dev"},
		{"stamped", stamped, "1.2.3"},
	} {
		for _, flag := range []string{"--version", "-v"} {
			t.Run(tc.name+"/"+flag, func(t *testing.T) {
				out, err := exec.Command(tc.binary, flag).CombinedOutput()
				if err != nil || string(out) != "bookbeam version "+tc.version+"\n" {
					t.Fatalf("version: err=%v output=%q", err, out)
				}
			})
		}
	}
}
