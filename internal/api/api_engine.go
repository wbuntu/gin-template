package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"gitbub.com/wbuntu/gin-template/internal/api/middleware"
	"gitbub.com/wbuntu/gin-template/internal/model"
	"gitbub.com/wbuntu/gin-template/internal/pkg/config"
	"gitbub.com/wbuntu/gin-template/internal/pkg/log"
)

func (s *Server) setupEngine(ctx context.Context, c *config.Config) error {
	// 设置模式
	gin.SetMode(gin.DebugMode)
	// 初始化
	g := gin.New()
	// 启动 Context 回落，g 的 request context 不为空时，相关请求回落到这个 context
	g.ContextWithFallback = true
	// 配置中间件
	g.Use(
		gin.Recovery(),
	)
	// healthz && pprof && swagger && metrics
	addCommonRoutes(g)
	// controller
	addControllerRoute(g)
	// srv
	s.srv = &http.Server{
		Addr:    c.API.Addr,
		Handler: g,
	}
	// tlsSrv
	if len(c.API.TLSAddr) > 0 && len(c.API.TLSCrt) > 0 && len(c.API.TLSCrt) > 0 {
		s.tlsSrv = &http.Server{
			Addr:    c.API.TLSAddr,
			Handler: g,
		}
	}
	return nil
}

func addCommonRoutes(g *gin.Engine) {
	// readyz
	g.GET("/readyz", func(c *gin.Context) {
		c.JSON(http.StatusOK, model.RespSuccess)
	})
	// healthz
	g.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, model.RespSuccess)
	})
	// pprof
	pprof.Register(g)
	// swagger
	g.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, func(c *ginSwagger.Config) {
		c.DefaultModelsExpandDepth = 10
	}))
}

func addControllerRoute(g *gin.Engine) {
	v1 := g.Group("/api/v1.0")
	// 配置中间件
	v1.Use(
		middleware.HeaderExtracter(),
		middleware.RequestLogger(),
		middleware.Gzip(),
	)
	// 配置 routes
	funcMap := map[string]func(string, ...gin.HandlerFunc) gin.IRoutes{
		http.MethodGet:     v1.GET,
		http.MethodPost:    v1.POST,
		http.MethodDelete:  v1.DELETE,
		http.MethodPatch:   v1.PATCH,
		http.MethodPut:     v1.PUT,
		http.MethodOptions: v1.OPTIONS,
		http.MethodHead:    v1.HEAD,
		"ANY":              v1.Any,
	}
	routes := getRoutes()
	for i := range routes {
		v := routes[i]
		fn, ok := funcMap[v.Method]
		if !ok {
			continue
		}
		v.Middleware = append(v.Middleware, handlerFuncWrapper(v.Factory))
		fn(v.Path, v.Middleware...)
	}
}

func handlerFuncWrapper(factory func() model.Controller) gin.HandlerFunc {
	return func(g *gin.Context) {
		// 根据类型创建实例
		ctrlInstance := factory()
		// 初始化控制器
		ctrlInstance.Init()
		// 获取请求与响应的指针
		request := ctrlInstance.GetRequest()
		response := ctrlInstance.GetResponse()
		// 设置请求 ID
		requestID := g.GetString(middleware.GinCtxRequestID)
		request.SetRequestID(requestID)
		response.SetRequestID(requestID)
		// 非 request/response 模式的请求
		// 跳过自动化的 request 反序列化和 response 序列化，由 controller 直接处理请求，例如 Websocket、SSE、Proxy 等
		if ctrlInstance.Codec().Streaming() {
			ctrlInstance.Serve(g)
			return
		}
		// 根据请求方法选择binding
		var bindFn func(any) error
		// POST、PATCH、PUT：从 Body 中解析 request
		// 其他：从 Query 中解析 request ，参数值禁止嵌套，不允许使用数组、字典和结构体
		switch g.Request.Method {
		case http.MethodPost, http.MethodPatch, http.MethodPut:
			bindFn = g.ShouldBindJSON
		case http.MethodGet, http.MethodDelete, http.MethodOptions, http.MethodHead:
			bindFn = g.ShouldBindQuery
		}
		// 解析Body和Query 反序列化 + 参数检查
		if err := bindFn(request); err != nil {
			log.G(g).Errorf("unmarshal and validate request: %s", err)
			g.JSON(http.StatusOK, model.BaseResponse{
				Code:    model.CodeParamError,
				Message: fmt.Sprintf("param error: %s", err),
			})
			return
		}
		// 执行控制器逻辑
		ctrlInstance.Serve(g)
		// JSON格式响应
		g.JSON(http.StatusOK, response)
	}
}
