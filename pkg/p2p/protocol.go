package p2p

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

var (
	MagicHeader = [2]byte{0x53, 0x4D} // "SM" SovereignMesh
)

const (
	MsgTypeHandshake    byte = 0x01
	MsgTypeHandshakeAck byte = 0x02
	MsgTypeGossipPost   byte = 0x03
	MsgTypePing         byte = 0x04
	MsgTypePong         byte = 0x05
)

type HandshakeMessage struct {
	Address   string `json:"address"`
	PubKey    string `json:"pubkey"`
	Timestamp int64  `json:"timestamp"`
	Nonce     string `json:"nonce"`
	Signature string `json:"signature"`
}

type Packet struct {
	Type    byte
	Payload []byte
}

// WritePacket writes a framed packet to the writer.
// Format: [2 bytes Magic][1 byte Type][4 bytes Length][Payload]
func WritePacket(w io.Writer, msgType byte, payload []byte) error {
	if _, err := w.Write(MagicHeader[:]); err != nil {
		return err
	}
	if _, err := w.Write([]byte{msgType}); err != nil {
		return err
	}
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, uint32(len(payload)))
	if _, err := w.Write(lengthBuf); err != nil {
		return err
	}
	if len(payload) > 0 {
		if _, err := w.Write(payload); err != nil {
			return err
		}
	}
	return nil
}

// ReadPacket reads a framed packet from the reader.
func ReadPacket(r io.Reader) (*Packet, error) {
	var magic [2]byte
	if _, err := io.ReadFull(r, magic[:]); err != nil {
		return nil, err
	}
	if magic != MagicHeader {
		return nil, errors.New("invalid protocol magic bytes")
	}

	var msgType [1]byte
	if _, err := io.ReadFull(r, msgType[:]); err != nil {
		return nil, err
	}

	var lengthBuf [4]byte
	if _, err := io.ReadFull(r, lengthBuf[:]); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(lengthBuf[:])

	// Max payload cap: 4 MB
	if length > 4*1024*1024 {
		return nil, fmt.Errorf("payload length %d exceeds maximum limit", length)
	}

	payload := make([]byte, length)
	if length > 0 {
		if _, err := io.ReadFull(r, payload); err != nil {
			return nil, err
		}
	}

	return &Packet{
		Type:    msgType[0],
		Payload: payload,
	}, nil
}

func EncodeJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func DecodeJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
