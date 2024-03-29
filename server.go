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
	"strconv"
	"strings"
)

type server struct {
	app          *app
	Configurator *config.Configurator
	Router       *router
}

func (this *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	req := &HttpRequest{Request: r}
	res := &HttpResponse{Writer: w}
	req.init()
	if nil == this.app.finally {
		defer this.finally(res, req)
	} else {
		defer this.app.finally(res, req)
	}

	route, params, notfound := this.Router.getHandler(r)
	if nil != notfound {
		_, _ = w.Write([]byte(notfound.Error()))
		return
	}

	var svc = this.app.servicer.New()

	controller := tool.StructCopy(route.Controller)
	cv := reflect.ValueOf(controller)
	cve := cv.Elem()

	cve.FieldByName("Router").Set(reflect.ValueOf(route))
	cve.FieldByName("Service").Set(reflect.ValueOf(svc))
	cve.FieldByName("Request").Set(reflect.ValueOf(req))
	cve.FieldByName("Response").Set(reflect.ValueOf(res))

	for _, iface := range this.app.reqControllerInjectorChain {
		iface.InjectRequestController(route, cve, svc)
	}

	this.parseRequestParam(req, params)

	if nil == route.interceptor {
		this.render(w, cv, &route, params)
	} else {
		var (
			result   bool
			resData  []byte
			inp      = tool.StructCopy(route.interceptor)
			inpValue = reflect.ValueOf(inp)
			inpParam = []reflect.Value{
				reflect.ValueOf(route),
				reflect.ValueOf(svc),
				reflect.ValueOf(req),
				reflect.ValueOf(res),
			}
		)
		result, resData = this.callInterceptor(inpValue, inpParam)
		if !result {
			w.Write(resData)
		} else {
			this.render(w, cv, &route, params)
		}
	}
}

