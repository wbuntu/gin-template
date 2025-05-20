package utils

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gitbub.com/wbuntu/gin-template/internal/pkg/log"
	"github.com/pkg/errors"
)

type HTTPContextKey string

const (
	CustomTimeoutKey      HTTPContextKey = "CustomTimeoutKey"
	CustomResolveKey      HTTPContextKey = "CustomResolveKey"
	SkipUnmarshalKey      HTTPContextKey = "SkipUnmarshalKey"
	ResponseStatusCodeKey HTTPContextKey = "ResponseStatusCodeKey"
)

// NewHTTPClient 根据提供的参数构建专用的HTTPClient
func NewHTTPClient(baseURL string) *HTTPClient {
	dialer := &customDialer{
		Dialer: &net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 15 * time.Second,
		},
	}
	return &HTTPClient{
		client: &http.Client{
			Transport: &http.Transport{
				// 忽略证书验证
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify:       true,
					PreferServerCipherSuites: true,
				},
				// 配置Dialer
				DialContext: dialer.DialContext,
				// 默认启用HTTP2
				ForceAttemptHTTP2:     true,
				MaxIdleConns:          100,
				MaxIdleConnsPerHost:   100,
				MaxConnsPerHost:       100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   5 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
		},
		baseURL: baseURL,
		// 重试次数默认为3
		retryCount: 3,
		// 重试间隔默认为200毫秒
		retryWait: time.Millisecond * 200,
		// 超时时长，默认为30秒
		timeout: time.Second * 30,
		// 自定义头部
		headers: map[string]string{},
		// 自定义域名解析器
		dialer: dialer,
		// 是否为 debug 模式
		debug: os.Getenv(DEBUG_HTTP) == "true",
	}
}

// HTTPClient 专用于调用接口的通用HTTP客户端
type HTTPClient struct {
	client               *http.Client
	baseURL              string
	retryCount           int
	retryWait            time.Duration
	timeout              time.Duration
	headers              map[string]string
	headersFn            func(context.Context) (map[string]string, error)
	responseStatusCodeFn func(context.Context, *http.Request) int
	responseUnmarshalFn  func(ctx context.Context, respData []byte, response Response) error
	responseCodeFn       func(ctx context.Context, response Response) error
	dialer               *customDialer
	debug                bool
}

// SetRequestHeadersFn 设置请求超时
func (c *HTTPClient) SetRequestTimeout(timeout time.Duration) {
	c.timeout = timeout
}

// AddRequestHeader 添加请求头
func (c *HTTPClient) AddRequestHeader(k, v string) {
	c.headers[k] = v
}

// SetRequestHeadersFn 设置生成请求头的函数，用于添加动态请求头
func (c *HTTPClient) SetRequestHeadersFn(fn func(ctx context.Context) (map[string]string, error)) {
	c.headersFn = fn
}

// SetResponseStatusCodeFn 设置 HTTP 层响应状态码生成函数，根据请求生成预期的响应状态码
func (c *HTTPClient) SetResponseStatusCodeFn(fn func(ctx context.Context, req *http.Request) int) {
	c.responseStatusCodeFn = fn
}

// SetResponseUnmarshalFn 设置应用层响应反序列化函数
func (c *HTTPClient) SetResponseUnmarshalFn(fn func(ctx context.Context, respData []byte, response Response) error) {
	c.responseUnmarshalFn = fn
}

// SetResponseCodeFn 设置应用层响应状态码检查函数
func (c *HTTPClient) SetResponseCodeFn(fn func(ctx context.Context, response Response) error) {
	c.responseCodeFn = fn
}

func (c *HTTPClient) SetHostMapping(m map[string]string) {
	c.dialer.setHostMapping(m)
}

func (c *HTTPClient) POST(ctx context.Context, path string, request Request, response Response) error {
	return c.do(ctx, http.MethodPost, path, request, response)
}

func (c *HTTPClient) PATCH(ctx context.Context, path string, request Request, response Response) error {
	return c.do(ctx, http.MethodPatch, path, request, response)
}

