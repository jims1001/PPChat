package store

import (
	"context"
	"log"

	"PProject/module/chatBox/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Projector struct {
	DB *mongo.Database
}

func isPublic(ev *model.ConversationEvent) bool {
	// public 且 deny 列表为空（或未设置）
	if ev.VisMode != "public" {
		return false
	}
	if ev.VisDeny != nil {
		if len(ev.VisDeny.Teams) > 0 || len(ev.VisDeny.Agents) > 0 || len(ev.VisDeny.Roles) > 0 {
			return false
		}
	}
	return true
}

func (p *Projector) downgrade(content map[string]any) map[string]any {
	// 对客裁剪/脱敏逻辑（示例：掩码）
	return content
}

func (p *Projector) upsertPublic(ev *model.ConversationEvent) error {
	doc := bson.M{
		"tenant_id":       ev.TenantID,
		"conversation_id": ev.ConversationID,
		"seq":             ev.Seq,
	}
	setOnInsert := bson.M{
		"source_event_id": ev.EventID,
		"from":            ev.ActorID,
		"msg_type":        ev.Payload["msg_type"],
		"content":         p.downgrade(anyToMap(ev.Payload["content"])),
		"sensitivity":     ev.Sensitivity,
		"edited":          false,
		"redacted":        false,
		"created_at_ms":   ev.CreatedAtMS,
	}
	_, err := p.DB.Collection("public_timeline").UpdateOne(context.Background(),
		doc, bson.M{"$setOnInsert": setOnInsert}, optionsUpdateUpsert())
	return err
}

func (p *Projector) upsertPrivate(ev *model.ConversationEvent) error {
	doc := bson.M{
		"tenant_id":       ev.TenantID,
		"conversation_id": ev.ConversationID,
		"seq":             ev.Seq,
	}
	setOnInsert := bson.M{
		"source_event_id": ev.EventID,
		"kind":            kindFromEvent(ev.EventType),
		"content":         anyToMap(ev.Payload["content"]),
		"attachments":     anyToSliceMap(ev.Payload["attachments"]),
		"vis_mode":        ev.VisMode,
		"vis_allow":       ev.VisAllow,
		"vis_deny":        ev.VisDeny,
		"sensitivity":     ev.Sensitivity,
		"edited":          false,
		"redacted":        false,
		"created_at_ms":   ev.CreatedAtMS,
	}
	_, err := p.DB.Collection("private_notes").UpdateOne(context.Background(),
		doc, bson.M{"$setOnInsert": setOnInsert}, optionsUpdateUpsert())
	return err
}

func (p *Projector) applyEdit(targetColl string, where bson.M, newContent map[string]any) error {
	_, err := p.DB.Collection(targetColl).UpdateOne(context.Background(), where,
		bson.M{"$set": bson.M{"content": newContent, "edited": true}})
	return err
}
func (p *Projector) applyRedact(targetColl string, where bson.M) error {
	_, err := p.DB.Collection(targetColl).UpdateOne(context.Background(), where,
		bson.M{"$set": bson.M{"content": bson.M{}, "attachments": []any{}, "redacted": true}})
	return err
}
func (p *Projector) movePrivateToPublic(ev *model.ConversationEvent) error {
	// 读取 private，写入 public，再可选标记 private published=true（不删除保审计）
	where := bson.M{"tenant_id": ev.TenantID, "conversation_id": ev.ConversationID, "seq": ev.TargetSeq}
	var pri bson.M
	err := p.DB.Collection("private_notes").FindOne(context.Background(), where).Decode(&pri)
	if err != nil {
		return err
	}
	pub := bson.M{
		"tenant_id":       ev.TenantID,
		"conversation_id": ev.ConversationID,
		"seq":             ev.TargetSeq,
	}
	setOnInsert := bson.M{
		"source_event_id": pri["source_event_id"],
		"from":            pri["from"],
		"msg_type":        pri["msg_type"],
		"content":         p.downgrade(anyToMap(pri["content"])),
		"sensitivity":     pri["sensitivity"],
		"edited":          pri["edited"],
		"redacted":        pri["redacted"],
		"created_at_ms":   pri["created_at_ms"],
	}
	_, err = p.DB.Collection("public_timeline").UpdateOne(context.Background(),
		pub, bson.M{"$setOnInsert": setOnInsert}, optionsUpdateUpsert())
	return err
}

func (p *Projector) Run(ctx context.Context) error {
	cs, err := p.DB.Collection("conversation_events").Watch(ctx, mongo.Pipeline{})
	if err != nil {
		return err
	}
	defer cs.Close(ctx)
	for cs.Next(ctx) {
		var ch struct {
			FullDocument model.ConversationEvent `bson:"fullDocument"`
		}
		if err := cs.Decode(&ch); err != nil {
			log.Println("decode:", err)
			continue
		}
		ev := ch.FullDocument

		switch ev.EventType {
		case "CustomerMessageCreated", "AgentMessageCreated", "AttachmentAdded", "InternalNoteAdded":
			if isPublic(&ev) {
				_ = p.upsertPublic(&ev)
			} else {
				_ = p.upsertPrivate(&ev)
			}
		case "MessageEdited":
			where := bson.M{"tenant_id": ev.TenantID, "conversation_id": ev.ConversationID, "seq": ev.TargetSeq}
			content := anyToMap(ev.Payload["content"])
			_ = p.applyEdit("public_timeline", where, content)
			_ = p.applyEdit("private_notes", where, content)
		case "MessageRedacted":
			where := bson.M{"tenant_id": ev.TenantID, "conversation_id": ev.ConversationID, "seq": ev.TargetSeq}
			_ = p.applyRedact("public_timeline", where)
			_ = p.applyRedact("private_notes", where)
		case "VisibilityChanged":
			// 简化：若变为 public 则 private→public；反之 public→private（你可按 ev.VisMode 判断）
			if ev.VisMode == "public" {
				_ = p.movePrivateToPublic(&ev)
			}
			// 反向移动同理（留作扩展）
		case "ConversationTransferred":
			// 可写 system 提示到 public / private
		}
	}
	return cs.Err()
}

// helpers
func optionsUpdateUpsert() *options.UpdateOptions { return options.Update().SetUpsert(true) }
func anyToMap(v any) map[string]any {
	if v == nil {
		return map[string]any{}
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}
func anyToSliceMap(v any) []map[string]any {
	if v == nil {
		return nil
	}
	if a, ok := v.([]map[string]any); ok {
		return a
	}
	return nil
}
func kindFromEvent(t string) string {
	if t == "InternalNoteAdded" {
		return "internal_note"
	}
	if t == "AgentMessageCreated" {
		return "agent"
	}
	return "system"
}
