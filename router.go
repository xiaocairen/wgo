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

type router struct {
	RouteRegister   *RouteRegister
	RouteCollection RouteCollection
}

func (this *router) init(chain []RouteControllerInjector) {
	this.RouteRegister = &RouteRegister{injectChain: chain}
	this.RouteCollection.call(this.RouteRegister)
}

func (this *router) getHandler(r *http.Request) (router Router, params []routePathParam, err error) {
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

type routePathParam struct {
	ppname  string
	pptype  string
	ppvalue interface{}
}

func (this *router) searchRoute(routes []*routeNamespace, r *http.Request) (router Router, params []routePathParam, err error) {
	if 0 == len(routes) {
		err = RouteNotFoundError{path: r.RequestURI}
		return
	}

	var subdomain string
	if subdomain, err = this.parseHost(r); err != nil {
		return
	}

	var (
		routeIt *routeNamespace
		routeSp *routeNamespace
	)
	for key, rns := range routes {
		if rns.subdomain == subdomain {
			routeIt = routes[key]
			break
		} else if rns.subdomain == "*" {
			routeSp = routes[key]
		}
	}
	if nil == routeIt {
		if nil == routeSp {
			err = RouteNotFoundError{path: r.Host + r.RequestURI}
			return
		} else {
			routeIt = routeSp
		}
	}

	var (
		urlpathlen = len(r.URL.Path)
		routelist  []*Router
		paramlist  [][]routePathParam
	)
	for key, ru := range routeIt.routers {
		if 0 == ru.pathParamsNum {
			if urlpathlen < ru.Pathlen || ru.Path != r.URL.Path[0:ru.Pathlen] || (urlpathlen > ru.Pathlen && '/' != r.URL.Path[ru.Pathlen]) {
				continue
			}
			paramlist = append(paramlist, nil)
			routelist = append(routelist, routeIt.routers[key])
			continue
		}

		if urlpathlen <= ru.Pathlen || ru.Path != r.URL.Path[0:ru.Pathlen] || '/' != r.URL.Path[ru.Pathlen] {
			continue
		}

		pathParams := strings.Split(r.URL.Path[ru.Pathlen+1:], "/")
		if len(pathParams) < ru.pathParamsNum {
			continue
		}

		var (
			nomatch   = false
			tmpParams []routePathParam
		)
		for k, pp := range ru.PathParams {
			ppname, _ := pp[0].(string)
			pptype, _ := pp[1].(string)
			switch pptype {
			case "string":
				tmpParams = append(tmpParams, routePathParam{
					ppname:  ppname,
					pptype:  pptype,
					ppvalue: pathParams[k],
				})

			case "int":
				i, e := strconv.Atoi(pathParams[k])
				if e != nil {
					nomatch = true
					break
				}
				pplen := len(pp)
				if pplen >= 3 {
					min, _ := pp[2].(int)
					if i < min {
						nomatch = true
						break
					}
				}
				if pplen >= 4 {
					max, _ := pp[3].(int)
					if i > max {
						nomatch = true
						break
					}
				}
				tmpParams = append(tmpParams, routePathParam{
					ppname:  ppname,
					pptype:  pptype,
					ppvalue: i,
				})

			case "float64":
				f, e := strconv.ParseFloat(pathParams[k], 64)
				if e != nil {
					nomatch = true
					break
				}
				pplen := len(pp)
				if pplen >= 3 {
					min, _ := pp[2].(float64)
					if f < min {
						nomatch = true
						break
					}
				}
				if pplen >= 4 {
					max, _ := pp[3].(float64)
					if f > max {
						nomatch = true
						break
					}
				}
				tmpParams = append(tmpParams, routePathParam{
					ppname:  ppname,
					pptype:  pptype,
					ppvalue: f,
				})

			default:
				nomatch = true
				break
			}
		}
		if nomatch {
			continue
		}

		paramlist = append(paramlist, tmpParams)
		routelist = append(routelist, routeIt.routers[key])
	}

	switch len(routelist) {
	case 0:
		err = RouteNotFoundError{path: r.Host + r.RequestURI}
	case 1:
		router = *routelist[0]
		params = paramlist[0]
	default:
		router = *routelist[0]
		for k, r := range routelist[1:] {
			if router.Pathlen < r.Pathlen {
				router = *r
				params = paramlist[k+1]
			}
		}
	}
	return
}

func (this *router) parseHost(r *http.Request) (subdomain string, err error) {
	reg := regexp.MustCompile(`^(?i:\d+\.\d+\.\d+\.\d+|localhost)(:\d+)?$`)
	if reg.MatchString(r.Host) {
		subdomain = "www"
	} else {
		ln := strings.LastIndexByte(r.Host, '.')
		if -1 == ln || 0 == ln {
			err = fmt.Errorf("bad host '%s'", r.Host)
			return
		}

		sn := strings.LastIndexByte(r.Host[0:ln], '.')
		switch sn {
		case -1:
			subdomain = "www"
		case 0:
			err = fmt.Errorf("bad host '%s'", r.Host)
			return
		default:
			subdomain = r.Host[0:sn]
		}
	}
	return
}

type RouteNotFoundError struct {
	path string
}

func (nf RouteNotFoundError) Error() string {
	return fmt.Sprintf("not found route of path '%s'", nf.path)
}

type RouteCollection func(register *RouteRegister)

func (fn RouteCollection) call(register *RouteRegister) {
	fn(register)
	/*for _, r := range register.get {
		fmt.Printf("r.subdomain => %s\n", r.subdomain)
		for _, n := range r.routers {
			fmt.Printf("Path => %s\n", n.Path)
			fmt.Printf("ControllerName => %s\n", n.ControllerName)
			fmt.Printf("Method.Name => %s\n", n.Method.Name)
		}
	}*/
}

type RouteUnit struct {
	Path       string
	Controller interface{}
	Action     string
}

type Router struct {
	Path           string
	Pathlen        int
	PathParams     [][]interface{}
	pathParamsNum  int
	Controller     interface{}
	ControllerName string
	Method         reflect.Method
	HasInit        bool
	interceptor    RouteInterceptor
}

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
			parseRouteMethod(s, this.ns, unit, this.register.injectChain, this.interceptor)
			return
		}
	}

	rns := &routeNamespace{subdomain: this.sd}
	this.register.get = append(this.register.get, rns)

	parseRouteMethod(rns, this.ns, unit, this.register.injectChain, this.interceptor)
}
func (this routeUnitHttpMethod) Post(unit RouteUnit) {
	for _, s := range this.register.post {
		if this.sd == s.subdomain {
			parseRouteMethod(s, this.ns, unit, this.register.injectChain, this.interceptor)
			return
		}
	}

	rns := &routeNamespace{subdomain: this.sd}
	this.register.post = append(this.register.post, rns)

	parseRouteMethod(rns, this.ns, unit, this.register.injectChain, this.interceptor)
}
func (this routeUnitHttpMethod) Put(unit RouteUnit) {
	for _, s := range this.register.put {
		if this.sd == s.subdomain {
			parseRouteMethod(s, this.ns, unit, this.register.injectChain, this.interceptor)
			return
		}
	}

	rns := &routeNamespace{subdomain: this.sd}
	this.register.put = append(this.register.put, rns)

	parseRouteMethod(rns, this.ns, unit, this.register.injectChain, this.interceptor)
}
func (this routeUnitHttpMethod) Delete(unit RouteUnit) {
	for _, s := range this.register.delete {
		if this.sd == s.subdomain {
			parseRouteMethod(s, this.ns, unit, this.register.injectChain, this.interceptor)
			return
		}
	}

	rns := &routeNamespace{subdomain: this.sd}
	this.register.delete = append(this.register.delete, rns)

	parseRouteMethod(rns, this.ns, unit, this.register.injectChain, this.interceptor)
}

