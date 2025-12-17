package service

import (
	chatmodel "PProject/module/chatBox/model"
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// =============== 业务错误统一输出 ===============

type BizError struct {
	Code int
	Msg  string
	Err  error
}

func (e *BizError) Error() string { return e.Msg }

func bad(msg string) *BizError      { return &BizError{Code: http.StatusBadRequest, Msg: msg} }
func notFound(msg string) *BizError { return &BizError{Code: http.StatusNotFound, Msg: msg} }
func internal(msg string, err error) *BizError {
	return &BizError{Code: http.StatusInternalServerError, Msg: msg, Err: err}
}

// 给 api.go 用
func WriteBizErr(c interface{ JSON(int, any) }, err error) {
	if be, ok := err.(*BizError); ok {
		c.JSON(be.Code, map[string]any{"error": be.Msg})
		return
	}
	c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
}

// =============== 工具函数 ===============

func toSeconds(v int64, unit string) int64 {
	switch unit {
	case "minute":
		return v * 60
	case "hour":
		return v * 60 * 60
	case "day":
		return v * 24 * 60 * 60
	default:
		return 0
	}
}
func isLangOk(v string) bool  { return v == "zh-CN" || v == "en-US" }
func isScopeOk(v string) bool { return v == "account" || v == "inbox" || v == "team" }

// 推荐：统一生成 accountId（UUID v7）
func newAccountID() string {
	return "acct_" + strings.ReplaceAll(uuid.Must(uuid.NewV7()).String(), "-", "")
}

// =============== DTO（保持你原样） ===============

type CreateAccountReq struct {
	AccountID    string `json:"account_id"` // 建议前端不传
	AccountName  string `json:"account_name" binding:"required"`
	SiteLanguage string `json:"site_language" binding:"required"` // zh-CN / en-US
}

type PatchAccountSettingReq struct {
	AccountName  *string `json:"account_name,omitempty"`
	SiteLanguage *string `json:"site_language,omitempty"`
}

type PatchAccountAutoResolveReq struct {
	AutoResolve bool `json:"auto_resolve"`
}

type UpsertPolicyReq struct {
	ScopeType string `json:"scope_type" binding:"required"`
	ScopeID   string `json:"scope_id,omitempty"`

	Enabled bool `json:"enabled"`

	InactiveRawValue int64  `json:"inactive_raw_value"`
	InactiveRawUnit  string `json:"inactive_raw_unit"` // minute|hour|day

	ResolveMessage string `json:"resolve_message,omitempty"`

	SkipWaitingConversations bool     `json:"skip_waiting_conversations"`
	PostResolveTagIDs        []string `json:"post_resolve_tag_ids,omitempty"`

	UpdatedBy string `json:"updated_by,omitempty"`
}

type PatchPolicyEnabledReq struct {
	ScopeType string `json:"scope_type" binding:"required"`
	ScopeID   string `json:"scope_id,omitempty"`
	Enabled   bool   `json:"enabled"`
	UpdatedBy string `json:"updated_by,omitempty"`
}

type PatchPolicyPreferenceReq struct {
	ScopeType                string `json:"scope_type" binding:"required"`
	ScopeID                  string `json:"scope_id,omitempty"`
	SkipWaitingConversations bool   `json:"skip_waiting_conversations"`
	UpdatedBy                string `json:"updated_by,omitempty"`
}

type PatchPolicyTagsReq struct {
	ScopeType         string   `json:"scope_type" binding:"required"`
	ScopeID           string   `json:"scope_id,omitempty"`
	PostResolveTagIDs []string `json:"post_resolve_tag_ids"`
	UpdatedBy         string   `json:"updated_by,omitempty"`
}

// =============== 业务函数（被 api handler 调用） ===============
// 创建租户
func CreateAccountSettingBiz(ctx context.Context, tenantID string, req CreateAccountReq) (*chatmodel.AccountSetting, error) {
	req.AccountName = strings.TrimSpace(req.AccountName)
	if req.AccountName == "" {
		return nil, bad("account_name required")
	}
	if !isLangOk(req.SiteLanguage) {
		return nil, bad("invalid site_language")
	}

	// 业界建议：后端生成
	if strings.TrimSpace(req.AccountID) == "" {
		req.AccountID = newAccountID()
	}

	now := time.Now()

	filter := bson.M{"tenant_id": tenantID, "account_id": req.AccountID}
	var m chatmodel.AccountSetting

	// 1) 先查是否存在
	var exists chatmodel.AccountSetting
	err := exists.Collection().FindOne(ctx, filter).Decode(&exists)
	if err == nil {
		return nil, &BizError{Code: http.StatusConflict, Msg: "account already exists"}
	}
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, internal("db error", err)
	}

	update := bson.M{
		"$setOnInsert": bson.M{
			"tenant_id":     tenantID,
			"account_id":    req.AccountID,
			"account_name":  req.AccountName,
			"site_language": req.SiteLanguage,
			"auto_resolve":  false,
			"status":        1,
			"created_at":    now,
			"updated_at":    now,
		},
	}

	opt := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)

	var out chatmodel.AccountSetting
	if err := m.Collection().FindOneAndUpdate(ctx, filter, update, opt).Decode(&out); err != nil {
		return nil, internal("db error", err)
	}
	return &out, nil
}

