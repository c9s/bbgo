package server

import (
	"net/http"

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