func (this routeUnitHttpMethod) Any(unit RouteUnit) {
	for _, s := range this.register.any {
		if this.sd == s.subdomain {
			parseRouteMethod(s, this.ns, unit, this.register.injectChain, this.interceptor)
			return
		}
	}

	rns := &routeNamespace{subdomain: this.sd}
	this.register.any = append(this.register.any, rns)

	parseRouteMethod(rns, this.ns, unit, this.register.injectChain, this.interceptor)
}

func parseRouteMethod(m *routeNamespace, ns string, unit RouteUnit, chain []RouteControllerInjector, interceptor RouteInterceptor) {
	var path string
	if "/" == ns {
		path = unit.Path
	} else {
		path = "/" + ns + unit.Path
	}

	queryPath, pathParams := parseRoutePath(path)
	actName, actParam := parseRouteAction(unit.Action)
	ctlName, method, hasInit := parseRouteController(unit.Controller, actName, actParam, pathParams, chain)

	m.routers = append(m.routers, &Router{
		Path:           queryPath,
		Pathlen:        len(queryPath),
		PathParams:     pathParams,
		pathParamsNum:  len(pathParams),
		Controller:     unit.Controller,
		ControllerName: ctlName,
		Method:         method,
		HasInit:        hasInit,
		interceptor:    interceptor,
	})
}

