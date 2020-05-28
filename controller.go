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
	ShareData    []map[string]interface{}
}

func (this *WgoController) GetCookie(name string) string {
	c, e := this.Request.GetCookie(name)
	if e != nil {
		return ""
	}
	return c.Value
}

func (this *WgoController) SetCookie(name, value string, maxAge int) {
	this.Response.SetCookie(name, value, maxAge, false, true)
}

func (this *WgoController) DelCookie(name string) {
	this.Response.DelCookie(name)
}

func (this *WgoController) AppendBody(body []byte) *WgoController {
	this.Response.Append(body)
	return this
}

func (this *WgoController) AddShare(data map[string]interface{}) {
	this.ShareData = append(this.ShareData, data)
}

func (this *WgoController) Render(body []byte) []byte {
	return this.Response.Send(body)
}

func (this *WgoController) RenderJson(body interface{}) []byte {
	this.Response.SetHeader("content-type", "application/json")
	return this.Response.SendJson(body)
}

func (this *WgoController) RenderHtml(filename string, data interface{}) (string, interface{}) {
	return filename, mergeShareDatas(data, this.ShareData)
}

func (this *WgoController) RenderHtmlStr(htmlStr string, data interface{}) (*template.Template, interface{}) {
	name := tool.MD5(htmlStr)
	t := this.Template.Lookup(name)
	if nil == t {
		tpl, err := this.Template.New(name).Parse(htmlStr)
		if err != nil {
			log.Panic(err)
		}
		return tpl, mergeShareDatas(data, this.ShareData)
	} else {
		return t, mergeShareDatas(data, this.ShareData)
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

func mergeShareDatas(dst interface{}, datas []map[string]interface{}) interface{} {
	if len(datas) == 0 {
		return dst
	}
	si, ok := dst.(map[string]interface{})
	if !ok {
		return dst
	}

	for _, m := range datas {
		for k, i := range m {
			_, f := si[k]
			if !f {
				si[k] = i
			}
		}
	}
	return si
}
