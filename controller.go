package wgo

import (
	"github.com/xiaocairen/wgo/config"
	"github.com/xiaocairen/wgo/service"
	"html/template"
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

func (this *WgoController) Render(body []byte) {
	this.HttpResponse.Send(body)
}

func (this *WgoController) RenderJson(body interface{}) {
	this.HttpResponse.SetHeader("content-type", "application/json")
	this.HttpResponse.SendJson(body)
}

func (this *WgoController) RenderHtml(filename string, data interface{}) {
	this.Template.ExecuteTemplate(this.HttpResponse, filename, data)
}

func (this *WgoController) RenderHtmlStr(htmlStr string, data interface{}) {
	if t, e := this.Template.Parse(htmlStr); e != nil {
		log.Panic(e)
	} else {
		t.Execute(this.HttpResponse, data)
	}
}
