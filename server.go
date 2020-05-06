package wgo

import (
	"encoding/json"
	"fmt"
	"github.com/xiaocairen/wgo/config"
	"github.com/xiaocairen/wgo/tool"
	"html/template"
	"net/http"
	"reflect"
)

type server struct {
	app          *app
	Configurator *config.Configurator
	Router       *router
}

func (this server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !this.app.debug {
		defer this.finaly(w, r)
	}

	route, params, notfound := this.Router.getHandler(r)
	if nil != notfound {
		panic(notfound)
	}

	req := &HttpRequest{Request: r}
	res := &HttpResponse{ResponseWriter: w}
	req.init()

	controller := tool.StructCopy(route.controller)
	cv := reflect.ValueOf(controller)
	cve := cv.Elem()

	cve.FieldByName("HttpRequest").Set(reflect.ValueOf(req))
	cve.FieldByName("HttpResponse").Set(reflect.ValueOf(res))

	for _, iface := range this.app.reqControllerInjectorChain {
		iface.InjectRequestController(controller, cve)
	}

	if route.hasInit {
		cv.MethodByName("Init").Call(nil)
	}

	if nil == params {
		cv.MethodByName(route.method.Name).Call(nil)
	} else {
		var values = make([]reflect.Value, len(params))
		for k, p := range params {
			values[k] = reflect.ValueOf(p.ppvalue)
		}

		cv.MethodByName(route.method.Name).Call(values)
	}
}

func (this server) InjectRouteController(controller interface{}) {
	var (
		ctltyp = reflect.TypeOf(controller).Elem()
		ctlval = reflect.ValueOf(controller).Elem()
		name   = ctltyp.String()
		sf     reflect.StructField
	)
	if w, f := ctltyp.FieldByName("WgoController"); !f || w.Type.Kind() != reflect.Struct || "wgo.WgoController" != w.Type.String() {
		panic("struct WgoController must be embeded in " + name)
	}

	sf, _ = ctltyp.FieldByName("Configurator")
	if sf.Type.Kind() != reflect.Ptr || sf.Type.Elem().Kind() != reflect.Struct || sf.Type.String() != "*config.Configurator" {
		panic("Configurator of " + name + " must be struct *config.Configurator")
	}

	sf, _ = ctltyp.FieldByName("Service")
	if sf.Type.Kind() != reflect.Ptr || sf.Type.Elem().Kind() != reflect.Struct || sf.Type.String() != "*service.Service" {
		panic("Service of " + name + " must be struct *service.Service")
	}

	sf, _ = ctltyp.FieldByName("HttpResponse")
	if sf.Type.Kind() != reflect.Ptr || sf.Type.Elem().Kind() != reflect.Struct || sf.Type.String() != "*wgo.HttpResponse" {
		panic("HttpResponse of " + name + " must be struct *wgo.HttpResponse")
	}

	sf, _ = ctltyp.FieldByName("HttpRequest")
	if sf.Type.Kind() != reflect.Ptr || sf.Type.Elem().Kind() != reflect.Struct || sf.Type.String() != "*wgo.HttpRequest" {
		panic("HttpRequest of " + name + " must be struct *wgo.HttpRequest")
	}

	sf, _ = ctltyp.FieldByName("Tpl")
	if sf.Type.Kind() != reflect.Ptr || sf.Type.Elem().Kind() != reflect.Struct || sf.Type.String() != "*template.Template" {
		panic("Tpl of " + name + " must be struct *template.Template")
	}

	src := reflect.ValueOf(this.Configurator)
	dst := ctlval.FieldByName("Configurator")
	if !dst.CanSet() || !src.Type().AssignableTo(dst.Type()) {
		panic("Configurator of " + name + " can't be assignableTo")
	}
	dst.Set(src)

	if nil != this.app.service {
		src = reflect.ValueOf(this.app.service)
		dst = ctlval.FieldByName("Service")
		if !dst.CanSet() || !src.Type().AssignableTo(dst.Type()) {
			panic("field Service of " + name + " can't be assign")
		}
		dst.Set(src)
	}

	if nil == this.app.htmlTemplate {
		src = reflect.ValueOf(template.New("WgoTemplateEngine"))
	} else {
		src = reflect.ValueOf(this.app.htmlTemplate)
	}

	dst = ctlval.FieldByName("Tpl")
	if !dst.CanSet() || !src.Type().AssignableTo(dst.Type()) {
		panic("field Tpl of " + name + " can't be assign")
	}
	dst.Set(src)
}

func (this *server) finaly(res http.ResponseWriter, req *http.Request) {
	if e := recover(); e != nil {
		if "application/json" == req.Header.Get("Accept") {
			b, _ := json.Marshal(map[string]interface{}{
				"success":   false,
				"error_msg": e,
			})
			res.Write(b)
		} else {
			res.Write(tool.String2Bytes(fmt.Sprintf("%s", e)))
		}
	}
}

type RequestControllerInjector interface {
	InjectRequestController(controller interface{}, cve reflect.Value)
}
