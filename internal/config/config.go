// Package config resolves settings from flags, environment, and a TOML file.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config is the whole tunable surface. Nothing here is specific to any
// organization -- every value is discovered on first run or set by the user.
type Config struct {
	// Who to measure. Empty Author means "the authenticated user".
	Author string `mapstructure:"author"`
	// Optional narrowing. Empty means all of GitHub.
	Orgs  []string `mapstructure:"orgs"`
	Repos []string `mapstructure:"repos"`

	// Lines in files matching these count as generated, not authored. Setting
	// generated_paths replaces the built-in list; extra_generated_paths extends it.
	GeneratedPaths    []string `mapstructure:"generated_paths"`
	GeneratedSuffixes []string `mapstructure:"generated_suffixes"`
	// ExtraGeneratedPaths are appended to GeneratedPaths, so a user can add to the
	// built-in list without freezing it -- the defaults keep improving on upgrade.
	ExtraGeneratedPaths []string `mapstructure:"extra_generated_paths"`
	// Any single file changing more lines than this is data, not authored code.
	BigFileLines int `mapstructure:"big_file_lines"`

	// Rendering.
	WindowDays  int `mapstructure:"window_days"`
	RollingDays int `mapstructure:"rolling_days"`
	// Hours outside WorkdayStart..WorkdayEnd count toward the after-hours share.
	WorkdayStart int `mapstructure:"workday_start"`
	WorkdayEnd   int `mapstructure:"workday_end"`

	// Where state lives. Resolved from XDG when unset.
	DataDir string `mapstructure:"data_dir"`
}

// defaultGeneratedPaths are language-agnostic on purpose: a path fragment that
// means "a machine wrote this" in most ecosystems. Users add their own.
var defaultGeneratedPaths = []string{
	"pnpm-lock.yaml", "package-lock.json", "yarn.lock", "Cargo.lock",
	"go.sum", "poetry.lock", "Gemfile.lock", "composer.lock", "uv.lock",
	".terraform.lock.hcl", "bun.lock", "bun.lockb", "flake.lock", "Podfile.lock",
	"pubspec.lock", "mix.lock", "Package.resolved", "packages.lock.json", "gradle.lockfile",
	"generated/", "dist/", "build/", "vendor/", "node_modules/",
	"__snapshots__/", "__pycache__/", "migrations/",
	"fixture/", "fixtures/", "__fixtures__/", "testdata/",
}

var defaultGeneratedSuffixes = []string{
	".snap", ".min.js", ".min.css", ".map", ".tsbuildinfo", ".pb.go", "_pb2.py",
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("author", "")
	v.SetDefault("orgs", []string{})
	v.SetDefault("repos", []string{})
	v.SetDefault("generated_paths", defaultGeneratedPaths)
	v.SetDefault("generated_suffixes", defaultGeneratedSuffixes)
	v.SetDefault("extra_generated_paths", []string{})
	v.SetDefault("big_file_lines", 2000)
	v.SetDefault("window_days", 90)
	v.SetDefault("rolling_days", 7)
	v.SetDefault("workday_start", 8)
	v.SetDefault("workday_end", 18)
	v.SetDefault("data_dir", "")
}

// Dir returns the config directory, honouring XDG_CONFIG_HOME.
func Dir() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "shipped")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".shipped"
	}
	return filepath.Join(home, ".config", "shipped")
}

// File is the path to the TOML config.
func File() string { return filepath.Join(Dir(), "config.toml") }

// Exists reports whether the user has been through first-run setup.
func Exists() bool {
	_, err := os.Stat(File())
	return err == nil
}

// Load reads the config, applying defaults for anything absent. A missing file
// is not an error -- defaults alone are a working configuration.
func Load() (*Config, error) {
	v := viper.New()
	setDefaults(v)
	v.SetConfigFile(File())
	v.SetConfigType("toml")
	v.SetEnvPrefix("SHIPPED")
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if _, isPathErr := err.(*os.PathError); !isPathErr && !errorsAs(err, &notFound) {
			return nil, fmt.Errorf("reading %s: %w", File(), err)
		}
	}

	var c Config
	if err := v.Unmarshal(&c); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", File(), err)
	}
	// A fresh slice, so appending can never write into the package default.
	c.GeneratedPaths = append(append([]string(nil), c.GeneratedPaths...), c.ExtraGeneratedPaths...)
	if c.DataDir == "" {
		c.DataDir = defaultDataDir()
	}
	return &c, nil
}

func defaultDataDir() string {
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		return filepath.Join(x, "shipped")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".shipped"
	}
	return filepath.Join(home, ".local", "share", "shipped")
}

// Save writes only the choices setup makes -- who and where to measure. Copying
// the default lists into the file would freeze them, and the user would never
// receive improved defaults on upgrade.
func (c *Config) Save() error {
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return err
	}
	v := viper.New()
	v.Set("author", c.Author)
	v.Set("orgs", c.Orgs)
	v.Set("repos", c.Repos)
	v.SetConfigType("toml")
	return v.WriteConfigAs(File())
}

// DataFile is where collected pull requests are stored.
func (c *Config) DataFile() string { return filepath.Join(c.DataDir, "data.json") }

// DashboardFile is where the rendered HTML is written.
func (c *Config) DashboardFile() string { return filepath.Join(c.DataDir, "dashboard.html") }

func errorsAs(err error, target *viper.ConfigFileNotFoundError) bool {
	e, ok := err.(viper.ConfigFileNotFoundError)
	if ok {
		*target = e
	}
	return ok
}
