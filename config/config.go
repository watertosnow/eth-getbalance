package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	App    AppConfig
	Client ClientConfig
}

type AppConfig struct {
	Filename string
	Num      uint
}

type ClientConfig struct {
	Uri   []string
	Key   []string
	Limit []uint
}

func GetConfig(name string) Config {

	var c Config

	viper.SetConfigName(name)
	viper.SetConfigType("toml")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()
	err := viper.ReadInConfig()
	if err != nil {
		panic("error reading config file, please check your config file: " + err.Error())
	}
	err = viper.Unmarshal(&c)
	if err != nil {
		panic("error unmarshal config file, please check your config file")
	}
	if len(c.Client.Key) != len(c.Client.Uri) {
		panic("error unmarshal config file, please check your config file")
	}
	return c
}
