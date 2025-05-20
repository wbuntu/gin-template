package model

import (
	"strings"

	"github.com/gin-gonic/gin"
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
	CodecXML   Codec = "xml"
	CodecProto Codec = "proto"
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
