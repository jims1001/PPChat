package security

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	jwtlib "github.com/golang-jwt/jwt/v5"
	"strings"
	"time"
)

// Options 控制签名与TTL等参数。
type Options struct {
	Secret []byte        // HMAC 密钥（生产用ENV/KMS）
	Alg    string        // HS256/HS384/HS512（默认 HS256）
	TTL    time.Duration // 令牌有效期（默认 2h）
}

type JWTClaims struct {
	jwtlib.MapClaims
}

func DefaultOptions(secret []byte) Options {
	return Options{Secret: secret, Alg: "HS256", TTL: 2 * time.Hour}
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func Generate(opts Options, userID string, scopes []string) (token string, accessTokenHash string, expireAt time.Time, err error) {
	method, err := signingMethod(opts.Alg)
	if err != nil {
		return "", "", time.Time{}, err
	}
	if opts.TTL <= 0 {
		opts.TTL = 2 * time.Hour
	}
	now := time.Now()
	exp := now.Add(opts.TTL)

	claims := jwtlib.MapClaims{
		"sub": userID,
		"iat": now.Unix(),
		"nbf": now.Unix(),
		"exp": exp.Unix(),
	}
	if len(scopes) > 0 {
		claims["scope"] = scopes
	}

	tok := jwtlib.NewWithClaims(method, claims)
	signed, err := tok.SignedString(opts.Secret)
	if err != nil {
		return "", "", time.Time{}, err
	}
	return signed, HashToken(signed), exp, nil
}

func Verify(opts Options, token string, expectedHash string) (*JWTClaims, error) {
	_, err := signingMethod(opts.Alg) // 校验 alg 合法
	if err != nil {
		return nil, err
	}
	parsed, err := jwtlib.Parse(token, func(t *jwtlib.Token) (interface{}, error) {
		// 仅允许 HMAC 家族
		if _, ok := t.Method.(*jwtlib.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected alg: %v", t.Header["alg"])
		}
		return opts.Secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	if expectedHash != "" && HashToken(token) != expectedHash {
		return nil, errors.New("access token hash mismatch")
	}
	claims, ok := parsed.Claims.(jwtlib.MapClaims)
	if !ok {
		return nil, errors.New("claims type mismatch")
	}
	return &JWTClaims{claims}, nil
}

func signingMethod(alg string) (jwtlib.SigningMethod, error) {
	switch strings.ToUpper(strings.TrimSpace(alg)) {
	case "", "HS256":
		return jwtlib.SigningMethodHS256, nil
	case "HS384":
		return jwtlib.SigningMethodHS384, nil
	case "HS512":
		return jwtlib.SigningMethodHS512, nil
	default:
		return nil, fmt.Errorf("unsupported alg: %s (use HS256/HS384/HS512)", alg)
	}
}
