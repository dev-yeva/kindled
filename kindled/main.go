package main

import (
	"flag"
	"kindled/config"
	"kindled/lib"
	"kindled/logger"
	"kindled/torrent/peer"
	"kindled/torrent/progress_file"
	"kindled/torrent/torrent_file"
	"kindled/torrent/tracker"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {

	cfg, err := config.New()
	if err != nil {
		slog.Error("Failed to create config", "error", err)
		os.Exit(1)
	}

	logger := logger.New(&cfg.Logger)

	torrentFilePath, saveFolder := GetFlags(logger)
	torrentFile, err := torrent_file.Open(torrentFilePath, saveFolder)
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

	progressFile, err := progress_file.New(
		logger,
		torrentFile.Info.Name,
		saveFolder,
		torrentFile.Info.Files.Number() > 1,
		uint32(len(torrentFile.Info.PieceHashes)),
	)
	if err != nil {
		logger.Error("Failed to create progress file", "error", err)
		os.Exit(1)
	}

	go GracefulShutdown(progressFile)

	peerClient := peer.NewPeerClient(logger, torrentFile, progressFile, &cfg.Peer, addrs, localPeerID)
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

func GracefulShutdown(pf *progress_file.ProgressFile) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan
	pf.Save()
	os.Exit(0)
}
