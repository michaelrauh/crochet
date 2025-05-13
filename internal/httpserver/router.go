package httpserver

import (
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func NewRouter(serviceName string) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	r.Use(otelgin.Middleware(serviceName))

	return r
}
