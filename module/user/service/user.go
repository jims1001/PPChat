package service

import (
	config "PProject/global"
	global "PProject/global"
	usermodel "PProject/module/user/model"
	"PProject/service/mgo"
	"PProject/service/storage/redis"
	"PProject/tools/errs"
	jwtlib "PProject/tools/security"
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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

type UserSessionKey struct {
	UserId     string `json:"user_id"`
	DeviceType string `json:"device_type"`
	DeviceID   string `json:"device_id"`
}

func Login(ctx context.Context, in LoginParams) (*usermodel.UserSession, error) {

	user, err := GetUserById(ctx, in.UserID)
	if err != nil {
		return nil, err
	}

	opts := jwtlib.DefaultOptions(config.GetJwtSecret())
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
		return nil, err
	}

	key := UserSessionKey{UserId: user.UserID,
		DeviceType: in.DeviceType,
		DeviceID:   in.DeviceID,
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

	err = ReLoginArchiveAndReplace(ctx, key, rec)
	if err != nil {
		return nil, err
	}

	return &rec, nil
}

func Verify(ctx context.Context,
	tokenStr string, tokenHash string) (*usermodel.UserSession, error) {
	// A) JWT 签名/基本 claims 校验（确保不是伪造；不决定是否可用）
	opts := jwtlib.DefaultOptions(config.GetJwtSecret())
	_, err := jwtlib.Verify(opts, tokenStr, tokenHash)
	if err != nil {
		return nil, err // 签名/格式错误，直接拒绝
	}

	rk := fmt.Sprintf(global.UserSessionKey, tokenHash) // e.g. "ts:%s"

	rdb := redis.GetRedis()

	// C) 先查 Redis
	v, err := rdb.Get(ctx, rk).Result()
	if err == nil {
		if v == "-" {
			// 负缓存命中
			return nil, &errs.ErrTokenExpired
		}
		// 命中 sid → 为避免绕过撤销，做一次最小回源校验（强一致需求更高可保留；需要极致性能可去掉）
		var s usermodel.UserSession
		// 读超时
		cctx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
		defer cancel()
		coll := s.Collection()

		// 索引: { session_id:1, is_valid:1, expire_time:1 }
		err = coll.FindOne(cctx, bson.M{
			"session_id": v, "is_valid": true, "expire_time": bson.M{"$gt": time.Now()},
		}).Decode(&s)
		if err == nil {
			return &s, nil
		}
		// Redis 命中但 Mongo 未命中：可能被撤销/过期 → 写负缓存，返回过期
		_ = rdb.Set(ctx, rk, "-", 30*time.Second).Err()
		return nil, &errs.ErrTokenExpired
	}

	// D) 回源 Mongo（权威）
	var s usermodel.UserSession
	// 索引: { access_token_hash:1, is_valid:1 }
	cctx, cancel := context.WithTimeout(ctx, 600*time.Millisecond)
	defer cancel()

	db := mgo.GetDB()
	coll := db.Collection("user_session")

	err = coll.FindOne(cctx, bson.M{
		"access_token_hash": tokenHash, // 统一：Mongo 也存裸 hex
		"is_valid":          true,
		"expire_time":       bson.M{"$gt": time.Now()},
	}).Decode(&s)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			// 放负缓存，防止打穿
			_ = rdb.Set(ctx, rk, "-", 30*time.Second).Err()
			return nil, &errs.ErrTokenExpired
		}
		return nil, err
	}

	// E) 回写 Redis（TTL = 剩余寿命）
	ttl := time.Until(s.ExpireTime)
	if ttl <= 0 {
		// 防御：Mongo 刚好过期边界
		_ = rdb.Set(ctx, rk, "-", 30*time.Second).Err()
		return nil, &errs.ErrTokenExpired
	}
	_ = rdb.Set(ctx, rk, s.SessionID, ttl).Err()
	return &s, nil
}

