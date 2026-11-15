package message

import (
	"encoding/binary"
	"log/slog"
)

type PieceMessage struct {
	Index uint32
	Begin uint32 // First index of block
	Data  []byte
}

func (pm PieceMessage) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("type", "piece"),
		slog.Any("index", pm.Index),
		slog.Any("begin", pm.Begin),
		slog.Any("data", len(pm.Data)),
	)
}

func NewPiece(msg *Message) *PieceMessage {
	return &PieceMessage{
		Index: binary.BigEndian.Uint32(msg.Payload[0:4]),
		Begin: binary.BigEndian.Uint32(msg.Payload[4:8]),
		Data:  msg.Payload[8:],
	}
}
