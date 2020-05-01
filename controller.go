package wgo

import (
	"github.com/xiaocairen/wgo/config"
	"github.com/xiaocairen/wgo/service"
	"html/template"
)

type WgoController struct {
	Configurator *config.Configurator
	Service      *service.Service
	HttpResponse *HttpResponse
	HttpRequest  *HttpRequest
	Tpl          *template.Template
}

func (this *WgoController) AppendBody(body []byte) *WgoController {
	this.HttpResponse.Append(body)
	return this
}

func (this WgoController) Render(body []byte) {
	this.HttpResponse.Send(body)
}

func (this WgoController) RenderJson(body interface{}) {
	this.HttpResponse.SetHeader("content-type", "application/json")
	this.HttpResponse.SendJson(body)
}

func (this WgoController) RenderHtml(filename string, data interface{}) {
	this.Tpl.ExecuteTemplate(this.HttpResponse, filename, data)
}

func (this WgoController) RenderHtmlStr(htmlStr string, data interface{}) {
	t, e := this.Tpl.Parse(htmlStr)
	if e != nil {
		panic(e)
	}

	t.Execute(this.HttpResponse, data)
}
