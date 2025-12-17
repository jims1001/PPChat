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
	// Role
	r.POST("/tenants/:tenantId/roles", roleH.Create)
	r.GET("/tenants/:tenantId/roles", roleH.List)
	r.GET("/tenants/:tenantId/roles/:roleId", roleH.Get)
	r.PATCH("/tenants/:tenantId/roles/:roleId", roleH.Patch)
	r.DELETE("/tenants/:tenantId/roles/:roleId", roleH.Delete)

	// Permission
	r.POST("/permissions", permH.Create) // 你的 Permission 没 tenantId 字段，这里做全局权限点
	r.GET("/permissions", permH.List)
	r.GET("/permissions/:permissionId", permH.Get)

	// Role-Permission
	r.POST("/tenants/:tenantId/roles/:roleId/permissions", rpH.BindBatch)
	r.GET("/tenants/:tenantId/roles/:roleId/permissions", rpH.ListByRole)
	r.DELETE("/tenants/:tenantId/roles/:roleId/permissions/:permissionId", rpH.Unbind)

	// Conversation - Assignee (AgentConversation)
	r.POST("/tenants/:tenantId/conversations/:conversationId/assignee", asgH.Set)
	r.GET("/tenants/:tenantId/conversations/:conversationId/assignee", asgH.Get)
	r.DELETE("/tenants/:tenantId/conversations/:conversationId/assignee/:agentId", asgH.Remove)

	// Conversation - Collaborators (AgentCollaborator)
	r.POST("/tenants/:tenantId/conversations/:conversationId/collaborators", colH.Add)
	r.GET("/tenants/:tenantId/conversations/:conversationId/collaborators", colH.List)
	r.DELETE("/tenants/:tenantId/conversations/:conversationId/collaborators/:agentId", colH.Remove)
}

func RegisterAgentRoutes(r *gin.Engine, h *handler.AgentHandler) {
	r.POST("/tenants/:tenantId/agents", h.CreateAgent)
	r.GET("/tenants/:tenantId/agents", h.ListAgents)
	r.GET("/tenants/:tenantId/agents/:agentId", h.GetAgent)
	r.PATCH("/tenants/:tenantId/agents/:agentId", h.PatchAgent)

	r.POST("/tenants/:tenantId/agents/:agentId/resend-invite", h.ResendInvite)
	r.POST("/tenants/:tenantId/agents/invites/accept", h.AcceptInvite)
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