func (this *server) parseRequestParam(r *HttpRequest, params []methodParam) {
	switch r.Request.Method {
	case GET:
		for k, p := range params {
			if p.IsStruct {
				var qmap = make(map[string]any)
				for i := 0; i < p.ParamType.NumField(); i++ {
					var (
						pt      = p.ParamType.Field(i)
						name    = pt.Name
						tagJson = pt.Tag.Get("json")
					)
					if "" != tagJson {
						name = tagJson
					}

					qmap[name] = convertParam2Value(r.Get(name), pt.Type.Name())
				}

				if tmp, e := json.Marshal(qmap); e == nil {
					val := reflect.New(p.ParamType)
					ifa := val.Interface()
					json.Unmarshal(tmp, ifa)

					if p.ParamKind == reflect.Ptr {
						params[k].StructValue = val
					} else {
						params[k].StructValue = val.Elem()
					}
				}

			} else if nil == p.Value {
				params[k].Value = convertParam2Value(r.Get(p.Name), p.Type)
			}
		}
	case DELETE:
		var (
			body        = r.Body()
			contentType = r.GetHeader("Content-Type")
		)
		if 0 == len(body) {
			for k, p := range params {
				if p.IsStruct {
					var qmap = make(map[string]any)
					for i := 0; i < p.ParamType.NumField(); i++ {
						var (
							pt      = p.ParamType.Field(i)
							name    = pt.Name
							tagJson = pt.Tag.Get("json")
						)
						if "" != tagJson {
							name = tagJson
						}

						qmap[name] = convertParam2Value(r.Get(name), pt.Type.Name())
					}

					if tmp, e := json.Marshal(qmap); e == nil {
						val := reflect.New(p.ParamType)
						ifa := val.Interface()
						json.Unmarshal(tmp, ifa)

						if p.ParamKind == reflect.Ptr {
							params[k].StructValue = val
						} else {
							params[k].StructValue = val.Elem()
						}
					}

				} else if nil == p.Value {
					params[k].Value = convertParam2Value(r.Get(p.Name), p.Type)
				}
			}
		} else {
			if strings.Contains(contentType, "application/json") {
				var m = make(map[string]any)
				json.Unmarshal(body, &m)
				for k, p := range params {
					if p.IsStruct {
						val := reflect.New(p.ParamType)
						ifa := val.Interface()
						json.Unmarshal(body, ifa)
						if p.ParamKind == reflect.Ptr {
							params[k].StructValue = val
						} else {
							params[k].StructValue = val.Elem()
						}
					} else if nil == p.Value {
						var queryVal = r.Get(p.Name)
						if "" != queryVal {
							params[k].Value = convertParam2Value(queryVal, p.Type)
						} else {
							params[k].Value = convertAny2Value(m[p.Name], p.Type)
						}
					}
				}
			} else if strings.Contains(contentType, "application/x-www-form-urlencoded") {
				for k, p := range params {
					if p.IsStruct {
						var qmap = make(map[string]any)
						for i := 0; i < p.ParamType.NumField(); i++ {
							var (
								pt      = p.ParamType.Field(i)
								name    = pt.Name
								tagJson = pt.Tag.Get("json")
							)
							if "" != tagJson {
								name = tagJson
							}

							qmap[name] = convertParam2Value(r.GetPost(name), pt.Type.Name())
						}

						if tmp, e := json.Marshal(qmap); e == nil {
							var (
								val = reflect.New(p.ParamType)
								ifa = val.Interface()
							)
							json.Unmarshal(tmp, ifa)
							if p.ParamKind == reflect.Ptr {
								params[k].StructValue = val
							} else {
								params[k].StructValue = val.Elem()
							}
						}
					} else if nil == p.Value {
						params[k].Value = convertParam2Value(r.GetRequest(p.Name), p.Type)
					}
				}
			} else {
				for k, p := range params {
					if !p.IsStruct && nil == p.Value {
						params[k].Value = convertParam2Value(r.GetRequest(p.Name), p.Type)
					}
				}
			}
		}
	case POST, PUT, PATCH:
		var contentType = r.GetHeader("Content-Type")
		if strings.Contains(contentType, "application/json") {
			var (
				m    = make(map[string]any)
				body = r.Body()
			)
			json.Unmarshal(body, &m)
			for k, p := range params {
				if p.IsStruct {
					val := reflect.New(p.ParamType)
					ifa := val.Interface()
					json.Unmarshal(body, ifa)
					if p.ParamKind == reflect.Ptr {
						params[k].StructValue = val
					} else {
						params[k].StructValue = val.Elem()
					}
				} else if nil == p.Value {
					params[k].Value = convertAny2Value(m[p.Name], p.Type)
				}
			}
		} else if strings.Contains(contentType, "application/x-www-form-urlencoded") {
			for k, p := range params {
				if p.IsStruct {
					var qmap = make(map[string]any)
					for i := 0; i < p.ParamType.NumField(); i++ {
						var (
							pt      = p.ParamType.Field(i)
							name    = pt.Name
							tagJson = pt.Tag.Get("json")
						)
						if "" != tagJson {
							name = tagJson
						}

						qmap[name] = convertParam2Value(r.GetPost(name), pt.Type.Name())
					}

					if tmp, e := json.Marshal(qmap); e == nil {
						var (
							val = reflect.New(p.ParamType)
							ifa = val.Interface()
						)
						json.Unmarshal(tmp, ifa)
						if p.ParamKind == reflect.Ptr {
							params[k].StructValue = val
						} else {
							params[k].StructValue = val.Elem()
						}
					}
				} else if nil == p.Value {
					params[k].Value = convertParam2Value(r.GetRequest(p.Name), p.Type)
				}
			}
		} else {
			for k, p := range params {
				if !p.IsStruct && nil == p.Value {
					params[k].Value = convertParam2Value(r.GetRequest(p.Name), p.Type)
				}
			}
		}
	}
}

func (this *server) callInterceptor(inp reflect.Value, params []reflect.Value) (bool, []byte) {
	ret := inp.MethodByName("Before").Call(params)
	if 2 != len(ret) {
		log.Panicf("controller interceptor '%s' return must be (bool, []byte)", inp.Type().Kind().String())
	}

	res := ret[0]
	dat := ret[1]
	rtp := res.Type()
	dtp := dat.Type()
	if rtp.Kind() != reflect.Bool || dtp.Kind() != reflect.Slice || dtp.Elem().Kind() != reflect.Uint8 {
		log.Panicf("controller interceptor '%s' return must be (bool, []byte)", inp.Type().Kind().String())
	}
	return res.Bool(), dat.Bytes()
}

