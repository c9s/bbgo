package server

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

const DefaultBindAddress = "localhost:8080"

type Setup struct {
	// Context is the trader context
	Context context.Context

	// Cancel is the trader context cancel function you want to cancel
	Cancel context.CancelFunc

	// Token is used for setup api authentication
	Token string

	BeforeRestart func()
}

type Server struct {
	Config        *bbgo.Config
	Environ       *bbgo.Environment
	Trader        *bbgo.Trader
	Setup         *Setup
	OpenInBrowser bool

	srv *http.Server
}

func (s *Server) newEngine(ctx context.Context) *gin.Engine {
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowHeaders:     []string{"Origin", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
		AllowWebSockets:  true,
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.GET("/api/ping", s.ping)

	if s.Setup != nil {
		r.POST("/api/setup/test-db", s.setupTestDB)
		r.POST("/api/setup/configure-db", s.setupConfigureDB)
		r.POST("/api/setup/strategy/single/:id/session/:session", s.setupAddStrategy)
		r.POST("/api/setup/save", s.setupSaveConfig)
		r.POST("/api/setup/restart", s.setupRestart)
	}

	r.GET("/api/environment/syncing", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"syncing": s.Environ.IsSyncing(),
		})
	})

	r.POST("/api/environment/sync", func(c *gin.Context) {
		if s.Environ.IsSyncing() != bbgo.Syncing {
			go func() {
				// We use the root context here because the syncing operation is a background goroutine.
				// It should not be terminated if the request is disconnected.
				if err := s.Environ.Sync(ctx); err != nil {
					logrus.WithError(err).Error("sync error")
				}
			}()
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
		})
	})

	r.GET("/api/outbound-ip", func(c *gin.Context) {
		outboundIP, err := GetOutboundIP()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"outboundIP": outboundIP.String(),
		})
	})

	r.GET("/api/trades", func(c *gin.Context) {
		if s.Environ.TradeService == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database is not configured"})
			return
		}

		exchange := c.Query("exchange")
		symbol := c.Query("symbol")
		gidStr := c.DefaultQuery("gid", "0")
		lastGID, err := strconv.ParseInt(gidStr, 10, 64)
		if err != nil {
			logrus.WithError(err).Error("last gid parse error")
			c.Status(http.StatusBadRequest)
			return
		}

		trades, err := s.Environ.TradeService.Query(service.QueryTradesOptions{
			Exchange: types.ExchangeName(exchange),
			Symbol:   symbol,
			LastGID:  lastGID,
			Ordering: "DESC",
		})
		if err != nil {
			c.Status(http.StatusBadRequest)
			logrus.WithError(err).Error("order query error")
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"trades": trades,
		})
	})

	r.GET("/api/orders/closed", s.listClosedOrders)
	r.GET("/api/trading-volume", s.tradingVolume)

	r.POST("/api/sessions/test", func(c *gin.Context) {
		var session bbgo.ExchangeSession
		if err := c.BindJSON(&session); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		err := session.InitExchange(session.ExchangeName.String(), nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		var anyErr error
		_, openOrdersErr := session.Exchange.QueryOpenOrders(c, "BTCUSDT")
		if openOrdersErr != nil {
			anyErr = openOrdersErr
		}

		_, balanceErr := session.Exchange.QueryAccountBalances(c)
		if balanceErr != nil {
			anyErr = balanceErr
		}

		c.JSON(http.StatusOK, gin.H{
			"success":    anyErr == nil,
			"error":      anyErr,
			"balance":    balanceErr == nil,
			"openOrders": openOrdersErr == nil,
		})
	})

	r.GET("/api/sessions", func(c *gin.Context) {
		var sessions []*bbgo.ExchangeSession
		for _, session := range s.Environ.Sessions() {
			sessions = append(sessions, session)
		}

		if len(sessions) == 0 {
			c.JSON(http.StatusOK, gin.H{"sessions": []int{}})
		}

		c.JSON(http.StatusOK, gin.H{"sessions": sessions})
	})

	r.POST("/api/sessions", func(c *gin.Context) {
		var session bbgo.ExchangeSession
		if err := c.BindJSON(&session); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		if err := session.InitExchange(session.ExchangeName.String(), nil); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		if s.Config.Sessions == nil {
			s.Config.Sessions = make(map[string]*bbgo.ExchangeSession)
		}
		s.Config.Sessions[session.Name] = &session

		s.Environ.AddExchangeSession(session.Name, &session)

		if err := session.Init(c, s.Environ); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	r.GET("/api/assets", s.listAssets)
	r.GET("/api/sessions/:session", s.listSessions)
	r.GET("/api/sessions/:session/trades", s.listSessionTrades)
	r.GET("/api/sessions/:session/open-orders", s.listSessionOpenOrders)
	r.GET("/api/sessions/:session/account", s.getSessionAccount)
	r.GET("/api/sessions/:session/account/balances", s.getSessionAccountBalance)
	r.GET("/api/sessions/:session/symbols", s.listSessionSymbols)

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

	r.GET("/api/strategies/single", s.listStrategies)
	r.NoRoute(s.assetsHandler)
	return r
}

func (s *Server) RunWithListener(ctx context.Context, l net.Listener) error {
	r := s.newEngine(ctx)
	bind := l.Addr().String()

	if s.OpenInBrowser {
		openBrowser(ctx, bind)
	}

	s.srv = newServer(r, bind)
	return serve(s.srv, l)
}

func (s *Server) Run(ctx context.Context, bindArgs ...string) error {
	r := s.newEngine(ctx)
	bind := resolveBind(bindArgs)
	if s.OpenInBrowser {
		openBrowser(ctx, bind)
	}

	s.srv = newServer(r, bind)
	return listenAndServe(s.srv)
}

func (s *Server) ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "pong"})
}

