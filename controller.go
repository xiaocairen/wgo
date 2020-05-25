package wgo

import (
	"github.com/xiaocairen/wgo/config"
	"github.com/xiaocairen/wgo/service"
	"github.com/xiaocairen/wgo/tool"
	"html/template"
	"log"
)

type WgoController struct {
	Configurator *config.Configurator
	Router       Router
	Template     *template.Template
	Service      *service.Service
	Request      *HttpRequest
	Response     *HttpResponse
}

func (this *WgoController) SetCookie(name, value string) {
	this.Response.SetCookie(name, value, 0)
}

func (this *WgoController) RemoveCookie(name string) {
	this.Response.RemoveCookie(name)
}

func (this *WgoController) AppendBody(body []byte) *WgoController {
	this.Response.Append(body)
	return this
}

func (this *WgoController) Render(body []byte) []byte {
	return this.Response.Send(body)
}

func (this *WgoController) RenderJson(body interface{}) []byte {
	this.Response.SetHeader("content-type", "application/json")
	return this.Response.SendJson(body)
}

func (this *WgoController) RenderHtml(filename string, data interface{}) (string, interface{}) {
	return filename, data
}

func (this *WgoController) RenderHtmlStr(htmlStr string, data interface{}) (*template.Template, interface{}) {
	name := tool.MD5(htmlStr)
	t := this.Template.Lookup(name)
	if nil == t {
		tpl, err := this.Template.New(name).Parse(htmlStr)
		if err != nil {
			log.Panic(err)
		}
		return tpl, data
	} else {
		return t, data
	}
}

func (this *WgoController) Success(body interface{}) (json []byte) {
	json = this.RenderJson(struct {
		Success bool        `json:"success"`
		Data    interface{} `json:"data,omitempty"`
	}{Success: true, Data: body})
	return
}
func (this *WgoController) SuccessExtras(body interface{}, extras interface{}) (json []byte) {
	json = this.RenderJson(struct {
		Success bool        `json:"success"`
		Data    interface{} `json:"data,omitempty"`
		Extras  interface{} `json:"extras,omitempty"`
	}{Success: true, Data: body, Extras: extras})
	return
}

func (this *WgoController) Failure(msg string, code int) (json []byte) {
	json = this.RenderJson(struct {
		Success   bool   `json:"success"`
		ErrorCode int    `json:"error_code,omitempty"`
		ErrorMsg  string `json:"error_msg,omitempty"`
	}{Success: false, ErrorCode: code, ErrorMsg: msg})
	return
}

func (this *WgoController) FailureExtras(msg string, code int, extras interface{}) (json []byte) {
	json = this.RenderJson(struct {
		Success   bool        `json:"success"`
		ErrorCode int         `json:"error_code,omitempty"`
		ErrorMsg  string      `json:"error_msg,omitempty"`
		Extras    interface{} `json:"extras,omitempty"`
	}{Success: false, ErrorCode: code, ErrorMsg: msg, Extras: extras})
	return
}