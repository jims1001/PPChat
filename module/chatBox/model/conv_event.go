package model

import (
	"PProject/service/mgo"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type ACLList struct {
	Teams  []string `bson:"teams,omitempty"  json:"teams,omitempty"`
	Agents []string `bson:"agents,omitempty" json:"agents,omitempty"`
	Roles  []string `bson:"roles,omitempty"  json:"roles,omitempty"`
	Tags   []string `bson:"tags,omitempty"   json:"tags,omitempty"`
}

type ConversationEvent struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"         json:"_id,omitempty"`
	TenantID       string             `bson:"tenant_id"             json:"tenant_id"`
	ConversationID string             `bson:"conversation_id"       json:"conversation_id"`
	Seq            int64              `bson:"seq"                   json:"seq"`
	EventID        string             `bson:"event_id"              json:"event_id"` // 幂等键（uuidv7/雪花）

	EventType   string `bson:"event_type"                json:"event_type"`           // CustomerMessageCreated | AgentMessageCreated | InternalNoteAdded | MessageEdited | MessageRedacted | VisibilityChanged | ConversationTransferred
	ActorType   string `bson:"actor_type,omitempty"      json:"actor_type,omitempty"` // user|agent|system
	ActorID     string `bson:"actor_id,omitempty"        json:"actor_id,omitempty"`
	ContentType int32  `bson:"content_type,omitempty"    json:"content_type,omitempty"`
	// Payload 可直接复用你现有 MessageModel 的子结构；此处用 map 兼容
	Payload map[string]any `bson:"payload,omitempty" json:"payload,omitempty"`

	VisMode     string   `bson:"vis_mode,omitempty"     json:"vis_mode,omitempty"` // public|team_only|agent_only|role_only|custom
	VisAllow    *ACLList `bson:"vis_allow,omitempty"   json:"vis_allow,omitempty"`
	VisDeny     *ACLList `bson:"vis_deny,omitempty"    json:"vis_deny,omitempty"`
	Sensitivity string   `bson:"sensitivity,omitempty"  json:"sensitivity,omitempty"` // none|pii|secret
	EffectiveMS int64    `bson:"effective_ms,omitempty" json:"effective_ms,omitempty"`
	ExpireAtMS  int64    `bson:"expire_at_ms,omitempty" json:"expire_at_ms,omitempty"`
	VisVersion  int32    `bson:"vis_version,omitempty"  json:"vis_version,omitempty"`

	CreatedAtMS int64  `bson:"created_at_ms"           json:"created_at_ms"`
	TraceID     string `bson:"trace_id,omitempty"      json:"trace_id,omitempty"`

	// 对于编辑/撤回/可见性变更，建议带目标定位
	TargetEventID string `bson:"target_event_id,omitempty" json:"target_event_id,omitempty"`
	TargetSeq     int64  `bson:"target_seq,omitempty"      json:"target_seq,omitempty"`
}

// PublicTimeline public 投影：极简快照（可渲染）
type PublicTimeline struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"    json:"_id,omitempty"`
	TenantID       string             `bson:"tenant_id"        json:"tenant_id"`
	ConversationID string             `bson:"conversation_id"  json:"conversation_id"`
	Seq            int64              `bson:"seq"              json:"seq"`
	SourceEventID  string             `bson:"source_event_id"  json:"source_event_id"`
	From           string             `bson:"from"             json:"from"`     // user_xxx / agent_xxx / system
	MsgType        string             `bson:"msg_type"         json:"msg_type"` // text/image/file/markdown...
	Content        map[string]any     `bson:"content"          json:"content"`  // 已脱敏/降级内容
	Sensitivity    string             `bson:"sensitivity"      json:"sensitivity"`
	Edited         bool               `bson:"edited"           json:"edited"`
	Redacted       bool               `bson:"redacted"         json:"redacted"`
	CreatedAtMS    int64              `bson:"created_at_ms"    json:"created_at_ms"`
}

func (sess *PublicTimeline) GetTableName() string {
	return "public_timeline"
}

func (sess *PublicTimeline) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(sess.GetTableName())
}

// PrivateNote private 投影：内部/私密 + ACL
type PrivateNote struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"    json:"_id,omitempty"`
	TenantID       string             `bson:"tenant_id"        json:"tenant_id"`
	ConversationID string             `bson:"conversation_id"  json:"conversation_id"`
	Seq            int64              `bson:"seq"              json:"seq"`
	SourceEventID  string             `bson:"source_event_id"  json:"source_event_id"`
	Kind           string             `bson:"kind"             json:"kind"` // internal_note|agent|system
	Content        map[string]any     `bson:"content"          json:"content"`
	Attachments    []map[string]any   `bson:"attachments,omitempty" json:"attachments,omitempty"`

	VisMode     string   `bson:"vis_mode"      json:"vis_mode"`
	VisAllow    *ACLList `bson:"vis_allow"     json:"vis_allow"`
	VisDeny     *ACLList `bson:"vis_deny"      json:"vis_deny"`
	Sensitivity string   `bson:"sensitivity"   json:"sensitivity"`

	Edited      bool  `bson:"edited"          json:"edited"`
	Redacted    bool  `bson:"redacted"        json:"redacted"`
	CreatedAtMS int64 `bson:"created_at_ms"   json:"created_at_ms"`
}

func (sess *PrivateNote) GetTableName() string {
	return "private_notes"
}

func (sess *PrivateNote) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(sess.GetTableName())
}