func (c *HTTPClient) PUT(ctx context.Context, path string, request Request, response Response) error {
	return c.do(ctx, http.MethodPut, path, request, response)
}

func (c *HTTPClient) DELETE(ctx context.Context, path string, request Request, response Response) error {
	return c.do(ctx, http.MethodDelete, path, request, response)
}

func (c *HTTPClient) GET(ctx context.Context, path string, request Request, response Response) error {
	return c.do(ctx, http.MethodGet, path, request, response)
}

func (c *HTTPClient) do(ctx context.Context, method string, path string, request Request, response Response) error {
	var reqUUID string
	if c.debug {
		reqUUID = UUID()
	}
	var (
		req  *http.Request
		resp *http.Response
		err  error
	)
	requestURL := c.baseURL + path
	// 设置请求超时时长
	timeout := c.timeout
	// 若有自定义超时，则使用自定义超时
	if customTimeout := ctx.Value(CustomTimeoutKey); customTimeout != nil {
		if v, ok := customTimeout.(time.Duration); ok {
			timeout = v
		}
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	// 序列化请求
	reqData, err := json.Marshal(request)
	if err != nil {
		return errors.Wrap(err, "marshal request")
	}
	switch method {
	case http.MethodPost, http.MethodPatch, http.MethodPut:
		// 参数放入Body
		req, err = http.NewRequestWithContext(
			reqCtx,
			method,
			requestURL,
			bytes.NewReader(reqData),
		)
	case http.MethodGet, http.MethodDelete, http.MethodOptions, http.MethodHead:
		// 参数放入URL 注意：只支持基本类型和数组，嵌套结构体不做展开
		// 做一次反序列化展开字段
		m := map[string]any{}
		if err := json.Unmarshal(reqData, &m); err != nil {
			return errors.Wrapf(err, "convert request")
		}
		if len(m) > 0 {
			// 对参数进行 URL 编码
			params := url.Values{}
			for k, v := range m {
				// 跳过空值
				if v == nil {
					continue
				}
				// 检查是否为数组类型
				if arr, ok := v.([]any); ok {
					// 数组类型，使用重复键名方式
					for _, item := range arr {
						if item != nil {
							params.Add(k, fmt.Sprintf("%v", item))
						}
					}
				} else {
					// 非数组类型，直接添加
					params.Add(k, fmt.Sprintf("%v", v))
				}
			}
			requestURL += "?" + params.Encode()
		}
		req, err = http.NewRequestWithContext(
			reqCtx,
			method,
			requestURL,
			nil,
		)
	default:
		return errors.Errorf("unsupported method: %s", method)
	}
	if err != nil {
		return errors.Wrap(err, "build http request")
	}
	// 配置默认头部
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	// 配置动态头部
	if c.headersFn != nil {
		headers, err := c.headersFn(reqCtx)
		if err != nil {
			return errors.Wrap(err, "exec headerFn")
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}
	// 配置请求相关的特定头部
	for k, v := range request.GetHeaders() {
		req.Header.Set(k, v)
	}
	// POST、PUT和 PATCH 请求需要设置Content-Type头部
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if c.debug {
		log.Infof("request %s: \n\n%s %s\n\n", reqUUID, method, requestURL)
		log.Infof("request %s: headers: \n\n%s\n\n", reqUUID, c.formatHeaders(req.Header))
		log.Infof("request %s: object: \n\n%s\n\n", reqUUID, string(reqData))
	}
	// HTTP请求出错时自动重试
	for range c.retryCount {
		resp, err = c.client.Do(req)
		// 请求成功，停止循环
		if err == nil {
			break
		}
		if c.debug {
			log.Infof("request %s: do: %s", reqUUID, err)
		}
		// 请求超时或取消，停止循环
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			break
		}
		// 若 body 不为空，则重置 body
		if req.Body != nil {
			if bodyReader, ok := req.Body.(io.Seeker); ok {
				bodyReader.Seek(0, io.SeekStart)
			}
		}
		// 休眠等待重试
		time.Sleep(c.retryWait)
	}
	if err != nil {
		return errors.Wrap(err, "send http request")
	}
	// 解析body
	defer resp.Body.Close()
	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "read http response")
	}
	if c.debug {
		log.Infof("response %s: headers: \n\n%s\n\n", reqUUID, c.formatHeaders(resp.Header))
		log.Infof("response %s: object: \n\n%s\n\n", reqUUID, string(respData))
	}
	// 检查响应状态码，默认情况下只认 HTTP 层 200 响应，优先检查自定义状态码，其次检查自定义状态码生成函数
	expectedStatusCode := http.StatusOK
	if v, ok := reqCtx.Value(ResponseStatusCodeKey).(int); ok {
		expectedStatusCode = v
	} else if c.responseStatusCodeFn != nil {
		expectedStatusCode = c.responseStatusCodeFn(reqCtx, req)
	}
	if resp.StatusCode != expectedStatusCode {
		errStr := fmt.Sprintf("%s: %s: status: %s:", req.Method, req.URL, resp.Status)
		if len(respData) > 0 {
			errStr += fmt.Sprintf(" msg: %s", string(respData))
		}
		return errors.New(errStr)
	}
	// 检查是否需要跳过反序列化，因为部分接口不返回 JSON 结构体
	if skipUnmarshal := reqCtx.Value(SkipUnmarshalKey); skipUnmarshal != nil {
		return nil
	}
	// 检查是否存在自定义反序列化函数，否则使用默认的 json 反序列化
	if c.responseUnmarshalFn != nil {
		if err := c.responseUnmarshalFn(reqCtx, respData, response); err != nil {
			return errors.Wrap(err, "exec responseUnmarshalFn")
		}
	} else {
		if err := json.Unmarshal(respData, response); err != nil {
			return errors.Wrapf(err, "unmarshal response: %s", string(respData))
		}
	}
	// 检查应用层返回码
	if c.responseCodeFn != nil {
		if err := c.responseCodeFn(reqCtx, response); err != nil {
			return errors.Wrap(err, "exec responseCodeFn")
		}
	} else {
		if response.GetCode() != CodeSuccess {
			return errors.Errorf("response: code: %s message: %s", response.GetCode(), response.GetMessage())
		}
	}
	return nil
}

