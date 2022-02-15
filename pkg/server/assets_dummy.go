//go:build !web

package server

import (
	"github.com/gin-gonic/gin"
)

func (s *Server) assetsHandler(c *gin.Context) {}
