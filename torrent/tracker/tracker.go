package tracker

import (
	"kindled/config"
	"kindled/lib/addresses"
	"kindled/torrent/torrentfile"
	"kindled/torrent/tracker/common"
	"kindled/torrent/tracker/httptracker"
	"kindled/torrent/tracker/udptracker"
	"log/slog"
	"net/http"
	"net/netip"
	"net/url"
	"sync"
)

type AnnounceClient struct {
	logger      slog.Logger
	torrentFile *torrentfile.TorrentFile
	httpClient  *http.Client
	httpCfg     *config.HTTPTracker
	udpCfg      *config.UDPTracker
	localPort   uint16
}

func NewAnnounceClient(
	logger slog.Logger,
	tf *torrentfile.TorrentFile,
	httpCfg *config.HTTPTracker,
	udpCfg *config.UDPTracker,
	localPort uint16,
) *AnnounceClient {
	return &AnnounceClient{
		logger:      logger,
		torrentFile: tf,
		httpClient:  &http.Client{},
		httpCfg:     httpCfg,
		udpCfg:      udpCfg,
		localPort:   localPort,
	}
}

func (a *AnnounceClient) MakeAnnounce(localPeerID []byte) addresses.Addresses {
	respChan := make(chan *common.AnnounceResponse)
	addrs := addresses.New(len(a.torrentFile.TrackerList))
	workingTrackers := 0

	go a.makeAnnounce(localPeerID, respChan)

	for resp := range respChan {
		addrs.Add(resp.Peers...)
		workingTrackers++
	}
	a.logger.Info("Completed announces", "working_trackers", workingTrackers)
	return addrs
}

func (a *AnnounceClient) MakeLocalAnnounce(localPeerID []byte) addresses.Addresses {
	addrs := addresses.New(len(a.torrentFile.TrackerList))
	addr, err := netip.ParseAddr("127.0.0.1")
	if err != nil {
		a.logger.Debug("Failed to parse localhost", "error", err)
	}
	addrs.Add(netip.AddrPortFrom(addr, 26784)) // qbitorrent port
	return addrs
}

func (a *AnnounceClient) makeAnnounce(localPeerID []byte, respChan chan<- *common.AnnounceResponse) {

	var wg sync.WaitGroup

	for _, u := range a.torrentFile.TrackerList {

		trackerURL, err := url.Parse(u)
		if err != nil {
			a.logger.Debug("Failed to parse tracker url", "tracker_url", u)
			continue
		}
		logger := a.logger.With("tracker_url", trackerURL)

		wg.Go(func() {

			switch trackerURL.Scheme {
			case "http":
				resp, err := httptracker.MakeHTTPAnnounce(
					a.httpCfg,
					a.httpClient,
					trackerURL,
					a.torrentFile,
					localPeerID,
					a.localPort,
				)
				if err != nil {
					logger.Debug("Tracker announce failed", "error", err)
				} else {
					logger.Debug("Tracker announce succeeded")
					respChan <- resp
				}

			case "udp":
				resp, err := udptracker.MakeAnnounce(
					a.udpCfg,
					"udp4",
					trackerURL,
					a.torrentFile,
					localPeerID,
					a.localPort,
				)
				if err != nil {
					logger.Debug("Failed to announce", "error", err)
				} else {
					logger.Debug("Completed announce")
					respChan <- resp
				}

			default:
				logger.Debug("unsupported tracker scheme")
			}
		})
	}

	go func() {
		wg.Wait()
		close(respChan)
	}()
}
