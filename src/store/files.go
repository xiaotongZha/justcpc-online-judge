package store

import (
	"fmt"
	"os"
	"path/filepath"
)

func (s *Store) Absolute(relative string) string { return filepath.Join(s.DataRoot, "store", relative) }

func (s *Store) EnsureLayout() error {
	for _, relative := range []string{"contest-statements", "contest-rosters", "tests", "submissions", "outputs"} {
		if err := os.MkdirAll(s.Absolute(relative), 0700); err != nil {
			return fmt.Errorf("create %s: %w", relative, err)
		}
	}
	return nil
}

func (s *Store) WriteFile(relative string, data []byte) error {
	path := s.Absolute(relative)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".write-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Chmod(0600); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryName, path)
}
