package wgo

import (
	"fmt"
	"github.com/xiaocairen/wgo/service"
	"log"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

type RouteNotFoundError struct {
	path string
}

func (nf RouteNotFoundError) Error() string {
	return fmt.Sprintf("not found route of path '%s'", nf.path)
}

type RouteCollection func(register *RouteRegister)

func (fn RouteCollection) call(register *RouteRegister) {
	fn(register)
}

// --------------------------------------------------------------------------------
// router call by app, return Router
// --------------------------------------------------------------------------------
type router struct {
	RouteRegister   *RouteRegister
	RouteCollection RouteCollection
}

func (this *router) init(chain []RouteControllerInjector) {
	this.RouteRegister = &RouteRegister{injectChain: chain}
	this.RouteCollection.call(this.RouteRegister)
}

func (this *router) getHandler(r *http.Request) (router Router, params []methodParam, err error) {
	switch r.Method {
	case GET:
		router, params, err = this.searchRoute(this.RouteRegister.get, r)
		if err != nil {
			router, params, err = this.searchRoute(this.RouteRegister.any, r)
		}

	case POST:
		router, params, err = this.searchRoute(this.RouteRegister.post, r)
		if err != nil {
			router, params, err = this.searchRoute(this.RouteRegister.any, r)
		}

	case PUT:
		router, params, err = this.searchRoute(this.RouteRegister.put, r)

	case DELETE:
		router, params, err = this.searchRoute(this.RouteRegister.delete, r)

	default:
		err = fmt.Errorf("not support http method '%s'", r.Method)
	}
	return
}

func (this *router) searchRoute(routes []*routeNamespace, req *http.Request) (router Router, params []methodParam, err error) {
	if 0 == len(routes) {
		err = RouteNotFoundError{path: req.RequestURI}
		return
	}

	var (
		routeIt *routeNamespace
		routeSp *routeNamespace
		domain  = this.parseHost(req)
	)
	for key, rns := range routes {
		if strings.Contains(domain, rns.subdomain+".") {
			routeIt = routes[key]
			break
		} else if rns.subdomain == "*" {
			routeSp = routes[key]
		}
	}
	if nil == routeIt {
		if nil == routeSp {
			err = RouteNotFoundError{path: req.Host + req.RequestURI}
			return
		}
		routeIt = routeSp
	}

	var (
		routerall  *Router
		urlpathlen = len(req.URL.Path)
		routelist  = make([]*Router, 0)
		paramlist  = make([][]string, 0)
	)
	for key, route := range routeIt.routers {
		if route.Path == req.URL.Path {
			router = *route
			params = route.MethodParams
			return
		}

		if route.Path == "/*" {
			routerall = route
			continue
		}

		if route.PathIsRegexp {
			if route.PathRegexp.MatchString(req.URL.Path) {
				values := route.PathRegexp.FindStringSubmatch(req.URL.Path)
				paramlist = append(paramlist, values[1:])
				routelist = append(routelist, routeIt.routers[key])
			}
			continue
		}

		if urlpathlen < route.Pathlen || route.Path != req.URL.Path[0:route.Pathlen] ||
			(urlpathlen > route.Pathlen && '/' != req.URL.Path[route.Pathlen]) ||
			(route.Pathlen > 1 && '/' != req.URL.Path[route.Pathlen]) {
			continue
		}

		paramlist = append(paramlist, nil)
		routelist = append(routelist, routeIt.routers[key])
	}

	var parameters []string
	switch len(routelist) {
	case 0:
		if nil == routerall {
			err = RouteNotFoundError{path: req.Host + req.RequestURI}
		} else {
			router = *routerall
		}
	case 1:
		router = *routelist[0]
		parameters = paramlist[0]
	default:
		for k, r := range routelist {
			if r.Pathlen > router.Pathlen {
				router = *r
				parameters = paramlist[k]
			}
		}
	}

	if nil != parameters {
		for _, mp := range router.MethodParams {
			var found = false
			for k, pp := range router.PathParams {
				if mp.Name == pp {
					found = true

					var (
						value   interface{}
						e       error
						pathVal = parameters[k]
					)
					switch mp.Type {
					case "int":
						value, e = strconv.Atoi(pathVal)
						if e != nil {
							value = 0
						}
					case "int64":
						value, e = strconv.ParseInt(pathVal, 10, 64)
						if e != nil {
							value = int64(0)
						}
					case "uint64":
						value, e = strconv.ParseUint(pathVal, 10, 64)
						if e != nil {
							value = uint64(0)
						}
					case "float64":
						value, e = strconv.ParseFloat(pathVal, 64)
						if e != nil {
							value = float64(0)
						}
					case "string":
						value = pathVal
					}

					params = append(params, methodParam{
						Name:      mp.Name,
						Type:      mp.Type,
						ParamKind: mp.ParamKind,
						ParamType: mp.ParamType,
						IsStruct:  mp.IsStruct,
						Value:     value,
					})

					break
				}
			}

			if !found {
				params = append(params, methodParam{
					Name:        mp.Name,
					Type:        mp.Type,
					ParamKind:   mp.ParamKind,
					ParamType:   mp.ParamType,
					IsStruct:    mp.IsStruct,
					Value:       nil,
					StructValue: mp.StructValue,
				})
			}
		}
	}

	return
}

func (this *router) parseHost(r *http.Request) string {
	reg := regexp.MustCompile(`^(?i:\d+\.\d+\.\d+\.\d+|localhost)(:\d+)?$`)
	if reg.MatchString(r.Host) {
		return "www.test.cn"
	}
	return r.Host
}

func (this *router) getRouter(method string, controller string, action string) (Router, error) {
	var rns []*routeNamespace
	switch strings.ToUpper(method) {
	case GET:
		rns = this.RouteRegister.get
	case POST:
		rns = this.RouteRegister.post
	case PUT:
		rns = this.RouteRegister.put
	case DELETE:
		rns = this.RouteRegister.delete
	case "ANY":
		rns = this.RouteRegister.any
	default:
		return Router{}, fmt.Errorf("no router %s to %s:%s", method, controller, action)
	}

	for _, rn := range rns {
		for _, r := range rn.routers {
			if r.ControllerName == controller && r.Method.Name == action {
				return *r, nil
			}
		}
	}
	return Router{}, fmt.Errorf("no router %s to %s:%s", method, controller, action)
}

type methodParam struct {
	Name        string
	Type        string
	ParamKind   reflect.Kind
	ParamType   reflect.Type
	IsStruct    bool
	Value       interface{}
	StructValue reflect.Value
}

// --------------------------------------------------------------------------------
// Router
// --------------------------------------------------------------------------------
type Router struct {
	Path           string
	Pathlen        int
	PathIsRegexp   bool
	PathRegexp     *regexp.Regexp
	PathParams     []string
	pathParamsNum  int
	Controller     interface{}
	ControllerName string
	Method         reflect.Method
	MethodParams   []methodParam
	HasInit        bool
	interceptor    RouteInterceptor
	register       *RouteRegister
}

func (r Router) GetRouter(method string, controller string, action string) (Router, error) {
	var rns []*routeNamespace
	switch strings.ToUpper(method) {
	case GET:
		rns = r.register.get
	case POST:
		rns = r.register.post
	case PUT:
		rns = r.register.put
	case DELETE:
		rns = r.register.delete
	case "ANY":
		rns = r.register.any
	default:
		return Router{}, fmt.Errorf("no router %s to %s:%s", method, controller, action)
	}

	for _, rn := range rns {
		for _, r := range rn.routers {
			if r.ControllerName == controller && r.Method.Name == action {
				return *r, nil
			}
		}
	}
	return Router{}, fmt.Errorf("no router %s to %s:%s", method, controller, action)
}

// --------------------------------------------------------------------------------
// RouteRegister
// --------------------------------------------------------------------------------
type routeNamespace struct {
	subdomain string
	routers   []*Router
}

type RouteRegister struct {
	domains     []string
	get         []*routeNamespace
	post        []*routeNamespace
	put         []*routeNamespace
	delete      []*routeNamespace
	any         []*routeNamespace
	injectChain []RouteControllerInjector
}

func (this *RouteRegister) Registe(subdomain, namespace string, interceptor RouteInterceptor, fn func(um UnitHttpMethod, m HttpMethod)) {
	sd := strings.TrimSpace(subdomain)
	ns := strings.TrimLeft(strings.TrimSpace(namespace), "/")
	if 0 == len(sd) {
		sd = "www"
	}
	if 0 == len(ns) {
		ns = "/"
	}
	this.domains = append(this.domains, sd)

	uhm := routeUnitHttpMethod{
		sd:          sd,
		ns:          ns,
		interceptor: interceptor,
		register:    this,
	}
	fn(uhm, routeHttpMethod{uhm: uhm})
}

type RouteUnit struct {
	Path       string
	Controller interface{}
	Action     string
}

type routeHttpMethod struct {
	uhm routeUnitHttpMethod
}

func (this routeHttpMethod) Get(p string, c interface{}, a string) {
	this.uhm.Get(RouteUnit{
		Path:       p,
		Controller: c,
		Action:     a,
	})
}
func (this routeHttpMethod) Post(p string, c interface{}, a string) {
	this.uhm.Post(RouteUnit{
		Path:       p,
		Controller: c,
		Action:     a,
	})
}
func (this routeHttpMethod) Put(p string, c interface{}, a string) {
	this.uhm.Put(RouteUnit{
		Path:       p,
		Controller: c,
		Action:     a,
	})
}
func (this routeHttpMethod) Delete(p string, c interface{}, a string) {
	this.uhm.Delete(RouteUnit{
		Path:       p,
		Controller: c,
		Action:     a,
	})
}
func (this routeHttpMethod) Any(p string, c interface{}, a string) {
	this.uhm.Any(RouteUnit{
		Path:       p,
		Controller: c,
		Action:     a,
	})
}

type routeUnitHttpMethod struct {
	sd          string
	ns          string
	register    *RouteRegister
	interceptor RouteInterceptor
}

func (this routeUnitHttpMethod) Get(unit RouteUnit) {
	for _, s := range this.register.get {
		if this.sd == s.subdomain {
			this.parseRouteMethod(s, unit)
			return
		}
	}

	rns := &routeNamespace{subdomain: this.sd}
	this.register.get = append(this.register.get, rns)

	this.parseRouteMethod(rns, unit)
}
func (this routeUnitHttpMethod) Post(unit RouteUnit) {
	for _, s := range this.register.post {
		if this.sd == s.subdomain {
			this.parseRouteMethod(s, unit)
			return
		}
	}

	rns := &routeNamespace{subdomain: this.sd}
	this.register.post = append(this.register.post, rns)

	this.parseRouteMethod(rns, unit)
}
func (this routeUnitHttpMethod) Put(unit RouteUnit) {
	for _, s := range this.register.put {
		if this.sd == s.subdomain {
			this.parseRouteMethod(s, unit)
			return
		}
	}

	rns := &routeNamespace{subdomain: this.sd}
	this.register.put = append(this.register.put, rns)

	this.parseRouteMethod(rns, unit)
}
func (this routeUnitHttpMethod) Delete(unit RouteUnit) {
	for _, s := range this.register.delete {
		if this.sd == s.subdomain {
			this.parseRouteMethod(s, unit)
			return
		}
	}

	rns := &routeNamespace{subdomain: this.sd}
	this.register.delete = append(this.register.delete, rns)

	this.parseRouteMethod(rns, unit)
}

func (this routeUnitHttpMethod) Any(unit RouteUnit) {
	for _, s := range this.register.any {
		if this.sd == s.subdomain {
			this.parseRouteMethod(s, unit)
			return
		}
	}

	rns := &routeNamespace{subdomain: this.sd}
	this.register.any = append(this.register.any, rns)

	this.parseRouteMethod(rns, unit)
}

func (this routeUnitHttpMethod) parseRouteMethod(m *routeNamespace, unit RouteUnit) {
	var path string
	if "/" == this.ns {
		path = unit.Path
	} else {
		path = "/" + this.ns + unit.Path
	}

	pathIsRegexp, queryPath, pathRegexp, pathParams := parseRoutePath(path)
	actName, actParam := parseRouteAction(unit.Action)
	ctlName, method, methodParams, hasInit := parseRouteController(unit.Controller, actName, actParam, pathParams, this.register.injectChain)

	m.routers = append(m.routers, &Router{
		Path:           queryPath,
		Pathlen:        len(queryPath),
		PathIsRegexp:   pathIsRegexp,
		PathRegexp:     pathRegexp,
		PathParams:     pathParams,
		pathParamsNum:  len(pathParams),
		Controller:     unit.Controller,
		ControllerName: ctlName,
		Method:         method,
		MethodParams:   methodParams,
		HasInit:        hasInit,
		interceptor:    this.interceptor,
		register:       this.register,
	})
}

func parseRoutePath(routePath string) (pathIsRegexp bool, path string, pathRegexp *regexp.Regexp, params []string) {
	if routePath == "/*" {
		pathIsRegexp = false
		path = "/*"
		return
	}

	var n = strings.Index(routePath, "/:")
	if -1 == n {
		pathIsRegexp = false
		path = routePath
		return
	}

	pathIsRegexp = true

	r := regexp.MustCompile(`/:([^/]+)`)
	all := r.FindAllStringSubmatch(routePath, -1)
	var exp = routePath
	for _, v := range all {
		params = append(params, v[1])
		exp = strings.Replace(exp, v[0], "/([^/]+)", -1)
	}

	// replace ^+*$
	exp = strings.ReplaceAll(exp, `^`, `\^`)
	exp = strings.ReplaceAll(exp, `\^/]`, `^/]`)
	exp = strings.ReplaceAll(exp, `+`, `\+`)
	exp = strings.ReplaceAll(exp, `]\+)`, `]+)`)
	exp = strings.ReplaceAll(exp, `*`, `\*`)
	exp = strings.ReplaceAll(exp, `$`, `\$`)

	path = "^" + exp
	pathRegexp = regexp.MustCompile(path)
	return
}

func parseRouteAction(action string) (name string, params [][]string) {
	n := strings.Index(action, "(")
	if n <= 0 {
		log.Panicf("not support route action '%s'", action)
	}

	name = action[0:n]
	prm := strings.TrimSpace(action[n+1 : len(action)-1])
	if len(prm) > 0 {
		arr := strings.Split(prm, ",")
		for _, p := range arr {
			p = strings.TrimSpace(p)
			tmp := strings.Split(p, " ")
			if 2 != len(tmp) {
				log.Panicf("illegal route action '%s'", action)
			}
			tmp[0] = strings.TrimSpace(tmp[0])
			tmp[1] = strings.TrimSpace(tmp[1])
			params = append(params, tmp)
		}
	}
	return
}

func parseRouteController(controller interface{}, action string, actParams [][]string, pathParams []string, chain []RouteControllerInjector) (ctlName string, actionMethod reflect.Method, methodParams []methodParam, hasInit bool) {
	rtc := reflect.TypeOf(controller)
	if rtc.Kind() != reflect.Ptr || rtc.Elem().Kind() != reflect.Struct {
		log.Panicf("controller must be ptr point to struct")
	}
	ctlName = rtc.Elem().String()

	var ok bool
	for _, param := range pathParams {
		ok = false
		for _, ap := range actParams {
			if ap[0] == param {
				ok = true
				break
			}
		}
		if !ok {
			log.Panicf("path param '%s' not found in method '%s:%s'", param, ctlName, action)
		}
	}

	_, hasInit = rtc.MethodByName("Init")
	actionMethod, ok = rtc.MethodByName(action)
	if !ok {
		log.Panicf("not found method '%s' in '%s'", action, rtc.String())
	}

	m := reflect.ValueOf(controller).MethodByName(action).Type()
	n := m.NumIn()
	if n != len(actParams) {
		log.Panicf("number of parameters of method '%s:%s' mismatch route register", rtc.String(), action)
	}

	for i := 0; i < n; i++ {
		var (
			actPt = actParams[i][1]
			tlen  = len(actPt)
			pt    = m.In(i)
		)
		if pt.Kind() == reflect.Ptr {
			if pt.Elem().Kind() == reflect.Struct {
				name := pt.Elem().Name()
				if actPt[tlen-len(name):] != name {
					log.Panicf("type of param[%d] of method '%s:%s' mismatch router", i, rtc.String(), action)
				}
			} else if pt.Name() != actPt {
				log.Panicf("type of param[%d] of method '%s:%s' mismatch router", i, rtc.String(), action)
			}
		} else if pt.Kind() == reflect.Struct {
			name := pt.Name()
			if actPt[tlen-len(name):] != name {
				log.Panicf("type of param[%d] of method '%s:%s' mismatch router", i, rtc.String(), action)
			}
		} else if pt.Name() != actPt {
			log.Panicf("type of param[%d] of method '%s:%s' mismatch router", i, rtc.String(), action)
		}
	}

	for i := 0; i < n; i++ {
		var (
			structVal reflect.Value
			isStruct  bool
			pt        = m.In(i)
			pk        = pt.Kind()
			actParam  = actParams[i]
		)
		if pk == reflect.Ptr {
			if pt.Elem().Kind() == reflect.Struct {
				isStruct = true
				structVal = reflect.New(pt.Elem())
			}
			pt = pt.Elem()
		} else if pk == reflect.Struct {
			isStruct = true
			structVal = reflect.Zero(pt)
		}

		methodParams = append(methodParams, methodParam{
			Name:        actParam[0],
			Type:        actParam[1],
			ParamKind:   pk,
			ParamType:   pt,
			IsStruct:    isStruct,
			Value:       nil,
			StructValue: structVal,
		})
	}

	for _, injector := range chain {
		injector.InjectRouteController(controller)
	}

	return
}

type RouteControllerInjector interface {
	InjectRouteController(controller interface{})
}

type RouteInterceptor interface {
	Before(router Router, svc *service.Service, r *HttpRequest, w *HttpResponse) (pass bool, res []byte)
}

const (
	GET    = "GET"
	POST   = "POST"
	PUT    = "PUT"
	DELETE = "DELETE"
	PATCH  = "PATCH"
)

type HttpMethod interface {
	Get(path string, controller interface{}, action string)
	Post(path string, controller interface{}, action string)
	Put(path string, controller interface{}, action string)
	Delete(path string, controller interface{}, action string)
	Any(path string, controller interface{}, action string)
}

type UnitHttpMethod interface {
	Get(unit RouteUnit)
	Post(unit RouteUnit)
	Put(unit RouteUnit)
	Delete(unit RouteUnit)
	Any(unit RouteUnit)
}
