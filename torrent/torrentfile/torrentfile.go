package torrentfile

import (
	"crypto/sha1"
	"io"
	"os"

	"github.com/zeebo/bencode"
)

type TorrentFile struct {
	Info Info `bencode:"info"`

	// SHA-1 hash of Info field. Acts as torrent id
	InfoHash []byte

	// Size of all files in bytes
	TotalFileSize uint64

	// announce + announce list
	TrackerList []string `bencode:"-"`

	Comment      string `bencode:"comment"`
	CreatedBy    string `bencode:"created by"`
	CreationDate int64  `bencode:"creation date"`
}

type Info struct {
	// Torrent name
	Name string `bencode:"name"`

	// File size in bytes. Empty for a multi-file torrent
	Length uint64 `bencode:"length"`

	// Info about files. Empty for a single-file torrent
	Files []File `bencode:"files"`

	// Size of the chunks the files are split into
	PieceLength int `bencode:"piece length"`

	// Concatenated SHA-1 hashes of all chunks
	Pieces []byte `bencode:"pieces"`
}

type File struct {
	Path []string `bencode:"path"`

	Length uint64 `bencode:"length"`

	// Often omitted. The only impotant one is "p" (padding file)
	Attr string `bencode:"attr"`
}

func Open(fileName string) (*TorrentFile, error) {

	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var torrentFile TorrentFile
	if err := bencode.DecodeBytes(data, &torrentFile); err != nil {
		return nil, err
	}

	supportStruct := struct {
		// Primary tracker
		Announce string `bencode:"announce"`

		// Alternate trackers: {{tier1}, {tier2},...}
		AnnounceList [][]string `bencode:"announce-list"`

		InfoData bencode.RawMessage `bencode:"info"`
	}{}

	if err := bencode.DecodeBytes(data, &supportStruct); err != nil {
		return nil, err
	}

	hash := sha1.Sum(supportStruct.InfoData)
	torrentFile.InfoHash = hash[:]

	torrentFile.TrackerList = make([]string, 0, 1+len(supportStruct.AnnounceList))
	torrentFile.TrackerList = append(torrentFile.TrackerList, supportStruct.Announce)
	for _, URL := range supportStruct.AnnounceList {
		torrentFile.TrackerList = append(torrentFile.TrackerList, URL...)
	}

	var totalFileSize uint64 = 0
	if torrentFile.Info.Files != nil {
		for _, files := range torrentFile.Info.Files {
			totalFileSize += files.Length
		}
	} else {
		totalFileSize = torrentFile.Info.Length
	}
	torrentFile.TotalFileSize = totalFileSize

	return &torrentFile, nil
}
