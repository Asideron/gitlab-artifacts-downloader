package config

import (
	"github.com/BurntSushi/toml"
)

func parseToml(filename string) (*Config, error) {
	var conf Config
	if _, err := toml.DecodeFile(filename, &conf); err != nil {
		return nil, err
	}
	return &conf, nil
}
