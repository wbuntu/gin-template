package model

import (
	"net/http"
	"strings"

	"gitbub.com/wbuntu/gin-template/internal/pkg/log"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/pkg/errors"
)

type Route struct {
	Method     string
	Path       string
	Middleware []gin.HandlerFunc // 中间件
	Factory    func() Controller // 工厂函数代替反射，生成控制器的效率更高
}

type Codec string

const (
	CodecJSON  Codec = "json"
	CodecProto Codec = "proto"
	CodecYaml  Codec = "yaml"
	CodecWS    Codec = "ws"
)

func (c Codec) Streaming() bool {
	switch c {
	case CodecWS:
		return true
	default:
		return false
	}
}

func (c Codec) BindRequest(g *gin.Context, req Request) error {
	switch c {
	case CodecJSON:
		return c.bindJSON(g, req)
	case CodecProto:
		return g.ShouldBindWith(req, binding.ProtoBuf)
	case CodecYaml:
		return g.ShouldBindYAML(req)
	default:
		return errors.Errorf("unsupported codec: %s", c)
	}
}

func (c Codec) bindJSON(g *gin.Context, req Request) error {
	// 使用 json 同时处理 body 和 query 参数
	// 先根据请求方法选择binding
	// POST、PATCH、PUT：从 Body 中解析 request
	// 其他：从 Query 中解析 request ，参数值禁止嵌套，不允许使用数组、字典和结构体
	switch g.Request.Method {
	case http.MethodPost, http.MethodPatch, http.MethodPut:
		return g.ShouldBindJSON(req)
	case http.MethodGet, http.MethodDelete, http.MethodOptions, http.MethodHead:
		// 移植 binding.Query 的逻辑，支持 json 的 tag 解析
		values := g.Request.URL.Query()
		if err := binding.MapFormWithTag(req, values, "json"); err != nil {
			return err
		}
		if binding.Validator != nil {
			return binding.Validator.ValidateStruct(req)
		}
	}
	return nil
}

func (c Codec) RenderResponse(g *gin.Context, resp Response) {
	switch c {
	case CodecJSON:
		g.JSON(http.StatusOK, resp)
	case CodecProto:
		g.ProtoBuf(http.StatusOK, resp)
	case CodecYaml:
		g.YAML(http.StatusOK, resp)
	default:
		log.G(g).Errorf("unsupported codec: %s", c)
	}
}

type Controller interface {
	Init()                 // 初始化
	Codec() Codec          // 内容编码
	Serve(*gin.Context)    // 控制器逻辑
	GetRequest() Request   // 获取请求
	GetResponse() Response // 获取响应
}

type BaseController[I any, O any] struct {
	Request     I
	Response    O
	requestPtr  Request
	responsePtr Response
}

func (c *BaseController[I, O]) Init() {
	// 类型转换与初始化
	c.requestPtr = any(&c.Request).(Request)
	c.requestPtr.Init()
	c.responsePtr = any(&c.Response).(Response)
	c.responsePtr.Init()
}

func (c *BaseController[I, O]) Codec() Codec {
	return CodecJSON
}

func (c *BaseController[I, O]) Serve(g *gin.Context) {
	panic("Serve Method must be implemented")
}

func (c *BaseController[I, O]) GetRequest() Request {
	return c.requestPtr
}
func (c *BaseController[I, O]) GetResponse() Response {
	return c.responsePtr
}

type Request interface {
	Init()
	SetRequestID(string)
	GetRequestID() string
}

type Response interface {
	Init()
	GetRequestID() string
	SetRequestID(string)
}

type BaseRequest struct {
	RequestID string `json:"-"` // 请求ID
}

func (r *BaseRequest) GetHeaders() map[string]string {
	return nil
}

func (r *BaseRequest) Init() {}

func (r *BaseRequest) SetRequestID(item string) {
	r.RequestID = item
}

func (r *BaseRequest) GetRequestID() string {
	return r.RequestID
}

type BaseResponse struct {
	Code      Code   `json:"code" example:"Success"`                                             // 响应码
	Message   string `json:"message" example:"调用成功"`                                             // 响应消息
	RequestID string `json:"requestId,omitempty" example:"6893b1e9-da8f-4c6c-a161-eba4b81ea5b3"` // 请求ID
}

func (r *BaseResponse) Init() {
	r.Code = RespSuccess.Code
	r.Message = RespSuccess.Message
}

func (r *BaseResponse) GetCode() string {
	return string(r.Code)
}

func (r *BaseResponse) GetMessage() string {
	return r.Message
}

func (r *BaseResponse) GetRequestID() string {
	return r.RequestID
}

func (r *BaseResponse) SetRequestID(item string) {
	r.RequestID = item
}

// Update 更新response的code并设置默认message，若传入的messages不为空，使用:分隔拼接附加到response的message中
func (r *BaseResponse) Update(code Code, messages ...string) {
	r.Code = code
	r.Message = code.Message()
	if len(messages) > 0 {
		r.Message += ": " + strings.Join(messages, ": ")
	}
}
