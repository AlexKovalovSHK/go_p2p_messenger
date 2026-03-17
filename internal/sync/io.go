package sync

import (
	"fmt"

	"github.com/libp2p/go-msgio"
	"google.golang.org/protobuf/proto"
)

// WriteMsg writes a length-prefixed protobuf message to the writer.
func WriteMsg(w msgio.WriteCloser, msg proto.Message) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal proto: %w", err)
	}
	return w.WriteMsg(data)
}

// ReadMsg reads a length-prefixed protobuf message from the reader.
func ReadMsg(r msgio.ReadCloser, msg proto.Message) error {
	data, err := r.ReadMsg()
	if err != nil {
		return fmt.Errorf("read msgio: %w", err)
	}
	defer r.ReleaseMsg(data)

	if err := proto.Unmarshal(data, msg); err != nil {
		return fmt.Errorf("unmarshal proto: %w", err)
	}
	return nil
}
