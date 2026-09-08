// Implementation of https://www.bittorrent.org/beps/bep_0015.html
// TODO: add ipv6
package udptracker

import (
	"encoding/binary"
	"errors"
	"fmt"
	"kindled/config"
	"kindled/torrent/torrentfile"
	"kindled/torrent/tracker/common"
	"net"
	"net/url"
	"time"

	"math/rand/v2"
)

const (
	actionConnect  uint32 = 0
	actionAnnounce uint32 = 1
	actionScrap    uint32 = 2
	actionError    uint32 = 3

	eventNone      = 0 // No special event: refreshing the peer list & updating statistics
	eventCompleted = 1 // Torrent download completed
	eventStart     = 2 // Torrent download started
	eventStop      = 3 // Torrent download stopped
)

var (
	ErrResponse        = errors.New("tracker encountered an error") // action = 3
	ErrInvalidResponse = errors.New("invalid tracker response")     // malformed resp
	ErrRetryLimit      = errors.New("reached retry limit")          // error on every request
)

type connectRequest struct {
	ProtocolID    uint64 // magic constant
	Action        uint32 // defines the type of operation
	TransactionID uint32 // UDP doesn't guarantee packet arrival order, so we assign a unique ID to every request
}

type connectResponse struct {
	Action        uint32 // mirrors the request action on success, or changes to 3 if a tracker error occurs
	TransactionID uint32 // UDP doesn't guarantee packet arrival order, so we assign a unique ID to every request
	ConnectionID  uint64 // prevents IP spoofing & enables seamless client roaming
}

type AnnounceRequest struct {
	ConnectionID  uint64 // ID obtained from the connect request to verify the datagram's source IP address
	Action        uint32 // Type of operation
	TransactionID uint32 // ID of a request: UDP doesn't guarantee packet arrival order
	InfoHash      []byte // SHA-1 hash of Info field in torrent file. Acts as torrent ID
	PeerID        []byte // Identifier of this peer to other peers
	Downloaded    uint64 // Number of bytes downloaded
	Left          uint64 // Number of bytes left to complete download
	Uploaded      uint64 // Number of bytes sent to other peers
	Event         uint32 // Event associated with this announce: 0 = none, 1 = completed, 2 = started, 3 = stopped
	IP            uint32 // IP address. If 0, the tracker determines the address itself
	Key           uint32 // Random key used to identify this peer
	NumWant       uint32 // Number of peers to recive. The tracker may send more or less than this number. Default value: -1.
	Port          uint16 // Local incoming connection port
}

func MakeAnnounce(
	cfg config.UDPTracker,
	network string,
	trackerURL *url.URL,
	torrentFile *torrentfile.TorrentFile,
	peerID []byte,
	localPort uint16,
) (*common.AnnounceResponse, error) {

	conn, err := net.Dial(network, trackerURL.Host)
	if err != nil {
		return nil, err
	}

	connectRequest := connectRequest{
		ProtocolID:    0x41727101980,
		Action:        actionConnect,
		TransactionID: rand.Uint32(),
	}

	connectResponse, err := connect(cfg, conn, &connectRequest)
	if err != nil {
		return nil, err
	}

	announceReq := AnnounceRequest{
		ConnectionID:  connectResponse.ConnectionID,
		Action:        actionAnnounce,
		TransactionID: rand.Uint32(),
		InfoHash:      torrentFile.InfoHash,
		PeerID:        peerID,
		Downloaded:    0,
		Left:          torrentFile.TotalFileSize,
		Uploaded:      0,
		Event:         eventStart,
		IP:            0,
		Key:           rand.Uint32(),
		NumWant:       100,
		Port:          localPort,
	}

	return announce(cfg, conn, &announceReq)
}

