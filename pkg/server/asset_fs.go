// +build web

package server

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed "out/*" "out/_next/*"
var embededFiles embed.FS

func (s *Server) assetsHandler(c *gin.Context) {

	fsys, err := fs.Sub(embededFiles, "out")
	if err != nil {
		c.JSON(404, gin.H{"code": "PAGE_NOT_FOUND", "message": "Page not found"})
	}

	wfs := http.FileServer(http.FS(fsys))
	wfs.ServeHTTP(c.Writer, c.Request)
}
