package main

import (
	"PProject/logger"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgraded = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func wsEcho(c *gin.Context) {
	conn, err := upgraded.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Errorf("upgrade:", err)
		return
	}
	defer func(conn *websocket.Conn) {
		err := conn.Close()
		if err != nil {
			println("close conn:", err)
		}
	}(conn)

	conn.SetReadLimit(1 << 20) // 1MB
	err = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	if err != nil {
		println(err)
		return
	}

	conn.SetPongHandler(func(string) error {
		err := conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		if err != nil {
			return err
		}
		return nil
	})

	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for range t.C {
			_ = conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(5*time.Second))
		}
	}()

	for {
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			logger.Errorf("read:", err)
			return
		}

		if err := conn.WriteMessage(mt, msg); err != nil {
			logger.Errorf("write:", err)
			return
		}
	}
}

func postLogin(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"token": "abc123",
		"user": gin.H{
			"id":   1001,
			"name": "Alice",
		},
	})
}

func getUserInfo(c *gin.Context) {

	if c.Request.Header.Get("Authorization") == "" {
		c.JSON(http.StatusUnauthorized, gin.H{})
		return
	}

	c.JSON(http.StatusOK, gin.H{})
}

func main() {
	r := gin.Default()
	r.GET("/ws", wsEcho)
	r.POST("api/login", postLogin)
	r.POST("api/user", getUserInfo)
	err := r.Run(":8080")
	if err != nil {
		println(err.Error())
		return
	}

}
