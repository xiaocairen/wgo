package wgo

import (
	"encoding/json"
	"fmt"
	"github.com/xiaocairen/wgo/config"
	"github.com/xiaocairen/wgo/service"
	"github.com/xiaocairen/wgo/tool"
	"html/template"
	"log"
	"net/http"
	"reflect"
	"runtime/debug"
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

	router, params, notfound := this.Router.getHandler(r)
	if nil != notfound {
		w.Write([]byte(notfound.Error()))
		return
	}

	svc := this.app.servicer.New()
	req := &HttpRequest{Request: r}
	res := &HttpResponse{writer: w}
	req.init()

	controller := tool.StructCopy(router.Controller)
	cv := reflect.ValueOf(controller)
	cve := cv.Elem()

	cve.FieldByName("Service").Set(reflect.ValueOf(svc))
	cve.FieldByName("HttpRequest").Set(reflect.ValueOf(req))
	cve.FieldByName("HttpResponse").Set(reflect.ValueOf(res))

	for _, iface := range this.app.reqControllerInjectorChain {
		iface.InjectRequestController(router, cve, svc)
	}

	this.render(w, cv, router, params)
}

func (this *server) render(w http.ResponseWriter, cv reflect.Value, router Router, params []routePathParam) {
	if router.HasInit {
		cv.MethodByName("Init").Call(nil)
	}

	var ret []reflect.Value
	if nil == params {
		ret = cv.MethodByName(router.Method.Name).Call(nil)
	} else {
		var values = make([]reflect.Value, len(params))
		for k, p := range params {
			values[k] = reflect.ValueOf(p.ppvalue)
		}

		ret = cv.MethodByName(router.Method.Name).Call(values)
	}

	switch len(ret) {
	default:
		log.Panicf("%s of %s return must be []byte or (string, interface{}) or (*template.Template, interface{})")
		
	case 1:
		rt := ret[0].Type()
		if rt.Kind() != reflect.Slice || rt.Elem().Kind() != reflect.Uint8 {
			log.Panicf("%s of %s first return must be []byte, '%s' given", router.Method.Name, router.ControllerName, rt.Kind())
		}
		if ret[0].IsNil() {
			log.Panicf("%s of %s first return is nil", router.Method.Name, router.ControllerName)
		}
		w.Write(ret[0].Bytes())
		
	case 2:
		var (
			r1  = ret[0]
			r2  = ret[1]
			rt1 = r1.Type()
		)
		switch rt1.Kind() {
		default:
			log.Panicf("%s of %s return must be (string, interface{}) or (*template.Template, interface{})", router.Method.Name, router.ControllerName)
		case reflect.String:
			this.app.htmlTemplate.ExecuteTemplate(w, r1.String(), r2)
		case reflect.Ptr:
			re1 := rt1.Elem()
			if re1.Kind() != reflect.Struct || re1.String() != "template.Template" {
				log.Panicf("%s of %s return must be (string, interface{}) or (template.Template, interface{})", router.Method.Name, router.ControllerName)
			}
			r1.MethodByName("Execute").Call([]reflect.Value{reflect.ValueOf(w), r2})
		}
	}
}

func (this *server) finally(res http.ResponseWriter, req *http.Request) {
	if e := recover(); e != nil {
		if this.app.debug {
			res.Write(tool.String2Bytes(fmt.Sprintf("%s\n\n%s", e, debug.Stack())))
		} else {
			b, _ := json.Marshal(map[string]interface{}{
				"success":   false,
				"error_msg": e,
			})
			res.Write(b)
		}
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
		this.app.htmlTemplate = template.New("WgoTemplateEngine")
	}
	src = reflect.ValueOf(this.app.htmlTemplate)
	dst = ctlval.FieldByName("Template")
	if !dst.CanSet() || !src.Type().AssignableTo(dst.Type()) {
		log.Panicf("Template of %s can't be assignableTo", name)
	}
	dst.Set(src)
}

type RequestControllerInjector interface {
	InjectRequestController(router Router, cve reflect.Value, svc *service.Service)
}
