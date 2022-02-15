//go:build web

package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) assetsHandler(c *gin.Context) {
	// redirect to .html page if the page exists
	if pageRoutePattern.MatchString(c.Request.URL.Path) {
		_, err := FS.Open(c.Request.URL.Path + ".html")
		if err == nil {
			c.Request.URL.Path += ".html"
		}
	}

	fs := http.FileServer(FS)
	fs.ServeHTTP(c.Writer, c.Request)
}
