package config

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/Gthulhu/api/util"
)

// Config represents the application configuration
type Config struct {
	Server     ServerConfig     `json:"server"`
	Logging    LoggingConfig    `json:"logging"`
	Strategies StrategiesConfig `json:"strategies"`
	JWT        JWTConfig        `json:"jwt"`
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

// SchedulingStrategy represents a strategy for process scheduling
type SchedulingStrategy struct {
	Priority      bool            `json:"priority"`                // If true, set vtime to minimum vtime
	ExecutionTime uint64          `json:"execution_time"`          // Time slice for this process in nanoseconds
	PID           int             `json:"pid,omitempty"`           // Process ID to apply this strategy to
	Selectors     []LabelSelector `json:"selectors,omitempty"`     // Label selectors to match pods
	CommandRegex  string          `json:"command_regex,omitempty"` // Regex to match process command
}

// LabelSelector represents a key-value pair for pod label selection
type LabelSelector struct {
	Key   string `json:"key"`   // Label key
	Value string `json:"value"` // Label value
}

// StrategiesConfig represents scheduling strategies configuration
type StrategiesConfig struct {
	Default []SchedulingStrategy `json:"default"`
}

// JWTConfig represents JWT authentication configuration
type JWTConfig struct {
	PrivateKeyPath string `json:"private_key_path"`
	TokenDuration  int    `json:"token_duration"` // Token duration in hours
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
		JWT: JWTConfig{
			PrivateKeyPath: "/etc/bss-api/private_key.pem",
			TokenDuration:  24, // 24 hours
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

// InitJWTRsaKey initializes RSA private key for JWT authentication
func InitJWTRsaKey(config JWTConfig) (*rsa.PrivateKey, error) {

	// Try to load existing private key
	key, err := loadPrivateKey(config.PrivateKeyPath)
	if err != nil {
		util.GetLogger().Warn("Failed to load private key, generating a new one", slog.String("path", config.PrivateKeyPath), util.LogErrAttr(err))
		// Generate new private key
		key, err = generatePrivateKey(config.PrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to generate private key: %v", err)
		}
	}

	return key, nil
}

// loadPrivateKey loads RSA private key from PEM file
func loadPrivateKey(keyPath string) (*rsa.PrivateKey, error) {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %v", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block containing private key")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS8 format
		keyInterface, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %v", err)
		}
		var ok bool
		key, ok = keyInterface.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is not RSA")
		}
	}

	return key, nil
}

// generatePrivateKey generates a new RSA private key and saves it to file
func generatePrivateKey(keyPath string) (*rsa.PrivateKey, error) {
	// Generate RSA private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %v", err)
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(strings.TrimSuffix(keyPath, "/private_key.pem"), 0755); err != nil {
		util.GetLogger().Warn("Failed to create directory for private key", slog.String("path", keyPath), util.LogErrAttr(err))
	}

	// Save private key to file
	keyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyBytes,
	})

	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		util.GetLogger().Warn("Failed to save private key to file", slog.String("path", keyPath), util.LogErrAttr(err))
	} else {
		util.GetLogger().Info("Generated and saved new private key", slog.String("path", keyPath))
	}
	return privateKey, nil
}
