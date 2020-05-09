package wgo

import (
	"github.com/xiaocairen/wgo/config"
	"github.com/xiaocairen/wgo/service"
	"html/template"
	"github.com/xiaocairen/wgo/tool"
	"log"
)

type WgoController struct {
	Configurator *config.Configurator
	Template     *template.Template
	Service      *service.Service
	HttpRequest  *HttpRequest
	HttpResponse *HttpResponse
}

func (this *WgoController) AppendBody(body []byte) *WgoController {
	this.HttpResponse.Append(body)
	return this
}

func (this *WgoController) Render(body []byte) []byte {
	return this.HttpResponse.Send(body)
}

func (this *WgoController) RenderJson(body interface{}) []byte {
	this.HttpResponse.SetHeader("content-type", "application/json")
	return this.HttpResponse.SendJson(body)
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
