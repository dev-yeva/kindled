package torrentfile

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"iter"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"unicode"
)

const (
	progressFileMode   = 0644
	progressFileSiffix = "_progress.bin"
)

type ProgressFile struct {
	file             *os.File
	downloadedPieces *atomic.Uint32
	pieceCount       uint32
}

func NewProgressFile(name string, folderPath string, pieceCount uint32) (*ProgressFile, error) {

	path := filepath.Join(folderPath, progressPath(name))
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, progressFileMode)
	if err != nil {
		return nil, fmt.Errorf("Failed to create/open progress file: %w", err)
	}

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("Failed to get progress file stat: %w", err)
	}

	bitmapSize := pieceCount/8 + 1
	if stat.Size() != 4+int64(bitmapSize) {
		bitmap := make([]byte, 4+bitmapSize)
		lastByte := 0
		for i := range 8 - pieceCount%8 {
			lastByte |= 1 << i
		}
		bitmap[len(bitmap)-1] = byte(lastByte)
		file.Write(bitmap)
	}

	downloadedPiecesBytes := make([]byte, 4)
	// ReadAt does not advance the file offset.
	// Next read must skip the 4-byte header containing downloaded pieces.
	if _, err = file.ReadAt(downloadedPiecesBytes, 0); err != nil {
		return nil, fmt.Errorf("Failed to get downloaded pieces from progress file: %w", err)
	}
	var downloadedPieces atomic.Uint32
	downloadedPieces.Add(binary.BigEndian.Uint32(downloadedPiecesBytes))

	return &ProgressFile{
		file:             file,
		downloadedPieces: &downloadedPieces,
		pieceCount:       pieceCount,
	}, nil
}

func progressPath(name string) string {

	var b strings.Builder
	b.Grow(len(name) + len(progressFileSiffix))

	for _, r := range name {
		if r == ' ' || r == '.' || r == '-' {
			b.WriteByte('_')
		} else {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	b.WriteString(progressFileSiffix)

	return b.String()
}

func (pf *ProgressFile) DownloadedPieces() *atomic.Uint32 {
	return pf.downloadedPieces
}

func (pf *ProgressFile) PieceCount() uint32 {
	return pf.pieceCount
}

func (pf *ProgressFile) MissingPiecesRange() iter.Seq[uint32] {

	pf.file.Seek(4, 0)
	reader := bufio.NewReader(pf.file)

	return func(yield func(uint32) bool) {
		for index := uint32(0); ; index++ {
			b, err := reader.ReadByte()
			if err != nil {
				return
			}
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
	return pf.pieceCount - pf.downloadedPieces.Load()
}

func (pf *ProgressFile) RecordPiece(index int64) error {
	byteArr := make([]byte, 1)
	byteIndex := index / 8
	offset := index % 8

	_, err := pf.file.ReadAt(byteArr, byteIndex+4)
	if err != nil {
		return err
	}

	byteArr[0] |= 1 << (7 - offset)

	_, err = pf.file.WriteAt(byteArr, byteIndex+4)
	if err != nil {
		return err
	}

	pf.downloadedPieces.Add(1)
	pf.SaveDownloadedPieces()

	return nil
}

func (pf *ProgressFile) SaveDownloadedPieces() {
	downloadedPiecesBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(downloadedPiecesBytes, pf.downloadedPieces.Load())
	pf.file.WriteAt(downloadedPiecesBytes, 0)
}

func (pf *ProgressFile) Complete() {
	pf.file.Close()
	os.Remove(pf.file.Name())
}
