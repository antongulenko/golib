package golib

import (
	"net/http"
	"net/http/httputil"
	"sync"

	"github.com/gin-gonic/contrib/ginrus"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

var ginLogHandler = ginrus.Ginrus(log.StandardLogger(), "", false)
var initGinOnce sync.Once

func InitGin() {
	initGinOnce.Do(func() {
		gin.SetMode(gin.ReleaseMode)
	})
}

func NewGinEngine() *gin.Engine {
	InitGin()
	engine := gin.New()
	engine.Use(ginLogHandler, ginRecover)
	engine.NoRoute(func(c *gin.Context) {
		c.Writer.WriteHeader(http.StatusNotFound)
		c.Writer.WriteString("404 page not found\n")
	})
	return engine
}

func ginRecover(c *gin.Context) {
	defer func() {
		if err := recover(); err != nil {
			stack := stack(3)
			httpRequest, _ := httputil.DumpRequest(c.Request, false)
			log.Errorf("[Recovery] panic recovered:\n%s\n%s\n%s", string(httpRequest), err, stack)
			c.AbortWithStatus(500)
		}
	}()
	c.Next()
}
