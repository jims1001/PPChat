package user

import (
	"PProject/global"
	service "PProject/module/user/service"
	"PProject/tools/errs"
	"PProject/tools/ids"
	"context"
	"github.com/gin-gonic/gin"
	"net/http"
)

type CheckParams struct {
	Token     string `json:"token"`
	TokenHash string `json:"token_hash"`
}

func HandlerLogin(c *gin.Context) {
	var loginParams service.LoginParams
	if err := c.ShouldBindJSON(&loginParams); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	sessionId := ids.GenerateString()
	loginParams.SessionID = sessionId

	ctx := context.Background()
	defer ctx.Done()

	login, err := service.Login(ctx, loginParams)
	if err != nil {
		c.JSON(http.StatusOK, errs.ErrArgs)
		return
	}

	c.JSON(http.StatusOK, login)

}

func HandlerCheck(c *gin.Context) {

	var inParams CheckParams
	if err := c.ShouldBindJSON(&inParams); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()
	defer ctx.Done()

	session, err := service.Verify(ctx, inParams.Token, inParams.TokenHash)
	if err != nil {
		c.JSON(http.StatusOK, errs.ErrTokenExpired)
		return
	}

	c.JSON(http.StatusOK, global.Sucess(session))
}

func handleUserInfo(c *gin.Context) {
	ctx := context.Background()
	defer ctx.Done()

	c.JSON(http.StatusOK, global.Sucess(""))
}
