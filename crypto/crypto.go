package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"syscall"
	"unsafe"
)

var (
	crypt32                = syscall.NewLazyDLL("crypt32.dll")
	procCryptProtectData   = crypt32.NewProc("CryptProtectData")
	procCryptUnprotectData = crypt32.NewProc("CryptUnprotectData")
)

type DATA_BLOB struct {
	CbData uint32
	PbData *byte
}

// CryptoKey is the 32-byte key used for AES-256 encryption.
var CryptoKey = []byte("StyledMDSecretKeyForAPIEncryption")[:32]

func CryptProtect(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var in DATA_BLOB
	var out DATA_BLOB
	in.CbData = uint32(len(data))
	in.PbData = &data[0]
	r, _, err := procCryptProtectData.Call(
		uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		return nil, fmt.Errorf("DPAPI Encrypt failed: %v", err)
	}
	defer syscall.LocalFree(syscall.Handle(unsafe.Pointer(out.PbData)))
	result := make([]byte, out.CbData)
	copy(result, unsafe.Slice(out.PbData, out.CbData))
	return result, nil
}

func CryptUnprotect(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var in DATA_BLOB
	var out DATA_BLOB
	in.CbData = uint32(len(data))
	in.PbData = &data[0]
	r, _, err := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		return nil, fmt.Errorf("DPAPI Decrypt failed: %v", err)
	}
	defer syscall.LocalFree(syscall.Handle(unsafe.Pointer(out.PbData)))
	result := make([]byte, out.CbData)
	copy(result, unsafe.Slice(out.PbData, out.CbData))
	return result, nil
}

// Encrypt encrypts plain text using AES-GCM + Windows DPAPI.
func Encrypt(text string) (string, error) {
	if text == "" {
		return "", nil
	}
	block, err := aes.NewCipher(CryptoKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(text), nil)
	dpapiCipher, err := CryptProtect(ciphertext)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(dpapiCipher), nil
}

// Decrypt decrypts a string previously encrypted with Encrypt.
func Decrypt(cryptoText string) (string, error) {
	if cryptoText == "" {
		return "", nil
	}
	dpapiCipher, err := base64.StdEncoding.DecodeString(cryptoText)
	if err != nil {
		return "", err
	}
	ciphertext, err := CryptUnprotect(dpapiCipher)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(CryptoKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, actual := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, actual, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