func (s *Server) listClosedOrders(c *gin.Context) {
	if s.Environ.OrderService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database is not configured"})
		return
	}

	exchange := c.Query("exchange")
	symbol := c.Query("symbol")
	gidStr := c.DefaultQuery("gid", "0")

	lastGID, err := strconv.ParseInt(gidStr, 10, 64)
	if err != nil {
		logrus.WithError(err).Error("last gid parse error")
		c.Status(http.StatusBadRequest)
		return
	}

	orders, err := s.Environ.OrderService.Query(service.QueryOrdersOptions{
		Exchange: types.ExchangeName(exchange),
		Symbol:   symbol,
		LastGID:  lastGID,
		Ordering: "DESC",
	})
	if err != nil {
		c.Status(http.StatusBadRequest)
		logrus.WithError(err).Error("order query error")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"orders": orders,
	})
}

func (s *Server) listStrategies(c *gin.Context) {
	var stashes []map[string]interface{}

	for _, mount := range s.Config.ExchangeStrategies {
		stash, err := mount.Map()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		stash["strategy"] = mount.Strategy.ID()

		stashes = append(stashes, stash)
	}

	if len(stashes) == 0 {
		c.JSON(http.StatusOK, gin.H{"strategies": []int{}})
	}
	c.JSON(http.StatusOK, gin.H{"strategies": stashes})
}

func (s *Server) listSessions(c *gin.Context) {
	sessionName := c.Param("session")
	session, ok := s.Environ.Session(sessionName)

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("session %s not found", sessionName)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"session": session})
}

func (s *Server) listSessionSymbols(c *gin.Context) {
	sessionName := c.Param("session")
	session, ok := s.Environ.Session(sessionName)

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("session %s not found", sessionName)})
		return
	}

	var symbols []string
	for symbol := range session.Markets() {
		symbols = append(symbols, symbol)
	}

	c.JSON(http.StatusOK, gin.H{"symbols": symbols})
}

func (s *Server) listSessionTrades(c *gin.Context) {
	sessionName := c.Param("session")
	session, ok := s.Environ.Session(sessionName)

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("session %s not found", sessionName)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"trades": session.Trades})
}

func (s *Server) getSessionAccount(c *gin.Context) {
	sessionName := c.Param("session")
	session, ok := s.Environ.Session(sessionName)

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("session %s not found", sessionName)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"account": session.GetAccount()})
}

func (s *Server) getSessionAccountBalance(c *gin.Context) {
	sessionName := c.Param("session")
	session, ok := s.Environ.Session(sessionName)

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("session %s not found", sessionName)})
		return
	}

	if session.Account == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("the account of session %s is nil", sessionName)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"balances": session.GetAccount().Balances()})
}

func (s *Server) listSessionOpenOrders(c *gin.Context) {
	sessionName := c.Param("session")
	session, ok := s.Environ.Session(sessionName)

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("session %s not found", sessionName)})
		return
	}

	marketOrders := make(map[string][]types.Order)
	for symbol, orderStore := range session.OrderStores() {
		marketOrders[symbol] = orderStore.Orders()
	}

	c.JSON(http.StatusOK, gin.H{"orders": marketOrders})
}

