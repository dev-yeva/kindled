package main

import (
	"flag"
	"kindled/config"
	"kindled/lib"
	"kindled/logger"
	"kindled/torrent/peer"
	"kindled/torrent/torrentfile"
	"kindled/torrent/tracker"
	"log/slog"
	"os"
)

func main() {

	cfg, err := config.New()
	if err != nil {
		slog.Error("Failed to create config", "error", err)
		os.Exit(1)
	}

	logger := logger.New(&cfg.Logger)

	torrentFilePath, saveFolder := GetFlags(logger)
	torrentFile, err := torrentfile.Open(torrentFilePath, saveFolder)
	if err != nil {
		logger.Error("Failed to parse torrent file", "path", torrentFilePath, "error", err)
		os.Exit(1)
	}

	logger.Info("Parsed torrent file", "torrent_file", torrentFile)

	localPeerID := lib.GenerateLocalPeerID()

	announceClient := tracker.NewAnnounceClient(
		*logger,
		torrentFile,
		&cfg.HTTPTracker,
		&cfg.UDPTracker,
		cfg.LocalPort,
	)

	addrs := announceClient.MakeAnnounce(localPeerID)
	if len(addrs) == 0 {
		os.Exit(0)
	}

	peerClient := peer.NewPeerClient(*logger, torrentFile, &cfg.Peer, addrs, localPeerID)
	peerClient.Download()
}

func GetFlags(lgr *slog.Logger) (string, string) {
	torrentPath := flag.String("torrent", "", "Path to .torrent file")
	saveFolder := flag.String("dest", "", "Path to destination folder")

	flag.Usage = func() {
		lgr.Info("Program usage:", "kindled --torrent", "some.torrent", "--dest", "./downloads")
	}

	flag.Parse()

	if *torrentPath == "" || *saveFolder == "" {
		flag.Usage()
		os.Exit(2)
	}

	return *torrentPath, *saveFolder
}
