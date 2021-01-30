package bbgo

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/markbates/pkger"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

func RunServer(ctx context.Context, userConfig *Config, environ *Environment) error {
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowHeaders:     []string{"Origin"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.GET("/api/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})

	r.GET("/api/trades", func(c *gin.Context) {
		exchange := c.Query("exchange")
		symbol := c.Query("symbol")
		gidStr := c.DefaultQuery("gid", "0")
		lastGID, err := strconv.ParseInt(gidStr, 10, 64)
		if err != nil {
			log.WithError(err).Error("last gid parse error")
			c.Status(http.StatusBadRequest)
			return
		}

		trades, err := environ.TradeService.Query(service.QueryTradesOptions{
			Exchange: types.ExchangeName(exchange),
			Symbol:   symbol,
			LastGID:  lastGID,
			Ordering: "DESC",
		})
		if err != nil {
			c.Status(http.StatusBadRequest)
			log.WithError(err).Error("order query error")
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"trades": trades,
		})
	})

	r.GET("/api/orders/closed", func(c *gin.Context) {
		exchange := c.Query("exchange")
		symbol := c.Query("symbol")
		gidStr := c.DefaultQuery("gid", "0")

		lastGID, err := strconv.ParseInt(gidStr, 10, 64)
		if err != nil {
			log.WithError(err).Error("last gid parse error")
			c.Status(http.StatusBadRequest)
			return
		}

		orders, err := environ.OrderService.Query(service.QueryOrdersOptions{
			Exchange: types.ExchangeName(exchange),
			Symbol:   symbol,
			LastGID:  lastGID,
			Ordering: "DESC",
		})
		if err != nil {
			c.Status(http.StatusBadRequest)
			log.WithError(err).Error("order query error")
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"orders": orders,
		})
	})

	r.GET("/api/trading-volume", func(c *gin.Context) {
		period := c.DefaultQuery("period", "day")
		segment := c.DefaultQuery("segment", "exchange")
		startTimeStr := c.Query("start-time")

		var startTime time.Time

		if startTimeStr != "" {
			v, err := time.Parse(time.RFC3339, startTimeStr)
			if err != nil {
				c.Status(http.StatusBadRequest)
				log.WithError(err).Error("start-time format incorrect")
				return
			}
			startTime = v

		} else {
			switch period {
			case "day":
				startTime = time.Now().AddDate(0, 0, -30)

			case "month":
				startTime = time.Now().AddDate(0, -6, 0)

			case "year":
				startTime = time.Now().AddDate(-2, 0, 0)

			default:
				startTime = time.Now().AddDate(0, 0, -7)

			}
		}

		rows, err := environ.TradeService.QueryTradingVolume(startTime, service.TradingVolumeQueryOptions{
			SegmentBy:     segment,
			GroupByPeriod: period,
		})
		if err != nil {
			log.WithError(err).Error("trading volume query error")
			c.Status(http.StatusInternalServerError)
			return
		}

		c.JSON(http.StatusOK, gin.H{"tradingVolumes": rows})
		return
	})

	r.GET("/api/sessions", func(c *gin.Context) {
		var sessions []*ExchangeSession
		for _, session := range environ.Sessions() {
			sessions = append(sessions, session)
		}

		c.JSON(http.StatusOK, gin.H{"sessions": sessions})
	})

	r.GET("/api/assets", func(c *gin.Context) {
		totalAssets := types.AssetMap{}

		for _, session := range environ.sessions {
			balances := session.Account.Balances()

			if err := session.UpdatePrices(ctx); err != nil {
				log.WithError(err).Error("price update failed")
				c.Status(http.StatusInternalServerError)
				return
			}

			assets := balances.Assets(session.lastPrices)

			for currency, asset := range assets {
				totalAssets[currency] = asset
			}
		}

		c.JSON(http.StatusOK, gin.H{"assets": totalAssets})
	})

	r.GET("/api/sessions/:session", func(c *gin.Context) {
		sessionName := c.Param("session")
		session, ok := environ.Session(sessionName)

		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("session %s not found", sessionName)})
			return
		}

		c.JSON(http.StatusOK, gin.H{"session": session})
	})

	r.GET("/api/sessions/:session/trades", func(c *gin.Context) {
		sessionName := c.Param("session")
		session, ok := environ.Session(sessionName)

		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("session %s not found", sessionName)})
			return
		}

		c.JSON(http.StatusOK, gin.H{"trades": session.Trades})
	})

	r.GET("/api/sessions/:session/open-orders", func(c *gin.Context) {
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

	r.GET("/api/sessions/:session/account", func(c *gin.Context) {
		sessionName := c.Param("session")
		session, ok := environ.Session(sessionName)

		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("session %s not found", sessionName)})
			return
		}

		c.JSON(http.StatusOK, gin.H{"account": session.Account})
	})

	r.GET("/api/sessions/:session/account/balances", func(c *gin.Context) {
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

	r.GET("/api/sessions/:session/symbols", func(c *gin.Context) {

		sessionName := c.Param("session")
		session, ok := environ.Session(sessionName)

		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("session %s not found", sessionName)})
			return
		}

		var symbols []string
		for s := range session.usedSymbols {
			symbols = append(symbols, s)
		}

		c.JSON(http.StatusOK, gin.H{"symbols": symbols})
	})

	r.GET("/api/sessions/:session/pnl", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	r.GET("/api/sessions/:session/market/:symbol/open-orders", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	r.GET("/api/sessions/:session/market/:symbol/trades", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	r.GET("/api/sessions/:session/market/:symbol/pnl", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	fs := pkger.Dir("/frontend/out")
	r.NoRoute(func(c *gin.Context) {
		http.FileServer(fs).ServeHTTP(c.Writer, c.Request)
	})

	return r.Run() // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}
