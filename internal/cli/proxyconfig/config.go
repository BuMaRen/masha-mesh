package proxyconfig

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	HTTP HTTPConfig `yaml:"http"`
}

type HTTPConfig struct {
	Enabled bool                      `yaml:"enabled"`
	Proxies map[string]ProxyRuleConfig `yaml:"proxies"`
}

type ProxyRuleConfig struct {
	LoadBalance   bool `yaml:"loadBalance"`
	UseBreaker    bool `yaml:"useBreaker"`
}

func LoadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}
