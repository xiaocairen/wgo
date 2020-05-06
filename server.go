package wgo

import (
	"encoding/json"
	"fmt"
	"github.com/xiaocairen/wgo/config"
	"github.com/xiaocairen/wgo/tool"
	"html/template"
	"log"
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
		log.Panic(notfound)
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
		log.Panicf("struct WgoController must be embeded in %s", name)
	}

	sf, _ = ctltyp.FieldByName("Configurator")
	if sf.Type.Kind() != reflect.Ptr || sf.Type.Elem().Kind() != reflect.Struct || sf.Type.String() != "*config.Configurator" {
		log.Panicf("Configurator of %s must be struct *config.Configurator", name)
	}

	sf, _ = ctltyp.FieldByName("Service")
	if sf.Type.Kind() != reflect.Ptr || sf.Type.Elem().Kind() != reflect.Struct || sf.Type.String() != "*service.Service" {
		log.Panicf("Service of %s must be struct *service.Service", name)
	}

	sf, _ = ctltyp.FieldByName("HttpResponse")
	if sf.Type.Kind() != reflect.Ptr || sf.Type.Elem().Kind() != reflect.Struct || sf.Type.String() != "*wgo.HttpResponse" {
		log.Panicf("HttpResponse of %s must be struct *wgo.HttpResponse", name)
	}

	sf, _ = ctltyp.FieldByName("HttpRequest")
	if sf.Type.Kind() != reflect.Ptr || sf.Type.Elem().Kind() != reflect.Struct || sf.Type.String() != "*wgo.HttpRequest" {
		log.Panicf("HttpRequest of %s must be struct *wgo.HttpRequest", name)
	}

	sf, _ = ctltyp.FieldByName("Tpl")
	if sf.Type.Kind() != reflect.Ptr || sf.Type.Elem().Kind() != reflect.Struct || sf.Type.String() != "*template.Template" {
		log.Panicf("Tpl of %s must be struct *template.Template", name)
	}

	src := reflect.ValueOf(this.Configurator)
	dst := ctlval.FieldByName("Configurator")
	if !dst.CanSet() || !src.Type().AssignableTo(dst.Type()) {
		log.Panicf("Configurator of %s can't be assignableTo", name)
	}
	dst.Set(src)

	if nil != this.app.service {
		src = reflect.ValueOf(this.app.service)
		dst = ctlval.FieldByName("Service")
		if !dst.CanSet() || !src.Type().AssignableTo(dst.Type()) {
			log.Panicf("field Service of %s can't be assign", name)
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
		log.Panicf("field Tpl of %s can't be assign", name)
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
