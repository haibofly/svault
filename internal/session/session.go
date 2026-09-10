package session

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultTTL is how long a cached password stays valid.
const DefaultTTL = 15 * time.Minute

type entry struct {
	Password string `json:"password"`
	Expires  int64  `json:"expires"`
}

type store struct {
	Entries map[string]entry `json:"entries"`
}

func newStore() *store {
	return &store{Entries: map[string]entry{}}
}

// FilePath returns the session cache file location.
func FilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".svault", "session")
}

// normalize makes a vault path a stable, case-insensitive cache key.
func normalize(vaultPath string) string {
	p, err := filepath.Abs(vaultPath)
	if err != nil {
		p = vaultPath
	}
	return strings.ToLower(filepath.Clean(p))
}

func load() *store {
	p := FilePath()
	if p == "" {
		return newStore()
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return newStore()
	}
	plain, err := unprotect(data)
	if err != nil {
		return newStore()
	}
	s := newStore()
	if err := json.Unmarshal(plain, s); err != nil {
		return newStore()
	}
	if s.Entries == nil {
		s.Entries = map[string]entry{}
	}
	return s
}

func save(s *store) error {
	p := FilePath()
	if p == "" {
		return errors.New("cannot determine home directory")
	}
	plain, err := json.Marshal(s)
	if err != nil {
		return err
	}
	blob, err := protect(plain)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, blob, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// Get returns the cached password for a vault if present and not expired.
func Get(vaultPath string) (string, bool) {
	s := load()
	k := normalize(vaultPath)
	e, ok := s.Entries[k]
	if !ok {
		return "", false
	}
	if time.Now().Unix() >= e.Expires {
		delete(s.Entries, k)
		_ = save(s)
		return "", false
	}
	return e.Password, true
}

// Put caches the password for a vault for the given ttl.
func Put(vaultPath, password string, ttl time.Duration) error {
	s := load()
	s.Entries[normalize(vaultPath)] = entry{
		Password: password,
		Expires:  time.Now().Add(ttl).Unix(),
	}
	return save(s)
}

// Delete removes the cached password for a vault.
func Delete(vaultPath string) error {
	s := load()
	delete(s.Entries, normalize(vaultPath))
	return save(s)
}

// Clear removes every cached password.
func Clear() error {
	return save(newStore())
}

// Status reports how long the vault stays unlocked, if at all.
func Status(vaultPath string) (time.Duration, bool) {
	s := load()
	e, ok := s.Entries[normalize(vaultPath)]
	if !ok {
		return 0, false
	}
	rem := time.Until(time.Unix(e.Expires, 0))
	if rem <= 0 {
		return 0, false
	}
	return rem, true
}