func (this *server) render(w http.ResponseWriter, cv reflect.Value, router *Router, params []methodParam) {
	if router.HasInit {
		cv.MethodByName("Init").Call(nil)
	}

	var ret []reflect.Value
	if nil == params {
		ret = cv.MethodByName(router.Method.Name).Call(nil)
	} else {
		var values = make([]reflect.Value, len(params))
		for k, p := range params {
			if p.IsStruct {
				values[k] = p.StructValue
			} else {
				values[k] = reflect.ValueOf(p.Value)
			}
		}

		ret = cv.MethodByName(router.Method.Name).Call(values)
	}

	switch len(ret) {
	default:
		log.Panicf("%s of %s return must be []byte or (string, interface{}) or (*template.Template, interface{})", router.Method.Name, router.ControllerName)

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
			if e := this.app.template.ExecuteTemplate(w, r1.String(), r2); nil != e {
				log.Panic(e)
			}
		case reflect.Ptr:
			re1 := rt1.Elem()
			if re1.Kind() != reflect.Struct || re1.String() != "template.Template" {
				log.Panicf("%s of %s return must be (string, interface{}) or (template.Template, interface{})", router.Method.Name, router.ControllerName)
			}
			cr := r1.MethodByName("Execute").Call([]reflect.Value{reflect.ValueOf(w), r2})
			if len(cr) > 0 && !cr[0].IsNil() {
				log.Panic(cr[0].Interface())
			}
		}
	}
}

func (this *server) finally(res *HttpResponse, req *HttpRequest) {
	var e = recover()
	if e == nil {
		return
	}

	var msg string
	if this.app.debug {
		debug.PrintStack()

		stacks := strings.Split(string(debug.Stack()), "\n")
		var key int
		for k, s := range stacks {
			if strings.Contains(s, "reflect.Value.call") {
				key = k - 1
				break
			}
		}

		if key < 2 {
			res.Writer.Write(tool.String2Bytes(fmt.Sprintf("%s\n\n%s", e, debug.Stack())))
			return
		}

		var (
			fnPos   = strings.LastIndex(stacks[key-1], "/")
			lastPos = strings.LastIndex(stacks[key], "/")
			nextPos = strings.LastIndex(stacks[key][:lastPos], "/")
		)
		msg = fmt.Sprintf("%s at file %s func %s", e, stacks[key][nextPos:], stacks[key-1][fnPos:])
	} else {
		msg = fmt.Sprintf("%s", e)
	}

	var code = -1
	if res.statusCode > 0 {
		code = res.statusCode
	}
	b, _ := json.Marshal(map[string]any{"code": code, "msg": msg})
	res.Writer.Write(b)
}

func (this *server) InjectRouteController(controller any) {
	var (
		objt = reflect.TypeOf(controller).Elem()
		objv = reflect.ValueOf(controller).Elem()
		name = objt.String()
	)

	this.checkController(objt, objv, name)
	this.assigConfigurator(objt, objv, name)
	this.assigTemplate(objt, objv, name)
}

func (this *server) checkController(objt reflect.Type, objv reflect.Value, name string) {
	if w, f := objt.FieldByName("WgoController"); !f || w.Type.Kind() != reflect.Struct || "wgo.WgoController" != w.Type.String() {
		log.Panicf("struct WgoController must be embeded in %s", name)
	}

	sf, _ := objt.FieldByName("Router")
	if sf.Type.Kind() != reflect.Struct || sf.Type.String() != "wgo.Router" {
		log.Panicf("Router of %s must be ptr to struct Router", name)
	}
	if !objv.FieldByName("Router").CanSet() {
		log.Panicf("Router of %s can't be assignableTo", name)
	}

	sf, _ = objt.FieldByName("Service")
	if sf.Type.Kind() != reflect.Ptr || sf.Type.Elem().Kind() != reflect.Struct || sf.Type.String() != "*service.Service" {
		log.Panicf("Service of %s must be ptr to struct service.Service", name)
	}
	if !objv.FieldByName("Service").CanSet() {
		log.Panicf("Service of %s can't be assignableTo", name)
	}

	sf, _ = objt.FieldByName("Request")
	if sf.Type.Kind() != reflect.Ptr || sf.Type.Elem().Kind() != reflect.Struct || sf.Type.String() != "*wgo.HttpRequest" {
		log.Panicf("Request of %s must be ptr to struct wgo.HttpRequest", name)
	}
	if !objv.FieldByName("Request").CanSet() {
		log.Panicf("Request of %s can't be assignableTo", name)
	}

	sf, _ = objt.FieldByName("Response")
	if sf.Type.Kind() != reflect.Ptr || sf.Type.Elem().Kind() != reflect.Struct || sf.Type.String() != "*wgo.HttpResponse" {
		log.Panicf("Response of %s must be ptr to struct wgo.HttpResponse", name)
	}
	if !objv.FieldByName("Response").CanSet() {
		log.Panicf("Response of %s can't be assignableTo", name)
	}
}

