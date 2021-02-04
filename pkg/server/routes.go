package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/markbates/pkger"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
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
}

type Server struct {
	Config        *bbgo.Config
	Environ       *bbgo.Environment
	Trader        *bbgo.Trader
	Setup         *Setup
	OpenInBrowser bool

	srv *http.Server
}

func (s *Server) newEngine() *gin.Engine {
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
		var sessionConfig bbgo.ExchangeSession
		if err := c.BindJSON(&sessionConfig); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		session, err := bbgo.NewExchangeSessionFromConfig(sessionConfig.ExchangeName, &sessionConfig)
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
		var sessionConfig bbgo.ExchangeSession
		if err := c.BindJSON(&sessionConfig); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		session, err := bbgo.NewExchangeSessionFromConfig(sessionConfig.ExchangeName, &sessionConfig)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		if s.Config.Sessions == nil {
			s.Config.Sessions = make(map[string]*bbgo.ExchangeSession)
		}
		s.Config.Sessions[sessionConfig.Name] = session

		s.Environ.AddExchangeSession(sessionConfig.Name, session)

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
	r.NoRoute(s.pkgerHandler)

	return r
}

func (s *Server) RunWithListener(ctx context.Context, l net.Listener) error {
	r := s.newEngine()
	bind := l.Addr().String()

	if s.OpenInBrowser {
		openBrowser(ctx, bind)
	}

	s.srv = newServer(r, bind)
	return serve(s.srv, l)
}

func (s *Server) Run(ctx context.Context, bindArgs ...string) error {
	r := s.newEngine()
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

func (s *Server) setupRestart(c *gin.Context) {
	if s.srv == nil {
		logrus.Error("nil srv")
		return
	}

	go func() {
		logrus.Info("shutting down web server...")

		// The context is used to inform the server it has 5 seconds to finish
		// the request it is currently handling
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := s.srv.Shutdown(ctx); err != nil {
			logrus.WithError(err).Error("server forced to shutdown")
		}

		logrus.Info("web server shutdown completed")

		bin := os.Args[0]
		args := os.Args[0:]

		// filter out setup parameters
		args = filterStrings(args, "--setup")

		envVars := os.Environ()

		logrus.Infof("%s %v %+v", bin, args, envVars)

		if err := syscall.Exec(bin, args, envVars); err != nil {
			logrus.WithError(err).Errorf("failed to restart %s", bin)
		}

		s.Setup.Cancel()
	}()

	c.JSON(http.StatusOK, gin.H{"success": true})
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

	c.JSON(http.StatusOK, gin.H{"account": session.Account})
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

	c.JSON(http.StatusOK, gin.H{"balances": session.Account.Balances()})
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

func (s *Server) listAssets(c *gin.Context) {
	totalAssets := types.AssetMap{}

	for _, session := range s.Environ.Sessions() {
		balances := session.Account.Balances()

		if err := session.UpdatePrices(c); err != nil {
			logrus.WithError(err).Error("price update failed")
			c.Status(http.StatusInternalServerError)
			return
		}

		assets := balances.Assets(session.LastPrices())

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

	if len(s.Environ.MysqlURL) > 0 {
		envVars["MYSQL_URL"] = s.Environ.MysqlURL
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

func (s *Server) pkgerHandler(c *gin.Context) {
	fs := pkger.Dir("/frontend/out")

	// redirect to .html page if the page exists
	if pageRoutePattern.MatchString(c.Request.URL.Path) {

		_, err := pkger.Stat("/frontend/out/" + c.Request.URL.Path + ".html")
		if err == nil {
			c.Request.URL.Path += ".html"
		}
	}

	http.FileServer(fs).ServeHTTP(c.Writer, c.Request)
}

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

func collectSessionEnvVars(sessions map[string]*bbgo.ExchangeSession) (envVars map[string]string, err error) {
	envVars = make(map[string]string)

	for _, session := range sessions {
		if len(session.Key) == 0 && len(session.Secret) == 0 {
			err = fmt.Errorf("session %s key & secret is not empty", session.Name)
			return
		}

		if len(session.EnvVarPrefix) > 0 {
			envVars[session.EnvVarPrefix+"_API_KEY"] = session.Key
			envVars[session.EnvVarPrefix+"_API_SECRET"] = session.Secret
		} else if len(session.Name) > 0 {
			sn := strings.ToUpper(session.Name)
			envVars[sn+"_API_KEY"] = session.Key
			envVars[sn+"_API_SECRET"] = session.Secret
		} else {
			err = fmt.Errorf("session %s name or env var prefix is not defined", session.Name)
			return
		}

		// reset key and secret so that we won't marshal them to the config file
		session.Key = ""
		session.Secret = ""
	}

	return
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
	return
}

func getJSON(url string, data interface{}) error {
	var client = &http.Client{Timeout: 500 * time.Millisecond}
	r, err := client.Get(url)
	if err != nil {
		return err
	}

	defer r.Body.Close()

	return json.NewDecoder(r.Body).Decode(data)
}

func openURL(url string) error {
	cmd := exec.Command("open", url)
	return cmd.Start()
}

func pingUntil(ctx context.Context, baseURL string, callback func()) {
	pingURL := baseURL + "/api/ping"
	timeout := time.NewTimer(time.Minute)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {

		case <-timeout.C:
			logrus.Warnf("ping hits 1 minute timeout")
			return

		case <-ctx.Done():
			return

		case <-ticker.C:
			var response map[string]interface{}
			var err = getJSON(pingURL, &response)
			if err == nil {
				go callback()
				return
			}
		}
	}
}

func pingAndOpenURL(ctx context.Context, baseURL string) {
	setupURL := baseURL + "/setup"
	go pingUntil(ctx, baseURL, func() {
		if err := openURL(setupURL); err != nil {
			logrus.WithError(err).Errorf("can not call open command to open the web page")
		}
	})
}

func filterStrings(slice []string, needle string) (ns []string) {
	for _, str := range slice {
		if str == needle {
			continue
		}

		ns = append(ns, str)
	}

	return ns
}

func openBrowser(ctx context.Context, bind string) {
	if runtime.GOOS == "darwin" {
		baseURL := "http://" + bind
		go pingAndOpenURL(ctx, baseURL)
	} else {
		logrus.Warnf("%s is not supported for opening browser automatically", runtime.GOOS)
	}
}

func resolveBind(a []string) string {
	switch len(a) {
	case 0:
		return DefaultBindAddress

	case 1:
		return a[0]

	default:
		panic("too many parameters for binding")
	}

	return ""
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
