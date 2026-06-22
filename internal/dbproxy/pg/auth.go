package pg

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
)

func computeMD5Response(user, password string, salt []byte) []byte {
	if len(salt) != 4 {
		return nil
	}
	inner := md5.Sum([]byte(password + user))
	innerHex := fmt.Sprintf("%x", inner)
	outer := md5.Sum(append([]byte(innerHex), salt...))
	return []byte("md5" + fmt.Sprintf("%x", outer))
}

type scramClient struct {
	user            string
	password        string
	clientNonce     string
	clientFirstBare string
	serverFirst     string
	saltedPassword  []byte
	authMessage     string
}

func newScramClient(user, password string) *scramClient {
	nonceBytes := make([]byte, 18)
	rand.Read(nonceBytes)
	return &scramClient{
		user:        user,
		password:    password,
		clientNonce: base64.RawStdEncoding.EncodeToString(nonceBytes),
	}
}

func (s *scramClient) FirstMessage() []byte {
	s.clientFirstBare = "n=" + s.user + ",r=" + s.clientNonce
	return []byte("n,," + s.clientFirstBare)
}

type scramServerFirst struct {
	nonce     string
	salt      string
	iteration int
}

func parseScramServerFirst(data []byte) (*scramServerFirst, error) {
	parts := strings.Split(string(data), ",")
	result := &scramServerFirst{}
	for _, p := range parts {
		if len(p) < 2 || p[1] != '=' {
			continue
		}
		switch p[0] {
		case 'r':
			result.nonce = p[2:]
		case 's':
			result.salt = p[2:]
		case 'i':
			if _, err := fmt.Sscanf(p[2:], "%d", &result.iteration); err != nil {
				return nil, fmt.Errorf("invalid iteration count: %w", err)
			}
		}
	}
	if result.nonce == "" || result.salt == "" || result.iteration <= 0 {
		return nil, fmt.Errorf("incomplete SCRAM server-first message")
	}
	return result, nil
}

func hi(password, salt []byte, iterations int) []byte {
	mac := hmac.New(sha256.New, password)
	mac.Write(salt)
	mac.Write([]byte{0, 0, 0, 1})
	u1 := mac.Sum(nil)
	result := make([]byte, len(u1))
	copy(result, u1)
	prev := u1

	for i := 2; i <= iterations; i++ {
		mac.Reset()
		mac.Write(prev)
		u := mac.Sum(nil)
		for j := 0; j < len(u); j++ {
			result[j] ^= u[j]
		}
		prev = u
	}
	return result
}

func (s *scramClient) FinalMessage(sf *scramServerFirst) ([]byte, error) {
	if !strings.HasPrefix(sf.nonce, s.clientNonce) {
		return nil, fmt.Errorf("server nonce does not start with client nonce")
	}

	salt, err := base64.StdEncoding.DecodeString(sf.salt)
	if err != nil {
		return nil, fmt.Errorf("decoding salt: %w", err)
	}

	s.saltedPassword = hi([]byte(s.password), salt, sf.iteration)
	s.serverFirst = fmt.Sprintf("r=%s,s=%s,i=%d", sf.nonce, sf.salt, sf.iteration)
	s.authMessage = fmt.Sprintf("%s,%s,c=biws,r=%s", s.clientFirstBare, s.serverFirst, sf.nonce)

	clientProof := s.computeClientProof()
	return []byte(fmt.Sprintf("c=biws,r=%s,p=%s", sf.nonce, base64.StdEncoding.EncodeToString(clientProof))), nil
}

func (s *scramClient) computeClientProof() []byte {
	mac := hmac.New(sha256.New, s.saltedPassword)
	mac.Write([]byte("Client Key"))
	clientKey := mac.Sum(nil)

	mac2 := hmac.New(sha256.New, clientKey)
	mac2.Write([]byte(s.authMessage))
	storedKey := mac2.Sum(nil)

	mac3 := hmac.New(sha256.New, storedKey)
	mac3.Write([]byte(s.authMessage))
	clientProof := mac3.Sum(nil)

	for i := 0; i < len(clientKey); i++ {
		clientProof[i] = clientKey[i] ^ clientProof[i]
	}
	return clientProof
}

func (s *scramClient) VerifyServerFinal(data []byte) error {
	parts := strings.Split(string(data), ",")
	var verifier string
	for _, p := range parts {
		if strings.HasPrefix(p, "v=") {
			verifier = p[2:]
			break
		}
	}
	if verifier == "" {
		return fmt.Errorf("no verifier in server final message")
	}

	mac := hmac.New(sha256.New, s.saltedPassword)
	mac.Write([]byte("Server Key"))
	serverKey := mac.Sum(nil)

	mac2 := hmac.New(sha256.New, serverKey)
	mac2.Write([]byte(s.authMessage))
	expected := base64.StdEncoding.EncodeToString(mac2.Sum(nil))

	if verifier != expected {
		return fmt.Errorf("SCRAM server verification failed")
	}
	return nil
}
