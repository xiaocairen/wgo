package wgo

import (
	"encoding/json"
	"github.com/xiaocairen/wgo/config"
	"github.com/xiaocairen/wgo/tool"
	"github.com/xiaocairen/wgo/service"
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
	if nil == this.app.finally {
		defer this.finally(w, r)
	} else {
		defer this.app.finally(w, r)
	}

	route, params, notfound := this.Router.getHandler(r)
	if nil != notfound {
		w.Write([]byte(notfound.Error()))
		return
	}

	svc := this.app.servicer.New()
	req := &HttpRequest{Request: r}
	res := &HttpResponse{ResponseWriter: w}
	req.init()

	controller := tool.StructCopy(route.controller)
	cv := reflect.ValueOf(controller)
	cve := cv.Elem()

	cve.FieldByName("Service").Set(reflect.ValueOf(svc))
	cve.FieldByName("HttpRequest").Set(reflect.ValueOf(req))
	cve.FieldByName("HttpResponse").Set(reflect.ValueOf(res))

	for _, iface := range this.app.reqControllerInjectorChain {
		iface.InjectRequestController(controller, cve, svc)
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

func (this *server) finally(res http.ResponseWriter, req *http.Request) {
	if e := recover(); e != nil {
		b, _ := json.Marshal(map[string]interface{}{
			"success":   false,
			"error_msg": e,
		})
		res.Write(b)
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
		log.Panicf("Configurator of %s must be ptr to struct config.Configurator", name)
	}

	sf, _ = ctltyp.FieldByName("Template")
	if sf.Type.Kind() != reflect.Ptr || sf.Type.Elem().Kind() != reflect.Struct || sf.Type.String() != "*template.Template" {
		log.Panicf("Template of %s must be ptr to struct template.Template", name)
	}

	sf, _ = ctltyp.FieldByName("Service")
	if sf.Type.Kind() != reflect.Ptr || sf.Type.Elem().Kind() != reflect.Struct || sf.Type.String() != "*service.Service" {
		log.Panicf("Service of %s must be ptr to struct service.Service", name)
	}
	if !ctlval.FieldByName("Service").CanSet() {
		log.Panicf("Service of %s can't be assignableTo", name)
	}

	sf, _ = ctltyp.FieldByName("HttpRequest")
	if sf.Type.Kind() != reflect.Ptr || sf.Type.Elem().Kind() != reflect.Struct || sf.Type.String() != "*wgo.HttpRequest" {
		log.Panicf("HttpRequest of %s must be ptr to struct wgo.HttpRequest", name)
	}
	if !ctlval.FieldByName("HttpRequest").CanSet() {
		log.Panicf("HttpRequest of %s can't be assignableTo", name)
	}

	sf, _ = ctltyp.FieldByName("HttpResponse")
	if sf.Type.Kind() != reflect.Ptr || sf.Type.Elem().Kind() != reflect.Struct || sf.Type.String() != "*wgo.HttpResponse" {
		log.Panicf("HttpResponse of %s must be ptr to struct wgo.HttpResponse", name)
	}
	if !ctlval.FieldByName("HttpResponse").CanSet() {
		log.Panicf("HttpResponse of %s can't be assignableTo", name)
	}

	src := reflect.ValueOf(this.Configurator)
	dst := ctlval.FieldByName("Configurator")
	if !dst.CanSet() || !src.Type().AssignableTo(dst.Type()) {
		log.Panicf("Configurator of %s can't be assignableTo", name)
	}
	dst.Set(src)

	if nil == this.app.htmlTemplate {
		src = reflect.ValueOf(template.New("WgoTemplateEngine"))
	} else {
		src = reflect.ValueOf(this.app.htmlTemplate)
	}
	dst = ctlval.FieldByName("Template")
	if !dst.CanSet() || !src.Type().AssignableTo(dst.Type()) {
		log.Panicf("Template of %s can't be assignableTo", name)
	}
	dst.Set(src)
}

type RequestControllerInjector interface {
	InjectRequestController(controller interface{}, cve reflect.Value, svc *service.Service)
}
