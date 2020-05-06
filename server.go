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
		ctltype = reflect.TypeOf(controller).Elem()
		ctlname = ctltype.String()
	)
	if wgo, found := ctltype.FieldByName("WgoController"); !found {
		panic("struct WgoController must be embeded in " + ctlname)
	} else if wgo.Type.Kind() != reflect.Struct || "wgo.WgoController" != wgo.Type.String() {
		panic("WgoController of " + ctlname + " must be wgo.WgoController")
	}

	var sf reflect.StructField

	sf, _ = ctltype.FieldByName("Configurator")
	if sf.Type.Kind() != reflect.Ptr || sf.Type.String() != "*config.Configurator" {
		panic("Configurator of " + ctlname + " must be *config.Configurator")
	}

	sf, _ = ctltype.FieldByName("Service")
	if sf.Type.Kind() != reflect.Ptr || sf.Type.String() != "*service.Service" {
		panic("Service of " + ctlname + " must be *service.Service")
	}

	sf, _ = ctltype.FieldByName("HttpResponse")
	if sf.Type.Kind() != reflect.Ptr || sf.Type.String() != "*wgo.HttpResponse" {
		panic("HttpResponse of " + ctlname + " must be *wgo.HttpResponse")
	}

	sf, _ = ctltype.FieldByName("HttpRequest")
	if sf.Type.Kind() != reflect.Ptr || sf.Type.String() != "*wgo.HttpRequest" {
		panic("HttpRequest of " + ctlname + " must be *wgo.HttpRequest")
	}

	sf, _ = ctltype.FieldByName("Tpl")
	if sf.Type.Kind() != reflect.Ptr || sf.Type.String() != "*template.Template" {
		panic("Tpl of " + ctlname + " must be *template.Template")
	}

	var (
		ctlval = reflect.ValueOf(controller).Elem()
		srcCfg = reflect.ValueOf(this.Configurator)
		dstCfg = ctlval.FieldByName("Configurator")
	)
	if !dstCfg.CanSet() || !srcCfg.Type().AssignableTo(dstCfg.Type()) {
		panic("Configurator of " + ctlname + " can't be assignableTo")
	}
	dstCfg.Set(srcCfg)

	if nil != this.app.service {
		var (
			srcSvc = reflect.ValueOf(this.app.service)
			dstSvc = ctlval.FieldByName("Service")
		)
		if !dstSvc.CanSet() || !srcSvc.Type().AssignableTo(dstSvc.Type()) {
			panic("field Service of " + ctlname + " can't be assign")
		}
		dstSvc.Set(srcSvc)
	}

	var srcTpl reflect.Value
	if nil == this.app.htmlTemplate {
		srcTpl = reflect.ValueOf(template.New("WgoTemplateEngine"))
	} else {
		srcTpl = reflect.ValueOf(this.app.htmlTemplate)
	}

	var dstTpl = ctlval.FieldByName("Tpl")
	if !dstTpl.CanSet() || !srcTpl.Type().AssignableTo(dstTpl.Type()) {
		panic("field Tpl of " + ctlname + " can't be assign")
	}
	dstTpl.Set(srcTpl)
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
