package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	AIRunner  string `yaml:"ai_runner,omitempty"`
	AIConsent *bool  `yaml:"ai_consent,omitempty"`
}

type Store struct {
	Config   Config
	Path     string
	CacheDir string
}

func Load() (*Store, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}

	appConfigDir := filepath.Join(configDir, "slicediff")
	path := filepath.Join(appConfigDir, "config.yaml")
	store := &Store{
		Path:     path,
		CacheDir: filepath.Join(cacheDir, "slicediff"),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return store, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(data, &store.Config); err != nil {
		return nil, fmt.Errorf("could not parse %s: %w", path, err)
	}
	return store, nil
}

func (s *Store) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(s.Config)
	if err != nil {
		return err
	}
	return os.WriteFile(s.Path, data, 0o600)
}

func (s *Store) SetConsent(value bool) error {
	s.Config.AIConsent = &value
	return s.Save()
}

func (s *Store) SetRunner(runner string) error {
	s.Config.AIRunner = runner
	return s.Save()
}
