package api

import (
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

func (s *Server) setupEngine(cfg *config.Config) error {
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
	s.addCommonRoutes(g)
	// controller
	s.addControllerRoute(g)
	// srv
	s.srv = &http.Server{
		Addr:    cfg.API.Addr,
		Handler: g,
	}
	// tlsSrv
	if len(cfg.API.TLSAddr) > 0 && len(cfg.API.TLSCrt) > 0 && len(cfg.API.TLSCrt) > 0 {
		s.tlsSrv = &http.Server{
			Addr:    cfg.API.TLSAddr,
			Handler: g,
		}
	}
	return nil
}

func (s *Server) addCommonRoutes(g *gin.Engine) {
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

func (s *Server) addControllerRoute(g *gin.Engine) {
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
		// 反序列化请求
		if err := ctrlInstance.Codec().BindRequest(g, request); err != nil {
			log.G(g).Errorf("unmarshal and validate request: %s", err)
			g.JSON(http.StatusOK, model.BaseResponse{
				Code:    model.CodeParamError,
				Message: fmt.Sprintf("param error: %s", err),
			})
			return
		}
		// 执行控制器逻辑
		ctrlInstance.Serve(g)
		// 序列化响应
		ctrlInstance.Codec().RenderResponse(g, response)
	}
}
