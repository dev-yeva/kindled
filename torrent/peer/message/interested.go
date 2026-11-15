package message

import "io"

func SendInterested(writer io.Writer) error {
	msg := Message{Type: TypeInterested}
	return msg.Send(writer)
}
