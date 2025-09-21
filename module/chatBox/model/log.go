package model

import (
	"PProject/service/mgo"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// AgentRuleLog {
// "_id": { "$oid": "66ef1d18b2d2f6a8e2c9a201" },
// "tenant_id": "t_1001",
// "rule_id": { "$oid": "66ef1b8fb2d2f6a8e2c9a001" },
// "event_id": "evt_20250921_001122",
// "conversation_id": "c_778899",
// "action": { "type": "emit_webhook", "params": { "url": "https://xxx/hook", "payload": {"x":1} } },
// "status": "pending",                      // pending/sent/failed
// "attempt": 0,
// "next_retry_at": { "$date": 1726893700000 },
// "last_error": "",
// "idempotency_key": "auto:t_1001:c_778899:r_66ef1b8f:emit_webhook:evt_20250921_001122",
// "created_at": { "$date": 1726893620015 },
// "updated_at": { "$date": 1726893620015 }
// }
// AgentRuleLog 规则执行的日志表
type AgentRuleLog struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"`
	TenantID       string             `bson:"tenant_id"`
	RuleID         primitive.ObjectID `bson:"rule_id"`
	RuleName       string             `bson:"rule_name"`
	EventID        string             `bson:"event_id"`
	EventType      string             `bson:"event_type"`
	ConversationID string             `bson:"conversation_id,omitempty"`
	MessageID      *string            `bson:"message_id,omitempty"`
	Matched        bool               `bson:"matched"`
	Depth          int32              `bson:"depth"`
	Actions        []RunAction        `bson:"actions"`
	NodeID         string             `bson:"node_id"`
	RequestID      string             `bson:"request_id"`
	StartedAt      time.Time          `bson:"started_at"`
	FinishedAt     time.Time          `bson:"finished_at"`
}

func (sess *AgentRuleLog) GetTableName() string {
	return "chatbox_agent_rule_log"
}

func (sess *AgentRuleLog) Collection() *mongo.Collection {
	return mgo.GetDB().Collection(sess.GetTableName())
}

type RunAction struct {
	Type       string `bson:"type"`
	OK         bool   `bson:"ok"`
	Error      string `bson:"error,omitempty"`
	DurationMS int32  `bson:"duration_ms"`
}
