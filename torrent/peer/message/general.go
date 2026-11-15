package message

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
)

const (
	TypeChoke         = 0
	TypeUnchoke       = 1
	TypeInterested    = 2
	TypeNotInterested = 3
	TypeHave          = 4
	TypeBitfield      = 5
	TypeRequest       = 6
	TypePiece         = 7
	TypeCancel        = 8
)

type Message struct {
	Type    uint8
	Payload []byte
}

func (m *Message) LogValue() slog.Value {

	switch m.Type {

	case TypeChoke:
		return slog.GroupValue(slog.String("type", "choke"))

	case TypeUnchoke:
		return slog.GroupValue(slog.String("type", "unchoke"))

	case TypeHave:
		return slog.GroupValue(
			slog.String("type", "have"),
			slog.String("payload", hex.EncodeToString(m.Payload)),
		)

	case TypeBitfield:
		return slog.GroupValue(
			slog.String("type", "bitfield"),
			slog.String("payload", "[...]"),
		)

	case TypePiece:
		pieceMsg := NewPiece(m)
		return pieceMsg.LogValue()
	}

	return slog.GroupValue(
		slog.String("type", fmt.Sprintf("unknown: %d", m.Type)),
	)
}

func (m *Message) Send(writer io.Writer) error {
	buf := make([]byte, 4+1+len(m.Payload))

	binary.BigEndian.PutUint32(buf[0:4], uint32(len(m.Payload)+1))
	buf[4] = m.Type
	copy(buf[5:], m.Payload)

	if _, err := writer.Write(buf); err != nil {
		return err
	}
	return nil
}

func Recive(reader io.Reader) (*Message, error) {
	bufMsgSize := make([]byte, 4)

	_, err := io.ReadFull(reader, bufMsgSize)
	if err != nil {
		return nil, err
	}

	msgSize := binary.BigEndian.Uint32(bufMsgSize)
	if msgSize == 0 {
		return nil, nil
	}

	msgBuf := make([]byte, msgSize)
	_, err = io.ReadFull(reader, msgBuf)
	if err != nil {
		return nil, err
	}

	return &Message{Type: msgBuf[0], Payload: msgBuf[1:]}, nil
}
