// Package secrets is a small file-backed store for optional engine
// credentials (e.g. VIRUSTOTAL_API_KEY, ESET_LICENSE_KEY). It lets the web UI
// persist a key on the host without hardcoding it in the repo, and exposes it
// to the engine containers through getPluginEnv.
//
// The store is a KEY=VALUE file (0600) mirrored into an in-memory map. Values
// set here take precedence over the process environment, so a key entered in
// the UI overrides one set via systemd/compose. Values are never logged and
// are only ever exposed through Mask (last four characters).
package secrets

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// maxValLen caps a stored secret's length. VirusTotal keys are 64 hex chars;
// ESET license keys are shorter. 512 is generous without allowing abuse.
const maxValLen = 512

var keyRe = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`)

var (
	mu   sync.RWMutex
	vals = map[string]string{}
	path string
)

// Init loads the secrets file (if present) into memory. A missing file is not
// an error — it just means no keys have been set via the UI yet. Call once at
// serve startup, before the HTTP server handles requests.
func Init(p string) error {
	mu.Lock()
	defer mu.Unlock()
	path = p
	vals = map[string]string{}

	f, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok || !keyRe.MatchString(k) {
			continue
		}
		vals[k] = v
	}
	return sc.Err()
}

// Get returns the value for key from the in-memory store, falling back to the
// process environment. This lets keys set via systemd/compose (env) and keys
// set via the web UI (file) both work; the UI value wins when both are set.
// Safe to call before Init (the map starts empty, so it falls through to env).
func Get(key string) string {
	mu.RLock()
	v, ok := vals[key]
	mu.RUnlock()
	if ok && v != "" {
		return v
	}
	return os.Getenv(key)
}

// IsSet reports whether key has a non-empty value (store or environment).
func IsSet(key string) bool { return Get(key) != "" }

// Mask returns a short, non-reversible representation for display: the last
// four characters prefixed with bullets, or "" if the key is not set. It never
// returns the full value, so it is safe to send to the browser.
func Mask(key string) string {
	v := Get(key)
	if v == "" {
		return ""
	}
	if len(v) <= 4 {
		return "••••"
	}
	return "••••" + v[len(v)-4:]
}

// Set stores key=value, or clears the key when value is "". It validates the
// key name and value, updates the in-memory map, and persists the whole store
// to disk atomically with 0600 permissions.
func Set(key, value string) error {
	if !keyRe.MatchString(key) {
		return fmt.Errorf("invalid key name")
	}
	if value != "" {
		if len(value) > maxValLen {
			return fmt.Errorf("value too long (max %d)", maxValLen)
		}
		if hasControlChars(value) {
			return fmt.Errorf("value contains invalid characters")
		}
	}

	mu.Lock()
	defer mu.Unlock()

	if value == "" {
		delete(vals, key)
	} else {
		vals[key] = value
	}
	return persistLocked()
}

// persistLocked rewrites the secrets file from the in-memory map. The caller
// must hold mu (write lock).
func persistLocked() error {
	if path == "" {
		return fmt.Errorf("secrets store not initialized")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	keys := make([]string, 0, len(vals))
	for k := range vals {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(vals[k])
		b.WriteString("\n")
	}

	tmp, err := os.CreateTemp(dir, ".secrets-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.WriteString(b.String()); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

func hasControlChars(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}
