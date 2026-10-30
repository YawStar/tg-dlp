package session

import (
	"bufio"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/gotd/td/session"
	"golang.org/x/crypto/argon2"
	"golang.org/x/term"

	"tg-dlp/pkg/runtime"
)

// EncryptedSessionStorage implements session.Storage with AES-GCM encryption.
type EncryptedSessionStorage struct {
	Path     string
	Password string
}

func (s *EncryptedSessionStorage) LoadSession(ctx context.Context) ([]byte, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, session.ErrNotFound
		}
		return nil, err
	}
	return DecryptData(data, s.Password)
}

func (s *EncryptedSessionStorage) StoreSession(ctx context.Context, data []byte) error {
	encData, err := EncryptData(data, s.Password)
	if err != nil {
		return err
	}
	return os.WriteFile(s.Path, encData, 0600)
}

func deriveKey(password string, salt []byte) []byte {
	return argon2.IDKey([]byte(password), salt, 3, 64*1024, 4, 32)
}

func EncryptData(plaintext []byte, password string) ([]byte, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	key := deriveKey(password, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return append(salt, ciphertext...), nil
}

func DecryptData(data []byte, password string) ([]byte, error) {
	if len(data) < 16 {
		return nil, fmt.Errorf("invalid encrypted data")
	}
	salt := data[:16]
	ciphertext := data[16:]

	key := deriveKey(password, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce := ciphertext[:nonceSize]
	ciphertext = ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("incorrect password or corrupted file")
	}
	return plaintext, nil
}

// PromptPassword reads a password without echo.
func PromptPassword(prompt string) string {
	fmt.Print(prompt)
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		reader := bufio.NewReader(os.Stdin)
		text, _ := reader.ReadString('\n')
		return strings.TrimSpace(text)
	}
	fmt.Println()
	return string(bytePassword)
}

// ---------- Password cache ----------

type cachedPassword struct {
	Password  string    `json:"password"`
	ExpiresAt time.Time `json:"expires_at"`
}

func cacheFilePath(sessionPath string) string {
	return sessionPath + ".cache"
}

func SavePasswordToCache(sessionPath, password string, duration time.Duration) {
	cachePath := cacheFilePath(sessionPath)
	cache := cachedPassword{
		Password:  password,
		ExpiresAt: time.Now().Add(duration),
	}
	data, err := json.Marshal(cache)
	if err != nil {
		runtime.DebugLog("⚠ Failed to marshal cache: %v", err)
		return
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	if err := os.WriteFile(cachePath, []byte(encoded), 0600); err != nil {
		runtime.DebugLog("⚠ Failed to write cache file: %v", err)
	} else {
		runtime.DebugLog("✅ Cache saved to %s (expires: %v)", cachePath, cache.ExpiresAt.Format("15:04:05"))
	}
}

func LoadPasswordFromCache(sessionPath string) (string, bool) {
	cachePath := cacheFilePath(sessionPath)
	encodedData, err := os.ReadFile(cachePath)
	if err != nil {
		runtime.DebugLog("⚠ Cache not found: %v", err)
		return "", false
	}
	data, err := base64.StdEncoding.DecodeString(string(encodedData))
	if err != nil {
		runtime.DebugLog("⚠ Failed to decode cache: %v", err)
		return "", false
	}
	var cache cachedPassword
	if err := json.Unmarshal(data, &cache); err != nil {
		runtime.DebugLog("⚠ Failed to unmarshal cache: %v", err)
		return "", false
	}

	runtime.DebugLog("📅 Cache Expires At: %v", cache.ExpiresAt.Format("2006-01-02 15:04:05"))
	runtime.DebugLog("⏰ Current Time: %v", time.Now().Format("2006-01-02 15:04:05"))

	if time.Now().After(cache.ExpiresAt) {
		runtime.DebugLog("⚠ Cache expired!")
		os.Remove(cachePath)
		return "", false
	}
	runtime.DebugLog("✅ Cache valid")
	return cache.Password, true
}

func ClearPasswordCache(sessionPath string) {
	os.Remove(cacheFilePath(sessionPath))
}