package config

import (
	"os"
	"path/filepath"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	LocalPort   uint16      `yaml:"local_port"`
	Logger      Logger      `yaml:"logger"`
	HTTPTracker HTTPTracker `yaml:"http_tracker"`
	UDPTracker  UDPTracker  `yaml:"udp_tracker"`
	Peer        Peer        `yaml:"peer"`
}

type Logger struct {
	Format string `yaml:"format"`
	Level  string `yaml:"level"`
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

type Peer struct {
	BlockSize        uint32        `yaml:"block_size"`
	ConnectTimeout   time.Duration `yaml:"connect_timeout"`
	HandshakeTimeout time.Duration `yaml:"handshake_timeout"`
	PieceTimeout     time.Duration `yaml:"piece_timeout"`
	MaxPendingPieces int           `yaml:"max_pending_pieces"`
	MaxPendingBlocks int           `yaml:"max_pending_blocks"`
}

func New() (*Config, error) {
	baseConfigDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	filePath := filepath.Join(baseConfigDir, "kindled", "config.yaml")

	// filePath := filepath.Join("config", "config.yaml")

	var config Config
	if err := cleanenv.ReadConfig(filePath, &config); err != nil {
		return nil, err
	}
	config.Peer.BlockSize = config.Peer.BlockSize * 1024
	return &config, nil
}
