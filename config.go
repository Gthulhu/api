package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config represents the application configuration
type Config struct {
	Server     ServerConfig     `json:"server"`
	Logging    LoggingConfig    `json:"logging"`
	Strategies StrategiesConfig `json:"strategies"`
}

// ServerConfig represents server-specific configuration
type ServerConfig struct {
	Port         string `json:"port"`
	ReadTimeout  int    `json:"read_timeout"`
	WriteTimeout int    `json:"write_timeout"`
	IdleTimeout  int    `json:"idle_timeout"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level  string `json:"level"`
	Format string `json:"format"`
}

// StrategiesConfig represents scheduling strategies configuration
type StrategiesConfig struct {
	Default []SchedulingStrategy `json:"default"`
}

// LoadConfig loads configuration from file or returns default config
func LoadConfig(filename string) (*Config, error) {
	// Default configuration
	config := &Config{
		Server: ServerConfig{
			Port:         ":8080",
			ReadTimeout:  15,
			WriteTimeout: 15,
			IdleTimeout:  60,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
		Strategies: StrategiesConfig{
			Default: []SchedulingStrategy{
				{
					Priority:      true,
					ExecutionTime: 20000000,
					Selectors: []LabelSelector{
						{
							Key:   "nf",
							Value: "upf",
						},
					},
				},
			},
		},
	}

	// Try to load from file
	if filename != "" {
		file, err := os.Open(filename)
		if err != nil {
			return config, nil // Return default config if file doesn't exist
		}
		defer file.Close()

		decoder := json.NewDecoder(file)
		if err := decoder.Decode(config); err != nil {
			return nil, fmt.Errorf("failed to decode config file: %v", err)
		}
	}

	return config, nil
}
