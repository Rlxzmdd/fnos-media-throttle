package downloader

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"strings"
)

func randomAlphaNumeric(length int) (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	raw := make([]byte, length)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	for i := range raw {
		raw[i] = alphabet[int(raw[i])%len(alphabet)]
	}
	return string(raw), nil
}

func fnosDeviceID(d Downloader) string {
	// The official web client persists a three-part, lower-case device ID in
	// localStorage. A service has no browser storage, so derive a stable ID from
	// the gateway and account instead of registering a new device every poll.
	seed := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(d.BaseURL)) + "\x00" + strings.ToLower(strings.TrimSpace(d.Username)) + "\x00fnos-media-throttle"))
	encoded := hex.EncodeToString(seed[:])
	return "media-" + encoded[:13] + "-" + encoded[13:26]
}

func fnosLoginPayload(username, password, si, deviceID, requestID string) map[string]any {
	return map[string]any{
		"reqid": requestID,
		"user":  username, "password": password,
		"stay": true, "deviceType": "Browser", "deviceName": "Linux-Google Chrome", "did": deviceID,
		"req": "user.login", "si": si,
	}
}

func publicKeyBits(value string) int {
	key, err := parseRSAPublicKey(value)
	if err != nil {
		return 0
	}
	return key.N.BitLen()
}

func parseRSAPublicKey(value string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(value))
	if block == nil {
		return nil, errors.New("无法解析飞牛 RSA 公钥")
	}
	if parsed, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if key, ok := parsed.(*rsa.PublicKey); ok {
			return key, nil
		}
	}
	if key, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return key, nil
	}
	return nil, errors.New("飞牛 RSA 公钥格式不受支持")
}

func aesCBCEncrypt(key, iv, plain []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	padding := aes.BlockSize - len(plain)%aes.BlockSize
	plain = append(plain, bytes.Repeat([]byte{byte(padding)}, padding)...)
	out := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, plain)
	return out, nil
}

func aesCBCDecrypt(key, iv, encrypted []byte) ([]byte, error) {
	if len(encrypted) == 0 || len(encrypted)%aes.BlockSize != 0 {
		return nil, errors.New("AES 密文长度无效")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(encrypted))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(out, encrypted)
	padding := int(out[len(out)-1])
	if padding <= 0 || padding > aes.BlockSize || padding > len(out) {
		return nil, errors.New("AES 填充无效")
	}
	for _, value := range out[len(out)-padding:] {
		if int(value) != padding {
			return nil, errors.New("AES 填充无效")
		}
	}
	return out[:len(out)-padding], nil
}

// Referenced in protocol diagnostics to make key material formatting explicit.
func shortKeyFingerprint(key []byte) string {
	sum := sha256.Sum256(key)
	return hex.EncodeToString(sum[:4])
}
