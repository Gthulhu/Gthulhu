package config

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/viper"
)

type SecretValue string

func (s SecretValue) String() string {
	return "****"
}

func (s SecretValue) Value() string {
	return string(s)
}

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
	K8S     K8SConfig     `mapstructure:"k8s"`
	MTLS    MTLSConfig    `mapstructure:"mtls"`
}

// MTLSConfig holds the mutual TLS configuration used for Manager ↔ Decision Maker communication.
// CertPem and KeyPem are the service's own certificate/key pair signed by the private CA.
// CAPem is the private CA certificate used to verify the peer's certificate.
type MTLSConfig struct {
	Enable     bool        `mapstructure:"enable"`
	ServerName string      `mapstructure:"server_name"`
	CertPem    SecretValue `mapstructure:"cert_pem"`
	KeyPem     SecretValue `mapstructure:"key_pem"`
	CAPem      SecretValue `mapstructure:"ca_pem"`
}

type MongoDBConfig struct {
	Database    string      `mapstructure:"database"`
	CAPem       SecretValue `mapstructure:"ca_pem"`
	CAPemEnable bool        `mapstructure:"ca_pem_enable"`
	User        string      `mapstructure:"user"`
	Password    SecretValue `mapstructure:"password"`
	Port        string      `mapstructure:"port"`
	Host        string      `mapstructure:"host"`
	Options     string      `mapstructure:"options"`
}

func (mc MongoDBConfig) GetURI() string {
	return fmt.Sprintf("mongodb://%s:%s@%s:%s/?%s", mc.User, mc.Password.Value(), mc.Host, mc.Port, mc.Options)
}

type KeyConfig struct {
	RsaPrivateKeyPem SecretValue `mapstructure:"rsa_private_key_pem"`
	DMPublicKeyPem   SecretValue `mapstructure:"dm_public_key_pem"`
	ClientID         string      `mapstructure:"client_id"`
}

type AccountConfig struct {
	AdminEmail    string      `mapstructure:"admin_email"`
	AdminPassword SecretValue `mapstructure:"admin_password"`
}

type K8SConfig struct {
	KubeConfigPath string `mapstructure:"kube_config_path"`
	IsInCluster    bool   `mapstructure:"in_cluster"`
}

var managerCfg *ManageConfig

func GetManagerConfig() *ManageConfig {
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
