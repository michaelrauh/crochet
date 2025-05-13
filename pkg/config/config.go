package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	Port        string
	DatabaseURL string
}

func Load() *Config {
	viper.AutomaticEnv()

	if !viper.IsSet("PORT") {
		panic("PORT environment variable is required but not set")
	}

	return &Config{
		Port: viper.GetString("PORT"),
	}
}
