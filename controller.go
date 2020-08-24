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
	this.Response.SetCookie(name, value, "/", maxAge, false, true)
}

func (this *WgoController) DelCookie(name string) {
	this.Response.DelCookie(name, "/")
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
	name := tool.MD5([]byte(htmlStr))
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
		Code int         `json:"code"`
		Data interface{} `json:"data,omitempty"`
	}{Code: 0, Data: body})
	return
}
func (this *WgoController) SuccessExtra(body interface{}, extra interface{}) (json []byte) {
	json = this.RenderJson(struct {
		Code  int        `json:"code"`
		Data  interface{} `json:"data,omitempty"`
		Extra interface{} `json:"extra,omitempty"`
	}{Code: 0, Data: body, Extra: extra})
	return
}

func (this *WgoController) Failure(code int, msg string) (json []byte) {
	json = this.RenderJson(struct {
		Code int    `json:"code"`
		Msg  string `json:"msg,omitempty"`
	}{Code: code, Msg: msg})
	return
}

func (this *WgoController) FailureExtra(code int, msg string, extra interface{}) (json []byte) {
	json = this.RenderJson(struct {
		Code  int         `json:"code"`
		Msg   string      `json:"msg,omitempty"`
		Extra interface{} `json:"extra,omitempty"`
	}{Code: code, Msg: msg, Extra: extra})
	return
}

func mergeShareDatas(dst interface{}, datas []map[string]interface{}) interface{} {
	n := len(datas)
	if n == 0 {
		return dst
	}

	if nil == dst {
		if n == 1 {
			return datas[0]
		}

		for _, m := range datas[1:] {
			for k, i := range m {
				datas[0][k] = i
			}
		}
		return datas[0]
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
