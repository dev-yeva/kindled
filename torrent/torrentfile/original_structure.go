package torrentfile

type torrentFile struct {
	Info info `bencode:"info"`

	// Primary tracker
	Announce string `bencode:"announce"`

	// Alternate trackers in a form {{tier1}, {tier2},...}
	AnnounceList [][]string `bencode:"announce-list"`

	Comment      string `bencode:"comment"`
	CreatedBy    string `bencode:"created by"`
	CreationDate int64  `bencode:"creation date"`
}

type info struct {
	// Torrent name
	Name string `bencode:"name"`

	// File size in bytes. Empty for multi-file torrents
	Length uint64 `bencode:"length"`

	// Info about files. Empty for single-file torrents
	Files []file `bencode:"files"`

	// Size of the chunks the files are split into
	PieceLength uint32 `bencode:"piece length"`

	// Concatenated SHA-1 hashes of all chunks
	Pieces []byte `bencode:"pieces"`
}

type file struct {
	Path   []string `bencode:"path"`
	Length uint64   `bencode:"length"`
	Attr   string   `bencode:"attr"` // Often omitted. The only impotant one is "p" (padding file)
}
