package httpserver

import (
	"github.com/gin-gonic/gin"
)

func NewRouter() *gin.Engine {
	return gin.Default()
}
