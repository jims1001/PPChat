package model

import (
	"PProject/service/mgo"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Team 团队
type Team struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"          json:"_id,omitempty"`
	TenantID  string             `bson:"tenant_id"              json:"tenant_id"`
	AccountID string             `bson:"account_id"             json:"account_id"`

	Name        string `bson:"name"                         json:"name"`        // 团队名称（木材销售）
	Description string `bson:"description,omitempty"        json:"description"` // 团队描述
	// 允许为这个团队自动分配（页面的勾选）
	AutoAssignEnabled bool `bson:"auto_assign_enabled"         json:"auto_assign_enabled"`

	// —— 可选：自动分配策略（以后扩展时用得上） —— //
	// round_robin / load / idle_time / custom
	AutoAssignStrategy string `bson:"auto_assign_strategy,omitempty" json:"auto_assign_strategy,omitempty"`
	// 限流：每客服最大并发会话数（0=不限制）
	AgentMaxActiveConvs int `bson:"agent_max_active_convs,omitempty" json:"agent_max_active_convs,omitempty"`

	// 统计（便于列表页直接展示；由服务端异步刷新）
	AgentCount int       `bson:"agent_count"                  json:"agent_count"`
	CreatedAt  time.Time `bson:"created_at"                   json:"created_at"`
	UpdatedAt  time.Time `bson:"updated_at"                   json:"updated_at"`
}

func (sess *Team) GetTableName() string {
	return "chat_box_team"
}

func (sess *Team) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(sess.GetTableName())
}

// TeamAgent 团队-客服 关联（用勾选更新）
type TeamAgent struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"          json:"_id,omitempty"`
	TenantID  string             `bson:"tenant_id"              json:"tenant_id"`
	AccountID string             `bson:"account_id"             json:"account_id"`
	TeamID    string             `bson:"team_id"                json:"team_id"`
	AgentID   string             `bson:"agent_id"               json:"agent_id"`

	// 该成员在本团队内的角色：agent/manager
	TeamRole string    `bson:"team_role"                      json:"team_role"`
	JoinedAt time.Time `bson:"joined_at"                      json:"joined_at"`
	// 可选：是否参与自动分配（个体级开关），默认 true
	AutoAssignable *bool `bson:"auto_assignable,omitempty"     json:"auto_assignable,omitempty"`
}

// 建立索引
//// Team
//db.team.createIndex({ tenant_id: 1, account_id: 1, name: 1 }, { unique: true, name: "uniq_team_name" })
//
//// TeamAgent
//db.team_agent.createIndex({ tenant_id: 1, account_id: 1, team_id: 1, agent_id: 1 }, { unique: true, name: "uniq_team_agent" })
//db.team_agent.createIndex({ tenant_id: 1, team_id: 1 }, { name: "idx_team_members" })
//db.team_agent.createIndex({ tenant_id: 1, agent_id: 1 }, { name: "idx_agent_teams" })

func (sess *TeamAgent) GetTableName() string {
	return "chat_box_team_agent"
}

func (sess *TeamAgent) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(sess.GetTableName())
}
