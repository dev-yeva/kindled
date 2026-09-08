package config

import (
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	LocalPort   uint16      `yaml:"local_port"`
	HTTPTracker HTTPTracker `yaml:"http_tracker"`
	UDPTracker  UDPTracker  `yaml:"udp_tracker"`
}

type HTTPTracker struct {
	AnnounceRetries     int           `yaml:"announce_retries"`
	BaseAnnounceTimeout time.Duration `yaml:"base_announce_timeout"`
}

type UDPTracker struct {
	ConnectRetries      int           `yaml:"connect_retries"`
	BaseConnectTimeout  time.Duration `yaml:"base_connect_timeout"`
	AnnounceRetries     int           `yaml:"announce_retries"`
	BaseAnnounceTimeout time.Duration `yaml:"base_announce_timeout"`
}

func New(filePath string) *Config {
	var config Config
	if err := cleanenv.ReadConfig(filePath, &config); err != nil {
		panic("failed to parse config")
	}
	return &config
}
