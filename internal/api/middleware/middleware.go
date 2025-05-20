package middleware

import (
	"net/http"
	"time"

	"gitbub.com/wbuntu/gin-template/internal/pkg/log"
	"gitbub.com/wbuntu/gin-template/internal/pkg/utils"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

const (
	// 请求ID
	GinCtxRequestID = "X-Request-Id"
)

func HeaderExtracter() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 请求ID
		requestID := c.GetHeader(GinCtxRequestID)
		if len(requestID) == 0 {
			requestID = utils.UUID()
		}
		c.Set(GinCtxRequestID, requestID)
	}
}

func RequestLogger() gin.HandlerFunc {
	return func(g *gin.Context) {
		// other handler can change c.Path so:
		path := g.Request.URL.Path
		start := time.Now()
		// 初始化 logger 并保存到请求上下文中
		ctx := log.S(g.Request.Context(), log.WithFields(log.Fields{
			"module":    "api",
			"requestID": g.GetString(GinCtxRequestID),
		}))
		g.Request = g.Request.WithContext(ctx)
		// excute next
		g.Next()
		// 从请求上下文中获取 logger
		logger := log.G(g).WithFields(log.Fields{
			"latency":    time.Since(start), // time to process
			"clientIP":   g.ClientIP(),
			"method":     g.Request.Method,
			"dataLength": g.Writer.Size(),
		})
		// log request
		if len(g.Errors) > 0 {
			logger.Error(g.Errors.ByType(gin.ErrorTypePrivate).String())
			return
		}
		statusCode := g.Writer.Status()
		if statusCode >= http.StatusInternalServerError {
			logger.Error(path)
		} else if statusCode >= http.StatusBadRequest {
			logger.Warn(path)
		} else {
			logger.Info(path)
		}
	}
}

func Gzip() gin.HandlerFunc {
	return gzip.Gzip(gzip.DefaultCompression)
}
