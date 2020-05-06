package wgo

import (
	"fmt"
	"github.com/xiaocairen/wgo/config"
	"github.com/xiaocairen/wgo/service"
	"github.com/xiaocairen/wgo/mdb"
	"github.com/xiaocairen/wgo/tool"
	"html/template"
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
	debug                        bool
	configurator                 *config.Configurator
	service                      *service.Service
	routeCollection              RouteCollection
	routeControllerInjectorChain []RouteControllerInjector
	reqControllerInjectorChain   []RequestControllerInjector
	tableCollection              service.TableCollection
	htmlTemplate                 *template.Template
	taskers                      []func()
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

		appInstance = &app{configurator: config.New("app.json")}

		if err := appInstance.configurator.GetBool("debug", &appInstance.debug); err != nil {
			log.Fatal(err)
		}

		var dbTestPing bool
		if err := appInstance.configurator.GetBool("db_test_ping", &dbTestPing); err != nil {
			log.Fatal(err)
		}

		dbc, err := appInstance.configurator.Get("database")
		if nil != err {
			if dbTestPing {
				log.Fatal(err)
			}
			return
		}

		var dbcs []*mdb.DBConfig
		if err := parseDbConfig(dbc, &dbcs); err != nil {
			if dbTestPing {
				log.Fatal(err)
			}
			return
		}

		db, err := mdb.NewDB(dbcs, dbTestPing)
		if err != nil {
			if dbTestPing {
				log.Fatal(err)
			}
			return
		}

		appInstance.service = service.NewService(db)
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
		this.AddRouteControllerInjector(s)
		r.init(this.routeControllerInjectorChain)

		this.service.Registe(this.tableCollection)

		mux := http.NewServeMux()
		mux.Handle("/page/", http.StripPrefix("/page/", http.FileServer(http.Dir("web"))))
		mux.Handle("/static/", http.FileServer(http.Dir("web")))
		mux.Handle("/favicon.ico", http.FileServer(http.Dir("web")))
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

func (this *app) AddRequestControllerInjector(injector RequestControllerInjector) {
	this.reqControllerInjectorChain = append(this.reqControllerInjectorChain, injector)
}

func (this *app) AddRouteControllerInjector(injector RouteControllerInjector) {
	this.routeControllerInjectorChain = append(this.routeControllerInjectorChain, injector)
}

func (this *app) SetHtmlTemplate(tpl *template.Template) {
	if nil == this.htmlTemplate {
		this.htmlTemplate = tpl
	}
}

func (this *app) RegisteTaskers(taskers []func()) {
	if nil == this.taskers {
		this.taskers = taskers
	}
}

func (this *app) GetConfigurator() *config.Configurator {
	return this.configurator
}

func (this *app) GetService() *service.Service {
	return this.service
}

func parseDbConfig(dbc interface{}, dbcs *[]*mdb.DBConfig) error {
	if s, ok := dbc.([]interface{}); ok {
		for _, v := range s {
			m, ok := v.(map[string]interface{})
			if !ok {
				return fmt.Errorf("db config must be []map[string]interface{}")
			}
			var dbc mdb.DBConfig
			tool.StructFill(&m, &dbc)
			*dbcs = append(*dbcs, &dbc)
		}
		return nil

	} else if m, ok := dbc.(map[string]interface{}); ok {
		if _, f := m["driver"]; f {
			var dbc mdb.DBConfig
			tool.StructFill(&m, &dbc)
			*dbcs = append(*dbcs, &dbc)
		} else {
			write, fw := m["write"]
			read, fr := m["read"]
			if !fw || !fr {
				return fmt.Errorf("read and write db must be both exist")
			}

			if ws, ok := write.([]interface{}); ok {
				for _, v := range ws {
					m, ok := v.(map[string]interface{})
					if !ok {
						return fmt.Errorf("db config must be map[string]interface{}")
					}
					var dbc mdb.DBConfig
					tool.StructFill(&m, &dbc)
					dbc.ReadOrWrite = mdb.ONLY_WRITE
					*dbcs = append(*dbcs, &dbc)
				}
			} else if m, ok := write.(map[string]interface{}); ok {
				var dbc mdb.DBConfig
				tool.StructFill(&m, &dbc)
				dbc.ReadOrWrite = mdb.ONLY_WRITE
				*dbcs = append(*dbcs, &dbc)
			} else {
				return fmt.Errorf("db config must be map or slice")
			}

			if rs, ok := read.([]interface{}); ok {
				for _, v := range rs {
					m, ok := v.(map[string]interface{})
					if !ok {
						return fmt.Errorf("db config must be map[string]interface{}")
					}
					var dbc mdb.DBConfig
					tool.StructFill(&m, &dbc)
					dbc.ReadOrWrite = mdb.ONLY_READ
					*dbcs = append(*dbcs, &dbc)
				}
			} else if m, ok := read.(map[string]interface{}); ok {
				var dbc mdb.DBConfig
				tool.StructFill(&m, &dbc)
				dbc.ReadOrWrite = mdb.ONLY_READ
				*dbcs = append(*dbcs, &dbc)
			} else {
				return fmt.Errorf("db config must be map or slice")
			}
		}
		return nil
	}
	return fmt.Errorf("db config must be map or slice")
}
