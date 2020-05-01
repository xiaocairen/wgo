package wgo

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	GET    = "GET"
	POST   = "POST"
	PUT    = "PUT"
	DELETE = "DELETE"
	PATCH  = "PATCH"
)

type HttpMethod interface {
	Get(path string, controller interface{}, action string)
	Post(path string, controller interface{}, action string)
	Put(path string, controller interface{}, action string)
	Delete(path string, controller interface{}, action string)
	Any(path string, controller interface{}, action string)
}

type UnitHttpMethod interface {
	Get(unit *RouteUnit)
	Post(unit *RouteUnit)
	Put(unit *RouteUnit)
	Delete(unit *RouteUnit)
	Any(unit *RouteUnit)
}

type HttpRequest struct {
	Request *http.Request
	query   url.Values
}

func (r *HttpRequest) init() {
	r.query = r.Request.URL.Query()
	if r.Request.Method == POST || r.Request.Method == PUT || r.Request.Method == PATCH {
		r.Request.ParseForm()
	}
}

func (r *HttpRequest) Get(key string) string {
	return r.query.Get(key)
}

func (r *HttpRequest) GetInt(key string) int {
	v := r.query.Get(key)
	if "" == v {
		return 0
	}
	i, e := strconv.Atoi(v)
	if e != nil {
		return 0
	}
	return i
}

func (r *HttpRequest) Post(key string) string {
	if nil == r.Request.PostForm {
		return ""
	}
	return r.Request.PostForm.Get(key)
}

func (r *HttpRequest) PostInt(key string) int {
	v := r.Post(key)
	if "" == v {
		return 0
	}
	i, e := strconv.Atoi(v)
	if e != nil {
		return 0
	}
	return i
}

func (r *HttpRequest) GetRequest(key string) string {
	v := r.Post(key)
	if "" == v {
		v = r.Get(key)
	}
	return v
}

func (r *HttpRequest) GetRequestInt(key string) int {
	i := r.PostInt(key)
	if 0 == i {
		i = r.GetInt(key)
	}
	return i
}

func (r *HttpRequest) SetCookie(name, value string, maxAge int, secure, httpOnly bool) {
	cookie := &http.Cookie{
		Name:       name,
		Value:      value,
		Path:       "/",
		Domain:     "",
		Expires:    time.Time{},
		RawExpires: "",
		MaxAge:     maxAge,
		Secure:     secure,
		HttpOnly:   httpOnly,
		SameSite:   0,
		Raw:        "",
		Unparsed:   nil,
	}
	r.Request.AddCookie(cookie)
}

func (r *HttpRequest) GetCookie(name string) string {
	c, e := r.Request.Cookie(name)
	if e == http.ErrNoCookie {
		return ""
	}
	return c.String()
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

func (r *HttpRequest) GetMethod() string {
	return r.Request.Method
}

func (r *HttpRequest) GetReferer() string {
	return r.Request.Header.Get("referer")
}

func (r *HttpRequest) GetHost() string {
	return r.Request.Host
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
	ResponseWriter http.ResponseWriter
	Body           [][]byte
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

func (r HttpResponse) Send(body []byte) {
	if nil != body && 0 == len(r.Body) {
		r.ResponseWriter.Write(body)
	} else {
		if nil != body {
			r.Body = append(r.Body, body)
		}

		switch len(r.Body) {
		case 0:
			r.ResponseWriter.Write([]byte("empty response body"))
		case 1:
			r.ResponseWriter.Write(r.Body[0])
		default:
			r.ResponseWriter.Write(bytes.Join(r.Body, []byte("")))
		}
	}
}

func (r HttpResponse) SendJson(body interface{}) {
	if j, e := json.Marshal(body); e != nil {
		r.ResponseWriter.Write([]byte(e.Error()))
	} else {
		r.ResponseWriter.Write(j)
	}
}

func (r *HttpResponse) Addheader(key string, value string) {
	r.ResponseWriter.Header().Add(key, value)
}

func (r *HttpResponse) SetHeader(key string, value string) {
	r.ResponseWriter.Header().Set(key, value)
}

func (r *HttpResponse) Write(data []byte) (int, error) {
	return r.ResponseWriter.Write(data)
}