func ReLoginArchiveAndReplace(ctx context.Context,

	key UserSessionKey,
	newRec usermodel.UserSession) error {

	var old *usermodel.UserSession

	session := usermodel.UserSession{}
	coll := session.Collection()

	logSession := usermodel.UserSessionLog{}
	logColl := logSession.Collection()

	// 查旧
	var o usermodel.UserSession
	findErr := coll.FindOne(ctx, bson.M{
		"user_id":     key.UserId,
		"device_type": key.DeviceType,
		"device_id":   key.DeviceID,
		"is_valid":    true,
	}).Decode(&o)

	if findErr != nil && !errors.Is(findErr, mongo.ErrNoDocuments) {
		return findErr
	}

	if findErr == nil {
		old = &o

		// 归档
		newSession := usermodel.UserSession{}
		if b, e := bson.Marshal(o); e == nil {
			_ = bson.Unmarshal(b, &newSession)
		}

		newSession.Reason = "relogin"
		_, err := logColl.InsertOne(ctx, newSession)
		if err != nil {
			return err
		}

		o.IsValid = false
		o.UpdateTime = time.Now()

		// 建议：标记旧会话无效
		_, err = coll.UpdateByID(ctx,
			o.SessionID, o)
	}

	// Upsert 新会话
	newRec.UpdateTime = time.Now()
	res, repErr := coll.ReplaceOne(ctx,
		bson.M{"user_id": key.UserId,
			"device_type": key.DeviceType,
			"device_id":   key.DeviceID},
		newRec,
		options.Replace().SetUpsert(true),
	)

	log.Printf("matched=%d modified=%d upserted=%v err=%v", res.MatchedCount, res.ModifiedCount, res.UpsertedID, repErr)

	// 同一 ctx/sc 下读回去确认
	var x usermodel.UserSession
	_ = coll.FindOne(ctx, bson.M{
		"user_id": key.UserId, "device_type": key.DeviceType, "device_id": key.DeviceID,
	}).Decode(&x)
	log.Printf("after replace: sid=%s valid=%v exp=%v update=%v", x.SessionID, x.IsValid, x.ExpireTime, x.UpdateTime)

	if repErr != nil {
		return repErr
	}

	// 2) Redis 同步（和 Verify 一致）
	rdb := redis.GetRedis()
	ttl := time.Until(newRec.ExpireTime)

	pipe := rdb.TxPipeline()

	// 撤销旧的 token 白名单
	if old != nil && old.AccessTokenHash != "" {
		oldKey := fmt.Sprintf(global.UserSessionKey, old.AccessTokenHash)
		pipe.Del(ctx, oldKey)
	}

	// 写入新的 token 白名单
	if newRec.IsValid && ttl > 0 && newRec.AccessTokenHash != "" && newRec.SessionID != "" {
		newKey := fmt.Sprintf(global.UserSessionKey, newRec.AccessTokenHash)
		pipe.Set(ctx, newKey, newRec.SessionID, ttl)
	} else if newRec.AccessTokenHash != "" {
		// 防御：新会话无效/已过期时，删掉可能残留的键
		newKey := fmt.Sprintf(global.UserSessionKey, newRec.AccessTokenHash)
		pipe.Del(ctx, newKey)
	}

	_, _ = pipe.Exec(ctx) // 失败可以做补偿或日志告警

	return nil
}

func GetUserByToken(ctx context.Context, tokenStr, tokenHash string) (*usermodel.User, error) {
	opts := jwtlib.DefaultOptions(config.GetJwtSecret())
	// 1. 验证 token // 假设你定义了
	claims, err := jwtlib.Verify(opts, tokenStr, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("token verify failed: %w", err)
	}

	// 2. 从 claims 里取 user_id
	userID, err := claims.GetSubject()
	if err != nil {
		return nil, fmt.Errorf("invalid token: no user_id in claims")
	}
	user, err := GetUserById(ctx, userID)

	if err != nil {
		return nil, err
	}

	return user, nil
}

func GetUserById(ctx context.Context, userId string) (*usermodel.User, error) {
	var user usermodel.User
	err := user.Collection().FindOne(ctx, bson.M{"user_id": strings.TrimSpace(userId)}).Decode(&user)
	if err != nil {
		return nil, fmt.Errorf("find user failed: %w", err)
	}
	return &user, nil
}
