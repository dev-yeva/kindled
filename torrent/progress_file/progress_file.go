package progress_file

import (
	"encoding/binary"
	"fmt"
	"iter"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

const (
	mode          = 0644
	suffix        = "_progress.bin"
	piecesPerSave = 50
)

type ProgressFile struct {
	lgr              *slog.Logger
	file             *os.File
	bitmap           []byte
	downloadedPieces uint32
	pieceCount       uint32
}

func New(
	lgr *slog.Logger,
	name string,
	folderPath string,
	createNameSubfolder bool,
	pieceCount uint32,
) (*ProgressFile, error) {

	var path string
	if createNameSubfolder {
		path = filepath.Join(folderPath, name, progressPath(name))
	} else {
		path = filepath.Join(folderPath, progressPath(name))
	}

	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, mode)
	if err != nil {
		return nil, fmt.Errorf("Failed to create/open progress file: %w", err)
	}

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("Failed to get progress file stat: %w", err)
	}
	bitmapSize := pieceCount/8 + 1
	bitmap := make([]byte, bitmapSize)
	var downloadedPieces uint32

	if stat.Size() != 4+int64(bitmapSize) {
		if stat.Size() != 0 && file.Truncate(0) != nil {
			return nil, fmt.Errorf("Failed to clear malformed progress file: %w", err)
		}

		lastByte := 0
		for i := range 8 - pieceCount%8 {
			lastByte |= 1 << i
		}
		bitmap[len(bitmap)-1] = byte(lastByte)
	} else {
		downloadedPiecesBytes := make([]byte, 4)
		if _, err := file.Read(downloadedPiecesBytes); err != nil {
			return nil, fmt.Errorf("Failed to read downloaded pieces count from progress file: %w", err)
		}
		downloadedPieces = binary.BigEndian.Uint32(downloadedPiecesBytes)

		if _, err := file.Read(bitmap); err != nil {
			return nil, fmt.Errorf("Failed to read bitmap from progress file: %w", err)
		}
	}

	return &ProgressFile{
		lgr:              lgr,
		file:             file,
		bitmap:           bitmap,
		downloadedPieces: downloadedPieces,
		pieceCount:       pieceCount,
	}, nil
}

func progressPath(name string) string {

	var b strings.Builder
	b.Grow(len(name) + len(suffix))

	for _, r := range name {
		if r == ' ' || r == '.' || r == '-' {
			b.WriteByte('_')
		} else {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	b.WriteString(suffix)

	return b.String()
}

func (pf *ProgressFile) MissingPiecesRange() iter.Seq[uint32] {
	return func(yield func(uint32) bool) {
		for i := range len(pf.bitmap) {
			index := uint32(i)
			b := pf.bitmap[index]
			for offset := range uint32(8) {
				if b&(1<<(7-offset)) == 0 {
					if !yield(index*8 + offset) {
						return
					}
				}
			}

		}
	}
}

func (pf *ProgressFile) MissingPiecesCount() uint32 {
	return pf.pieceCount - pf.downloadedPieces
}

func (pf *ProgressFile) RecordPiece(index uint32) {
	byteIndex := index / 8
	offset := index % 8

	pf.bitmap[byteIndex] |= 1 << (7 - offset)
	pf.downloadedPieces++

	if pf.downloadedPieces%piecesPerSave == 0 {
		pf.Save()
	}
}

func (pf *ProgressFile) Print(workingPeers int32) {
	pf.lgr.Info(
		"\rDownloading torrent",
		"progress", fmt.Sprintf("%.1f%%", 100*float64(pf.downloadedPieces)/float64(pf.pieceCount)),
		"peers", workingPeers,
	)
}

func (pf *ProgressFile) Save() {
	pf.file.Seek(0, 0)
	downloadedPiecesBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(downloadedPiecesBytes, pf.downloadedPieces)
	pf.file.Write(downloadedPiecesBytes)
	pf.file.Write(pf.bitmap)
}

func (pf *ProgressFile) Complete() {
	pf.file.Close()
	os.Remove(pf.file.Name())
}
