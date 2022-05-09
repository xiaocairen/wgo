package wgo

import (
	"encoding/json"
	"fmt"
	"github.com/xiaocairen/wgo/config"
	"github.com/xiaocairen/wgo/mdb"
	"github.com/xiaocairen/wgo/service"
	"github.com/xiaocairen/wgo/tool"
	"github.com/xiaocairen/wgo/tool/httputil"
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
	appinst *app
	oncerun sync.Once
)

type WebsocketHandler func(w http.ResponseWriter, r *http.Request, c *config.Configurator, s *service.Service)
type Tasker func(c *config.Configurator, s *service.Service)
type Finally func(w *HttpResponse, r *HttpRequest)

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
	websocketHandlers            map[string]WebsocketHandler
	taskers                      []Tasker
	finally                      Finally
}

func init() {
	if nil == appinst {
		path, err := filepath.Abs(filepath.Dir(os.Args[0]))
		if err != nil {
			log.Panic(err)
		}
		if err = os.Chdir(path); err != nil {
			log.Panic("unable to change working dir " + err.Error())
		}

		appinst = &app{
			configurator:      config.New("app.json"),
			websocketHandlers: make(map[string]WebsocketHandler),
		}

		if err = appinst.configurator.GetBool("debug", &appinst.debug); err != nil {
			log.Panic(err)
		}

		var dbTestPing bool
		appinst.configurator.GetBool("db_test_ping", &dbTestPing)

		dbc, err := appinst.configurator.Get("database")
		if nil != err {
			if dbTestPing {
				log.Panic(err)
			}
			return
		}

		var dbcs []*mdb.DBConfig
		if err := parseDBConfig(dbc, &dbcs); err != nil {
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

		appinst.servicer = service.NewServicer(db)
	}

	initLogger()
}

func GetApp() *app {
	return appinst
}

func (this *app) Run() {
	oncerun.Do(func() {
		this.router = &router{RouteCollection: this.routeCollection}
		var s = &server{app: this, Configurator: this.configurator, Router: this.router}
		this.router.init([]RouteControllerInjector{s})

		this.servicer.Registe(this.tableCollection)
		this.startTaskers()

		var (
			host, port, enableWebsocket = this.getHostAndPort()
			mux                         = http.NewServeMux()
			dirs                        = this.getStaticFileDirs()
		)
		if len(dirs) > 0 {
			for _, dir := range dirs {
				mux.Handle("/"+dir+"/", http.FileServer(http.Dir("web")))
			}
			mux.Handle("/favicon.ico", http.FileServer(http.Dir("web")))
		}
		if enableWebsocket && len(this.websocketHandlers) > 0 {
			for url, handler := range this.websocketHandlers {
				mux.HandleFunc(url, func(w http.ResponseWriter, r *http.Request) {
					handler(w, r, this.configurator, this.servicer.New())
				})
			}
		}

		mux.Handle("/", s)

		var httpServer = &http.Server{
			Addr:              host + ":" + strconv.Itoa(port),
			Handler:           mux,
			TLSConfig:         nil,
			ReadTimeout:       30 * time.Second,
			ReadHeaderTimeout: 0,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       0,
			MaxHeaderBytes:    1 << 20,
		}
		if e := httpServer.ListenAndServe(); e != nil {
			log.Fatal(e)
		}
	})
}

func (this *app) getHostAndPort() (host string, port int, enableWebSocket bool) {
	h := struct {
		Addr         string `json:"addr"`
		Port         int    `json:"port"`
		UseWebsocket bool   `json:"use_websocket"`
	}{}
	if err := this.configurator.GetStruct("http", &h); err != nil {
		panic(err)
	}

	host = h.Addr
	port = h.Port
	enableWebSocket = h.UseWebsocket
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
		go func(t Tasker) {
			defer func() {
				if e := recover(); e != nil {
					log.Printf("%s", e)
				}
			}()
			t(this.configurator, this.servicer.New())
		}(tasker)
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

func (this *app) AddWebsocketHandler(url string, handler WebsocketHandler) *app {
	this.websocketHandlers[url] = handler
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
	if e := appinst.configurator.GetInt("log_outer", &outer); e != nil {
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

func parseDBConfig(dbc any, dbcs *[]*mdb.DBConfig) error {
	switch val := dbc.(type) {
	case []any:
		for _, v := range val {
			m, ok := v.(map[string]any)
			if !ok {
				return fmt.Errorf("database config must be []map[string]interface{}")
			}

			var c mdb.DBConfig
			tool.StructFill(&m, &c)
			*dbcs = append(*dbcs, &c)
		}

	case map[string]any:
		if _, f := val["driver"]; f {
			var c mdb.DBConfig
			tool.StructFill(&val, &c)
			*dbcs = append(*dbcs, &c)
			return nil
		}

		if read, f := val["read"]; f {
			switch rval := read.(type) {
			case []any:
				for _, v := range rval {
					m, ok := v.(map[string]any)
					if !ok {
						return fmt.Errorf("read database config must be []map[string]interface{}")
					}

					var c mdb.DBConfig
					tool.StructFill(&m, &c)
					c.ReadOrWrite = mdb.ONLY_READ
					*dbcs = append(*dbcs, &c)
				}

			case map[string]any:
				var c mdb.DBConfig
				tool.StructFill(&rval, &c)
				c.ReadOrWrite = mdb.ONLY_READ
				*dbcs = append(*dbcs, &c)

			default:
				return fmt.Errorf("read database config is invalid")
			}
		}
		if write, f := val["write"]; f {
			switch wval := write.(type) {
			case []any:
				for _, v := range wval {
					m, ok := v.(map[string]any)
					if !ok {
						return fmt.Errorf("write database config must be []map[string]interface{}")
					}

					var c mdb.DBConfig
					tool.StructFill(&m, &c)
					c.ReadOrWrite = mdb.ONLY_WRITE
					*dbcs = append(*dbcs, &c)
				}

			case map[string]any:
				var c mdb.DBConfig
				tool.StructFill(&wval, &c)
				c.ReadOrWrite = mdb.ONLY_WRITE
				*dbcs = append(*dbcs, &c)

			default:
				fmt.Errorf("write database config is invalid")
			}
		}

	case string:
		if val[:5] == "https" {
			res, err := httputil.NewRequest().Get(val)
			if err != nil {
				return err
			}

			var data any
			if e := json.Unmarshal(res.Body, &data); e != nil {
				return e
			}
			parseDBConfig(data, dbcs)
		}

	default:
		fmt.Errorf("database config is invalid")
	}
	return nil
}
