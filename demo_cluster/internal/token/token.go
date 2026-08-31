package token

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	cherryCrypto "github.com/cherry-game/cherry/extend/crypto"
	cherryLogger "github.com/cherry-game/cherry/logger"
	"github.com/cherry-game/examples/demo_cluster/internal/code"
)

const (
	// 短 TTL：登录票建议 5 分钟（毫秒）
	tokenTTLMs = int64(5 * 60 * 1000)
	hashFormat = "pid:%d|openid:%s|tt:%d|jti:%s"
)

type Token struct {
	PID       int32  `json:"pid"`
	OpenID    string `json:"open_id"`
	Timestamp int64  `json:"tt"`  // 签发时间 ms
	JTI       string `json:"jti"` // 一次性票 ID
	Hash      string `json:"hash"`
}

func New(pid int32, openId string, appKey string) *Token {
	t := &Token{
		PID:       pid,
		OpenID:    openId,
		Timestamp: time.Now().UnixMilli(),
		JTI:       newJTI(),
	}
	t.Hash = BuildHash(t, appKey)
	return t
}

func newJTI() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func (t *Token) ToBase64() string {
	bytes, _ := json.Marshal(t)
	return cherryCrypto.Base64Encode(string(bytes))
}

func DecodeToken(base64Token string) (*Token, bool) {
	if len(base64Token) < 1 {
		return nil, false
	}
	raw, err := cherryCrypto.Base64DecodeBytes(base64Token)
	if err != nil {
		cherryLogger.Warnf("base64Token decode error = %v", err)
		return nil, false
	}
	tok := &Token{}
	if err := json.Unmarshal(raw, tok); err != nil {
		cherryLogger.Warnf("token unmarshal error = %v", err)
		return nil, false
	}
	return tok, true
}

// Validate 验签 + TTL（不消费 jti；消费在 Center/本地 store）
func Validate(token *Token, appKey string) (int32, bool) {
	if token == nil || token.OpenID == "" || token.JTI == "" || token.Hash == "" {
		return code.AccountTokenValidateFail, false
	}

	now := time.Now().UnixMilli()
	if token.Timestamp > now+60*1000 { // 允许 1 分钟时钟偏差
		return code.AccountTokenValidateFail, false
	}
	if now-token.Timestamp > tokenTTLMs {
		cherryLogger.Warnf("token expired pid=%d openId=%s ageMs=%d",
			token.PID, token.OpenID, now-token.Timestamp)
		return code.AccountTokenExpired, false
	}

	if BuildHash(token, appKey) != token.Hash {
		cherryLogger.Warnf("hmac validate fail pid=%d jti=%s", token.PID, token.JTI)
		return code.AccountTokenValidateFail, false
	}
	return code.OK, true
}

func BuildHash(t *Token, appKey string) string {
	payload := fmt.Sprintf(hashFormat, t.PID, t.OpenID, t.Timestamp, t.JTI)
	mac := hmac.New(sha256.New, []byte(appKey))
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
