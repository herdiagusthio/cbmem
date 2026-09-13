package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	RepoRoot      string   `yaml:"repo_root"`
	OutputDir     string   `yaml:"output_dir"`
	Languages     []string `yaml:"languages"`
	ExcludePaths  []string `yaml:"exclude_paths"`
	IndexComments bool     `yaml:"index_comments"`
	Embeddings    bool     `yaml:"embeddings"`
}

var DefaultConfig = Config{
	RepoRoot:      ".",
	OutputDir:     ".cbmem",
	Languages:     []string{"go", "typescript", "python", "sql", "yaml", "dockerfile", "protobuf"},
	ExcludePaths:  []string{"vendor", "node_modules", ".git", "dist", "build", "*.pb.go", "*_test.go"},
	IndexComments: true,
	Embeddings:    true,
}

func Load(configPath string) (*Config, error) {
	cfg := DefaultConfig

	candidates := []string{}
	if configPath != "" {
		candidates = append(candidates, configPath)
	} else {
		candidates = append(candidates,
			".cbmem.yaml",
			".cbmem.yml",
			filepath.Join(os.Getenv("HOME"), ".config", "cbmem", "config.yaml"),
		)
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			data, err := os.ReadFile(p)
			if err != nil {
				return nil, err
			}
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				return nil, err
			}
			break
		}
	}

	if v := os.Getenv("CBMEM_REPO_ROOT"); v != "" {
		cfg.RepoRoot = v
	}
	if v := os.Getenv("CBMEM_OUTPUT_DIR"); v != "" {
		cfg.OutputDir = v
	}
	if v := os.Getenv("CBMEM_LANGUAGES"); v != "" {
		cfg.Languages = strings.Split(v, ",")
	}

	return &cfg, nil
}

func GetConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "cbmem"), nil
}