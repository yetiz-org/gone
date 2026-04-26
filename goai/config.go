package goai

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the project-level goai.yaml schema. Every field has a sensible
// default so projects can opt in incrementally.
type Config struct {
	Title           string                     `yaml:"title,omitempty"`
	Description     string                     `yaml:"description,omitempty"`
	Version         string                     `yaml:"version,omitempty"`
	TermsOfService  string                     `yaml:"termsOfService,omitempty"`
	Contact         *Contact                   `yaml:"contact,omitempty"`
	License         *License                   `yaml:"license,omitempty"`
	ExternalDocs    *ExternalDocumentation     `yaml:"externalDocs,omitempty"`
	Servers         []Server                   `yaml:"servers,omitempty"`
	Tags            []Tag                      `yaml:"tags,omitempty"`
	GlobalSecurity  []map[string][]string      `yaml:"security,omitempty"`
	SecuritySchemes map[string]*SecurityScheme `yaml:"securitySchemes,omitempty"`

	// DefaultOutput is the single-output target used when neither Profiles
	// nor Output declare anything. Empty falls back to "openapi.generated.yaml".
	DefaultOutput string `yaml:"defaultOutput,omitempty"`

	// BaseSpecPath, when set, enables three-way merge with the hand-tuned
	// base yaml.
	BaseSpecPath string `yaml:"baseSpecPath,omitempty"`

	// RestrictToBaseSpecPaths constrains the generator to paths already
	// declared in the base spec.
	RestrictToBaseSpecPaths bool `yaml:"restrictToBaseSpecPaths,omitempty"`

	// ExcludePaths is the framework-level path blocklist (static assets,
	// well-known framework routes, …). Per-handler exclusion belongs on the
	// handler itself via the Hidden interface.
	ExcludePaths []string `yaml:"excludePaths,omitempty"`

	// Profiles is keyed by profile name and contains include/exclude rules.
	Profiles map[string]ConfigProfile `yaml:"profiles,omitempty"`
	// Output describes where to write each profile's yaml. Keyed by profile
	// name; value is a relative path from the config file's directory.
	Output map[string]string `yaml:"output,omitempty"`
}

// ConfigProfile is the on-disk shape of one profile's selector pair.
type ConfigProfile struct {
	Include Selector `yaml:"include,omitempty"`
	Exclude Selector `yaml:"exclude,omitempty"`
}

// LoadConfig reads goai.yaml from dir and returns a parsed Config. When
// the file is absent, a Config populated with builtin defaults is returned
// and err is nil — projects can run goai before authoring a config.
func LoadConfig(dir string) (*Config, error) {
	path := filepath.Join(dir, "goai.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}

		return nil, fmt.Errorf("goai: read %s: %w", path, err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("goai: parse %s: %w", path, err)
	}

	cfg.applyDefaults()

	return cfg, nil
}

// DefaultConfig returns a Config populated with profile buckets keyed off
// ProfileScope declarations. Selector.Profiles matches the names produced
// by handler / acceptance GOAIProfileScope so a handler that declares
// `[]string{"public"}` lands in the "public" output bucket.
func DefaultConfig() *Config {
	cfg := &Config{
		Title:   "API",
		Version: "0.0.0",
		Profiles: map[string]ConfigProfile{
			"public":   {Include: Selector{Profiles: []string{"public"}}},
			"mgmt":     {Include: Selector{Profiles: []string{"mgmt"}}},
			"internal": {Include: Selector{Profiles: []string{"internal"}}},
			"all":      {},
		},
		Output: map[string]string{
			"public":   "openapi.public.yaml",
			"mgmt":     "openapi.mgmt.yaml",
			"internal": "openapi.internal.yaml",
			"all":      "openapi.yaml",
		},
	}

	return cfg
}

// applyDefaults fills in unset fields after unmarshal from goai.yaml.
func (c *Config) applyDefaults() {
	if c.Title == "" {
		c.Title = "API"
	}

	if c.Version == "" {
		c.Version = "0.0.0"
	}

	if c.Profiles == nil {
		c.Profiles = DefaultConfig().Profiles
	}

	if c.Output == nil {
		c.Output = DefaultConfig().Output
	}
}

// ProfileNames returns the configured profile names in deterministic order.
func (c *Config) ProfileNames() []string {
	names := make([]string, 0, len(c.Profiles))
	for n := range c.Profiles {
		names = append(names, n)
	}

	// Stable order: lexical.
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			if names[j] < names[i] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}

	return names
}

// BuildProfile materialises a Profile struct from one entry in c.Profiles.
func (c *Config) BuildProfile(name string) *Profile {
	cp, ok := c.Profiles[name]
	if !ok {
		return &Profile{Name: name}
	}

	return &Profile{
		Name:    name,
		Include: cp.Include,
		Exclude: cp.Exclude,
	}
}

// ToRunOptions projects the Config onto a RunOptions for single-output
// generation. The Profiles / Output maps are NOT projected — multi-output
// drivers (RunCLIFromConfig) consult those directly.
func (c *Config) ToRunOptions() RunOptions {
	if c == nil {
		return RunOptions{}
	}

	return RunOptions{
		Title:                   c.Title,
		Description:             c.Description,
		Version:                 c.Version,
		TermsOfService:          c.TermsOfService,
		Contact:                 c.Contact,
		License:                 c.License,
		ExternalDocs:            c.ExternalDocs,
		Servers:                 c.Servers,
		Tags:                    c.Tags,
		GlobalSecurity:          c.GlobalSecurity,
		SecuritySchemes:         c.SecuritySchemes,
		DefaultOutput:           c.DefaultOutput,
		BaseSpecPath:            c.BaseSpecPath,
		RestrictToBaseSpecPaths: c.RestrictToBaseSpecPaths,
		ExcludePaths:            c.ExcludePaths,
	}
}

// ToBuildOptions projects the Config onto a BuildOptions, used by the
// multi-output driver where one Walk feeds many Build calls.
func (c *Config) ToBuildOptions() BuildOptions {
	if c == nil {
		return BuildOptions{}
	}

	return BuildOptions{
		Title:          c.Title,
		Description:    c.Description,
		Version:        c.Version,
		TermsOfService: c.TermsOfService,
		Contact:        c.Contact,
		License:        c.License,
		Servers:        c.Servers,
		Tags:           c.Tags,
		GlobalSecurity: c.GlobalSecurity,
		ExternalDocs:   c.ExternalDocs,
	}
}
