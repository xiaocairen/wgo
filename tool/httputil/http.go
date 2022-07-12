package httputil

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
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
	Headers  http.Header
	Body     []byte
}

func NewRequest() *Request {
	return &Request{
		headers: make(map[string]string),
		timeout: 45,
	}
}

func (r *Request) AddHeader(key string, value string) {
	if _, f := r.headers[key]; !f {
		r.headers[key] = value
	}
}

func (r *Request) SetHeader(key string, value string) {
	r.headers[key] = value
}

func (r *Request) SetHeaders(h map[string]string) {
	for k, v := range h {
		r.headers[k] = v
	}
}

func (r *Request) SetTimeout(t int) {
	r.timeout = t
}

func (r *Request) Get(url string) (*Response, error) {
	r.url = url
	r.method = "GET"

	return r.handleRequest()
}

func (r *Request) Post(url, body string) (*Response, error) {
	r.method = "POST"
	r.url = url
	r.body = body

	return r.handleRequest()
}

func (r *Request) PostJSON(url string, data interface{}) (*Response, error) {
	r.method = "POST"
	r.url = url

	var (
		buf = bytes.NewBuffer([]byte{})
		enc = json.NewEncoder(buf)
	)
	enc.SetEscapeHTML(false)
	if e := enc.Encode(data); e != nil {
		return nil, e
	}

	r.body = buf.String()
	r.SetHeader("Content-Type", "application/json")

	return r.handleRequest()
}

func (r *Request) PostForm(url string, body map[string]string) (*Response, error) {
	r.method = "POST"
	r.url = url
	r.body = buildBody(body)
	r.SetHeader("Content-Type", "application/x-www-form-urlencoded")

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
		return nil, err
	}
	defer req.Body.Close()

	if len(r.headers) > 0 {
		for k, v := range r.headers {
			req.Header.Set(k, v)
		}
	}

	res, err := c.Do(req)
	if err != nil {
		return nil, err
	}

	return r.handleResponse(res)
}

func (r *Request) handleResponse(res *http.Response) (*Response, error) {
	defer res.Body.Close()

	var (
		response = &Response{HttpCode: res.StatusCode, Headers: res.Header}
		buf      = make([]byte, 1024)
	)
	for {
		n, e := res.Body.Read(buf)
		if n > 0 {
			response.Body = append(response.Body, buf[:n]...)
		}

		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, e
		}
	}

	return response, nil
}

func (r *Response) GetHttpCode() int {
	return r.HttpCode
}

func (r *Response) GetHeaders() map[string][]string {
	return r.Headers
}

func (r *Response) GetHeader(key string) string {
	return r.Headers.Get(key)
}

func (r *Response) GetBody() string {
	return string(r.Body)
}

func buildBody(body map[string]string) string {
	var (
		tmp = make([]string, len(body))
		i   = 0
	)
	for k, v := range body {
		tmp[i] = k + "=" + v
		i++
	}

	return strings.Join(tmp, "&")
}
