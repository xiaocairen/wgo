package wgo

import (
	"github.com/xiaocairen/wgo/config"
	"github.com/xiaocairen/wgo/service"
	"github.com/xiaocairen/wgo/tool"
	"html/template"
	"log"
	"net/http"
)

type WgoController struct {
	Configurator *config.Configurator
	Router       Router
	Template     *template.Template
	Service      *service.Service
	Request      *HttpRequest
	Response     *HttpResponse
	ShareData    []map[string]any
}

func (this *WgoController) GetCookie(name string) string {
	c, e := this.Request.GetCookie(name)
	if e != nil {
		return ""
	}
	return c.Value
}

func (this *WgoController) AddCookie(name, value string, maxAge int) {
	this.Response.AddCookie(name, value, "/", maxAge, false, true)
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

func (this *WgoController) AddShare(data map[string]any) {
	this.ShareData = append(this.ShareData, data)
}

func (this *WgoController) Render(body []byte) []byte {
	return this.Response.Send(body)
}

func (this *WgoController) RenderJson(body any) []byte {
	this.Response.SetHeader("content-type", "application/json")
	return this.Response.SendJson(body)
}

func (this *WgoController) RenderHtml(filename string, data any) (string, any) {
	return filename, mergeShareDatas(data, this.ShareData)
}

func (this *WgoController) RenderHtmlStr(htmlStr string, data any) (*template.Template, any) {
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

func (this *WgoController) Success(body any) (json []byte) {
	json = this.RenderJson(struct {
		Code int `json:"code"`
		Data any `json:"data,omitempty"`
	}{Code: 0, Data: body})
	return
}
func (this *WgoController) SuccessExtra(body any, extra any) (json []byte) {
	json = this.RenderJson(struct {
		Code  int `json:"code"`
		Data  any `json:"data,omitempty"`
		Extra any `json:"extra,omitempty"`
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

func (this *WgoController) FailureExtra(code int, msg string, extra any) (json []byte) {
	json = this.RenderJson(struct {
		Code  int    `json:"code"`
		Msg   string `json:"msg,omitempty"`
		Extra any    `json:"extra,omitempty"`
	}{Code: code, Msg: msg, Extra: extra})
	return
}

func (this *WgoController) Redirect(url string, code int) []byte {
	switch code {
	case 201, 301, 302, 303, 307, 308:
	default:
		code = 301
	}
	http.Redirect(this.Response.Writer, this.Request.Request, url, code)
	return this.Render([]byte(""))
}

func mergeShareDatas(dst any, datas []map[string]any) any {
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

	si, ok := dst.(map[string]any)
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
