package downloader

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
)

func (g *fnosGateway) writeFrame(opcode byte, payload []byte) error {
	header := []byte{0x80 | opcode}
	length := len(payload)
	switch {
	case length < 126:
		header = append(header, 0x80|byte(length))
	case length <= 65535:
		header = append(header, 0x80|126, byte(length>>8), byte(length))
	default:
		header = append(header, 0x80|127)
		wide := make([]byte, 8)
		binary.BigEndian.PutUint64(wide, uint64(length))
		header = append(header, wide...)
	}
	mask := make([]byte, 4)
	if _, err := rand.Read(mask); err != nil {
		return err
	}
	header = append(header, mask...)
	masked := make([]byte, length)
	for i := range payload {
		masked[i] = payload[i] ^ mask[i%4]
	}
	_, err := g.conn.Write(append(header, masked...))
	return err
}

func (g *fnosGateway) readFrame() (byte, []byte, error) {
	first, err := g.reader.ReadByte()
	if err != nil {
		return 0, nil, err
	}
	second, err := g.reader.ReadByte()
	if err != nil {
		return 0, nil, err
	}
	length := uint64(second & 0x7f)
	if length == 126 {
		var value uint16
		if err = binary.Read(g.reader, binary.BigEndian, &value); err != nil {
			return 0, nil, err
		}
		length = uint64(value)
	} else if length == 127 {
		if err = binary.Read(g.reader, binary.BigEndian, &length); err != nil {
			return 0, nil, err
		}
	}
	if length > 4<<20 {
		return 0, nil, errors.New("飞牛 WebSocket 响应过大")
	}
	masked := second&0x80 != 0
	var mask [4]byte
	if masked {
		if _, err = io.ReadFull(g.reader, mask[:]); err != nil {
			return 0, nil, err
		}
	}
	payload := make([]byte, length)
	if _, err = io.ReadFull(g.reader, payload); err != nil {
		return 0, nil, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	if first&0x80 == 0 {
		return 0, nil, errors.New("暂不支持分片的飞牛 WebSocket 响应")
	}
	return first & 0x0f, payload, nil
}
