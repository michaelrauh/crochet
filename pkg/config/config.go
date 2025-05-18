package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Port        string
	DatabaseURL string
	DB          DB
	Redis       Redis
}

type DB struct {
	Host string
	Port int
	User string
	Pass string
	Name string
}

type Redis struct {
	Host string
	Port int
	Pass string
	DB   int
}

func Load() *Config {
	// 1. Set required environment variables
	viper.SetDefault("PORT", "8080")
	viper.SetDefault("DB_HOST", "localhost")
	viper.SetDefault("DB_PORT", "5432")
	viper.SetDefault("DB_USER", "postgres")
	viper.SetDefault("DB_PASS", "postgres")
	viper.SetDefault("DB_NAME", "postgres")
	viper.SetDefault("REDIS_HOST", "localhost")
	viper.SetDefault("REDIS_PORT", "6379")
	viper.SetDefault("REDIS_PASS", "")
	viper.SetDefault("REDIS_DB", "0")

	viper.AutomaticEnv()

	// Check required environment variables
	required := []string{
		"DB_HOST", "DB_PORT", "DB_USER", "DB_PASS", "DB_NAME",
		"REDIS_HOST", "REDIS_PORT",
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

	redisHost := viper.GetString("REDIS_HOST")
	redisPort := viper.GetInt("REDIS_PORT")
	redisPass := viper.GetString("REDIS_PASS")
	redisDB := viper.GetInt("REDIS_DB")

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
		Redis: Redis{
			Host: redisHost,
			Port: redisPort,
			Pass: redisPass,
			DB:   redisDB,
		},
	}
}
