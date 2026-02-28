package config

import (
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config is an interface for a configuration object.
type Config interface {
	viperConfig
	LoadPaths(configPaths []string) error
	Load(path string) error
	SetDefaultString(key string, value string, secure bool) string
}

type viperConfig interface {
	SetDefault(key string, value interface{})
	GetString(key string) string
	AllSettings() map[string]interface{}
	ReadInConfig() error
	SetConfigType(configType string)
	MergeConfig(in io.Reader) error
}

type config struct {
	*viper.Viper
	secureKeys map[string]string
	specific   map[string]struct{}
}

var (
	DefaultConfigPaths = []string{
		"../../config.yaml",
		"../../config.local.yaml",
		"config.yaml",
		"config.local.yaml",
	}
)

// NewConfig creates a new configuration object.
func NewConfig(configType string) Config {
	v := viper.New()
	v.SetConfigType(configType)
	v.AutomaticEnv()

	cfg := &config{
		Viper:      v,
		secureKeys: map[string]string{},
		specific:   map[string]struct{}{},
	}
	return cfg
}

func (c *config) AllSettings() map[string]interface{} {
	settings := map[string]interface{}{}
	for k, v := range c.Viper.AllSettings() {
		if _, ok := c.secureKeys[k]; ok {
			settings[k] = "****"
			continue
		}
		if _, ok := c.specific[k]; ok {
			settings[k] = v
		}
	}
	return settings
}

func (c *config) LoadPaths(configPaths []string) error {
	for _, path := range configPaths {
		if err := c.Load(path); err != nil {
			return err
		}
	}
	return nil
}

func (c *config) Load(path string) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return c.Viper.MergeConfig(f)
}

func (c *config) SetDefaultString(key string, value string, secure bool) string {
	c.SetDefault(key, value)
	if secure {
		c.secureKeys[key] = value
	}
	c.specific[key] = struct{}{}
	return c.GetString(key)
}
