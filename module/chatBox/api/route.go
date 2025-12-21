package api

import (
	mid "PProject/middleware"
	"PProject/module/chatBox/api/handler"
	"PProject/module/chatBox/service/repo"

	"github.com/gin-gonic/gin"
)

func RegisterAccountSettingRoutes(r *gin.Engine) {
	g := r.Group("/api/v1/:tenantId")
	// account setting
	mid.POST(g, "/accounts", hCreateAccountSetting, mid.RouteOpt{IsAuth: true})
	mid.GET(g, "/accounts/:accountId/setting", hGetAccountSetting, mid.RouteOpt{IsAuth: true})
	mid.PATCH(g, "/accounts/:accountId/setting", hPatchAccountSetting, mid.RouteOpt{IsAuth: true})
	mid.PATCH(g, "/accounts/:accountId/setting/auto-resolve", hPatchAccountAutoResolve, mid.RouteOpt{IsAuth: true})
	// auto-resolve policy
	mid.PUT(g, "/accounts/:accountId/auto-resolve/policy", hUpsertAutoResolvePolicy, mid.RouteOpt{IsAuth: true})
	mid.GET(g, "/accounts/:accountId/auto-resolve/policy", hGetAutoResolvePolicy, mid.RouteOpt{IsAuth: true})
	mid.PATCH(g, "/accounts/:accountId/auto-resolve/policy/enabled", hPatchPolicyEnabled, mid.RouteOpt{IsAuth: true})
	mid.PATCH(g, "/accounts/:accountId/auto-resolve/policy/preference", hPatchPolicyPreference, mid.RouteOpt{IsAuth: true})
	mid.PATCH(g, "/accounts/:accountId/auto-resolve/policy/tags", hPatchPolicyTags, mid.RouteOpt{IsAuth: true})

	//mid.POST(g, "/login", user.HandlerLogin, mid.RouteOpt{IsAuth: false})
}

func RegisterV1(r *gin.Engine,
	roleH *handler.RoleHandler,
	permH *handler.PermissionHandler,
	rpH *handler.RolePermissionHandler,
	asgH *handler.AssigneeHandler,
	colH *handler.CollaboratorHandler,
) {
	g := r.Group("/api/v1/:tenantId")

	// Role
	g.POST("/roles", roleH.Create)
	g.GET("/roles", roleH.List)
	g.GET("/roles/:roleId", roleH.Get)
	g.PATCH("/roles/:roleId", roleH.Patch)
	g.DELETE("/roles/:roleId", roleH.Delete)

	// Role-Permission
	g.POST("/roles/:roleId/permissions", rpH.BindBatch)
	g.GET("/roles/:roleId/permissions", rpH.ListByRole)
	g.DELETE("/roles/:roleId/permissions/:permissionId", rpH.Unbind)

	// Conversation - Assignee
	g.POST("/conversations/:conversationId/assignee", asgH.Set)
	g.GET("/conversations/:conversationId/assignee", asgH.Get)
	g.DELETE("/conversations/:conversationId/assignee/:agentId", asgH.Remove)

	// Conversation - Collaborators
	g.POST("/conversations/:conversationId/collaborators", colH.Add)
	g.GET("/conversations/:conversationId/collaborators", colH.List)
	g.DELETE("/conversations/:conversationId/collaborators/:agentId", colH.Remove)

	// Permission（全局，不带 tenantId）
	v1 := r.Group("/api/v1")
	v1.POST("/permissions", permH.Create)
	v1.GET("/permissions", permH.List)
	v1.GET("/permissions/:permissionId", permH.Get)
}

func RegisterAgentRoutes(r *gin.Engine, h *handler.AgentHandler) {
	g := r.Group("/api/v1/:tenantId")

	g.POST("/agents", h.CreateAgent)
	g.GET("/agents", h.ListAgents)
	g.GET("/agents/:agentId", h.GetAgent)
	g.PATCH("/agents/:agentId", h.PatchAgent)

	g.POST("/agents/:agentId/resend-invite", h.ResendInvite)
	g.POST("/agents/invites/accept", h.AcceptInvite)
}

func InitHandlers() (
	*handler.RoleHandler,
	*handler.PermissionHandler,
	*handler.RolePermissionHandler,
	*handler.AssigneeHandler,
	*handler.CollaboratorHandler,
) {
	// --- repos ---
	roleRepo := repo.NewRoleRepo()
	permRepo := repo.NewPermissionRepo()
	rpRepo := repo.NewRolePermissionRepo()
	agentConvRepo := repo.NewAgentConversationRepo()
	agentColRepo := repo.NewAgentCollaboratorRepo()

	// --- handlers ---
	roleH := handler.NewRoleHandler(roleRepo)
	permH := handler.NewPermissionHandler(permRepo)

	rpH := handler.NewRolePermissionHandler(
		roleRepo,
		permRepo,
		rpRepo,
	)

	asgH := handler.NewAssigneeHandler(agentConvRepo)
	colH := handler.NewCollaboratorHandler(agentColRepo)

	return roleH, permH, rpH, asgH, colH
}

func RegisterRoleAndPermissionRoutes(r *gin.Engine) {
	roleH, permH, rpH, asgH, colH := InitHandlers()

	RegisterV1(
		r,
		roleH,
		permH,
		rpH,
		asgH,
		colH,
	)
}

func RegisterAgentRoutesV1(r *gin.Engine) {
	// repo
	agentRepo := repo.NewAgentRepo()

	// handler
	agentH := handler.NewAgentHandler(agentRepo)

	// routes
	RegisterAgentRoutes(r, agentH)
}
