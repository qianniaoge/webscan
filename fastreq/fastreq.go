package fastreq

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"wenscan/util"

	"github.com/pkg/errors"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpproxy"
)

type ReqOptions struct {
	// Req           *fasthttp.Request
	Timeout       time.Duration
	Retry         int    // 0为默认值，-1 代表关闭不retry
	VerifySSL     bool   // default false
	AllowRedirect bool   // default false
	Proxy         string // proxy settings, support http/https proxy only, e.g. http://127.0.0.1:8080
}

// 自定义一些函数
type Response struct {
	fasthttp.Response
	// raw text Response
	Text string
}

type session struct {
	ReqOptions
	client *fasthttp.Client
}

func getTextFromResp(r *fasthttp.Response) string {
	// TODO: 编码转换
	body := r.Body()
	if len(body) == 0 {
		return ""
	}
	return string(body)
}

func NewResponse(r *fasthttp.Response) *Response {
	return &Response{
		Response: *r,
		Text:     getTextFromResp(r),
	}
}

const DefaultUa = "Mozilla/5.0 (Windows NT 6.1; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko)" +
	" Chrome/76.0.3809.132 Safari/537.36 C845D9D38B3A68F4F74057DB542AD252 tx/2.0"

// 最大获取100K的响应，适用于绝大部分场景
const defaultResponseLength = 10240
const defaultRetry = 0
const defaultTimeout int = 15

func CopyConfReq(data interface{}, dstRequest *fasthttp.Request) error {
	req := http.Request{}
	var (
		err  error
		Data []byte
	)
	switch json := data.(type) {
	case map[string]interface{}:
		req.Method = json["method"].(string)
		req.URL, _ = url.Parse(json["url"].(string))
		postform := url.Values{}
		postvalues := strings.Split(json["data"].(string), "&")
		for _, value := range postvalues {
			k := strings.Split(value, "=")[0]
			v := strings.Split(value, "=")[1]
			postform[k] = []string{v}
		}
		req.PostForm = postform
		for k, v := range json["headers"].(map[string]interface{}) {
			req.Header.Set(k, v.(string))
		}
		Data, err = util.GetOriginalReqBody(&req)
		util.CopyRequest(&req, dstRequest, Data)
	}
	return err
}

func (sess *session) doRequest(verb string, url string, headers map[string]string, body []byte) (*Response, error) {
	var err error
	verb = strings.ToUpper(verb)
	//bodyReader := bytes.NewReader(body)
	req := fasthttp.AcquireRequest()
	//设置自定义header
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	// 设置默认的headers头
	defaultHeaders := map[string]string{
		"User-Agent": DefaultUa,
		"Range":      fmt.Sprintf("bytes=0-%d", defaultResponseLength),
		"Connection": "close",
	}
	for k, v := range defaultHeaders {
		if _, ok := headers[k]; !ok {
			req.Header.Set(k, v)
		}
	}
	// 设置默认的Content-Type头
	if verb == "POST" && headers["Content-Type"] == "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
		// 应该手动设置Referer、Origin、和X-Requested-With字段
	}
	// 设置post数据
	if verb == "POST" {
		req.SetBody(body)
	}

	// 覆盖Connection头
	req.Header.Set("Connection", "close")
	// 设置重试次数
	retry := sess.ReqOptions.Retry
	if retry == 0 {
		retry = defaultRetry
	} else if retry == -1 {
		retry = 0
	}
	// 请求
	resp := &fasthttp.Response{}

	for i := 0; i <= retry; i++ {
		if sess.AllowRedirect {
			err = sess.client.DoRedirects(req, resp, 5)
		} else {
			err = sess.client.DoTimeout(req, resp, sess.ReqOptions.Timeout)
		}
		if err != nil {
			time.Sleep(100 * time.Microsecond)
			continue
		} else {
			break
		}
	}
	if err != nil {
		return nil, errors.Wrap(err, "error occurred during request")
	}
	// 带Range头后一般webserver响应都是206 PARTIAL CONTENT，修正为200 OK
	if resp.StatusCode() == 206 {
		resp.SetStatusCode(200)
	}
	return NewResponse(resp), nil
}

// Get Session的GET请求
func (sess *session) Get(url string, headers map[string]string) (*Response, error) {
	return sess.doRequest("GET", url, headers, nil)
}

// Post Session的POST请求
func (sess *session) Post(url string, headers map[string]string, body []byte) (*Response, error) {
	return sess.doRequest("POST", url, headers, body)
}

// Request Session的自定义请求类型
func (sess *session) Request(verb string, url string, headers map[string]string, body []byte) (*Response, error) {
	return sess.doRequest(verb, url, headers, body)
}

// Get GET请求
func Get(url string, headers map[string]string, options *ReqOptions) (*Response, error) {
	sess := getSessionByOptions(options)
	return sess.doRequest("GET", url, headers, nil)
}

// Request 自定义请求类型
func Request(verb string, url string, headers map[string]string, body []byte, options *ReqOptions) (*Response, error) {
	sess := getSessionByOptions(options)
	return sess.doRequest(verb, url, headers, body)
}

// getSessionByOptions 根据配置获取一个session
func getSessionByOptions(options *ReqOptions) *session {
	client := &fasthttp.Client{}

	if options == nil {
		options = &ReqOptions{}
	}
	// 设置client的超时与ssl验证
	// timeout := time.Duration(options.Timeout) * time.Second
	// if options.Timeout == 0 {
	// 	timeout = time.Duration(defaultTimeout) * time.Second
	// }
	//设置证书
	client.TLSConfig = &tls.Config{InsecureSkipVerify: !options.VerifySSL}
	// //设置超时
	// client.MaxConnWaitTimeout = timeout
	//设置代理
	if options.Proxy != "" {
		client.Dial = fasthttpproxy.FasthttpSocksDialer(options.Proxy)
	}

	// 设置是否跟踪跳转
	// if !options.AllowRedirect {
	// 	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
	// 		return http.ErrUseLastResponse
	// 	}
	// }
	// 设置是否跟踪跳转
	// if !options.AllowRedirect {
	// 	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
	// 		return http.ErrUseLastResponse
	// 	}
	// }
	// options内容同步到session中
	return &session{
		ReqOptions: ReqOptions{
			options.Timeout,
			options.Retry,
			options.VerifySSL,
			options.AllowRedirect,
			options.Proxy,
		},
		client: client,
	}
}