func (c *HTTPClient) formatHeaders(header http.Header) string {
	var items []string
	for k, v := range header {
		items = append(items, fmt.Sprintf("%s: %s", k, strings.Join(v, ",")))
	}
	sort.Strings(items)
	return strings.Join(items, "\n")
}

type customDialer struct {
	*net.Dialer
	hostMapping sync.Map
}

func (d *customDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	// 检查是否需要自定义解析
	if customResolv := ctx.Value(CustomResolveKey); customResolv != nil {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, errors.Wrap(err, "split host and port")
		}
		// 检查是否存在自定义解析的IP地址
		if v, ok := d.hostMapping.Load(host); ok {
			newAddress := net.JoinHostPort(v.(string), port)
			// 使用自定义解析的IP地址连接
			return d.Dialer.DialContext(ctx, network, newAddress)
		}
	}
	// 使用标准拨号器连接
	return d.Dialer.DialContext(ctx, network, address)
}

func (d *customDialer) setHostMapping(m map[string]string) {
	for k, v := range m {
		d.hostMapping.Store(k, v)
	}
}

const (
	CodeSuccess = "Success"
	CodeError   = "Error"
)

type Request interface {
	GetHeaders() map[string]string
}

type Response interface {
	GetCode() string
	GetMessage() string
}

type BaseRequest struct {
}

func (r *BaseRequest) GetHeaders() map[string]string {
	return nil
}

type BaseResponse struct {
	Code    int    `json:"code"`
	Message string `json:"msg"`
}

func (r *BaseResponse) GetCode() string {
	if r.Code != 0 {
		return strconv.Itoa(r.Code)
	}
	return CodeSuccess
}

func (r *BaseResponse) GetMessage() string {
	return r.Message
}
