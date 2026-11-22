package download_progress

import (
	"fmt"
	"log/slog"
	"sync/atomic"
)

type DownloadProgress struct {
	lgr              slog.Logger
	pieceCount       uint32
	downloadedPieces *atomic.Uint32
	workingPeers     *atomic.Int32
	updateChan       chan struct{}
}

func New(
	lgr slog.Logger,
	pieceCount uint32,
	downloadedPieces *atomic.Uint32,
	workingPeers *atomic.Int32,
) *DownloadProgress {
	return &DownloadProgress{
		lgr:              lgr,
		pieceCount:       pieceCount,
		downloadedPieces: downloadedPieces,
		workingPeers:     workingPeers,
	}
}

func (dp *DownloadProgress) Print() {
	dp.lgr.Info(
		"\rDownloading torrent",
		"progress", fmt.Sprintf("%.1f%%", 100*float64(dp.downloadedPieces.Load())/float64(dp.pieceCount)),
		"peers", dp.workingPeers.Load(),
	)
}
