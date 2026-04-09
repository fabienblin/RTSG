package config

import (
	"encoding/json"
	"log"
	"os"
)

type ConfigService struct {
	Config Configuration
}

func New() *ConfigService {
	rawConf, errRead := os.ReadFile("config.json")
	if errRead != nil {
		log.Fatalln(errRead)
	}
	
	config := Configuration{}
	errUnmarshal := json.Unmarshal(rawConf, &config)
	if errUnmarshal != nil {
		log.Fatalln(errUnmarshal)
	}

	return &ConfigService{
		Config: config,
	}
}
