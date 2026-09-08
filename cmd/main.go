package main

import (
	"crypto/rand"
	"fmt"
	"kindled/config"
	"kindled/torrent/torrentfile"
	"kindled/torrent/tracker"
	"kindled/torrent/tracker/common"
)

type torrentFile struct {
	Announce string `bencode:"announce"`
}

func main() {

	peerID := GeneratePeerId()

	torrentFile, err := torrentfile.Open("cachyos-desktop-linux-260809.torrent")
	if err != nil {
		panic(err)
	}

	cfg := config.New("config/config.yaml")

	announceClient := tracker.NewAnnounceClient(torrentFile, cfg)

	respChan := make(chan *common.AnnounceResponse)

	announceClient.MakeAnnounce(peerID, respChan)

	for resp := range respChan {
		fmt.Println(resp)
	}
}

func GeneratePeerId() []byte {
	peerId := make([]byte, 20)
	rand.Read(peerId)
	return peerId
}