func genFakeAssets() types.AssetMap {

	totalAssets := types.AssetMap{}
	balances := types.BalanceMap{
		"BTC":  types.Balance{Currency: "BTC", Available: fixedpoint.NewFromFloat(10.0 * rand.Float64())},
		"BCH":  types.Balance{Currency: "BCH", Available: fixedpoint.NewFromFloat(0.01 * rand.Float64())},
		"LTC":  types.Balance{Currency: "LTC", Available: fixedpoint.NewFromFloat(200.0 * rand.Float64())},
		"ETH":  types.Balance{Currency: "ETH", Available: fixedpoint.NewFromFloat(50.0 * rand.Float64())},
		"SAND": types.Balance{Currency: "SAND", Available: fixedpoint.NewFromFloat(11500.0 * rand.Float64())},
		"BNB":  types.Balance{Currency: "BNB", Available: fixedpoint.NewFromFloat(1000.0 * rand.Float64())},
		"GRT":  types.Balance{Currency: "GRT", Available: fixedpoint.NewFromFloat(1000.0 * rand.Float64())},
		"MAX":  types.Balance{Currency: "MAX", Available: fixedpoint.NewFromFloat(200000.0 * rand.Float64())},
		"COMP": types.Balance{Currency: "COMP", Available: fixedpoint.NewFromFloat(100.0 * rand.Float64())},
	}
	assets := balances.Assets(map[string]fixedpoint.Value{
		"BTCUSDT":  fixedpoint.NewFromFloat(38000.0),
		"BCHUSDT":  fixedpoint.NewFromFloat(478.0),
		"LTCUSDT":  fixedpoint.NewFromFloat(150.0),
		"COMPUSDT": fixedpoint.NewFromFloat(450.0),
		"ETHUSDT":  fixedpoint.NewFromFloat(1700.0),
		"BNBUSDT":  fixedpoint.NewFromFloat(70.0),
		"GRTUSDT":  fixedpoint.NewFromFloat(0.89),
		"DOTUSDT":  fixedpoint.NewFromFloat(20.0),
		"SANDUSDT": fixedpoint.NewFromFloat(0.13),
		"MAXUSDT":  fixedpoint.NewFromFloat(0.122),
	}, time.Now())
	for currency, asset := range assets {
		totalAssets[currency] = asset
	}

	return totalAssets
}

func (s *Server) listAssets(c *gin.Context) {
	if ok, err := strconv.ParseBool(os.Getenv("USE_FAKE_ASSETS")); err == nil && ok {
		c.JSON(http.StatusOK, gin.H{"assets": genFakeAssets()})
		return
	}

	totalAssets := types.AssetMap{}
	for _, session := range s.Environ.Sessions() {
		balances := session.GetAccount().Balances()

		if err := session.UpdatePrices(c, balances.Currencies(), "USDT"); err != nil {
			logrus.WithError(err).Error("price update failed")
			c.Status(http.StatusInternalServerError)
			return
		}

		assets := balances.Assets(session.LastPrices(), time.Now())

		for currency, asset := range assets {
			totalAssets[currency] = asset
		}
	}

	c.JSON(http.StatusOK, gin.H{"assets": totalAssets})
}

func (s *Server) setupSaveConfig(c *gin.Context) {
	if len(s.Config.Sessions) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session is not configured"})
		return
	}

	envVars, err := collectSessionEnvVars(s.Config.Sessions)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if s.Environ.DatabaseService != nil {
		envVars["DB_DRIVER"] = s.Environ.DatabaseService.Driver
		envVars["DB_DSN"] = s.Environ.DatabaseService.DSN
	}

	dotenvFile := ".env.local"
	if err := moveFileToBackup(dotenvFile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := godotenv.Write(envVars, dotenvFile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	out, err := s.Config.YAML()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	fmt.Println("config file")
	fmt.Println("=================================================")
	fmt.Println(string(out))
	fmt.Println("=================================================")

	filename := "bbgo.yaml"
	if err := moveFileToBackup(filename); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := ioutil.WriteFile(filename, out, 0666); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

var pageRoutePattern = regexp.MustCompile("/[a-z]+$")

func moveFileToBackup(filename string) error {
	stat, err := os.Stat(filename)

	if err == nil && stat != nil {
		err := os.Rename(filename, filename+"."+time.Now().Format("20060102_150405_07_00"))
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Server) tradingVolume(c *gin.Context) {
	if s.Environ.TradeService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database is not configured"})
		return
	}

	period := c.DefaultQuery("period", "day")
	segment := c.DefaultQuery("segment", "exchange")
	startTimeStr := c.Query("start-time")

	var startTime time.Time

	if startTimeStr != "" {
		v, err := time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			c.Status(http.StatusBadRequest)
			logrus.WithError(err).Error("start-time format incorrect")
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

	rows, err := s.Environ.TradeService.QueryTradingVolume(startTime, service.TradingVolumeQueryOptions{
		SegmentBy:     segment,
		GroupByPeriod: period,
	})
	if err != nil {
		logrus.WithError(err).Error("trading volume query error")
		c.Status(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, gin.H{"tradingVolumes": rows})
}

func newServer(r http.Handler, bind string) *http.Server {
	return &http.Server{
		Addr:    bind,
		Handler: r,
	}
}

func serve(srv *http.Server, l net.Listener) (err error) {
	defer func() {
		if err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Error("unexpected http server error")
		}
	}()

	err = srv.Serve(l)
	if err != http.ErrServerClosed {
		return err
	}

	return nil
}

func listenAndServe(srv *http.Server) error {
	var err error

	defer func() {
		if err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Error("unexpected http server error")
		}
	}()

	err = srv.ListenAndServe()
	if err != http.ErrServerClosed {
		return err
	}

	return nil
}

func GetOutboundIP() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP, nil
}
