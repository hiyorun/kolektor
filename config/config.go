package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type (
	Config struct {
		Collectors []Collector `yaml:"collectors"`
		Database   Database    `yaml:"database"`
		Store      Store       `yaml:"store"`
		HTTP       HTTP        `yaml:"http"`
	}
	Collector struct {
		Type     string        `yaml:"type"`
		Interval time.Duration `yaml:"interval"`
		Nodes    []Node        `yaml:"nodes"`
	}

	Database struct {
		Backend string `yaml:"backend"`
		Path    string `yaml:"path"`
	}
	Node struct {
		Username string    `yaml:"username"`
		Hostname string    `yaml:"hostname,omitempty"`
		IP       string    `yaml:"ip,omitempty"`
		Services []Service `yaml:"services,omitempty"`
	}
	Service struct {
		Name       string `yaml:"name"`
		Group      string `yaml:"group,omitempty"`
		Importance string `yaml:"importance,omitempty"`
		Ports      []int  `yaml:"ports,omitempty"`
	}
	Store struct {
		OnChange  bool          `yaml:"on_change,omitempty"`
		Retention time.Duration `yaml:"retention,omitempty"`
	}
	HTTP struct {
		Host string `yaml:"host"`
		Port string `yaml:"port"`
	}
)

// Load loads a Config from the specified configPath.
//
// It takes a string parameter called configPath which represents the path to the configuration file.
// It returns a pointer to a Config struct and an error.
func Load(configPath string) (*Config, error) {
	var cfg Config

	file, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(file, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
