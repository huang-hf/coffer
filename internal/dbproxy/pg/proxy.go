package pg

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

type DBConfig struct {
	Host     string
	Port     int
	User     string
	Database string
	Password string
}

type Proxy struct {
	config DBConfig
	ln     net.Listener
	mu     sync.Mutex
	closed bool
}

func NewProxy(cfg DBConfig) *Proxy {
	return &Proxy{config: cfg}
}

func (p *Proxy) Start() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("listening: %w", err)
	}
	p.ln = ln
	go p.acceptLoop()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

func (p *Proxy) Addr() net.Addr {
	if p.ln == nil {
		return nil
	}
	return p.ln.Addr()
}

func (p *Proxy) Close() error {
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()
	if p.ln != nil {
		return p.ln.Close()
	}
	return nil
}

func (p *Proxy) acceptLoop() {
	for {
		conn, err := p.ln.Accept()
		if err != nil {
			p.mu.Lock()
			closed := p.closed
			p.mu.Unlock()
			if closed {
				return
			}
			continue
		}
		go p.handleConn(conn)
	}
}

func (p *Proxy) handleConn(client net.Conn) {
	defer client.Close()

	startup, err := func() (*StartupMessage, error) {
		var first4 [4]byte
		if _, err := io.ReadFull(client, first4[:]); err != nil {
			return nil, err
		}
		length := binary.BigEndian.Uint32(first4[:])

		if length == sslRequestLength {
			var codeBuf [4]byte
			if _, err := io.ReadFull(client, codeBuf[:]); err != nil {
				return nil, err
			}
			if binary.BigEndian.Uint32(codeBuf[:]) == sslRequestCode {
				client.Write([]byte{'N'})
				return readStartupMessage(client)
			}
			return nil, fmt.Errorf("unknown short protocol code: %d", binary.BigEndian.Uint32(codeBuf[:]))
		}

		reader := io.MultiReader(bytes.NewReader(first4[:]), client)
		return readStartupMessage(reader)
	}()
	if err != nil {
		return
	}

	if startup.User != "" && startup.User != p.config.User {
		writeErrorResponse(client, fmt.Sprintf(`role "%s" does not exist`, startup.User))
		return
	}
	dbName := startup.Database
	if dbName == "" {
		dbName = p.config.Database
	}

	backend, err := p.connectBackend(dbName)
	if err != nil {
		writeErrorResponse(client, err.Error())
		return
	}
	defer backend.Close()

	if _, err := backend.Write(writeStartupMessage(p.config.User, dbName)); err != nil {
		return
	}

	if err := p.handleAuth(client, backend); err != nil {
		return
	}

	p.relayLoop(client, backend)
}

func (p *Proxy) connectBackend(database string) (net.Conn, error) {
	addr := fmt.Sprintf("%s:%d", p.config.Host, p.config.Port)
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", addr, err)
	}
	return conn, nil
}

