package tracker

import (
	"fmt"
	"kindled/config"
	"kindled/torrent/torrentfile"
	"kindled/torrent/tracker/common"
	"kindled/torrent/tracker/httptracker"
	"kindled/torrent/tracker/udptracker"
	"net/http"
	"net/url"
	"sync"
)

type AnnounceClient struct {
	torrentFile *torrentfile.TorrentFile
	httpClient  *http.Client
	cfg         *config.Config
}

func NewAnnounceClient(tf *torrentfile.TorrentFile, cfg *config.Config) *AnnounceClient {
	return &AnnounceClient{
		torrentFile: tf,
		httpClient:  &http.Client{},
		cfg:         cfg,
	}
}

func (a *AnnounceClient) MakeAnnounce(peerID []byte, respChan chan<- *common.AnnounceResponse) {

	var wg sync.WaitGroup

	for _, u := range a.torrentFile.TrackerList {

		trackerURL, err := url.Parse(u)
		if err != nil {
			continue
		}

		wg.Go(func() {

			switch trackerURL.Scheme {
			case "http":
				resp, err := httptracker.MakeHTTPAnnounce(
					a.cfg.HTTPTracker,
					a.httpClient,
					trackerURL,
					a.torrentFile,
					peerID,
					a.cfg.LocalPort,
				)
				if err != nil {
					fmt.Println(err)
					return
				}
				respChan <- resp

			case "udp":
				resp, err := udptracker.MakeAnnounce(
					a.cfg.UDPTracker,
					"udp4",
					trackerURL,
					a.torrentFile,
					peerID,
					a.cfg.LocalPort,
				)
				if err != nil {
					fmt.Println(err)
					return
				}
				respChan <- resp
			default:
				fmt.Printf("unsupported tracker scheme: %s", trackerURL.Scheme)
				return
			}
		})
	}

	go func() {
		wg.Wait()
		close(respChan)
	}()
}
