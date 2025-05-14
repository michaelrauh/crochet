package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Port        string
	DatabaseURL string
	DB          DB
}

type DB struct {
	Host string
	Port int
	User string
	Pass string
	Name string
}

func Load() *Config {
	viper.AutomaticEnv()

	// 1. Required vars
	required := []string{
		"PORT",
		"DB_HOST", "DB_PORT", "DB_USER", "DB_PASS", "DB_NAME",
	}
	for _, key := range required {
		if !viper.IsSet(key) {
			panic(fmt.Sprintf("%s environment variable is required but not set", key))
		}
	}

	// 2. Read them
	host := viper.GetString("DB_HOST")
	port := viper.GetInt("DB_PORT")
	user := viper.GetString("DB_USER")
	pass := viper.GetString("DB_PASS")
	name := viper.GetString("DB_NAME")
	appPort := viper.GetString("PORT")

	// 3. Build DSN
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		user, pass, host, port, name,
	)

	return &Config{
		Port:        appPort,
		DatabaseURL: dsn,
		DB: DB{
			Host: host,
			Port: port,
			User: user,
			Pass: pass,
			Name: name,
		},
	}
}
