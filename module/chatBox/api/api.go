package api

import (
	service "PProject/module/chatBox/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

// =============== HTTP handlers（只做：取参 + 调业务 + 返回） ===============

func hCreateAccountSetting(c *gin.Context) {
	tenantID := c.Param("tenantId")

	var req service.CreateAccountReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	out, err := service.CreateAccountSettingBiz(c.Request.Context(), tenantID, req)
	if err != nil {
		service.WriteBizErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func hGetAccountSetting(c *gin.Context) {
	tenantID := c.Param("tenantId")
	accountID := c.Param("accountId")

	out, err := service.GetAccountSettingBiz(c.Request.Context(), tenantID, accountID)
	if err != nil {
		service.WriteBizErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func hPatchAccountSetting(c *gin.Context) {
	tenantID := c.Param("tenantId")
	accountID := c.Param("accountId")

	var req service.PatchAccountSettingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := service.PatchAccountSettingBiz(c.Request.Context(), tenantID, accountID, req); err != nil {
		service.WriteBizErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func hPatchAccountAutoResolve(c *gin.Context) {
	tenantID := c.Param("tenantId")
	accountID := c.Param("accountId")

	var req service.PatchAccountAutoResolveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := service.PatchAccountAutoResolveBiz(c.Request.Context(), tenantID, accountID, req); err != nil {
		service.WriteBizErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func hUpsertAutoResolvePolicy(c *gin.Context) {
	tenantID := c.Param("tenantId")
	accountID := c.Param("accountId")

	var req service.UpsertPolicyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	out, err := service.UpsertAutoResolvePolicyBiz(c.Request.Context(), tenantID, accountID, req)
	if err != nil {
		service.WriteBizErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func hGetAutoResolvePolicy(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := c.Param("tenantId")
	accountID := c.Param("accountId")

	scopeType := c.Query("scope_type")
	scopeID := c.Query("scope_id")

	if scopeType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "scope_type required"})
		return
	}

	out, err := service.GetAutoResolvePolicyBiz(ctx, tenantID, accountID, scopeType, scopeID)
	if err != nil {
		service.WriteBizErr(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": out})
}

func hPatchPolicyEnabled(c *gin.Context) {
	tenantID := c.Param("tenantId")
	accountID := c.Param("accountId")

	var req service.PatchPolicyEnabledReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := service.PatchPolicyEnabledBiz(c.Request.Context(), tenantID, accountID, req); err != nil {
		service.WriteBizErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func hPatchPolicyPreference(c *gin.Context) {
	tenantID := c.Param("tenantId")
	accountID := c.Param("accountId")

	var req service.PatchPolicyPreferenceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := service.PatchPolicyPreferenceBiz(c.Request.Context(), tenantID, accountID, req); err != nil {
		service.WriteBizErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func hPatchPolicyTags(c *gin.Context) {
	tenantID := c.Param("tenantId")
	accountID := c.Param("accountId")

	var req service.PatchPolicyTagsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := service.PatchPolicyTagsBiz(c.Request.Context(), tenantID, accountID, req); err != nil {
		service.WriteBizErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
