package golib

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"sync"

	"github.com/gin-gonic/contrib/ginrus"
	"github.com/gin-gonic/gin"
)

var ginLogHandler = ginrus.Ginrus(Log, "", false)
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
			Log.Errorf("[Recovery] panic recovered:\n%s\n%s\n%s", string(httpRequest), err, stack)
			c.AbortWithStatus(500)
		}
	}()
	c.Next()
}

type GinTask struct {
	*gin.Engine
	Endpoint     string
	ShutdownHook func()

	server      *http.Server
	c           StopChan
	shutdownErr error
}

func NewGinTask(endpoint string) *GinTask {
	return &GinTask{
		Engine:   NewGinEngine(),
		Endpoint: endpoint,
	}
}

func (task *GinTask) Start(wg *sync.WaitGroup) StopChan {
	task.c = NewStopChan()
	if wg != nil {
		wg.Add(1)
	}
	go func() {
		if wg != nil {
			defer wg.Done()
		}
		task.server = &http.Server{Addr: task.Endpoint, Handler: task.Engine}
		Log.Infoln("Starting", task)
		err := task.server.ListenAndServe()
		if hook := task.ShutdownHook; hook != nil {
			hook()
		}
		if err == http.ErrServerClosed {
			err = nil
		}
		if task.shutdownErr != nil {
			if err == nil {
				err = task.shutdownErr
			} else {
				err = MultiError([]error{task.shutdownErr, err})
			}
		}
		task.c.StopErr(err)
	}()
	return task.c
}

func (task *GinTask) Stop() {
	server := task.server
	if server != nil {
		Log.Infoln("Shutting down", task)
		task.shutdownErr = server.Shutdown(context.Background())
	}
}

func (task *GinTask) String() string {
	return fmt.Sprintf("HTTP server on " + task.Endpoint)
}
