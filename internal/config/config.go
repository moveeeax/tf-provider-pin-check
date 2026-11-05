// Package config loads the optional exceptions file for tf-provider-pin-check.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Config holds the exception allowlists. Every field is optional; a missing
// file yields a zero-value Config that allows nothing.
type Config struct {
	// Providers lists provider names (e.g. "random") that may stay unpinned.
	Providers []string `yaml:"providers"`
	// Modules lists module source strings that may stay unpinned.
	Modules []string `yaml:"modules"`
	// Paths lists glob patterns (matched against the path relative to the scan
	// root) whose files are skipped entirely.
	Paths []string `yaml:"paths"`
	// Strict rejects the pessimistic "~>" operator too, demanding an exact pin.
	Strict bool `yaml:"strict"`
}

// DefaultFiles are the config names looked up in the scan root when no explicit
// path is given.
var DefaultFiles = []string{".tf-pin-check.yml", ".tf-pin-check.yaml"}

// Load reads the config at path. An empty path triggers auto-discovery in root.
// A missing file is not an error: it returns an empty Config.
func Load(path, root string) (*Config, error) {
	if path == "" {
		for _, name := range DefaultFiles {
			candidate := filepath.Join(root, name)
			if _, err := os.Stat(candidate); err == nil {
				path = candidate
				break
			}
		}
	}
	c := &Config{}
	if path == "" {
		return c, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return c, nil
}

// AllowsProvider reports whether provider name is on the allowlist.
func (c *Config) AllowsProvider(name string) bool {
	return contains(c.Providers, name)
}

// AllowsModule reports whether module source is on the allowlist.
func (c *Config) AllowsModule(source string) bool {
	return contains(c.Modules, source)
}

// SkipsPath reports whether rel (a slash-separated path relative to the scan
// root) matches any configured glob or substring pattern.
func (c *Config) SkipsPath(rel string) bool {
	rel = filepath.ToSlash(rel)
	for _, p := range c.Paths {
		p = filepath.ToSlash(p)
		if strings.Contains(rel, strings.TrimSuffix(strings.TrimPrefix(p, "**/"), "/**")) && strings.Contains(p, "**") {
			return true
		}
		if ok, _ := filepath.Match(p, rel); ok {
			return true
		}
		if p == rel {
			return true
		}
	}
	return false
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
