package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Port        string
	DatabaseURL string
	DB          DB
	RabbitMQ    RabbitMQ
}

type DB struct {
	Host string
	Port int
	User string
	Pass string
	Name string
}

type RabbitMQ struct {
	Host  string
	Port  int
	User  string
	Pass  string
	VHost string
}

func Load() *Config {
	// Setup viper to read from .env file
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")

	// Also read from environment variables
	viper.AutomaticEnv()

	// Try to read from .env file, but continue if it doesn't exist
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Only log if it's not a "file not found" error
			fmt.Printf("Warning: Error reading config file: %s\n", err)
		}
	}

	// 1. Required vars
	required := []string{
		"PORT",
		"DB_HOST", "DB_PORT", "DB_USER", "DB_PASS", "DB_NAME",
		"RABBITMQ_HOST", "RABBITMQ_PORT", "RABBITMQ_USER", "RABBITMQ_PASS", "RABBITMQ_VHOST",
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

	rmqHost := viper.GetString("RABBITMQ_HOST")
	rmqPort := viper.GetInt("RABBITMQ_PORT")
	rmqUser := viper.GetString("RABBITMQ_USER")
	rmqPass := viper.GetString("RABBITMQ_PASS")
	rmqVHost := viper.GetString("RABBITMQ_VHOST")

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
		RabbitMQ: RabbitMQ{
			Host:  rmqHost,
			Port:  rmqPort,
			User:  rmqUser,
			Pass:  rmqPass,
			VHost: rmqVHost,
		},
	}
}