// GetAccountSettingBiz 获取账户信息
func GetAccountSettingBiz(ctx context.Context, tenantID, accountID string) (*chatmodel.AccountSetting, error) {
	var s chatmodel.AccountSetting
	filter := bson.M{"tenant_id": tenantID, "account_id": accountID, "status": 1}
	if err := s.Collection().FindOne(ctx, filter).Decode(&s); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, notFound("account setting not found")
		}
		return nil, internal("db error", err)
	}
	return &s, nil
}

func PatchAccountSettingBiz(ctx context.Context, tenantID, accountID string, req PatchAccountSettingReq) error {
	set := bson.M{"updated_at": time.Now()}

	if req.AccountName != nil {
		name := strings.TrimSpace(*req.AccountName)
		if name == "" {
			return bad("account_name empty")
		}
		set["account_name"] = name
	}
	if req.SiteLanguage != nil {
		if !isLangOk(*req.SiteLanguage) {
			return bad("invalid site_language")
		}
		set["site_language"] = *req.SiteLanguage
	}

	filter := bson.M{"tenant_id": tenantID, "account_id": accountID, "status": 1}
	update := bson.M{"$set": set}

	var m chatmodel.AccountSetting
	res, err := m.Collection().UpdateOne(ctx, filter, update)
	if err != nil {
		return internal("db error", err)
	}
	if res.MatchedCount == 0 {
		return notFound("account setting not found")
	}
	return nil
}

func PatchAccountAutoResolveBiz(ctx context.Context, tenantID, accountID string, req PatchAccountAutoResolveReq) error {
	now := time.Now()
	filter := bson.M{"tenant_id": tenantID, "account_id": accountID}
	update := bson.M{
		"$set": bson.M{
			"auto_resolve": req.AutoResolve,
			"status":       1,
			"updated_at":   now,
		},
		"$setOnInsert": bson.M{
			"tenant_id":  tenantID,
			"account_id": accountID,
			"created_at": now,
		},
	}

	var m chatmodel.AccountSetting
	if _, err := m.Collection().UpdateOne(ctx, filter, update, options.Update().SetUpsert(true)); err != nil {
		return internal("db error", err)
	}
	return nil
}

func UpsertAutoResolvePolicyBiz(ctx context.Context, tenantID, accountID string, req UpsertPolicyReq) (*chatmodel.AutoResolvePolicy, error) {
	if !isScopeOk(req.ScopeType) {
		return nil, bad("invalid scope_type")
	}
	if req.InactiveRawUnit != "" &&
		req.InactiveRawUnit != "minute" && req.InactiveRawUnit != "hour" && req.InactiveRawUnit != "day" {
		return nil, bad("invalid inactive_raw_unit")
	}

	now := time.Now()
	seconds := toSeconds(req.InactiveRawValue, req.InactiveRawUnit)

	filter := bson.M{"tenant_id": tenantID, "account_id": accountID, "scope_type": req.ScopeType}
	if req.ScopeID != "" {
		filter["scope_id"] = req.ScopeID
	} else {
		filter["scope_id"] = bson.M{"$in": []any{nil, ""}}
	}

	update := bson.M{
		"$set": bson.M{
			"tenant_id":  tenantID,
			"account_id": accountID,
			"scope_type": req.ScopeType,
			"scope_id":   req.ScopeID,

			"enabled": req.Enabled,

			"inactive_raw_value":         req.InactiveRawValue,
			"inactive_raw_unit":          req.InactiveRawUnit,
			"inactive_threshold_seconds": seconds,

			"resolve_message":            req.ResolveMessage,
			"skip_waiting_conversations": req.SkipWaitingConversations,
			"post_resolve_tag_ids":       req.PostResolveTagIDs,

			"updated_by": req.UpdatedBy,
			"updated_at": now,
		},
		"$setOnInsert": bson.M{"created_at": now},
	}

	var m chatmodel.AutoResolvePolicy
	opt := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)

	var out chatmodel.AutoResolvePolicy
	if err := m.Collection().FindOneAndUpdate(ctx, filter, update, opt).Decode(&out); err != nil {
		return nil, internal("db error", err)
	}
	return &out, nil
}

