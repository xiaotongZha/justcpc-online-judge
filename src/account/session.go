package account

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const CookieName = "justcpc_session"

type SessionManager struct{ key []byte }

func NewSessionManager(secret string) (*SessionManager, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("JUSTCPC_SESSION_SECRET must contain at least 32 bytes")
	}
	return &SessionManager{key: []byte(secret)}, nil
}

func (m *SessionManager) Set(writer http.ResponseWriter, account Account) {
	expires := time.Now().Add(12 * time.Hour).Unix()
	payload := fmt.Sprintf("%d|%s|%d", account.ID, account.Role, expires)
	encoded := base64.RawURLEncoding.EncodeToString([]byte(payload))
	signature := m.sign(encoded)
	http.SetCookie(writer, &http.Cookie{Name: CookieName, Value: encoded + "." + signature, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 12 * 60 * 60})
}

func (m *SessionManager) Clear(writer http.ResponseWriter) {
	http.SetCookie(writer, &http.Cookie{Name: CookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
}

func (m *SessionManager) Parse(request *http.Request) (int64, string, bool) {
	cookie, err := request.Cookie(CookieName)
	if err != nil {
		return 0, "", false
	}
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 || !hmac.Equal([]byte(parts[1]), []byte(m.sign(parts[0]))) {
		return 0, "", false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return 0, "", false
	}
	values := strings.Split(string(decoded), "|")
	if len(values) != 3 {
		return 0, "", false
	}
	id, err := strconv.ParseInt(values[0], 10, 64)
	if err != nil {
		return 0, "", false
	}
	expires, err := strconv.ParseInt(values[2], 10, 64)
	if err != nil || time.Now().Unix() > expires {
		return 0, "", false
	}
	if values[1] != "admin" && values[1] != "user" {
		return 0, "", false
	}
	return id, values[1], true
}

func (m *SessionManager) sign(value string) string {
	mac := hmac.New(sha256.New, m.key)
	mac.Write([]byte(value))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
