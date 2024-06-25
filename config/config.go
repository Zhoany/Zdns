package config

import (
	"github.com/spf13/viper"
)

type Upstream struct {
	Address         string `mapstructure:"address"`
	Port            string `mapstructure:"port"`
	Protocol        string `mapstructure:"protocol"`
	DomainRulesFile string `mapstructure:"domain_rules_file"`
}

type ServerConfig struct {
	Address       string `mapstructure:"address"`
	ResolveIPv6   bool   `mapstructure:"resolve_ipv6"`
	CacheSize     int    `mapstructure:"cache_size"`
	MaxConnects   int    `mapstructure:"max_connects"`
	MaxWorkers    int    `mapstructure:"max_workers"`
	MaxClients    int    `mapstructure:"max_clients"`
	EnableLogging bool   `mapstructure:"enable_logging"`
	LogMaxSize    int    `mapstructure:"log_max_size"`
	LogMaxBackups int    `mapstructure:"log_max_backups"`
}

type Config struct {
	Server          ServerConfig `mapstructure:"server"`
	UpstreamServers []Upstream   `mapstructure:"upstream_servers"`
	BlocklistFile   string       `mapstructure:"blocklist_file"`
	CommonUpstream  Upstream     `mapstructure:"common_upstream"`
}

var Cfg Config

func LoadConfig(filename string) error {
	viper.SetConfigFile(filename)
	err := viper.ReadInConfig()
	if err != nil {
		return err
	}
	err = viper.Unmarshal(&Cfg)
	if err != nil {
		return err
	}
	return nil
}
