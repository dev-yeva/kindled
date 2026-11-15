package message

import (
	"encoding/binary"
	"io"
	"log/slog"
)

type Request struct {
	Index  uint32
	Begin  uint32
	Length uint32
}

func (r *Request) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("type", "request"),
		slog.Any("index", r.Index),
		slog.Any("begin", r.Begin),
		slog.Any("length", r.Length),
	)
}

func NewRequest(index, begin, length uint32) *Request {
	return &Request{Index: index, Begin: begin, Length: length}
}

func (r *Request) Send(writer io.Writer) error {
	payload := make([]byte, 4*3)

	binary.BigEndian.PutUint32(payload[0:4], r.Index)
	binary.BigEndian.PutUint32(payload[4:8], r.Begin)
	binary.BigEndian.PutUint32(payload[8:12], r.Length)

	msg := Message{Type: TypeRequest, Payload: payload}

	return msg.Send(writer)
}