func (this *server) assigConfigurator(objt reflect.Type, objv reflect.Value, name string) {
	sf, _ := objt.FieldByName("Configurator")
	if sf.Type.Kind() != reflect.Ptr || sf.Type.Elem().Kind() != reflect.Struct || sf.Type.String() != "*config.Configurator" {
		log.Panicf("Configurator of %s must be ptr to struct config.Configurator", name)
	}

	src := reflect.ValueOf(this.Configurator)
	dst := objv.FieldByName("Configurator")
	if !dst.CanSet() || !src.Type().AssignableTo(dst.Type()) {
		log.Panicf("Configurator of %s can't be assignableTo", name)
	}
	dst.Set(src)
}

func (this *server) assigTemplate(objt reflect.Type, objv reflect.Value, name string) {
	sf, _ := objt.FieldByName("Template")
	if sf.Type.Kind() != reflect.Ptr || sf.Type.Elem().Kind() != reflect.Struct || sf.Type.String() != "*template.Template" {
		log.Panicf("Template of %s must be ptr to struct template.Template", name)
	}

	if nil != this.app.templateFuncs {
		for n, f := range this.app.templateFuncs {
			tplBuiltins[n] = f
		}
	}

	this.app.template = template.New("WgoTemplateEngine").Funcs(tplBuiltins)
	if len(this.app.templatePath) > 0 {
		this.app.template = template.Must(this.app.template.ParseGlob(this.app.templatePath))
	}

	src := reflect.ValueOf(this.app.template)
	dst := objv.FieldByName("Template")
	if !dst.CanSet() || !src.Type().AssignableTo(dst.Type()) {
		log.Panicf("Template of %s can't be assignableTo", name)
	}
	dst.Set(src)
}

func convertParam2Value(value string, typ string) any {
	var (
		val any
		e   error
	)
	switch typ {
	case "int":
		val, e = strconv.Atoi(value)
		if e != nil {
			val = 0
		}
	case "int64":
		val, e = strconv.ParseInt(value, 10, 64)
		if e != nil {
			val = int64(0)
		}
	case "uint64":
		val, e = strconv.ParseUint(value, 10, 64)
		if e != nil {
			val = uint64(0)
		}
	case "float64":
		val, e = strconv.ParseFloat(value, 64)
		if e != nil {
			val = float64(0)
		}
	case "string":
		val = value
	case "bool":
		val, e = strconv.ParseBool(value)
		if e != nil {
			val = false
		}
	}
	return val
}

func convertAny2Value(value any, typ string) any {
	if nil == value {
		switch typ {
		case "int":
			return 0
		case "int64":
			return int64(0)
		case "uint64":
			return uint64(0)
		case "float64":
			return float64(0)
		case "string":
			return ""
		default:
			return nil
		}
	}
	var (
		val any
		e   error
	)
	switch typ {
	case "int":
		v, ok := value.(float64)
		if ok {
			val = int(v)
		} else {
			s, yes := value.(string)
			if yes {
				if val, e = strconv.Atoi(s); e != nil {
					val = 0
				}
			} else {
				val = 0
			}
		}
	case "int64":
		v, ok := value.(float64)
		if ok {
			val = int64(v)
		} else {
			s, yes := value.(string)
			if yes {
				if val, e = strconv.ParseInt(s, 10, 64); e != nil {
					val = int64(0)
				}
			} else {
				val = int64(0)
			}
		}
	case "uint64":
		v, ok := value.(float64)
		if ok {
			val = uint64(v)
		} else {
			s, yes := value.(string)
			if yes {
				if val, e = strconv.ParseUint(s, 10, 64); e != nil {
					val = uint64(0)
				}
			} else {
				val = uint64(0)
			}
		}
	case "float64":
		v, ok := value.(float64)
		if ok {
			val = v
		} else {
			s, yes := value.(string)
			if yes {
				if val, e = strconv.ParseFloat(s, 64); e != nil {
					val = float64(0)
				}
			} else {
				val = float64(0)
			}
		}
	case "string":
		v, ok := value.(string)
		if ok {
			val = v
		} else {
			val = ""
		}
	default:
		val = value
	}
	return val
}

type RequestControllerInjector interface {
	InjectRequestController(router Router, cve reflect.Value, svc *service.Service)
}
