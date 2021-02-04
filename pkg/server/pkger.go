package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/markbates/pkger"
)

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
