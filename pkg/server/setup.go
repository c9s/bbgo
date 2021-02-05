package server

import (
	"context"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
)

func (s *Server) setupTestDB(c *gin.Context) {
	payload := struct {
		DSN string `json:"dsn"`
	}{}

	if err := c.BindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing arguments"})
		return
	}

	dsn := payload.DSN
	if len(dsn) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing dsn argument"})
		return
	}

	db, err := bbgo.ConnectMySQL(dsn)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := db.Close(); err != nil {
		logrus.WithError(err).Error("db connection close error")
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) setupConfigureDB(c *gin.Context) {
	payload := struct {
		DSN string `json:"dsn"`
	}{}

	if err := c.BindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing arguments"})
		return
	}

	dsn := payload.DSN
	if len(dsn) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing dsn argument"})
		return
	}

	if err := s.Environ.ConfigureDatabase(c, dsn); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) setupAddStrategy(c *gin.Context) {
	sessionName := c.Param("session")
	strategyID := c.Param("id")

	_, ok := s.Environ.Session(sessionName)
	if !ok {
		c.JSON(http.StatusNotFound, "session not found")
		return
	}

	var conf map[string]interface{}

	if err := c.BindJSON(&conf); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing arguments"})
		return
	}

	strategy, err := bbgo.NewStrategyFromMap(strategyID, conf)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	mount := bbgo.ExchangeStrategyMount{
		Mounts:   []string{sessionName},
		Strategy: strategy,
	}

	s.Config.ExchangeStrategies = append(s.Config.ExchangeStrategies, mount)

	c.JSON(http.StatusOK, gin.H{"success": true})
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

		logrus.Info("server shutdown completed")

		if s.Setup.BeforeRestart != nil {
			s.Setup.BeforeRestart()
		}

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

