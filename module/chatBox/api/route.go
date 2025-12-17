package api

import (
	mid "PProject/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterAccountSettingRoutes(r gin.Engine) {
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