func connect(cfg config.UDPTracker, conn net.Conn, req *connectRequest) (*connectResponse, error) {
	sendBuf := make([]byte, 16)
	binary.BigEndian.PutUint64(sendBuf[0:8], req.ProtocolID)
	binary.BigEndian.PutUint32(sendBuf[8:12], req.Action)
	binary.BigEndian.PutUint32(sendBuf[12:16], req.TransactionID)

	readBuf := make([]byte, 512) // 16 bytes for success, up to 512 bytes for tracker error messages

	for retries := range cfg.ConnectRetries {

		deadline := time.Now().Add(cfg.BaseConnectTimeout * (1 << retries))
		if err := conn.SetDeadline(deadline); err != nil {
			return nil, fmt.Errorf("failed to set connection request deadline: %w", err)
		}

		if _, err := conn.Write(sendBuf); err != nil {
			return nil, fmt.Errorf("failed to write connection request: %w", err)
		}

		n, err := conn.Read(readBuf)

		if err, ok := err.(net.Error); ok && err.Timeout() {
			continue
		} else if err != nil {
			return nil, fmt.Errorf("failed to read connection response: %w", err)
		}

		// the smallest message: [action][transaction ID]
		if n < 8 {
			continue
		}

		transactionID := binary.BigEndian.Uint32(readBuf[4:8])

		// not expected packet: UDP doesn't garantee delivery order!!!
		if transactionID != req.TransactionID {
			continue
		}

		action := binary.BigEndian.Uint32(readBuf[0:4])

		if action == actionError {
			errorMsg := string(readBuf[8:n])
			return nil, fmt.Errorf("%w: %s", ErrResponse, errorMsg)
		} else if action != actionConnect {
			continue
		}

		if n != 16 {
			return nil, ErrInvalidResponse
		}

		resp := connectResponse{
			Action:        action,
			TransactionID: transactionID,
			ConnectionID:  binary.BigEndian.Uint64(readBuf[8:16]),
		}
		return &resp, nil
	}

	return nil, ErrRetryLimit
}

func announce(cfg config.UDPTracker, conn net.Conn, req *AnnounceRequest) (*common.AnnounceResponse, error) {
	sendBuf := make([]byte, 98)
	binary.BigEndian.PutUint64(sendBuf[0:8], req.ConnectionID)
	binary.BigEndian.PutUint32(sendBuf[8:12], req.Action)
	binary.BigEndian.PutUint32(sendBuf[12:16], req.TransactionID)
	copy(sendBuf[16:36], req.InfoHash)
	copy(sendBuf[36:56], req.PeerID)
	binary.BigEndian.PutUint64(sendBuf[56:64], req.Downloaded)
	binary.BigEndian.PutUint64(sendBuf[64:72], req.Left)
	binary.BigEndian.PutUint64(sendBuf[72:80], req.Uploaded)
	binary.BigEndian.PutUint32(sendBuf[80:84], req.Event)
	binary.BigEndian.PutUint32(sendBuf[84:88], req.IP)
	binary.BigEndian.PutUint32(sendBuf[88:92], req.Key)
	binary.BigEndian.PutUint32(sendBuf[92:96], req.NumWant)
	binary.BigEndian.PutUint16(sendBuf[96:98], req.Port)

	readBuf := make([]byte, 20+6*req.NumWant)

	for retry := range cfg.AnnounceRetries {

		deadline := time.Now().Add(cfg.BaseAnnounceTimeout * (1 << retry))
		if err := conn.SetDeadline(deadline); err != nil {
			return nil, fmt.Errorf("failed to set announce request deadline: %w", err)
		}

		if _, err := conn.Write(sendBuf); err != nil {
			return nil, fmt.Errorf("failed to write announce request: %w", err)
		}

		n, err := conn.Read(readBuf)

		if err, ok := err.(net.Error); ok && err.Timeout() {
			continue
		} else if err != nil {
			return nil, fmt.Errorf("failed to read announce response: %w", err)
		}

		// the smallest message: [action][transaction ID]
		if n < 8 {
			continue
		}

		// mirrors the request action on success, or changes to 3 if a tracker error occurs
		action := binary.BigEndian.Uint32(readBuf[0:4])

		if action == actionError {
			errorMsg := string(readBuf[8:n])
			return nil, fmt.Errorf("%w: %s", ErrResponse, errorMsg)
		} else if action != req.Action {
			continue
		}

		transactionID := binary.BigEndian.Uint32(readBuf[4:8])

		if transactionID != req.TransactionID { // not expected packet: UDP doesn't garantee delivery order!!!
			continue
		}

		if n < 20 {
			return nil, ErrInvalidResponse
		}

		resp := common.AnnounceResponse{
			Interval:   binary.BigEndian.Uint32(readBuf[8:12]),
			Incomplete: binary.BigEndian.Uint32(readBuf[12:16]),
			Complete:   binary.BigEndian.Uint32(readBuf[16:20]),
			Peers:      common.ParseSocket(readBuf[20:n], 4, 2),
		}
		return &resp, nil
	}

	return nil, ErrRetryLimit
}
