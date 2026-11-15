package peer

import (
	"context"
	"kindled/config"
	"kindled/lib/addresses"
	"kindled/lib/download_progress"
	"kindled/lib/set"
	"kindled/torrent/torrentfile"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

type PeerClient struct {
	logger       slog.Logger
	cfg          *config.Peer
	peerConnChan chan *PeerConn
	tf           *torrentfile.TorrentFile

	connClosed chan struct{}

	workingPeers     *atomic.Int32
	downloadedPieces *atomic.Uint32
	progress         *download_progress.DownloadProgress
}

type pieceWork struct {
	index            uint32           // Bit field index of piece
	hash             [20]byte         // SHA-1 hash of piece
	length           uint32           // Length of piece in bytes
	blocksToDownload *set.Set[uint32] // First indexes of blocks that hasn hasn't/failed to download
	data             []byte           // Downloaded data
}

type pieceResult struct {
	index uint32
	data  []byte
}

func (p *pieceWork) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Any("index", p.index),
		slog.Any("length", p.length),
		slog.Int("blocks", p.blocksToDownload.Size()),
	)
}

func (p *pieceResult) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Any("index", p.index),
	)
}

func NewPeerClient(
	logger slog.Logger,
	tf *torrentfile.TorrentFile,
	cfg *config.Peer,
	addrs addresses.Addresses,
	localPeerID []byte,
) *PeerClient {

	peerConnChan := make(chan *PeerConn, addrs.Size())
	wg := sync.WaitGroup{}
	var workingPeers atomic.Int32
	blocksInPiece := tf.Info.PieceLength / cfg.BlockSize

	connClosed := make(chan struct{})

	for addr := range addrs.Range() {
		lgr := logger.With("addr", addr)
		wg.Go(func() {
			peerConn, err := handshake(cfg, lgr, addr, localPeerID, len(tf.Info.PieceHashes), tf.InfoHash, blocksInPiece, connClosed)
			if err == nil {
				lgr.Debug("Peer handshake succeeded")
				peerConnChan <- peerConn
				workingPeers.Add(1)
			} else {
				lgr.Debug("Peer handshake failed", "error", err)
			}
		})
	}

	wg.Wait()
	close(peerConnChan)

	logger.Debug("Completed peer handshake", "working_peers", workingPeers.Load())

	var downloadedPieces atomic.Uint32
	return &PeerClient{
		logger:       logger,
		cfg:          cfg,
		peerConnChan: peerConnChan,
		tf:           tf,

		connClosed: connClosed,

		workingPeers:     &workingPeers,
		downloadedPieces: &downloadedPieces,
		progress:         download_progress.New(logger, uint32(len(tf.Info.PieceHashes)), &downloadedPieces, &workingPeers),
	}
}

func (pc *PeerClient) Download() {
	ctx, cancel := context.WithCancel(context.Background())

	if pc.workingPeers.Load() == 0 {
		cancel()
	}

	go func() {
		for range pc.connClosed {
			pc.workingPeers.Add(-1)
			pc.progress.Print()
			if pc.workingPeers.Load() == 0 {
				cancel()
				return
			}
		}
	}()

	workQueue := make(chan *pieceWork, len(pc.tf.Info.PieceHashes))
	resultQueue := make(chan *pieceResult)

	go func() {
		for peerConn := range pc.peerConnChan {
			if peerConn.conn != nil {
				go peerConn.downloadPieces(workQueue, resultQueue)
			}
		}
	}()

	pc.progress.Print()

	semaphore := make(chan struct{}, pc.cfg.MaxPendingPieces)

	go func() {
		for i, hash := range pc.tf.Info.PieceHashes {
			begin := uint64(i) * uint64(pc.tf.Info.PieceLength)
			end := min(begin+uint64(pc.tf.Info.PieceLength), uint64(pc.tf.Info.TotalLength))
			length := uint32(end - begin)

			blocksToDownload := set.New[uint32](int(length/pc.cfg.BlockSize) + 1)

			for offset := uint32(0); offset < length; offset += pc.cfg.BlockSize {
				blocksToDownload.Add(offset)
			}

			pw := &pieceWork{
				index:            uint32(i),
				hash:             hash,
				length:           length,
				blocksToDownload: blocksToDownload,
				data:             make([]byte, length),
			}
			pc.logger.Debug("Created piece work", "piece_work", pw)
			semaphore <- struct{}{}
			workQueue <- pw
		}
	}()

	for count := 0; count < len(pc.tf.Info.PieceHashes); count++ {
		select {
		case <-ctx.Done():
			pc.logger.Info("No peer to donwload from")
			return
		case res := <-resultQueue:
			pc.logger.Debug("Completed piece download", "piece_result", res)
			err := pc.tf.Info.Files.WritePiece(int64(res.index), res.data, &pc.logger)
			if err != nil {
				return
			}
			pc.progress.Print()
			pc.downloadedPieces.Add(1)
			<-semaphore
		case <-time.After(time.Minute):
			count--
			continue
		}
	}
	pc.progress.Print()
	pc.logger.Info("Completed download")
	close(workQueue)
}
