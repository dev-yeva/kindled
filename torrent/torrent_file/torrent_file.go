package torrent_file

import (
	"crypto/sha1"
	"io"
	"kindled/lib"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/zeebo/bencode"
)

type TorrentFile struct {
	InfoHash     [20]byte // SHA-1 hash of Info field. Acts as torrent ID
	TrackerList  []string // Tracker URLs
	Comment      string
	CreatedBy    string
	CreationDate int64
	Info         Info
}

type Info struct {
	Name        string
	TotalLength uint64     // Total size of data to download
	Files       *Files     // Unlike original structure, not empty for single-file torrents
	PieceLength uint32     // Size of the chunks files are split into
	PieceHashes [][20]byte // SHA-1 hashes of all chunks
}

func (tf TorrentFile) LogValue() slog.Value {

	return slog.GroupValue(
		slog.String("comment", tf.Comment),
		slog.String("created_by", tf.CreatedBy),
		slog.String("creation_date", time.Unix(tf.CreationDate, 0).Format("2/Jan/2006")),
		// slog.Int("trackers", len(tf.TrackerList)),
		// slog.String("comment", tf.Comment),
		// slog.String("created_by", tf.CreatedBy),
		slog.Int("files", len(tf.Info.Files.files)),
		slog.String("size", lib.ConvertBytes(tf.Info.TotalLength)),
		// slog.Int("pieces", len(tf.Info.PieceHashes)),
		// slog.Any("piece_size", lib.ConvertBytes(tf.Info.PieceLength)),
		//slog.String("info_hash", hex.EncodeToString(tf.InfoHash[:])),
	)
}

func Open(fileName, saveFolder string) (*TorrentFile, error) {

	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var tf torrentFile
	if err := bencode.DecodeBytes(data, &tf); err != nil {
		return nil, err
	}

	raw := struct {
		Info bencode.RawMessage `bencode:"info"`
	}{}
	if err := bencode.DecodeBytes(data, &raw); err != nil {
		return nil, err
	}

	infoHash := sha1.Sum(raw.Info)

	trackerList := make([]string, 0, len(tf.AnnounceList)+1)
	trackerList = append(trackerList, tf.Announce)
	for _, URL := range tf.AnnounceList {
		trackerList = append(trackerList, URL...)
	}

	var totalLength uint64 = 0
	files := NewFiles(max(1, len(tf.Info.Files)), int64(tf.Info.PieceLength))
	if tf.Info.Files == nil { // single-file torrent
		totalLength = tf.Info.Length
		files.AddFile(filepath.Join(saveFolder, tf.Info.Name), int64(tf.Info.Length))
	} else { // multi-file torrent
		for _, f := range tf.Info.Files {
			totalLength += f.Length
			path := filepath.Join(f.Path...)
			files.AddFile(filepath.Join(saveFolder, tf.Info.Name, path), int64(f.Length))
		}
	}

	pieceCount := uint32(len(tf.Info.Pieces) / 20)
	pieceHashes := make([][20]byte, 0, pieceCount)
	for i := 0; i < len(tf.Info.Pieces); i += 20 {
		var hash [20]byte
		copy(hash[:], tf.Info.Pieces[i:i+20])
		pieceHashes = append(pieceHashes, hash)
	}

	return &TorrentFile{
		InfoHash:     infoHash,
		TrackerList:  trackerList,
		Comment:      tf.Comment,
		CreatedBy:    tf.CreatedBy,
		CreationDate: tf.CreationDate,
		Info: Info{
			Name:        tf.Info.Name,
			TotalLength: totalLength,
			Files:       files,
			PieceLength: tf.Info.PieceLength,
			PieceHashes: pieceHashes,
		},
	}, nil
}
