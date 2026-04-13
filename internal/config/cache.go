package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func BuildSliceCacheKey(owner, repo string, number int, headSHA, runner, promptVersion, appVersion string) string {
	raw := strings.Join([]string{
		owner,
		repo,
		intString(number),
		headSHA,
		runner,
		promptVersion,
		appVersion,
	}, "\x00")
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (s *Store) ReadJSON(key string, dest any) error {
	data, err := os.ReadFile(s.cachePath(key))
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

func (s *Store) WriteJSON(key string, value any) error {
	if err := os.MkdirAll(s.CacheDir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.cachePath(key), data, 0o600)
}

func (s *Store) cachePath(key string) string {
	return filepath.Join(s.CacheDir, key+".json")
}

func intString(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
