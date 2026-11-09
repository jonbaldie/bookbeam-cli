package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFormalCredentialsLayerAndPermissionsCases(t *testing.T) {
	for _, env := range []string{"", "env"} {
		for _, flag := range []string{"", "flag"} {
			for _, stored := range []string{"", "old"} {
				dir := t.TempDir()
				path := filepath.Join(dir, "config.json")
				if err := os.WriteFile(path, []byte(`{"host":"https://bookbeam.app","token":"`+stored+`"}`), 0644); err != nil {
					t.Fatal(err)
				}
				s, err := Resolve(dir, func(key string) string {
					if key == TokenEnv {
						return env
					}
					return ""
				}, Overrides{Token: flag})
				if err != nil {
					t.Fatal(err)
				}
				want := stored
				if env != "" {
					want = env
				}
				if flag != "" {
					want = flag
				}
				if s.Token() != want {
					t.Fatalf("effective %q want %q", s.Token(), want)
				}
				if err := s.StoreToken("new"); err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				var disk Config
				if err := json.Unmarshal(data, &disk); err != nil {
					t.Fatal(err)
				}
				info, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				if disk.Token != "new" || info.Mode().Perm() != 0600 {
					t.Fatalf("disk=%+v mode=%v", disk, info.Mode())
				}
			}
		}
	}
}

func TestFormalCredentialsFailedSaveIsAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"host":"https://bookbeam.app","token":"old"}`), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := Resolve(dir, func(string) string { return "" }, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	// A directory at the target deterministically makes rename fail, including under root.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := s.StoreToken("new"); err == nil {
		t.Fatal("expected save failure")
	}
	if s.Token() != "old" {
		t.Fatalf("failed save changed memory to %q", s.Token())
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		t.Fatalf("target should remain a directory %v", err)
	}
}

func TestFormalCredentialsFindingLogoutOverride(t *testing.T) {
	s, err := Resolve(t.TempDir(), func(key string) string {
		if key == TokenEnv {
			return "env"
		}
		return ""
	}, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.StoreToken(""); err != nil {
		t.Fatal(err)
	}
	if s.Token() != "env" {
		t.Fatal("logout override finding not reproduced")
	}
}

func TestFormalConfigLoadMissingAndInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	s, err := Resolve(dir, func(string) string { return "" }, Overrides{})
	if err != nil || s.Host() != "https://bookbeam.app" {
		t.Fatalf("missing config defaults %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("load created a file")
	}
	if err := os.WriteFile(path, []byte(`broken`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(dir, func(string) string { return "" }, Overrides{}); err == nil {
		t.Fatal("accepted invalid config")
	}
}

type formalFailingConfigFile struct {
	configFile
	stage string
}

func (f formalFailingConfigFile) Write(data []byte) (int, error) {
	if f.stage == "write" {
		n, _ := f.configFile.Write(data[:len(data)/2])
		return n, errors.New("injected partial write")
	}
	return f.configFile.Write(data)
}
func (f formalFailingConfigFile) Close() error {
	err := f.configFile.Close()
	if f.stage == "close" {
		return errors.New("injected close")
	}
	return err
}

func TestFormalCredentialsFailedSaveStages(t *testing.T) {
	for _, stage := range []string{"create", "write", "close"} {
		t.Run(stage, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.json")
			old := []byte(`{"host":"https://bookbeam.app","token":"old"}`)
			if err := os.WriteFile(path, old, 0600); err != nil {
				t.Fatal(err)
			}
			s, err := Resolve(dir, func(string) string { return "" }, Overrides{})
			if err != nil {
				t.Fatal(err)
			}
			original := createTemp
			createTemp = func(dir, pattern string) (configFile, error) {
				if stage == "create" {
					return nil, errors.New("injected create")
				}
				file, err := original(dir, pattern)
				if err != nil {
					return nil, err
				}
				return formalFailingConfigFile{configFile: file, stage: stage}, nil
			}
			defer func() { createTemp = original }()
			if err := s.StoreToken("new"); err == nil {
				t.Fatal("expected save error")
			}
			data, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(data, old) || s.Token() != "old" {
				t.Fatalf("failed %s changed state memory=%q disk=%q err=%v", stage, s.Token(), data, err)
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 1 {
				t.Fatalf("temporary file leaked after %s: %v %v", stage, entries, err)
			}
		})
	}
}
