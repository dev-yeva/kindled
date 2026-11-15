package httptracker

import (
	"context"
	"fmt"
	"io"
	"kindled/config"
	"kindled/torrent/torrentfile"
	"kindled/torrent/tracker/common"
	"net"
	"net/http"
	"net/url"
	"strconv"

	"github.com/zeebo/bencode"
)

type AnnounceRequest struct {
	InfoHash    []byte // SHA-1 hash of Info field in torrent file. Acts as torrent ID
	localPeerID []byte // Identifier of this peer to other peers
	LocalPort   uint16 // Local incoming connection port
	Uploaded    uint64 // Number of bytes sent to other peers
	Downloaded  uint64 // Number of bytes downloaded
	Left        uint64 // Number of bytes left to complete download
}

// TODO: ОБРАБОТАТЬ ОШИБКИ ТРЕКЕРА ТИПА d14:failure reason20:unregistered torrente
func MakeHTTPAnnounce(
	cfg *config.HTTPTracker,
	client *http.Client,
	trackerURL *url.URL,
	torrentFile *torrentfile.TorrentFile,
	localPeerID []byte,
	localPort uint16,
) (*common.AnnounceResponse, error) {

	announceRequest := AnnounceRequest{
		InfoHash:    torrentFile.InfoHash[:],
		localPeerID: localPeerID,
		LocalPort:   localPort,
		Uploaded:    0,
		Downloaded:  0,
		Left:        torrentFile.Info.TotalLength,
	}

	announceURL := buildAnnounceURL(*trackerURL, announceRequest)
	var lastErr error

	for retry := range cfg.AnnounceRetries {

		ctx, cancel := context.WithTimeout(context.Background(), cfg.BaseAnnounceTimeout*(1<<retry))

		req, err := http.NewRequestWithContext(ctx, "GET", announceURL, nil)
		if err != nil {
			cancel()
			return nil, err
		}

		resp, err := client.Do(req)
		cancel()

		if err, ok := err.(net.Error); ok && err.Timeout() {
			lastErr = err
			continue
		} else if err != nil {
			return nil, err
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		// pp.Println(string(body))
		// 5xx - Server error; 429 - Too Many Requests
		if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
			lastErr = fmt.Errorf("tracker returned retryable status: %s", resp.Status)
			continue
		}

		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("tracker returned fatal status: %s", resp.Status)
		}

		var announceResponse common.AnnounceResponse
		if err := bencode.DecodeBytes(body, &announceResponse); err != nil {
			return nil, err
		}

		peers := struct {
			Peers  []byte `bencode:"peers"`
			Peers6 []byte `bencode:"peers6"`
		}{}
		_ = bencode.DecodeBytes(body, &peers)

		announceResponse.Peers = common.ParseSocket(peers.Peers, 4, 2)
		announceResponse.Peers6 = common.ParseSocket(peers.Peers6, 16, 2)

		return &announceResponse, nil
	}

	return nil, lastErr
}

func buildAnnounceURL(trackerURL url.URL, announceRequest AnnounceRequest) string {

	params := url.Values{
		"info_hash":  []string{string(announceRequest.InfoHash)},
		"peer_id":    []string{string(announceRequest.localPeerID)},
		"port":       []string{strconv.FormatUint(uint64(announceRequest.LocalPort), 10)},
		"uploaded":   []string{strconv.FormatUint(announceRequest.Uploaded, 10)},
		"downloaded": []string{strconv.FormatUint(announceRequest.Downloaded, 10)},
		"left":       []string{strconv.FormatUint(announceRequest.Left, 10)},
		"compact":    []string{"1"},
	}
	trackerURL.RawQuery = params.Encode()

	return trackerURL.String()
}
