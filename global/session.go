package global

import (
	"PProject/global/config"
	errors "PProject/tools/errs"
	jwtlib "PProject/tools/security"
	"strings"

	"github.com/gin-gonic/gin"
)

// UserSession 全局的接口请求 需要处理的session
type UserSession struct {
	SessionId string `json:"session_id"`
	TenantID  string `json:"tenant_id"` // 多租户场景（客服系统强烈建议加）
}

// AuthInfo 用户授权信息
type AuthInfo struct {
	Token  string `json:"token"` // Authorization
	Hash   string `json:"hash"`  // AuthorizationHash
	UserId string `json:"user_id"`
}

// GetAuthInfo 从 gin.Context 中统一获取用户授权信息
func GetAuthInfo(c *gin.Context) (*AuthInfo, error) {
	authHeader := c.GetHeader("authorization")
	authHash := c.GetHeader("authorizationHash")

	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))

	if token == "" || authHash == "" {
		return nil, errors.New("token or hash is empty")
	}

	opts := jwtlib.DefaultOptions(config.GetJwtSecret())
	claims, err := jwtlib.Verify(opts, token, authHash)
	if (err != nil) || (claims == nil) {
		return nil, err
	}

	subject, err := claims.GetSubject()

	if err != nil {
		return nil, err
	}

	return &AuthInfo{
		Token:  token,
		Hash:   authHash,
		UserId: subject,
	}, nil
}
