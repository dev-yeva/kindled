package torrentfile

import (
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
)

type Files struct {
	files       []*file1
	pieceLength int64
}

func (fs *Files) LogValue() slog.Value {
	attrs := make([]slog.Attr, len(fs.files))
	for i, f := range fs.files {
		attrs[i] = slog.Any(strconv.Itoa(i+1), f) // or slog.Attr{Value: f.LogValue()}
	}
	return slog.GroupValue(attrs...)
}

type file1 struct {
	Path             string
	CumulativeLength int64 // Sum of this and previous file lengths
	File             *os.File
}

func (f *file1) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("path", f.Path),
		slog.Int64("cumulative_length", f.CumulativeLength),
	)
}

func NewFiles(capacity int, pieceLength int64) *Files {
	return &Files{files: make([]*file1, 0, capacity), pieceLength: pieceLength}
}

func (fs *Files) AddFile(path string, length int64) error {

	if err := os.MkdirAll(filepath.Dir(path), 0775); err != nil {
		return err
	}

	var cumLen int64
	if len(fs.files) == 0 {
		cumLen = length
	} else {
		lastIdx := len(fs.files) - 1
		cumLen = length + fs.files[lastIdx].CumulativeLength
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	fs.files = append(fs.files, &file1{
		Path:             path,
		CumulativeLength: cumLen,
		File:             file,
	})
	return nil
}

func (fs *Files) WritePiece(index int64, data []byte, lgr *slog.Logger) error {
	writeStart := index * fs.pieceLength
	writeEnd := writeStart + int64(len(data))

	for i, file := range fs.files {
		if writeStart < file.CumulativeLength {

			var fitIn int64
			if writeEnd <= file.CumulativeLength {
				fitIn = int64(len(data))
			} else {
				fitIn = file.CumulativeLength - writeStart
			}

			var writePosition int64
			if i == 0 {
				writePosition = writeStart
			} else {
				writePosition = writeStart - fs.files[i-1].CumulativeLength
			}

			_, err := file.File.WriteAt(data[:fitIn], writePosition)
			if err != nil {
				lgr.Error("Failed to write piece", "error", err)

				return err
			}

			if writeEnd <= file.CumulativeLength {
				break
			} else {
				data = data[fitIn:]
				writeStart = file.CumulativeLength
			}
		}
	}
	return nil
}
