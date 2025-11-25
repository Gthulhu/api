package config

import (
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/viper"
)

type ServerConfig struct {
	Host string `mapstructure:"host"`
}

type LoggingConfig struct {
	Level    string `mapstructure:"level"`
	Console  bool   `mapstructure:"console"`
	FilePath string `mapstructure:"file_path"`
}

type ManageConfig struct {
	Server  ServerConfig  `mapstructure:"server"`
	Logging LoggingConfig `mapstructure:"logging"`
	MongoDB MongoDBConfig `mapstructure:"mongodb"`
	Key     KeyConfig     `mapstructure:"key"`
	Account AccountConfig `mapstructure:"account"`
}

type MongoDBConfig struct {
	Database string `mapstructure:"database"`
	CAPem    string `mapstructure:"ca_pem"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Port     string `mapstructure:"port"`
	Host     string `mapstructure:"host"`
}

type KeyConfig struct {
	RsaPrivateKeyPem string `mapstructure:"rsa_private_key_pem"`
}

type AccountConfig struct {
	AdminEmail    string `mapstructure:"admin_email"`
	AdminPassword string `mapstructure:"admin_password"`
}

var (
	managerCfg *ManageConfig
)

func GetConfig() *ManageConfig {
	return managerCfg
}

func InitManagerConfig(configName string, configPath string) (ManageConfig, error) {
	var cfg ManageConfig
	if configPath != "" {
		viper.AddConfigPath(configPath)
	}
	if configName == "" {
		configName = "manager_config"
	}
	viper.AddConfigPath(GetAbsPath("config"))
	viper.SetConfigName(configName)
	viper.SetConfigType("toml")
	viper.SetEnvPrefix("MANAGER")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	err := viper.ReadInConfig()
	if err != nil {
		return cfg, err
	}

	err = viper.Unmarshal(&cfg)
	if err != nil {
		return cfg, err
	}
	managerCfg = &cfg
	return cfg, nil
}

// GetAbsPath returns the absolute path by joining the given paths with the project root directory
func GetAbsPath(paths ...string) string {
	_, filePath, _, _ := runtime.Caller(1)
	basePath := filepath.Dir(filePath)
	rootPath := filepath.Join(basePath, "..")
	return filepath.Join(rootPath, filepath.Join(paths...))
}
