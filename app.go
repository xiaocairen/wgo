package wgo

import (
	"github.com/xiaocairen/wgo/config"
	"github.com/xiaocairen/wgo/service"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

var (
	appInstance *app
	onceAppRun  sync.Once
)

type app struct {
	debug                     bool
	configurator              *config.Configurator
	routeCollection           RouteCollection
	requestControllerInjector RequestControllerInjector
	tableCollection           service.TableCollection
}

func init() {
	if nil == appInstance {
		path, err := filepath.Abs(filepath.Dir(os.Args[0]))
		if err != nil {
			log.Fatal(err)
		}
		if err := os.Chdir(path); err != nil {
			log.Fatal("unable to change working dir " + err.Error())
		}

		appInstance = &app{
			configurator: config.New("app.json"),
		}
		if err := appInstance.configurator.GetBool("debug", &appInstance.debug); err != nil {
			log.Fatal(err)
		}
	}
}

func GetApp() *app {
	return appInstance
}

func (this *app) Run() {
	onceAppRun.Do(func() {
		h := struct {
			Addr string `json:"addr"`
			Port int    `json:"port"`
		}{}
		if err := this.configurator.GetStruct("http", &h); err != nil {
			panic(err)
		}

		r := &router{RouteCollection: this.routeCollection}
		s := &server{app: this, Configurator: this.configurator, Router: r}
		r.init([]RouteControllerInjector{s})

		mux := http.NewServeMux()
		mux.Handle("/page/", http.StripPrefix("/page/", http.FileServer(http.Dir("web"))))
		mux.Handle("/static/", http.FileServer(http.Dir("web")))
		mux.Handle("/", s)

		server := &http.Server{
			Addr:              h.Addr + ":" + strconv.Itoa(h.Port),
			Handler:           mux,
			TLSConfig:         nil,
			ReadTimeout:       30 * time.Second,
			ReadHeaderTimeout: 0,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       0,
			MaxHeaderBytes:    1 << 20,
			TLSNextProto:      nil,
			ConnState:         nil,
			ErrorLog:          nil,
			BaseContext:       nil,
			ConnContext:       nil,
		}

		if err := server.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	})
}

func (this *app) SetRouteCollection(rc RouteCollection) {
	if nil == this.routeCollection {
		this.routeCollection = rc
	}
}

func (this *app) SetTableCollection(tc service.TableCollection) {
	if nil == this.tableCollection {
		this.tableCollection = tc
	}
}

func (this *app) SetRequestControllerInjector(injector RequestControllerInjector) {
	if nil == this.requestControllerInjector {
		this.requestControllerInjector = injector
	}
}

func (this *app) GetConfigurator() *config.Configurator {
	return this.configurator
}
