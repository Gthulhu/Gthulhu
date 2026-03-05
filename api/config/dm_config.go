package config

import (
	"strings"

	"github.com/spf13/viper"
)

type DecisionMakerConfig struct {
	Server  ServerConfig  `mapstructure:"server"`
	Logging LoggingConfig `mapstructure:"logging"`
	Token   TokenConfig   `mapstructure:"token"`
	MTLS    MTLSConfig    `mapstructure:"mtls"`
}

var (
	dmConfig *DecisionMakerConfig
)

func GetDMConfig() *ManageConfig {
	return managerCfg
}

func InitDMConfig(configName string, configPath string) (DecisionMakerConfig, error) {
	var cfg DecisionMakerConfig
	if configPath != "" {
		viper.AddConfigPath(configPath)
	}
	if configName == "" {
		configName = "dm_config"
	}
	viper.AddConfigPath(GetAbsPath("config"))
	viper.SetConfigName(configName)
	viper.SetConfigType("toml")
	viper.SetEnvPrefix("DM")
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
	dmConfig = &cfg
	return cfg, nil
}

type TokenConfig struct {
	RsaPrivateKeyPem SecretValue `mapstructure:"rsa_private_key_pem"`
	TokenDurationHr  int         `mapstructure:"token_duration_hr"` // in hours
}
