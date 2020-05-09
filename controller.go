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

func (this *WgoController) Render(body []byte) []byte {
	return this.HttpResponse.Send(body)
}

func (this *WgoController) RenderJson(body interface{}) []byte {
	this.HttpResponse.SetHeader("content-type", "application/json")
	return this.HttpResponse.SendJson(body)
}

func (this *WgoController) RenderHtml(filename string, data interface{}) (string, interface{}) {
	//this.Template.ExecuteTemplate(this.HttpResponse, filename, data)
	return filename, data
}

func (this *WgoController) RenderHtmlStr(htmlStr string, data interface{}) (*template.Template, interface{}) {
	t, e := this.Template.Parse(htmlStr)
	if e != nil {
		log.Panic(e)
	}
		//t.Execute(this.HttpResponse, data)
	return t, data
}
