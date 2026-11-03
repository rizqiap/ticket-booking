package docs

import (
	"embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed index.html openapi.yaml
var files embed.FS

// RegisterRoutes serves Swagger UI and the embedded API contract.
func RegisterRoutes(r *gin.Engine) {
	r.GET("/swagger/*filepath", gin.WrapH(http.StripPrefix("/swagger/", http.FileServer(http.FS(files)))))
}
