package service

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Server struct{ DB *mongo.Database }

// GetPublicTimeline 普通聊天：只读 public_timeline
func (s *Server) GetPublicTimeline(c *gin.Context) error {
	tenant := c.Request.FormValue("tenant_id")
	conv := c.Param("conversation_id")
	cursor := parseInt64(c.Request.FormValue("cursor_seq"), 1<<62)
	limit := parseInt64(c.Request.FormValue("limit"), 50)

	cur, err := s.DB.Collection("public_timeline").Find(c.Request.Context(),
		bson.M{"tenant_id": tenant, "conversation_id": conv, "seq": bson.M{"$lt": cursor}},
		optionsFindSortLimit(bson.D{{Key: "seq", Value: -1}}, int(limit)),
	)
	if err != nil {
		return err
	}
	var items []bson.M
	if err := cur.All(c.Request.Context(), &items); err != nil {
		return err
	}

	c.JSON(http.StatusOK, map[string]any{"items": items})
	return nil
}

// GetAgentTimeline 坐席端：public ∪ private(ACL) 合并
func (s *Server) GetAgentTimeline(c *gin.Context) error {
	ctx := c.Request.Context()
	tenant := c.Request.FormValue("tenant_id")
	conv := c.Param("conversation_id")
	cursor := parseInt64(c.Request.FormValue("cursor_seq"), 1<<62)
	limit := int(parseInt64(c.Request.FormValue("limit"), 50))

	agentID := c.Request.FormValue("agent_id")
	teamIDs := c.Request.FormValue("team_id") // ?team_id=t1&team_id=t2
	roleIDs := c.Request.FormValue("role")
	nowMs := time.Now().UnixMilli()

	// public
	pubCur, _ := s.DB.Collection("public_timeline").Find(ctx,
		bson.M{"tenant_id": tenant, "conversation_id": conv, "seq": bson.M{"$lt": cursor}},
		optionsFindSortLimit(bson.D{{Key: "seq"}}, limit),
	)
	var pub []bson.M
	_ = pubCur.All(ctx, &pub)

	// private（ACL 过滤）
	privFilter := bson.M{
		"tenant_id": tenant, "conversation_id": conv, "seq": bson.M{"$lt": cursor},
		"$and": bson.A{
			bson.M{"$or": bson.A{bson.M{"created_at_ms": bson.M{"$exists": false}}, bson.M{"created_at_ms": bson.M{"$lte": nowMs}}}},
			bson.M{"$or": bson.A{bson.M{"expire_at_ms": bson.M{"$exists": false}}, bson.M{"expire_at_ms": 0}, bson.M{"expire_at_ms": bson.M{"$gt": nowMs}}}},
		},
		"$nor": bson.A{
			bson.M{"vis_deny.agents": agentID},
			bson.M{"vis_deny.teams": bson.M{"$in": teamIDs}},
			bson.M{"vis_deny.roles": bson.M{"$in": roleIDs}},
		},
		"$or": bson.A{
			bson.M{"vis_mode": "agent_only", "vis_allow.agents": agentID},
			bson.M{"vis_mode": "team_only", "vis_allow.teams": bson.M{"$in": teamIDs}},
			bson.M{"vis_mode": "role_only", "vis_allow.roles": bson.M{"$in": roleIDs}},
			bson.M{"vis_mode": "custom", "$or": bson.A{
				bson.M{"vis_allow.agents": agentID},
				bson.M{"vis_allow.teams": bson.M{"$in": teamIDs}},
				bson.M{"vis_allow.roles": bson.M{"$in": roleIDs}},
			}},
		},
	}
	privCur, _ := s.DB.Collection("private_notes").Find(ctx, privFilter, optionsFindSortLimit(bson.D{{Key: "seq"}}, limit))
	var pri []bson.M
	_ = privCur.All(ctx, &pri)

	// 合并按 seq（倒序）去重
	all := append(pub, pri...)
	sort.Slice(all, func(i, j int) bool { return all[i]["seq"].(int64) > all[j]["seq"].(int64) })
	if len(all) > limit {
		all = all[:limit]
	}

	c.JSON(http.StatusOK, map[string]any{"items": all})

	return nil
}

// helpers
func parseInt64(s string, def int64) int64 {
	var x int64
	_, err := fmt.Sscan(s, &x)
	if err != nil {
		return def
	}
	return x
}
func optionsFindSortLimit(sort bson.D, limit int) *options.FindOptions {
	return options.Find().SetSort(sort).SetLimit(int64(limit))
}