func parseRoutePath(routePath string) (path string, params [][]interface{}) {
	n := strings.Index(routePath, "/:")
	if 0 == n || 1 == n {
		log.Panicf("not support route path '%s'", routePath)
	}

	var pStr string
	if -1 == n {
		path = routePath
		pStr = ""
	} else {
		path = routePath[0:n]
		pStr = routePath[n+1:]
	}

	if len(pStr) > 0 {
		pArr := strings.Split(pStr, "/")
		for _, p := range pArr {
			var param []interface{}
			p = strings.TrimLeft(p, ":")
			tmp := strings.Split(p, ":")
			n := len(tmp)
			if n < 2 {
				log.Panicf("illegal route path '%s'", path)
			}

			tmp[1] = strings.ToLower(tmp[1])
			param = append(param, tmp[0])
			param = append(param, tmp[1])
			if "int" == tmp[1] {
				if n >= 3 {
					i, e := strconv.Atoi(tmp[2])
					if e != nil {
						log.Panicf("'%s' in route path '%s' can't convert to int", path, tmp[2])
					}
					param = append(param, i)
				}
				if n >= 4 {
					i, e := strconv.Atoi(tmp[3])
					if e != nil {
						log.Panicf("'%s' in route path '%s' can't convert to int", path, tmp[3])
					}
					param = append(param, i)
				}
			} else if "float64" == tmp[1] {
				if n >= 3 {
					f, e := strconv.ParseFloat(tmp[2], 64)
					if e != nil {
						log.Panicf("'%s' in route path '%s' can't convert to float64", path, tmp[2])
					}
					param = append(param, f)
				}
				if n >= 4 {
					f, e := strconv.ParseFloat(tmp[3], 64)
					if e != nil {
						log.Panicf("'%s' in route path '%s' can't convert to float64", path, tmp[3])
					}
					param = append(param, f)
				}
			}
			params = append(params, param)
		}
	}
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
		pArr := strings.Split(prm, ",")
		for _, p := range pArr {
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

func parseRouteController(controller interface{}, action string, actParams [][]string, pathParams [][]interface{}, chain []RouteControllerInjector) (ctlName string, actionMethod reflect.Method, hasInit bool) {
	rtc := reflect.TypeOf(controller)
	if rtc.Kind() != reflect.Ptr || rtc.Elem().Kind() != reflect.Struct {
		log.Panicf("controller must be ptr point to struct")
	}
	ctlName = rtc.Elem().String()

	var found bool
	for _, pp := range pathParams {
		ppname, _ := pp[0].(string)
		pptype, _ := pp[1].(string)
		found = false
		for _, ap := range actParams {
			if ap[0] == ppname {
				if ap[1] != pptype {
					log.Panicf("type of '%s' of '%s' mismatch path param", ppname, rtc.String())
				}
				found = true
				break
			}
		}
		if !found {
			log.Panicf("param '%s' of '%s' be not found in route path params", ppname, rtc.String())
		}
	}

	_, hasInit = rtc.MethodByName("Init")
	actionMethod, found = rtc.MethodByName(action)
	if !found {
		log.Panicf("not found method '%s' in '%s'", action, rtc.String())
	}

	m := reflect.ValueOf(controller).MethodByName(action).Type()
	n := m.NumIn()
	if n != len(actParams) {
		log.Panicf("param num of method '%s' of '%s' mismatch router", action, rtc.String())
	}
	for i := 0; i < n; i++ {
		in := m.In(i)
		if in.Name() != actParams[i][1] {
			log.Panicf("param[%d] type of method '%s' of '%s' mismatch router", i, action, rtc.String())
		}
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
