package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strings"
)

// ---------- 密码哈希（sha256 + 随机盐，纯标准库实现） ----------

func hashPassword(pw string) (string, error) {
	salt := randHex(16)
	sum := sha256.Sum256([]byte(salt + pw))
	return salt + ":" + hex.EncodeToString(sum[:]), nil
}

func checkPassword(pw, stored string) bool {
	parts := strings.SplitN(stored, ":", 2)
	if len(parts) != 2 {
		return false
	}
	sum := sha256.Sum256([]byte(parts[0] + pw))
	return hex.EncodeToString(sum[:]) == parts[1]
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

// ---------- Token（HMAC 签名，无第三方依赖） ----------

func generateToken(userID int64) string {
	payload := base64.RawURLEncoding.EncodeToString([]byte(itoa(userID)))
	sig := hmacSHA256(payload)
	return payload + "." + sig
}

func parseToken(tok string) (int64, bool) {
	parts := strings.SplitN(tok, ".", 2)
	if len(parts) != 2 {
		return 0, false
	}
	if hmacSHA256(parts[0]) != parts[1] {
		return 0, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return 0, false
	}
	id, err := atoi(string(raw))
	if err != nil {
		return 0, false
	}
	return id, true
}

func hmacSHA256(data string) string {
	mac := hmac.New(sha256.New, []byte(Cfg.JWTSecret))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

func itoa(n int64) string {
	return strconvFormatInt(n)
}

func strconvFormatInt(n int64) string {
	// 简单实现，避免引入额外包
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func atoi(s string) (int64, error) {
	var n int64
	neg := false
	i := 0
	if len(s) > 0 && s[0] == '-' {
		neg = true
		i = 1
	}
	if i >= len(s) {
		return 0, errBadToken
	}
	for ; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, errBadToken
		}
		n = n*10 + int64(c-'0')
	}
	if neg {
		n = -n
	}
	return n, nil
}

// ---------- 鉴权中间件 ----------

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, ok := extractUser(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "未登录或 token 失效")
			return
		}
		ctx := withUserID(r, uid)
		next(w, r.WithContext(ctx))
	}
}

func extractUser(r *http.Request) (int64, bool) {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return parseToken(strings.TrimPrefix(auth, "Bearer "))
	}
	// 也支持 ?token=
	if t := r.URL.Query().Get("token"); t != "" {
		return parseToken(t)
	}
	return 0, false
}

func currentUserID(r *http.Request) int64 {
	v, _ := r.Context().Value(ctxUserKey).(int64)
	return v
}

// adminMiddleware 若配置了 ADMIN_KEY 则校验请求头 x-admin-key
func adminMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if Cfg.AdminKey != "" {
			if r.Header.Get("x-admin-key") != Cfg.AdminKey {
				writeError(w, http.StatusForbidden, "需要管理员密钥")
				return
			}
		}
		next(w, r)
	}
}
