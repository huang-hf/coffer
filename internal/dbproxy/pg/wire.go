package pg

import (
	"encoding/binary"
	"fmt"
	"io"
)

const (
	protocolVersion3     = 196608
	sslRequestCode       = 80877103
	cancelRequestCode    = 80877102
	sslRequestLength     = 8
	cancelRequestLength  = 16
	authTypeOK         = 0
	authTypeMD5        = 5
	authTypeSASL       = 10
	authTypeSASLContinue = 11
	authTypeSASLFinal  = 12
)

type StartupMessage struct {
	User     string
	Database string
	Params   map[string]string
}

func readStartupMessage(r io.Reader) (*StartupMessage, error) {
	var length int32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return nil, fmt.Errorf("reading startup message length: %w", err)
	}
	if length < 8 {
		return nil, fmt.Errorf("startup message too short: %d", length)
	}

	var protocol int32
	if err := binary.Read(r, binary.BigEndian, &protocol); err != nil {
		return nil, fmt.Errorf("reading protocol version: %w", err)
	}

	if protocol == sslRequestCode || protocol == cancelRequestCode {
		return nil, fmt.Errorf("unexpected protocol code: %d", protocol)
	}
	if protocol != protocolVersion3 {
		return nil, fmt.Errorf("unsupported protocol version: %d", protocol)
	}

	payloadLen := length - 8
	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, fmt.Errorf("reading startup payload: %w", err)
	}

	msg := &StartupMessage{Params: make(map[string]string)}
	var key string
	for i := 0; i < len(payload); i++ {
		end := i
		for end < len(payload) && payload[end] != 0 {
			end++
		}
		val := string(payload[i:end])
		i = end
		if key == "" {
			key = val
		} else {
			switch key {
			case "user":
				msg.User = val
			case "database":
				msg.Database = val
			default:
				msg.Params[key] = val
			}
			key = ""
		}
	}
	return msg, nil
}

func writeStartupMessage(user, database string) []byte {
	params := "user\x00" + user + "\x00" + "database\x00" + database + "\x00"
	length := 4 + 4 + len(params)
	buf := make([]byte, length)
	binary.BigEndian.PutUint32(buf[0:4], uint32(length))
	binary.BigEndian.PutUint32(buf[4:8], protocolVersion3)
	copy(buf[8:], params)
	return buf
}

func readServerMessage(r io.Reader) (byte, []byte, error) {
	var header [5]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return 0, nil, fmt.Errorf("reading server message header: %w", err)
	}
	msgType := header[0]
	msgLen := binary.BigEndian.Uint32(header[1:5])
	if msgLen < 4 {
		return 0, nil, fmt.Errorf("invalid message length: %d", msgLen)
	}
	payload := make([]byte, msgLen-4)
	if len(payload) > 0 {
		if _, err := io.ReadFull(r, payload); err != nil {
			return 0, nil, fmt.Errorf("reading server message payload: %w", err)
		}
	}
	return msgType, payload, nil
}

func writeServerMessage(w io.Writer, msgType byte, payload []byte) error {
	length := uint32(4 + len(payload))
	buf := make([]byte, 5+len(payload))
	buf[0] = msgType
	binary.BigEndian.PutUint32(buf[1:5], length)
	copy(buf[5:], payload)
	_, err := w.Write(buf)
	return err
}

func readAuthType(payload []byte) (int32, []byte, error) {
	if len(payload) < 4 {
		return 0, nil, fmt.Errorf("auth message too short")
	}
	authType := int32(binary.BigEndian.Uint32(payload[0:4]))
	return authType, payload[4:], nil
}
