// Package doccrypt encrypts the document with a password, in a format that
// tools/decrypt/decrypt.html can also read with the browser's WebCrypto:
//
//	"DMHE"  version (1 byte)  iterations (uint32 BE)  salt (16)  nonce (12)
//	AES-256-GCM ciphertext and tag, with the 37-byte header as additional data
//
// The key is PBKDF2-HMAC-SHA256 of the password. The plaintext is the
// original file name (uint16 BE length, then UTF-8) followed by the content,
// so decrypting restores the name without the stored object revealing it.
package doccrypt

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"unicode/utf8"
)

// Extension is added to the original name to name the stored object
const Extension = ".dmh"

const (
	magic      = "DMHE"
	version    = 1
	saltSize   = 16
	nonceSize  = 12
	headerSize = len(magic) + 1 + 4 + saltSize + nonceSize

	// Iterations is the PBKDF2 work factor for new files (OWASP's 2023
	// figure for HMAC-SHA256)
	Iterations = 600_000
	// A file asking for more is refused, so it cannot make decrypting hang
	maxIterations = 10_000_000
)

var (
	// ErrNotEncrypted is returned for data not in this format
	ErrNotEncrypted = errors.New("not an encrypted document")
	// ErrWrongPassword is returned when the data does not decrypt, because
	// the password is wrong or the file has been changed or damaged
	ErrWrongPassword = errors.New("wrong password, or the file is damaged")
)

// IsEncrypted reports whether data starts like a file in this format
func IsEncrypted(data []byte) bool {
	return bytes.HasPrefix(data, []byte(magic))
}

// Encrypt returns content, with its file name, encrypted with password
func Encrypt(content []byte, name, password string) ([]byte, error) {
	salt := make([]byte, saltSize)
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return encryptWith(content, name, password, salt, nonce, Iterations)
}

func encryptWith(content []byte, name, password string, salt, nonce []byte, iterations uint32) ([]byte, error) {
	if password == "" {
		return nil, errors.New("the password is empty")
	}
	if name == "" || len(name) > math.MaxUint16 || !utf8.ValidString(name) {
		return nil, fmt.Errorf("%q is not a usable file name", name)
	}
	if len(salt) != saltSize || len(nonce) != nonceSize || iterations == 0 || iterations > maxIterations {
		return nil, errors.New("invalid encryption parameters")
	}

	header := make([]byte, 0, headerSize)
	header = append(header, magic...)
	header = append(header, version)
	header = binary.BigEndian.AppendUint32(header, iterations)
	header = append(header, salt...)
	header = append(header, nonce...)

	plaintext := make([]byte, 0, 2+len(name)+len(content))
	plaintext = binary.BigEndian.AppendUint16(plaintext, uint16(len(name)))
	plaintext = append(plaintext, name...)
	plaintext = append(plaintext, content...)

	gcm, err := newGCM(password, salt, int(iterations))
	if err != nil {
		return nil, err
	}
	return gcm.Seal(header, nonce, plaintext, header), nil
}

// Decrypt returns the original file name and content of data
func Decrypt(data []byte, password string) (name string, content []byte, err error) {
	if !IsEncrypted(data) {
		return "", nil, ErrNotEncrypted
	}
	if len(data) < headerSize {
		return "", nil, ErrWrongPassword
	}
	header := data[:headerSize]
	if v := header[len(magic)]; v != version {
		return "", nil, fmt.Errorf("the file is version %d, which this program can't read; use a newer one", v)
	}
	iterations := binary.BigEndian.Uint32(header[len(magic)+1:])
	if iterations == 0 || iterations > maxIterations {
		return "", nil, ErrWrongPassword
	}
	salt := header[len(magic)+5 : len(magic)+5+saltSize]
	nonce := header[headerSize-nonceSize:]

	gcm, err := newGCM(password, salt, int(iterations))
	if err != nil {
		return "", nil, err
	}
	plaintext, err := gcm.Open(nil, nonce, data[headerSize:], header)
	if err != nil {
		return "", nil, ErrWrongPassword
	}
	if len(plaintext) < 2 {
		return "", nil, ErrWrongPassword
	}
	nameLen := int(binary.BigEndian.Uint16(plaintext))
	if len(plaintext) < 2+nameLen {
		return "", nil, ErrWrongPassword
	}
	return string(plaintext[2 : 2+nameLen]), plaintext[2+nameLen:], nil
}

func newGCM(password string, salt []byte, iterations int) (cipher.AEAD, error) {
	key, err := pbkdf2.Key(sha256.New, password, salt, iterations, 32)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
