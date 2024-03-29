package wgo

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

type HttpRequest struct {
	Request *http.Request
	query   url.Values
	body    []byte
}

func (r *HttpRequest) init() {
	r.query = r.Request.URL.Query()
	if r.Request.Method == POST || r.Request.Method == PUT || r.Request.Method == PATCH {
		r.Request.ParseForm()
	}
}

func (r *HttpRequest) Body() []byte {
	if nil != r.body {
		return r.body
	}

	var buf = make([]byte, 1024)
	for {
		n, e := r.Request.Body.Read(buf)
		if n > 0 {
			r.body = append(r.body, buf[:n]...)
		}
		if e != nil && e == io.EOF {
			break
		}
	}

	return r.body
}

func (r *HttpRequest) Get(key string) string {
	return r.query.Get(key)
}

func (r *HttpRequest) GetInt(key string) int64 {
	v := r.query.Get(key)
	if "" == v {
		return 0
	}
	i, e := strconv.ParseInt(v, 10, 64)
	if e != nil {
		return 0
	}
	return i
}

func (r *HttpRequest) GetFloat(key string) float64 {
	v := r.query.Get(key)
	if "" == v {
		return 0
	}
	f, e := strconv.ParseFloat(v, 64)
	if e != nil {
		return 0
	}
	return f
}

func (r *HttpRequest) GetSlice(key string) []string {
	return r.query[key]
}

func (r *HttpRequest) GetIntSlice(key string) []int64 {
	ret := make([]int64, len(r.query[key]))
	for k, s := range r.query[key] {
		if i, e := strconv.ParseInt(s, 10, 64); e == nil {
			ret[k] = i
		} else {
			ret[k] = 0
		}
	}
	return ret
}

func (r *HttpRequest) GetPost(key string) string {
	if nil == r.Request.PostForm {
		return ""
	}
	return r.Request.PostForm.Get(key)
}

func (r *HttpRequest) GetPostInt(key string) int64 {
	v := r.GetPost(key)
	if "" == v {
		return 0
	}
	i, e := strconv.ParseInt(v, 10, 64)
	if e != nil {
		return 0
	}
	return i
}

func (r *HttpRequest) GetPostFloat(key string) float64 {
	v := r.GetPost(key)
	if "" == v {
		return 0
	}
	f, e := strconv.ParseFloat(v, 64)
	if e != nil {
		return 0
	}
	return f
}

func (r *HttpRequest) GetPostSlice(key string) []string {
	if nil == r.Request.PostForm {
		return nil
	}
	return r.Request.PostForm[key]
}

func (r *HttpRequest) GetPostIntSlice(key string) []int64 {
	if nil == r.Request.PostForm {
		return nil
	}
	ret := make([]int64, len(r.Request.PostForm[key]))
	for k, s := range r.Request.PostForm[key] {
		if i, e := strconv.ParseInt(s, 10, 64); e == nil {
			ret[k] = i
		} else {
			ret[k] = 0
		}
	}
	return ret
}

func (r *HttpRequest) GetRequest(key string) string {
	v := r.GetPost(key)
	if "" == v {
		v = r.Get(key)
	}
	return v
}

func (r *HttpRequest) GetRequestInt(key string) int64 {
	i := r.GetPostInt(key)
	if 0 == i {
		i = r.GetInt(key)
	}
	return i
}

func (r *HttpRequest) GetCookie(name string) (*http.Cookie, error) {
	return r.Request.Cookie(name)
}

func (r *HttpRequest) GetCookies() []*http.Cookie {
	return r.Request.Cookies()
}

func (r *HttpRequest) GetHeader(key string) string {
	return r.Request.Header.Get(key)
}

func (r *HttpRequest) GetHeaders(key string) []string {
	return r.Request.Header.Values(key)
}

func (r *HttpRequest) Addheader(key string, value string) {
	r.Request.Header.Add(key, value)
}

func (r *HttpRequest) SetHeader(key string, value string) {
	r.Request.Header.Set(key, value)
}

func (r *HttpRequest) DelHeader(key string) {
	r.Request.Header.Del(key)
}

func (r *HttpRequest) GetHost() string {
	return r.Request.Host
}

func (r *HttpRequest) GetRequestURI() string {
	return r.Request.URL.RequestURI()
}

func (r *HttpRequest) GetRequestPath() string {
	return r.Request.URL.Path
}

func (r *HttpRequest) GetUrlRawQuery() string {
	return r.Request.URL.RawQuery
}

func (r *HttpRequest) GetUrlQuery() url.Values {
	return r.Request.URL.Query()
}

func (r *HttpRequest) GetMethod() string {
	return r.Request.Method
}

func (r *HttpRequest) GetReferer() string {
	return r.Request.Header.Get("referer")
}

func (r *HttpRequest) GetRemoteAddr() string {
	return r.Request.RemoteAddr
}

func (r *HttpRequest) IsPost() bool {
	return POST == r.Request.Method
}

func (r *HttpRequest) IsAjax() bool {
	return "XMLHttpRequest" == r.Request.Header.Get("X-Requested-With")
}

// wrape the http.ResponseWriter
// use HttpResponse.Append([]byte("hello world")).send()
// or HttpResponse.send([]byte("hello world"))
type HttpResponse struct {
	Writer     http.ResponseWriter
	Body       [][]byte
	statusCode int
}

func (r *HttpResponse) SetCookie(name, value, path string, maxAge int, secure, httpOnly bool) {
	c := http.Cookie{Name: name, Value: value, Path: path, MaxAge: maxAge, Secure: secure, HttpOnly: httpOnly}
	r.Writer.Header().Set("Set-Cookie", c.String())
}

func (r *HttpResponse) AddCookie(name, value, path string, maxAge int, secure, httpOnly bool) {
	c := http.Cookie{Name: name, Value: value, Path: path, MaxAge: maxAge, Secure: secure, HttpOnly: httpOnly}
	r.Writer.Header().Add("Set-Cookie", c.String())
}

func (r *HttpResponse) DelCookie(name string, path string) {
	c := http.Cookie{Name: name, Value: "", Path: path, MaxAge: -1}
	r.Writer.Header().Set("Set-Cookie", c.String())
}

func (r *HttpResponse) Append(body []byte) *HttpResponse {
	r.Body = append(r.Body, body)
	return r
}

func (r *HttpResponse) GetBody() [][]byte {
	return r.Body
}

func (r *HttpResponse) HasBody() bool {
	return len(r.Body) > 0
}

func (r HttpResponse) Send(body []byte) []byte {
	if nil != body && 0 == len(r.Body) {
		return body
	}
	if nil != body {
		r.Body = append(r.Body, body)
	}

	switch len(r.Body) {
	case 0:
		return []byte("empty response body")
	case 1:
		return r.Body[0]
	default:
		return bytes.Join(r.Body, []byte(""))
	}
}

func (r HttpResponse) SendJson(body any) []byte {
	j, e := json.Marshal(body)
	if e != nil {
		return []byte(e.Error())
	}
	return j
}

func (r *HttpResponse) Addheader(key string, value string) {
	r.Writer.Header().Add(key, value)
}

func (r *HttpResponse) SetHeader(key string, value string) {
	r.Writer.Header().Set(key, value)
}

func (r *HttpResponse) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.Writer.WriteHeader(statusCode)
}
