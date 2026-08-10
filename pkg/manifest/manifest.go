package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	// FileName is the canonical package manifest filename.
	FileName = "nexus.toml"

	// DefaultVersion is used by nex init.
	DefaultVersion = "0.1.0"
)

var (
	namePattern    = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,63}$`)
	versionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+(-[a-zA-Z0-9.-]+)?$`)
)

// Manifest describes a Nex package as declared in nexus.toml.
type Manifest struct {
	Name         string            `toml:"name"`
	Version      string            `toml:"version"`
	Author       string            `toml:"author"`
	Description  string            `toml:"description,omitempty"`
	Dependencies map[string]string `toml:"dependencies"`
}

// Default returns a starter manifest for nex init.
func Default(name, author string) *Manifest {
	if name == "" {
		name = "my-package"
	}
	if author == "" {
		author = "anonymous"
	}
	return &Manifest{
		Name:         name,
		Version:      DefaultVersion,
		Author:       author,
		Description:  "A Nex package",
		Dependencies: map[string]string{},
	}
}

// Load reads and validates a nexus.toml file from path.
func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", path, err)
	}
	return Parse(data)
}

// LoadFromDir loads nexus.toml from the given directory.
func LoadFromDir(dir string) (*Manifest, error) {
	return Load(filepath.Join(dir, FileName))
}

// Parse parses and validates nexus.toml contents.
func Parse(data []byte) (*Manifest, error) {
	var m Manifest
	meta, err := toml.Decode(string(data), &m)
	if err != nil {
		return nil, fmt.Errorf("parse nexus.toml: %w", err)
	}
	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, len(undecoded))
		for i, k := range undecoded {
			keys[i] = k.String()
		}
		return nil, fmt.Errorf("unknown keys in nexus.toml: %s", strings.Join(keys, ", "))
	}
	if m.Dependencies == nil {
		m.Dependencies = map[string]string{}
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// Validate checks required fields and formats.
func (m *Manifest) Validate() error {
	if m == nil {
		return fmt.Errorf("manifest is nil")
	}
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("manifest field 'name' is required")
	}
	if !namePattern.MatchString(m.Name) {
		return fmt.Errorf("invalid package name %q: must start with a letter and contain only letters, digits, '_' or '-'", m.Name)
	}
	if strings.TrimSpace(m.Version) == "" {
		return fmt.Errorf("manifest field 'version' is required")
	}
	if !versionPattern.MatchString(m.Version) {
		return fmt.Errorf("invalid version %q: expected semver like 1.2.3", m.Version)
	}
	if strings.TrimSpace(m.Author) == "" {
		return fmt.Errorf("manifest field 'author' is required")
	}
	for dep, ver := range m.Dependencies {
		if !namePattern.MatchString(dep) {
			return fmt.Errorf("invalid dependency name %q", dep)
		}
		if strings.TrimSpace(ver) == "" {
			return fmt.Errorf("dependency %q has empty version", dep)
		}
	}
	return nil
}

// Write serializes the manifest to path as TOML.
func (m *Manifest) Write(path string) error {
	if err := m.Validate(); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create manifest %s: %w", path, err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(m); err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	return nil
}

// WriteToDir writes nexus.toml into dir.
func (m *Manifest) WriteToDir(dir string) error {
	return m.Write(filepath.Join(dir, FileName))
}

// String returns a human-readable summary.
func (m *Manifest) String() string {
	return fmt.Sprintf("%s@%s by %s", m.Name, m.Version, m.Author)
}
