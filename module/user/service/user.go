package service

import (
	usermodel "PProject/module/user/model"
	jwtlib "PProject/tools/security"
	"time"
)

// LoginParams 登录入参：用于生成会话记录
type LoginParams struct {
	SessionID  string        // 会话ID（建议UUID/雪花；若空则业务层自己生成）
	UserID     string        // 必填
	TenantID   string        // 多租户（可选）
	DeviceType string        // web/ios/android/pc...
	DeviceID   string        // 设备唯一标识
	IP         string        // 登录IP
	UserAgent  string        // UA
	Scopes     []string      // 令牌 scope
	TTL        time.Duration // 覆盖 opts.TTL；<=0 则使用 opts.TTL
	Now        time.Time     // 业务注入“当前时间”，零值时用 time.Now()
}

func Login(opts jwtlib.Options, in LoginParams) (usermodel.UserSession, error) {
	now := in.Now
	if now.IsZero() {
		now = time.Now()
	}
	ttl := in.TTL
	if ttl <= 0 {
		ttl = opts.TTL
	}
	// 生成 AccessToken & Hash
	token, hash, exp, err := jwtlib.Generate(opts, in.UserID, in.Scopes)
	if err != nil {
		return usermodel.UserSession{}, err
	}

	rec := usermodel.UserSession{
		SessionID:       in.SessionID,
		UserID:          in.UserID,
		TenantID:        in.TenantID,
		DeviceType:      in.DeviceType,
		DeviceID:        in.DeviceID,
		AccessToken:     token, // 生产环境建议去掉，不落库，仅存 hash
		AccessTokenHash: hash,

		IsValid:    true,
		Status:     "online",
		LoginTime:  now,
		LastActive: now,
		ExpireTime: exp,
		ExpireAt:   exp,

		CreateTime: now,
		UpdateTime: now,
	}
	return rec, nil
}

func Verify(opts jwtlib.Options, token string, expectedHash string) (*jwtlib.JWTClaims, error) {
	return jwtlib.Verify(opts, token, expectedHash)
}
