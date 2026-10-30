package dashboard

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine) {
	dashboard := r.Group("/dashboard")
	{
		dashboard.GET("/", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "Welcome to the Jutec Dashboard",
			})
		})
	}
}
