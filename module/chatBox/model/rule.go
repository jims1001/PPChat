package model

import (
	"PProject/service/mgo"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// ChatBoxAgentRule {
// "_id": { "$oid": "66ef1b8fb2d2f6a8e2c9a001" },
// "tenant_id": "t_1001",
// "name": "新对话自动分配客服1",
// "desc": "网站来访的打开状态对话分配给客服1并打标签",
// "event": "conversation.created",              // enum: conversation.created / conversation.updated / conversation.resolved / message.created / conversation.opened
// "scope": {                                    // 规则作用域（预过滤）
// "inbox_ids": ["ibx_web"],
// "team_ids": []
// },
// "priority": 100,                              // 越小越先执行
// "stop_on_match": true,                        // 命中后停止后续规则
// "enabled": true,
// "conditions": {                               // 条件树（AND/OR 嵌套）
// "type": "and",
// "children": [
// { "field": "conversation.status", "op": "eq", "value": "open", "value_type": "string" },
// { "field": "conversation.inbox_id", "op": "in", "value": ["ibx_web"], "value_type": "array_string" }
// ]
// },
// "actions": [                                  // 顺序执行
// { "type": "assign_agent", "params": { "agent_id": "ag_001" } },
// { "type": "add_tags", "params": { "tags": ["from_web", "vip"] } }
// ],
// "created_by": "u_001",
// "updated_by": "u_001",
// "version": 3,
// "created_at": { "$date": 1726890000000 },
// "updated_at": { "$date": 1726893600000 },
// "deleted_at": null
// }
// ChatBoxAgentRule 规则表
type ChatBoxAgentRule struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	TenantID    string             `bson:"tenant_id"     json:"tenantId"`
	Name        string             `bson:"name"          json:"name"`
	Desc        string             `bson:"desc,omitempty" json:"desc,omitempty"`
	Event       string             `bson:"event"         json:"event"`
	Scope       RuleScope          `bson:"scope,omitempty" json:"scope,omitempty"`
	Priority    int32              `bson:"priority"      json:"priority"`
	StopOnMatch bool               `bson:"stop_on_match" json:"stopOnMatch"`
	Enabled     bool               `bson:"enabled"       json:"enabled"`
	Conditions  Node               `bson:"conditions"    json:"conditions"`
	Actions     []Action           `bson:"actions"       json:"actions"`
	CreatedBy   string             `bson:"created_by,omitempty"`
	UpdatedBy   string             `bson:"updated_by,omitempty"`
	Version     int32              `bson:"version"`
	CreatedAt   time.Time          `bson:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at"`
	DeletedAt   *time.Time         `bson:"deleted_at,omitempty"`
}

func (sess *ChatBoxAgentRule) GetTableName() string {
	return "chat_box_agent_rule"
}

func (sess *ChatBoxAgentRule) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(sess.GetTableName())
}

type RuleScope struct {
	InboxIDs []string `bson:"inbox_ids,omitempty" json:"inboxIds,omitempty"`
	TeamIDs  []string `bson:"team_ids,omitempty"  json:"teamIds,omitempty"`
}

type Node struct {
	Type     string `bson:"type"                json:"type"`            // "and" / "or" / "cond"
	Field    string `bson:"field,omitempty"     json:"field,omitempty"` // 叶子结点
	Op       string `bson:"op,omitempty"        json:"op,omitempty"`    // eq/neq/in/not_in/contains/...
	Value    any    `bson:"value,omitempty"     json:"value,omitempty"`
	ValueTyp string `bson:"value_type,omitempty" json:"value_type,omitempty"`
	Children []Node `bson:"children,omitempty"  json:"children,omitempty"` // 非叶
}

type Action struct {
	Type   string         `bson:"type"   json:"type"`
	Params map[string]any `bson:"params" json:"params"`
}

// AgentMacro {
// "_id": { "$oid": "650c2f82d91c34a9e8b4d333" },
// "tenant_id": "tenant_001",
// "name": "高优先级分配到销售团队",
// "visibility": "public",
// "actions": [
// {
// "type": "add_tag",
// "params": { "tag_id": "64f8b222a0cdd9f66" }
// },
// {
// "type": "assign_team",
// "params": { "team_id": "64f8b2fa10eeaaf88" }
// },
// {
// "type": "set_priority",
// "params": { "level": "high" }
// }
// ],
// "created_by": "agent_123",
// "created_at": "2025-09-21T06:30:00Z",
// "updated_at": "2025-09-21T06:30:00Z"
// }
// AgentMacro 宏操作
type AgentMacro struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"    json:"id,omitempty"`
	TenantID    string             `bson:"tenant_id"        json:"tenant_id"` // 租户ID
	Name        string             `bson:"name"             json:"name"`      // 宏名称
	Description string             `bson:"description,omitempty" json:"description,omitempty"`
	Visibility  string             `bson:"visibility"       json:"visibility"` // public / private

	Actions []MacroAction `bson:"actions"          json:"actions"` // 宏中定义的动作

	CreatedBy string    `bson:"created_by"       json:"created_by"` // 创建人 AgentID
	CreatedAt time.Time `bson:"created_at"       json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"       json:"updated_at"`
}

func (sess *AgentMacro) GetTableName() string {
	return "chat_box_macro"
}

func (sess *AgentMacro) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(sess.GetTableName())
}

// MacroAction 宏操作动作
type MacroAction struct {
	Type   string                 `bson:"type"              json:"type"`             // 动作类型 (add_tag, assign_team, assign_agent, set_priority, close_conversation...)
	Params map[string]interface{} `bson:"params,omitempty"  json:"params,omitempty"` // 参数 (例如 tag_id, team_id, priority_level)
}

// AgentCannedReply {
// "_id": { "$oid": "650caaa1abcd5678ef901234" },
// "code": "welcome_cn",
// "message": "您好，感谢联系家具销售，我们的客服稍后会回复您。",
// "creator_id": { "$oid": "650c9876abcd5678ef901234" },
// "scope": "team",
// "team_id": { "$oid": "650c5555abcd5678ef901234" },
// "is_active": true,
// "usage_count": 12,
// "created_at": { "$date": "2025-09-21T12:00:00Z" },
// "updated_at": { "$date": "2025-09-21T12:10:00Z" }
// }
// AgentCannedReply 预设回复
type AgentCannedReply struct {
	ID         primitive.ObjectID  `bson:"_id,omitempty"        json:"id"`
	Code       string              `bson:"code"                 json:"code"`       // 短代码（唯一约束）
	Message    string              `bson:"message"              json:"message"`    // 消息模板内容
	CreatorID  primitive.ObjectID  `bson:"creator_id,omitempty" json:"creatorId"`  // 创建者 Agent ID
	Scope      string              `bson:"scope"                json:"scope"`      // 范围：personal / team / global
	TeamID     *primitive.ObjectID `bson:"team_id,omitempty"   json:"teamId"`      // 团队ID（scope=team 时有值）
	IsActive   bool                `bson:"is_active"            json:"isActive"`   // 是否启用
	UsageCount int64               `bson:"usage_count"          json:"usageCount"` // 使用次数
	CreatedAt  time.Time           `bson:"created_at"           json:"createdAt"`
	UpdatedAt  time.Time           `bson:"updated_at"           json:"updatedAt"`
}

func (sess *AgentCannedReply) GetTableName() string {
	return "chat_box_canned_reply"
}

func (sess *AgentCannedReply) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(sess.GetTableName())
}
