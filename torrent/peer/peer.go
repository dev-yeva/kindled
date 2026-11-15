package peer

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"io"
	"kindled/config"
	"kindled/torrent/peer/bitfield"
	"kindled/torrent/peer/message"
	"log/slog"
	"net"
	"net/netip"
	"time"
)

var (
	ErrPstrMismatch     = errors.New("peer handshake failed: pstr mismatch")
	ErrInfoHashMismatch = errors.New("peer handshake failed: infoHash mismatch")
	ErrOutOfWindow      = errors.New("Received msg outside window bounds")
)

type PeerConn struct {
	cfg    *config.Peer
	logger *slog.Logger

	conn       net.Conn
	connClosed chan struct{}
	ctx        context.Context
	cancel     context.CancelFunc

	writer *bufio.Writer

	bitField *bitfield.BitField

	peerState *PeerState

	blockChan      chan *message.PieceMessage
	blockSemaphore chan struct{}
}

func handshake(cfg *config.Peer, logger *slog.Logger, addr netip.AddrPort, localPeerID []byte,
	bitFieldLength int, infoHash [20]byte, blocksInPiece uint32, connClosed chan struct{}) (*PeerConn, error) {

	pstr := []byte("BitTorrent protocol")
	pstr_len := len(pstr)

	var buf bytes.Buffer
	buf.Grow(pstr_len + 49)
	buf.WriteByte(byte(pstr_len))
	buf.Write(pstr)
	buf.Write(make([]byte, 8)) // TODO: flags https://www.bittorrent.org/beps/bep_0010.html
	buf.Write(infoHash[:])
	buf.Write(localPeerID)

	conn, err := net.DialTimeout("tcp", addr.String(), cfg.ConnectTimeout)
	if err != nil {
		return nil, err
	}

	conn.SetDeadline(time.Now().Add(cfg.HandshakeTimeout))

	if _, err = conn.Write(buf.Bytes()); err != nil {
		conn.Close()
		return nil, err
	}

	resp := make([]byte, pstr_len+49)
	if _, err := io.ReadFull(conn, resp); err != nil {
		conn.Close()
		return nil, err
	}

	if !bytes.Equal(resp[1:20], pstr) {
		conn.Close()
		return nil, ErrPstrMismatch
	}

	if !bytes.Equal(resp[28:48], infoHash[:]) {
		conn.Close()
		return nil, ErrInfoHashMismatch
	}

	if err := conn.SetDeadline(time.Time{}); err != nil {
		conn.Close()
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	p := PeerConn{
		cfg:    cfg,
		logger: logger,

		conn:       conn,
		connClosed: connClosed,
		ctx:        ctx,
		cancel:     cancel,

		writer: bufio.NewWriterSize(conn, 256),
		// id:        resp[48:68],
		bitField:  bitfield.New(bitFieldLength),
		peerState: New(),

		blockChan:      make(chan *message.PieceMessage, blocksInPiece),
		blockSemaphore: make(chan struct{}, cfg.MaxPendingBlocks),
	}
	return &p, nil
}

func (p *PeerConn) downloadPieces(workQueue chan *pieceWork, results chan *pieceResult) {
	go p.readLoop()

	if p.peerState.IsChoked() {
		err := message.SendInterested(p.conn) // writer
		if err != nil {
			p.logger.Debug("Failed to send interested message", "error", err)
			return
		}
		p.logger.Debug("Sent interested message")
		p.peerState.WaitUnchoked()
	}

	for pw := range workQueue {

		if p.peerState.IsChoked() {
			p.logger.Debug("Waiting for unchoke message")
			workQueue <- pw
			p.peerState.WaitUnchoked()
			continue
		}

		lrg := p.logger.With("piece_work", pw)

		if !p.bitField.HasPiece(pw.index) {
			lrg.Debug("Peer doesn't have piece")
			workQueue <- pw
			continue
		}

		lrg.Debug("Piece download started")

		p.downloadBlocks(lrg, pw)

		if pw.blocksToDownload.Size() == 0 {
			hash := sha1.Sum(pw.data)
			if hash != pw.hash {
				for offset := uint32(0); offset < pw.length; offset += p.cfg.BlockSize {
					pw.blocksToDownload.Add(offset)
				}
				pw.data = make([]byte, 0, pw.length)
				workQueue <- pw
				lrg.Debug("Discarded piece: hash doesn't match")
			} else {
				results <- &pieceResult{
					index: pw.index,
					data:  pw.data,
				}
			}
		} else {
			lrg.Debug("Returned incomplete piece")
			workQueue <- pw
		}

		select {
		case <-p.ctx.Done():
			p.logger.Debug("Exited piece download loop")
			return
		default:
		}
	}
}

// func (p *PeerConn) downloadBlocks(lgr *slog.Logger, pw *pieceWork) {
// 	deadline := time.Now().Add(p.cfg.PieceTimeout)
// 	ctx, cancel := context.WithCancel(context.Background())
// 	proceed := make(chan struct{})

// 	defer func() {
// 		cancel()
// 		for {
// 			select {
// 			case <-p.blockSemaphore:
// 			default:
// 				return
// 			}
// 		}
// 	}()

// 	p.conn.SetDeadline(deadline)
// 	defer p.conn.SetDeadline(time.Time{})

// 	go func() {
// 		for begin := range pw.blocksToDownload.Range() {
// 			select {
// 			case p.blockSemaphore <- struct{}{}:
// 			default:
// 				if err := p.writer.Flush(); err != nil {
// 					p.conn.Close()
// 					lgr.Debug("Failed to flash block writer", "error", err)
// 				}
// 				lgr.Debug("Sent a banch of requests", "requests", cap(p.blockSemaphore))

// 				select {
// 				case <-proceed:
// 					p.blockSemaphore <- struct{}{}
// 				case <-ctx.Done():
// 					return
// 				}

// 			}
// 			length := min(p.cfg.BlockSize, pw.length-begin)

// 			msg := message.NewRequest(pw.index, begin, length)
// 			err := msg.Send(p.writer)

// 			if err != nil {
// 				lgr.Debug("Failed to send request message", "not_sent_msg", msg, "error", err)
// 				p.conn.Close()
// 				return

// 			}
// 			lgr.Debug("Sent request message", "sent_msg", msg)
// 		}
// 		if err := p.writer.Flush(); err != nil {
// 			p.conn.Close()
// 			lgr.Debug("Failed to flash block writer", "error", err)
// 		}
// 	}()

// 	downloadedBlocks := 0
// 	for {
// 		select {
// 		case <-time.After(time.Until(deadline)):
// 			select {
// 			case <-p.blockSemaphore:
// 				continue
// 			default:
// 				lgr.Debug("Piece timeout exceeded")
// 				return
// 			}

// 		case msg, ok := <-p.blockChan:
// 			if !ok {
// 				return
// 			}

// 			if msg.Index != pw.index || msg.Begin >= pw.length {
// 				lgr.Debug("Recived mesage out of window")
// 				continue
// 			}

// 			pw.blocksToDownload.Delete(msg.Begin)
// 			copy(pw.data[msg.Begin:pw.length], msg.Data)

// 			<-p.blockSemaphore
// 			lgr.Debug("Downloaded block", "recived_msg", msg)

// 			if pw.blocksToDownload.Size() == 0 {
// 				return
// 			}
// 			downloadedBlocks++
// 			if downloadedBlocks%cap(p.blockSemaphore) == 0 {
// 				proceed <- struct{}{}
// 			}
// 		}
// 	}
// }

func (p *PeerConn) downloadBlocks(lgr *slog.Logger, pw *pieceWork) {
	deadline := time.Now().Add(p.cfg.PieceTimeout)
	ctx, cancel := context.WithCancel(context.Background())

	defer func() {
		cancel()
	}()

	p.conn.SetDeadline(deadline)
	defer p.conn.SetDeadline(time.Time{})

	go func() {
		for begin := range pw.blocksToDownload.Range() {
			select {
			case p.blockSemaphore <- struct{}{}:
			default:
				if err := p.writer.Flush(); err != nil {
					p.conn.Close()
					lgr.Debug("Failed to flash block writer", "error", err)
				}
				lgr.Debug("Sent a banch of requests")

				select {
				case p.blockSemaphore <- struct{}{}:
				case <-ctx.Done():
					return
				}
			}
			length := min(p.cfg.BlockSize, pw.length-begin)

			msg := message.NewRequest(pw.index, begin, length)
			err := msg.Send(p.writer)

			if err != nil {
				lgr.Debug("Failed to send request message", "not_sent_msg", msg, "error", err)
				p.conn.Close()
				return

			}
			lgr.Debug("Sent request message", "sent_msg", msg)
		}
		if err := p.writer.Flush(); err != nil {
			p.conn.Close()
			lgr.Debug("Failed to flash block writer", "error", err)
		}
	}()

	for {
		select {
		case <-time.After(time.Until(deadline)):
			select {
			case <-p.blockSemaphore:
				continue
			default:
				lgr.Debug("Piece timeout exceeded")
				return
			}

		case msg, ok := <-p.blockChan:
			if !ok {
				return
			}

			if msg.Index != pw.index || msg.Begin >= pw.length {
				lgr.Debug("Recived mesage out of window")
				continue
			}

			pw.blocksToDownload.Delete(msg.Begin)
			copy(pw.data[msg.Begin:pw.length], msg.Data)

			<-p.blockSemaphore
			lgr.Debug("Downloaded block", "recived_msg", msg)

			if pw.blocksToDownload.Size() == 0 {
				return
			}
		}
	}
}

func (p *PeerConn) readLoop() {
	defer func() {
		p.conn.Close()
		p.connClosed <- struct{}{}
		p.cancel()
		p.logger.Debug("Exited read loop")
	}()

	for {
		msg, err := message.Recive(p.conn)

		if err != nil {
			close(p.blockChan)
			p.logger.Debug("Failed to recive message", "error", err)
			return
		}

		if msg == nil {
			p.logger.Debug("Recived keepalive message")
			continue
		}

		if msg.Type != message.TypeHave {
			p.logger.Debug("Recived message", "recived_msg", msg)
		}

		switch msg.Type {
		case message.TypeChoke:
			p.peerState.SetChoked()

		case message.TypeUnchoke:
			p.peerState.SetUnchoked()

		case message.TypeHave:
			p.bitField.SetPiece(binary.BigEndian.Uint32(msg.Payload))

		case message.TypeBitfield:
			p.bitField.Set(msg.Payload)

		case message.TypePiece:
			p.blockChan <- message.NewPiece(msg)
		}
	}
}
