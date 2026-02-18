package mcp

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"strings"

	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// DevHandler serves user-specific MCP endpoints in dev mode.
// Format: /mcp/{encrypted_uid} where encrypted_uid is AES-GCM encrypted.
type DevHandler struct {
	handler   *Handler
	gcm       cipher.AEAD
	logger    *common.Logger
	devMode   bool
	portalURL string
}

// NewDevHandler creates a dev-mode MCP handler with UID encryption.
func NewDevHandler(handler *Handler, jwtSecret []byte, devMode bool, portalURL string, logger *common.Logger) *DevHandler {
	dh := &DevHandler{
		handler:   handler,
		logger:    logger,
		devMode:   devMode,
		portalURL: portalURL,
	}

	// AES key must be 16, 24, or 32 bytes. Use SHA256 of jwtSecret for consistent length.
	if len(jwtSecret) > 0 {
		key := deriveKey(jwtSecret)
		block, err := aes.NewCipher(key)
		if err != nil {
			logger.Warn().Err(err).Msg("dev-handler: failed to create cipher")
			return dh
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			logger.Warn().Err(err).Msg("dev-handler: failed to create GCM")
			return dh
		}
		dh.gcm = gcm
	}

	return dh
}

// ServeHTTP handles /mcp/{encrypted_uid} requests in dev mode.
func (dh *DevHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !dh.devMode {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// Extract encrypted UID from path: /mcp/{encrypted_uid}
	path := strings.TrimPrefix(r.URL.Path, "/mcp/")
	path = strings.Trim(path, "/")

	if path == "" || strings.Contains(path, "/") {
		http.Error(w, "invalid endpoint", http.StatusBadRequest)
		return
	}

	userID, err := dh.decryptUID(path)
	if err != nil {
		http.Error(w, "invalid endpoint", http.StatusNotFound)
		return
	}

	// Inject user context and delegate to main handler
	ctx := WithUserContext(r.Context(), UserContext{UserID: userID})
	r = r.WithContext(ctx)
	dh.handler.streamable.ServeHTTP(w, r)
}

// GenerateEndpoint generates an encrypted MCP endpoint URL for a user.
// Returns empty string if not in dev mode or encryption fails.
func (dh *DevHandler) GenerateEndpoint(userID string) string {
	if !dh.devMode || dh.gcm == nil {
		return ""
	}

	encrypted, err := dh.encryptUID(userID)
	if err != nil {
		dh.logger.Warn().Err(err).Msg("dev-handler: failed to encrypt UID")
		return ""
	}

	baseURL := dh.portalURL
	if baseURL == "" {
		baseURL = "http://localhost:8881"
	}
	return baseURL + "/mcp/" + encrypted
}

// encryptUID encrypts a user ID using AES-GCM.
func (dh *DevHandler) encryptUID(userID string) (string, error) {
	if dh.gcm == nil {
		return "", errors.New("cipher not initialized")
	}

	plaintext := []byte(userID)
	nonce := make([]byte, dh.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := dh.gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.RawURLEncoding.EncodeToString(ciphertext), nil
}

// decryptUID decrypts an encrypted user ID.
func (dh *DevHandler) decryptUID(encrypted string) (string, error) {
	if dh.gcm == nil {
		return "", errors.New("cipher not initialized")
	}

	ciphertext, err := base64.RawURLEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}

	nonceSize := dh.gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := dh.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// deriveKey derives a 32-byte AES key from a secret.
func deriveKey(secret []byte) []byte {
	key := make([]byte, 32)
	for i, b := range secret {
		if i >= 32 {
			break
		}
		key[i] = b
	}
	// XOR remaining bytes for full 32 bytes
	for i := len(secret); i < 32; i++ {
		key[i] = byte(i) ^ key[i%len(secret)]
	}
	return key
}
