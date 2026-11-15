package download_progress

import (
	"fmt"
	"log/slog"
	"sync/atomic"
)

type DownloadProgress struct {
	lgr              slog.Logger
	piecesToDownload uint32
	downloadedPieces *atomic.Uint32
	workingPeers     *atomic.Int32
	updateChan       chan struct{}
}

func New(
	lgr slog.Logger,
	piecesToDownload uint32,
	downloadedPieces *atomic.Uint32,
	workingPeers *atomic.Int32,
) *DownloadProgress {
	return &DownloadProgress{
		lgr:              lgr,
		piecesToDownload: piecesToDownload,
		downloadedPieces: downloadedPieces,
		workingPeers:     workingPeers,
	}
}

func (dp *DownloadProgress) Print() {
	// dp.lgr.Info(fmt.Sprintf(
	// 	"\rDownload progress: %.1f%%    Working peers: %d",
	// 	100*float64(dp.downloadedPieces.Load())/float64(dp.piecesToDownload),
	// 	dp.workingPeers.Load(),
	// ))
	dp.lgr.Info(
		"\rDownloading torrent",
		"progress", fmt.Sprintf("%.1f%%", 100*float64(dp.downloadedPieces.Load())/float64(dp.piecesToDownload)),
		"peers", dp.workingPeers.Load(),
	)
}