func (p *Proxy) handleAuth(client, backend net.Conn) error {
	var sc *scramClient

	for {
		msgType, payload, err := readServerMessage(backend)
		if err != nil {
			return fmt.Errorf("reading auth message: %w", err)
		}

		switch msgType {
		case 'R':
			authType, data, err := readAuthType(payload)
			if err != nil {
				return err
			}

			switch authType {
			case authTypeOK:
				if err := writeServerMessage(client, 'R', payload); err != nil {
					return err
				}

			case authTypeMD5:
				resp := computeMD5Response(p.config.User, p.config.Password, data)
				if resp == nil {
					return fmt.Errorf("MD5 auth: invalid salt")
				}
				pwdMsg := buildPasswordMessage(resp)
				if _, err := backend.Write(pwdMsg); err != nil {
					return fmt.Errorf("sending MD5 password: %w", err)
				}

			case authTypeSASL:
				sc = newScramClient(p.config.User, p.config.Password)
				firstMsg := sc.FirstMessage()
				pwdMsg := buildSASLMessage(firstMsg)
				if _, err := backend.Write(pwdMsg); err != nil {
					return fmt.Errorf("sending SASL initial: %w", err)
				}

			case authTypeSASLContinue:
				if sc == nil {
					return fmt.Errorf("SCRAM state missing for SASLContinue")
				}
				sf, err := parseScramServerFirst(data)
				if err != nil {
					return fmt.Errorf("parsing SCRAM server-first: %w", err)
				}
				finalMsg, err := sc.FinalMessage(sf)
				if err != nil {
					return fmt.Errorf("SCRAM final message: %w", err)
				}
				pwdMsg := buildSASLMessage(finalMsg)
				if _, err := backend.Write(pwdMsg); err != nil {
					return fmt.Errorf("sending SASL final: %w", err)
				}

			case authTypeSASLFinal:
				if sc == nil {
					return fmt.Errorf("SCRAM state missing for SASLFinal")
				}
				if err := sc.VerifyServerFinal(data); err != nil {
					return fmt.Errorf("SCRAM server verification: %w", err)
				}
				if err := writeServerMessage(client, 'R', payload); err != nil {
					return err
				}

			default:
				return fmt.Errorf("unsupported auth type: %d", authType)
			}

		case 'K':
			if err := writeServerMessage(client, 'K', payload); err != nil {
				return err
			}

		case 'S':
			if err := writeServerMessage(client, 'S', payload); err != nil {
				return err
			}

		case 'Z':
			if err := writeServerMessage(client, 'Z', payload); err != nil {
				return err
			}
			return nil

		case 'E':
			client.Write(buildRawMessage('E', payload))
			return fmt.Errorf("backend auth error: %s", extractError(payload))

		default:
			return fmt.Errorf("unexpected message during auth: type=%c", msgType)
		}
	}
}

func (p *Proxy) relayLoop(client, backend net.Conn) {
	errc := make(chan error, 2)
	go func() { errc <- relay(backend, client) }()
	go func() { errc <- relay(client, backend) }()
	<-errc
}

func relay(dst, src net.Conn) error {
	buf := make([]byte, 32768)
	for {
		n, err := src.Read(buf)
		if err != nil {
			return err
		}
		if _, err := dst.Write(buf[:n]); err != nil {
			return err
		}
	}
}

func buildPasswordMessage(password []byte) []byte {
	payload := make([]byte, len(password)+4)
	binary.BigEndian.PutUint32(payload[0:4], uint32(len(payload)))
	copy(payload[4:], password)
	msg := make([]byte, len(payload)+1)
	msg[0] = 'p'
	copy(msg[1:], payload)
	return msg
}

func buildSASLMessage(data []byte) []byte {
	saslPayload := make([]byte, len(data)+4)
	binary.BigEndian.PutUint32(saslPayload[0:4], uint32(len(saslPayload)))
	copy(saslPayload[4:], data)
	return buildPasswordMessage(saslPayload)
}

func buildRawMessage(msgType byte, payload []byte) []byte {
	length := len(payload) + 4
	buf := make([]byte, length+1)
	buf[0] = msgType
	binary.BigEndian.PutUint32(buf[1:5], uint32(length))
	copy(buf[5:], payload)
	return buf
}

func trySSLRequest(r io.Reader) (bool, error) {
	var buf [8]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return false, err
	}
	length := binary.BigEndian.Uint32(buf[0:4])
	code := binary.BigEndian.Uint32(buf[4:8])
	if length == 8 && code == sslRequestCode {
		if _, err := r.Read(buf[:1]); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func writeErrorResponse(conn net.Conn, msg string) {
	payload := []byte("S\x00ERROR\x00M\x00" + msg + "\x00")
	writeServerMessage(conn, 'E', payload)
}

func extractError(payload []byte) string {
	for i := 0; i < len(payload)-1; i++ {
		if payload[i] == 'M' && payload[i+1] == 0 {
			start := i + 2
			end := start
			for end < len(payload) && payload[end] != 0 {
				end++
			}
			return string(payload[start:end])
		}
	}
	return string(payload)
}
