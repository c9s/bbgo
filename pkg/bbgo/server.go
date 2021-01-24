package bbgo

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/c9s/bbgo/pkg/types"
)

func RunServer(ctx context.Context, userConfig *Config, environ *Environment) error {
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})

	r.GET("/sessions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"sessions": userConfig.Sessions})
	})

	r.GET("/sessions/:session", func(c *gin.Context) {
		sessionName := c.Param("session")
		session, ok := environ.Session(sessionName)

		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("session %s not found", sessionName)})
			return
		}

		c.JSON(http.StatusOK, gin.H{"session": session})
	})

	r.GET("/sessions/:session/trades", func(c *gin.Context) {
		sessionName := c.Param("session")
		session, ok := environ.Session(sessionName)

		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("session %s not found", sessionName)})
			return
		}

		c.JSON(http.StatusOK, gin.H{"trades": session.Trades})
	})

	r.GET("/sessions/:session/open-orders", func(c *gin.Context) {
		sessionName := c.Param("session")
		session, ok := environ.Session(sessionName)

		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("session %s not found", sessionName)})
			return
		}

		marketOrders := make(map[string][]types.Order)
		for symbol, orderStore := range session.orderStores {
			marketOrders[symbol] = orderStore.Orders()
		}

		c.JSON(http.StatusOK, gin.H{"orders": marketOrders})
	})

	r.GET("/sessions/:session/account", func(c *gin.Context) {
		sessionName := c.Param("session")
		session, ok := environ.Session(sessionName)

		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("session %s not found", sessionName)})
			return
		}

		c.JSON(http.StatusOK, gin.H{"account": session.Account})
	})

	r.GET("/sessions/:session/account/balances", func(c *gin.Context) {
		sessionName := c.Param("session")
		session, ok := environ.Session(sessionName)

		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("session %s not found", sessionName)})
			return
		}

		if session.Account == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("the account of session %s is nil", sessionName)})
			return
		}

		c.JSON(http.StatusOK, gin.H{"balances": session.Account.Balances()})
	})

	r.GET("/sessions/:session/symbols", func(c *gin.Context) {

		sessionName := c.Param("session")
		session, ok := environ.Session(sessionName)

		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("session %s not found", sessionName)})
			return
		}

		var symbols []string
		for s := range session.loadedSymbols {
			symbols = append(symbols, s)
		}

		c.JSON(http.StatusOK, gin.H{"symbols": symbols})
	})

	r.GET("/sessions/:session/pnl", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	r.GET("/sessions/:session/market/:symbol/open-orders", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	r.GET("/sessions/:session/market/:symbol/trades", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	r.GET("/sessions/:session/market/:symbol/pnl", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	return r.Run() // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}