func GetAutoResolvePolicyBiz(
	ctx context.Context,
	tenantID string,
	accountID string,
	scopeType string,
	scopeID string,
) (*chatmodel.AutoResolvePolicy, error) {

	if !isScopeOk(scopeType) {
		return nil, bad("invalid scope_type")
	}

	filter := bson.M{
		"tenant_id":  tenantID,
		"account_id": accountID,
		"scope_type": scopeType,
	}

	// account 级别建议 scope_id 固定为空
	if scopeID != "" {
		filter["scope_id"] = scopeID
	} else {
		filter["scope_id"] = bson.M{"$in": []any{nil, ""}}
	}

	var p chatmodel.AutoResolvePolicy
	if err := p.Collection().FindOne(ctx, filter).Decode(&p); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, notFound("auto resolve policy not found")
		}
		return nil, internal("db error", err)
	}

	return &p, nil
}

func PatchPolicyEnabledBiz(ctx context.Context, tenantID, accountID string, req PatchPolicyEnabledReq) error {
	if !isScopeOk(req.ScopeType) {
		return bad("invalid scope_type")
	}
	filter := bson.M{"tenant_id": tenantID, "account_id": accountID, "scope_type": req.ScopeType}
	if req.ScopeID != "" {
		filter["scope_id"] = req.ScopeID
	}
	update := bson.M{"$set": bson.M{
		"enabled":    req.Enabled,
		"updated_by": req.UpdatedBy,
		"updated_at": time.Now(),
	}}

	var m chatmodel.AutoResolvePolicy
	res, err := m.Collection().UpdateOne(ctx, filter, update)
	if err != nil {
		return internal("db error", err)
	}
	if res.MatchedCount == 0 {
		return notFound("policy not found")
	}
	return nil
}

func PatchPolicyPreferenceBiz(ctx context.Context, tenantID, accountID string, req PatchPolicyPreferenceReq) error {
	if !isScopeOk(req.ScopeType) {
		return bad("invalid scope_type")
	}
	filter := bson.M{"tenant_id": tenantID, "account_id": accountID, "scope_type": req.ScopeType}
	if req.ScopeID != "" {
		filter["scope_id"] = req.ScopeID
	}
	update := bson.M{"$set": bson.M{
		"skip_waiting_conversations": req.SkipWaitingConversations,
		"updated_by":                 req.UpdatedBy,
		"updated_at":                 time.Now(),
	}}

	var m chatmodel.AutoResolvePolicy
	res, err := m.Collection().UpdateOne(ctx, filter, update)
	if err != nil {
		return internal("db error", err)
	}
	if res.MatchedCount == 0 {
		return notFound("policy not found")
	}
	return nil
}

func PatchPolicyTagsBiz(ctx context.Context, tenantID, accountID string, req PatchPolicyTagsReq) error {
	if !isScopeOk(req.ScopeType) {
		return bad("invalid scope_type")
	}
	filter := bson.M{"tenant_id": tenantID, "account_id": accountID, "scope_type": req.ScopeType}
	if req.ScopeID != "" {
		filter["scope_id"] = req.ScopeID
	}
	update := bson.M{"$set": bson.M{
		"post_resolve_tag_ids": req.PostResolveTagIDs,
		"updated_by":           req.UpdatedBy,
		"updated_at":           time.Now(),
	}}

	var m chatmodel.AutoResolvePolicy
	res, err := m.Collection().UpdateOne(ctx, filter, update)
	if err != nil {
		return internal("db error", err)
	}
	if res.MatchedCount == 0 {
		return notFound("policy not found")
	}
	return nil
}
