package httputil

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Request struct {
	url     string
	body    string
	method  string
	headers map[string]string
	timeout int
}

type Response struct {
	HttpCode int
	Headers  map[string][]string
	Body     []byte
}

func NewRequest() *Request {
	return &Request{}
}

func (r *Request) AddHeader(key string, value string) {
	if r.headers == nil {
		r.headers = make(map[string]string)
	}
	r.headers[key] = value
}

func (r *Request) SetTimeout(t int) {
	r.timeout = t
}

func (r *Request) Get(url string) (*Response, error) {
	r.url = url
	r.method = "GET"

	return r.handleRequest()
}

func (r *Request) Post(url string, body map[string]string) (*Response, error) {
	r.url = url
	r.body = buildBody(body)
	r.method = "POST"
	r.AddHeader("Content-Type", "application/x-www-form-urlencoded")

	return r.handleRequest()
}

func (r *Request) handleRequest() (*Response, error) {
	tr := &http.Transport{DisableCompression: true}
	if r.url[:5] == "https" {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	defer tr.CloseIdleConnections()

	c := &http.Client{
		Transport: tr,
		Timeout:   time.Duration(r.timeout) * time.Second,
	}
	defer c.CloseIdleConnections()

	req, err := http.NewRequest(r.method, r.url, strings.NewReader(r.body))
	if err != nil {
		return nil, fmt.Errorf("new GET request err, %s", err.Error())
	}
	defer req.Body.Close()

	if len(r.headers) > 0 {
		for k, v := range r.headers {
			req.Header.Add(k, v)
		}
	}

	res, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send HTTP request err, %s", err.Error())
	}

	return r.handleResponse(res)
}

func (r *Request) handleResponse(res *http.Response) (*Response, error) {
	defer res.Body.Close()

	hr := &Response{HttpCode: res.StatusCode, Headers: res.Header}
	buf := make([]byte, 1024)
	for {
		n, e := res.Body.Read(buf)
		if n > 0 {
			hr.Body = append(hr.Body, buf[:n]...)
		}

		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, fmt.Errorf("read response body failure, %s", e.Error())
		}
	}

	return hr, nil
}

func (r *Response) GetHttpCode() int {
	return r.HttpCode
}

func (r *Response) GetHeaders() map[string][]string {
	return r.Headers
}

func (r *Response) GetHeader(key string) []string {
	if h, f := r.Headers[key]; f {
		return h
	}

	return nil
}

func (r *Response) GetBody() string {
	return string(r.Body)
}

func buildBody(body map[string]string) string {
	tmp := make([]string, len(body))
	i := 0
	for k, v := range body {
		tmp[i] = k + "=" + v
		i++
	}

	return strings.Join(tmp, "&")
}
