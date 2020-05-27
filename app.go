package wgo

import (
	"fmt"
	"github.com/xiaocairen/wgo/config"
	"github.com/xiaocairen/wgo/mdb"
	"github.com/xiaocairen/wgo/service"
	"github.com/xiaocairen/wgo/tool"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	appInstance *app
	onceAppRun  sync.Once
)

type Tasker func(c *config.Configurator, s *service.Service)
type Finally func(w http.ResponseWriter, r *http.Request)

type app struct {
	debug                        bool
	configurator                 *config.Configurator
	servicer                     *service.Servicer
	router                       *router
	routeCollection              RouteCollection
	routeControllerInjectorChain []RouteControllerInjector
	reqControllerInjectorChain   []RequestControllerInjector
	tableCollection              service.TableCollection
	template                     *template.Template
	templatePath                 string
	templateFuncs                template.FuncMap
	taskers                      []Tasker
	finally                      Finally
}

func init() {
	if nil == appInstance {
		path, err := filepath.Abs(filepath.Dir(os.Args[0]))
		if err != nil {
			log.Panic(err)
		}
		if err := os.Chdir(path); err != nil {
			log.Panic("unable to change working dir " + err.Error())
		}

		appInstance = &app{configurator: config.New("app.json")}

		if err := appInstance.configurator.GetBool("debug", &appInstance.debug); err != nil {
			log.Panic(err)
		}

		var dbTestPing bool
		if err := appInstance.configurator.GetBool("db_test_ping", &dbTestPing); err != nil {
			log.Panic(err)
		}

		dbc, err := appInstance.configurator.Get("database")
		if nil != err {
			if dbTestPing {
				log.Panic(err)
			}
			return
		}

		var dbcs []*mdb.DBConfig
		if err := parseDbConfig(dbc, &dbcs); err != nil {
			if dbTestPing {
				log.Panic(err)
			}
			return
		}

		db, err := mdb.NewDB(dbcs, dbTestPing)
		if err != nil {
			if dbTestPing {
				log.Panic(err)
			}
			return
		}

		appInstance.servicer = service.NewServicer(db)
	}

	initLogger()
}

func GetApp() *app {
	return appInstance
}

func (this *app) Run() {
	onceAppRun.Do(func() {
		this.router = &router{RouteCollection: this.routeCollection}
		s := &server{app: this, Configurator: this.configurator, Router: this.router}
		this.router.init([]RouteControllerInjector{s})

		this.servicer.Registe(this.tableCollection)
		this.startTaskers()

		mux := http.NewServeMux()

		dirs := this.getStaticFileDirs()
		if len(dirs) > 0 {
			for _, dir := range dirs {
				mux.Handle("/" + dir + "/", http.FileServer(http.Dir("web")))
			}
			mux.Handle("/favicon.ico", http.FileServer(http.Dir("web")))
		}

		mux.Handle("/", s)

		host, port := this.getHostAndPort()
		server := &http.Server{
			Addr:              host + ":" + strconv.Itoa(port),
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

func (this *app) getHostAndPort() (host string, port int) {
	h := struct {
		Addr string `json:"addr"`
		Port int    `json:"port"`
	}{}
	if err := this.configurator.GetStruct("http", &h); err != nil {
		panic(err)
	}

	host = h.Addr
	port = h.Port
	return
}

func (this *app) getStaticFileDirs() []string {
	var (
		dirs string
		sd   []string
	)
	this.configurator.GetStr("static_file_dirs", &dirs)
	if len(dirs) > 0 {
		tmp := strings.Split(dirs, ",")
		for _, s := range tmp {
			sd = append(sd, strings.Trim(strings.TrimSpace(s), "/"))
		}
	}

	return sd
}

func (this *app) startTaskers() {
	for _, tasker := range this.taskers {
		go func() {
			defer func() {
				if e := recover(); e != nil {
					log.Printf("%s", e)
				}
			}()
			tasker(this.configurator, this.servicer.New())
		}()
	}
}

func (this *app) SetRouteCollection(rc RouteCollection) *app {
	if nil == this.routeCollection {
		this.routeCollection = rc
	}
	return this
}

func (this *app) SetTableCollection(tc service.TableCollection) *app {
	if nil == this.tableCollection {
		this.tableCollection = tc
	}
	return this
}

func (this *app) AddRequestControllerInjector(injector RequestControllerInjector) *app {
	this.reqControllerInjectorChain = append(this.reqControllerInjectorChain, injector)
	return this
}

//func (this *app) AddRouteControllerInjector(injector RouteControllerInjector) {
//	this.routeControllerInjectorChain = append(this.routeControllerInjectorChain, injector)
//}

func (this *app) SetHtmlPath(path string) *app {
	if "" == this.templatePath {
		this.templatePath = path
	}
	return this
}

func (this *app) SetHtmlFuncs(fnmap template.FuncMap) *app {
	if nil == this.templateFuncs {
		this.templateFuncs = fnmap
	}
	return this
}

func (this *app) AddTasker(tasker Tasker) *app {
	this.taskers = append(this.taskers, tasker)
	return this
}

func (this *app) SetFinally(f Finally) *app {
	if nil == this.finally {
		this.finally = f
	}
	return this
}

func (this *app) GetConfigurator() *config.Configurator {
	return this.configurator
}

func initLogger() {
	var outer int
	if e := appInstance.configurator.GetInt("log_outer", &outer); e != nil {
		log.Panic(e)
	}

	if outer == 0 {
		return
	}

	if _, err := os.Stat("log"); nil != err {
		if !os.IsExist(err) {
			if err := os.Mkdir("log", os.ModePerm); nil != err {
				log.Panic(err)
			}
		}
	}

	f, err := os.OpenFile("log/wgo.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Panic(err)
	}
	log.SetFlags(log.Ldate | log.Lmicroseconds | log.Lshortfile)
	log.SetOutput(f)
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
