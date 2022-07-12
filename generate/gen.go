package generate

import (
	"database/sql"
	"fmt"
	"github.com/xiaocairen/wgo/tool"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

func Run() {
	if len(os.Args) < 3 {
		return
	}

	switch os.Args[1] {
	case "init":
		NewProjectGenerater(os.Args[2]).genProject()
	case "gentable":
		NewTtableGenerater().genTable()
	default:
		fmt.Printf("not support cmd \"%s\"", os.Args[1])
	}
}

func NewTtableGenerater() *tableGenerater {
	var (
		user     = GetCmdOption("user", "root")
		pass     = GetCmdOption("pass", "")
		host     = GetCmdOption("host", "localhost")
		port     = GetCmdOption("port", "3306")
		dbname   = GetCmdOption("db", "")
		tag      = GetCmdOption("tag", "mdb")
		table    = GetCmdOption("table", "")
		jsonType = GetCmdOption("jsonType", "0") // 0 同数据库字段名(小写加下划线) 1小写开头驼峰 2大写开头驼峰
		yamlType = GetCmdOption("yamlType", "0")
	)

	db, e := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", user, pass, host, port, dbname))
	if nil != e {
		panic(e)
	}
	if err := db.Ping(); nil != err {
		panic(err)
	}

	return &tableGenerater{
		db:       db,
		tag:      tag,
		table:    table,
		jsonType: jsonType,
		yamlType: yamlType,
	}
}

type tableGenerater struct {
	db       *sql.DB
	tag      string
	table    string
	jsonType string
	yamlType string
}

func (this *tableGenerater) genTable() {
	rows, err := this.db.Query("SHOW TABLES")
	if nil != err {
		panic(err)
	}

	tlen := len(this.table)

	for rows.Next() {
		var t string
		if err := rows.Scan(&t); nil != err {
			panic(err)
		}

		if 0 == tlen || this.table == t {
			code := this.findTableFields(t)
			this.genGoFile(t+".go", code)
		}
	}
}

func (this *tableGenerater) genGoFile(filename, code string) {
	var dir = "entity"
	if _, err := os.Stat(dir); nil != err {
		if !os.IsExist(err) {
			if err := os.Mkdir("entity", os.ModePerm); nil != err {
				panic(err)
			}
		}
	}

	if err := ioutil.WriteFile(dir+"/"+filename, []byte(code), os.ModePerm); nil != err {
		panic(err)
	}
}

func (this *tableGenerater) findTableFields(table string) string {
	rows, err := this.db.Query("SHOW COLUMNS FROM " + table)
	if nil != err {
		panic(err)
	}

	structName := tool.Underline2Camel(table)
	code := `package entity

type ` + structName + ` struct {
`

	var (
		fieldStructs []*fieldStruct
		maxNameLen   = 0
		maxTypeLen   = 0
	)
	for rows.Next() {
		var (
			Field   string
			Type    string
			Null    string
			Key     string
			Default interface{}
			Extra   string
		)

		if err = rows.Scan(&Field, &Type, &Null, &Key, &Default, &Extra); nil != err {
			panic(err)
		}
		fs := this.genField(table, Field, Type, Null, Key, Default, Extra)
		if fs.NameLen > maxNameLen {
			maxNameLen = fs.NameLen
		}
		if fs.TypeLen > maxTypeLen {
			maxTypeLen = fs.TypeLen
		}
		fieldStructs = append(fieldStructs, fs)
	}

	for _, fs := range fieldStructs {
		code += fmt.Sprintf("    %-"+strconv.Itoa(maxNameLen)+"s %-"+strconv.Itoa(maxTypeLen)+"s %s\n", fs.Name, fs.Type, fs.Tag)
	}

	code += "}"

	return code
}

type fieldStruct struct {
	Name    string
	Type    string
	Tag     string
	NameLen int
	TypeLen int
}

func (this *tableGenerater) genField(table, field, typ, null, key string, def interface{}, extra string) *fieldStruct {
	fs := &fieldStruct{}
	fs.Name = tool.Underline2Camel(field)

	pos := strings.IndexByte(typ, '(')
	if -1 == pos {
		if strings.Contains(typ, "float") || strings.Contains(typ, "double") {
			fs.Type = "float64"
		} else {
			switch typ {
			case "char", "varchar", "tinytext", "mediumtext", "text", "longtext", "date", "datetime", "time", "year":
				fs.Type = "string"
			case "tinyint", "smallint", "mediumint", "int", "bigint":
				fs.Type = "int64"
			case "timestamp":
				fs.Type = "int64"
			case "decimal", "float", "double":
				fs.Type = "float64"
			case "enum":
				fs.Type = "string"
			default:
				panic(fmt.Sprintf("unkown field type '%s' at %s->%s", typ, table, field))
			}
		}
	} else {
		switch typ[:pos] {
		case "char", "varchar", "tinytext", "mediumtext", "text", "longtext", "date", "datetime", "time", "year":
			fs.Type = "string"
		case "tinyint", "smallint", "mediumint", "int", "bigint":
			fs.Type = "int64"
		case "timestamp":
			fs.Type = "int64"
		case "decimal", "float", "double":
			fs.Type = "float64"
		case "enum":
			fs.Type = "string"
		default:
			panic(fmt.Sprintf("unkown field type '%s' at %s->%s", typ, table, field))
		}
	}

	var (
		jsonName string
		yamlName string
	)
	switch this.jsonType {
	case "1":
		jsonName = tool.ToFirstLower(fs.Name)
	case "2":
		jsonName = tool.ToFirstUpper(fs.Name)
	default:
		jsonName = field
	}
	switch this.yamlType {
	case "1":
		yamlName = tool.ToFirstLower(fs.Name)
	case "2":
		yamlName = tool.ToFirstUpper(fs.Name)
	default:
		yamlName = field
	}

	if "PRI" == key {
		fs.Tag = fmt.Sprintf("`json:\"%s\" yaml:\"%s\" %s:\"%s\" pk:\"yes\"`", jsonName, yamlName, this.tag, field)
	} else {
		fs.Tag = fmt.Sprintf("`json:\"%s\" yaml:\"%s\" %s:\"%s\"`", jsonName, yamlName, this.tag, field)
	}

	fs.NameLen = len(fs.Name)
	fs.TypeLen = len(fs.Type)
	return fs
}

func GetCmdOption(k, def string) string {
	if len(os.Args) == 1 {
		return ""
	}

	var (
		klen = len(k)
		alen = len(os.Args[1:])
		args = make([]string, alen)
	)
	copy(args, os.Args[1:])

	switch klen {
	case 1:
		k = "-" + k
	case 2:
		if !strings.HasPrefix(k, "-") {
			k = "--" + k
		}
	default:
		if !strings.HasPrefix(k, "--") {
			k = "--" + k
		}
	}

	for i := 0; i < alen; {
		if args[i] == k {
			if i < alen {
				return args[i+1]
			} else {
				return def
			}
		}
		if strings.HasPrefix(args[i], k) && strings.Contains(args[i], "=") {
			ss := strings.SplitN(args[i], "=", 2)
			return strings.TrimSpace(ss[1])
		}
		i++
	}

	return def
}

type projectGenerater struct {
	projectName   string
	dirLdb        string
	dirLog        string
	dirMod        string
	dirSvc        string
	dirController string
	dirAggr       string
	dirEntity     string
	dirTask       string
	dirXutil      string
	dirLeveldb    string
}

func NewProjectGenerater(name string) *projectGenerater {
	var (
		ldb     = name + "/ldb"
		log     = name + "/log"
		mod     = name + "/mod"
		svc     = name + "/svc"
		ctl     = mod + "/controller"
		aggr    = svc + "/aggr"
		entity  = svc + "/entity"
		task    = svc + "/task"
		xutil   = svc + "/xutil"
		leveldb = xutil + "/leveldb"
	)

	return &projectGenerater{
		projectName:   name,
		dirLdb:        ldb,
		dirLog:        log,
		dirMod:        mod,
		dirSvc:        svc,
		dirController: ctl,
		dirAggr:       aggr,
		dirEntity:     entity,
		dirTask:       task,
		dirXutil:      xutil,
		dirLeveldb:    leveldb,
	}
}

func (this *projectGenerater) genProject() {
	if e := this.genProjectDir(); e != nil {
		panic(e)
	} else {
		this.genProjectFile()
	}
}

func (this *projectGenerater) genProjectDir() error {
	if _, err := os.Stat(this.projectName); nil != err {
		if !os.IsExist(err) {
			if err := os.Mkdir(this.projectName, os.ModePerm); nil != err {
				panic(err)
			}
		}
	}

	if _, err := os.Stat(this.dirLdb); nil != err {
		if !os.IsExist(err) {
			if err = os.Mkdir(this.dirLdb, os.ModePerm); nil != err {
				return err
			}
		}
	}
	if _, err := os.Stat(this.dirLog); nil != err {
		if !os.IsExist(err) {
			if err = os.Mkdir(this.dirLog, os.ModePerm); nil != err {
				return err
			}
		}
	}
	if _, err := os.Stat(this.dirMod); nil != err {
		if !os.IsExist(err) {
			if err = os.Mkdir(this.dirMod, os.ModePerm); nil != err {
				return err
			}
		}
	}
	if _, err := os.Stat(this.dirSvc); nil != err {
		if !os.IsExist(err) {
			if err = os.Mkdir(this.dirSvc, os.ModePerm); nil != err {
				return err
			}
		}
	}
	if _, err := os.Stat(this.dirController); nil != err {
		if !os.IsExist(err) {
			if err = os.Mkdir(this.dirController, os.ModePerm); nil != err {
				return err
			}
		}
	}
	if _, err := os.Stat(this.dirAggr); nil != err {
		if !os.IsExist(err) {
			if err = os.Mkdir(this.dirAggr, os.ModePerm); nil != err {
				return err
			}
		}
	}
	if _, err := os.Stat(this.dirEntity); nil != err {
		if !os.IsExist(err) {
			if err = os.Mkdir(this.dirEntity, os.ModePerm); nil != err {
				return err
			}
		}
	}
	if _, err := os.Stat(this.dirTask); nil != err {
		if !os.IsExist(err) {
			if err = os.Mkdir(this.dirTask, os.ModePerm); nil != err {
				return err
			}
		}
	}
	if _, err := os.Stat(this.dirXutil); nil != err {
		if !os.IsExist(err) {
			if err = os.Mkdir(this.dirXutil, os.ModePerm); nil != err {
				return err
			}
		}
	}
	if _, err := os.Stat(this.dirLeveldb); nil != err {
		if !os.IsExist(err) {
			if err = os.Mkdir(this.dirLeveldb, os.ModePerm); nil != err {
				return err
			}
		}
	}

	return nil
}

func (this *projectGenerater) genProjectFile() error {
	if e := this.genProjectMainFile(); e != nil {
		return e
	}
	if e := this.genProjectControllerFile(); e != nil {
		return e
	}
	if e := this.genProjectAggrFile(); e != nil {
		return e
	}
	if e := this.genProjectRouterFile(); e != nil {
		return e
	}
	if e := this.genProjectEntityFile(); e != nil {
		return e
	}
	if e := this.genProjectTaskerFile(); e != nil {
		return e
	}
	if e := this.genProjectUtilFile(); e != nil {
		return e
	}
	if e := this.genProjectLevelDBFile(); e != nil {
		return e
	}
	if e := this.genProjectAppJsonFile(); e != nil {
		return e
	}

	return nil
}

func (this *projectGenerater) genProjectMainFile() error {
	code := `package main

import (
	"github.com/xiaocairen/wgo"
	_ "` + this.projectName + `/mod"
	_ "` + this.projectName + `/svc/entity"
	_ "` + this.projectName + `/svc/task"
)

func main() {
	wgo.GetApp().Run()
}
`
	if err := ioutil.WriteFile(this.projectName+"/main.go", []byte(code), os.ModePerm); nil != err {
		return err
	}

	return nil
}

func (this *projectGenerater) genProjectControllerFile() error {
	code := `package controller

import (
	"github.com/xiaocairen/wgo"
	"` + this.projectName + `/svc/aggr"
)

type BaseController struct {
	wgo.WgoController
	Aggr *aggr.Aggregate
}

func (this *BaseController) Test() []byte {
	return this.Success("hello wgo")
}
`
	if err := ioutil.WriteFile(this.dirController+"/BaseController.go", []byte(code), os.ModePerm); nil != err {
		return err
	}

	return nil
}

func (this *projectGenerater) genProjectRouterFile() error {
	code := `package mod

import (
	"github.com/xiaocairen/wgo"
	"` + this.projectName + `/mod/controller"
)

func init() {
	wgo.GetApp().SetRouteCollection(RouterCollection)
}

func RouterCollection(r *wgo.RouteRegister) {
	var (
		base = &controller.BaseController{}
	)

	r.Registe("", "", nil, func(um wgo.UnitHttpMethod, m wgo.HttpMethod) {
		m.Get("/test", base, "Test()")
	})
}
`
	if err := ioutil.WriteFile(this.dirMod+"/router.go", []byte(code), os.ModePerm); nil != err {
		return err
	}

	return nil
}

func (this *projectGenerater) genProjectAggrFile() error {
	code := `package aggr

import (
	"github.com/xiaocairen/wgo"
	"github.com/xiaocairen/wgo/service"
	"log"
	"` + this.projectName + `/svc/xutil/leveldb"
	"reflect"
	"strconv"
)

func init() {
	var (
		useCache bool
		lifes    = make(map[string]any)
		lifeMap  = make(map[string]int64)
		app      = wgo.GetApp()
	)

	if e := app.GetConfigurator().GetBool("leveldb.use", &useCache); e != nil {
		panic(e)
	}
	if e := app.GetConfigurator().GetMap("leveldb.life", &lifes); e != nil {
		panic(e)
	}

	for k, v := range lifes {
		i, ok := v.(float64)
		if !ok {
			log.Panicf("leveldb.life '%s' value is not int", k)
		}
		lifeMap[k] = int64(i)
	}

	app.AddRequestControllerInjector(&aggregateInjector{
		a: &Aggregate{ldb: leveldb.NewLevelDB(useCache, lifeMap)},
	})
}

// -------------------------------------------------------------------------
// 封装service对数据表的操作, 注入到controller中
// -------------------------------------------------------------------------
type Aggregate struct {
	service *service.Service
	ldb     *leveldb.WgoLevelDB
}

func (this *Aggregate) Reset(svc *service.Service, ldb *leveldb.WgoLevelDB) {
	this.service = svc
	this.ldb = ldb
}

func (this *Aggregate) ResetLevelDB(ldb *leveldb.WgoLevelDB) {
	this.ldb = ldb
}

func (this *Aggregate) ResetService(svc *service.Service) {
	this.service = svc
}

func (this *Aggregate) LevelDB() *leveldb.WgoLevelDB {
	return this.ldb
}

func (this *Aggregate) GetObjectFromLDB(dbname string, key int64, obj any) error {
	return this.ldb.GetDB(dbname).GetStruct([]byte(strconv.FormatInt(key, 10)), obj)
}

func (this *Aggregate) PutObjectIntoLDB(dbname string, key int64, obj any) error {
	return this.ldb.GetDB(dbname).PutStruct([]byte(strconv.FormatInt(key, 10)), obj)
}

func (this *Aggregate) DelObjectFromLDB(dbname string, key int64) error {
	return this.ldb.GetDB(dbname).Delete([]byte(strconv.FormatInt(key, 10)))
}

type aggregateInjector struct {
	a *Aggregate
}

func (ai *aggregateInjector) InjectRequestController(router wgo.Router, cve reflect.Value, svc *service.Service) {
	f, ok := reflect.TypeOf(router.Controller).Elem().FieldByName("Aggr")
	if !ok {
		return
	}
	if f.Type.Kind() != reflect.Ptr || f.Type.Elem().Kind() != reflect.Struct || "*aggr.Aggregate" != f.Type.String() {
		return
	}

	ai.a.service = svc
	dst := cve.FieldByName("Aggr")
	if dst.IsNil() && dst.CanSet() {
		src := reflect.ValueOf(ai.a)
		if src.Type().AssignableTo(dst.Type()) {
			dst.Set(src)
		}
	}
}
`
	if err := ioutil.WriteFile(this.dirAggr+"/aggr.go", []byte(code), os.ModePerm); nil != err {
		return err
	}
	return nil
}

func (this *projectGenerater) genProjectEntityFile() error {
	code := `package entity

import (
	"github.com/xiaocairen/wgo"
	"github.com/xiaocairen/wgo/service"
)

func init() {
	wgo.GetApp().SetTableCollection(TableCollection)
}

func TableCollection(tr *service.TableRegister) {
	// 注册数据库表
	tr.RegisteTables([]any{

	})
}
`
	if err := ioutil.WriteFile(this.dirEntity+"/entity.go", []byte(code), os.ModePerm); nil != err {
		return err
	}
	return nil
}

func (this *projectGenerater) genProjectTaskerFile() error {
	code := `package task

import (
	"github.com/xiaocairen/wgo"
	"github.com/xiaocairen/wgo/service"
	"` + this.projectName + `/svc/aggr"
	"` + this.projectName + `/svc/xutil/leveldb"
)

var taskerAggregater *aggr.Aggregate

func init() {
	app := wgo.GetApp()

	var openTasker bool
	if e := app.GetConfigurator().GetBool("open_tasker", &openTasker); e != nil {
		panic(e)
	}

	//app.AddTasker(taskExample1).AddTasker(taskExample2)
	//app.AddWebsocketHandler("/wss-chat", chatWebsocketExample)
}

func getAggr(svc *service.Service) *aggr.Aggregate {
	if nil == taskerAggregater {
		var ldb = leveldb.NewLevelDB(true, nil)
		taskerAggregater = &aggr.Aggregate{}
		taskerAggregater.Reset(svc, ldb)
	}

	return taskerAggregater
}
`
	if err := ioutil.WriteFile(this.dirTask+"/tasker.go", []byte(code), os.ModePerm); nil != err {
		return err
	}
	return nil
}

func (this *projectGenerater) genProjectUtilFile() error {
	code := `package xutil

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/nfnt/resize"
	"github.com/xiaocairen/wgo/tool"
	"golang.org/x/crypto/bcrypt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"math/big"
	"math/rand"
	"mime/multipart"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	DATE_LAYOUT_0     = "2006-01-02"
	DATETIME_LAYOUT_0 = "2006-01-02 15:04:05"
	DATE_LAYOUT       = "2006-1-2"
	DATETIME_LAYOUT   = "2006-1-2 15:04:05"
)

var (
	LOCATION = time.FixedZone("CST", 8*3600)
)

type encryptor struct {
	key []byte
}

func NewEncryptor(key []byte) *encryptor {
	if len(key) < 7 {
		return &encryptor{key: []byte("wgo^2099_abc1234")}
	}
	return &encryptor{key: append([]byte("wgo!2099_"), key[0:7]...)}
}

func (enc *encryptor) Encode(data []byte) string {
	return enc.UrlEncode(data)
}

func (enc *encryptor) Decode(data string) ([]byte, error) {
	return enc.UrlDecode(data)
}

func (enc *encryptor) StdEncode(data []byte) string {
	s, _ := tool.Encrypt(data, enc.key, nil)
	return s
}

func (enc *encryptor) StdDecode(data string) ([]byte, error) {
	return tool.Decrypt(data, enc.key, nil)
}

func (enc *encryptor) UrlEncode(data []byte) string {
	eda, _ := tool.AesCBCEncrypt(data, enc.key, nil)
	return base64.RawURLEncoding.EncodeToString(eda)
}

func (enc *encryptor) UrlDecode(data string) ([]byte, error) {
	tmpData, err := base64.RawURLEncoding.DecodeString(data)
	if err != nil {
		return nil, err
	}

	decData, err := tool.AesCBCDecrypt(tmpData, enc.key, nil)
	if err != nil {
		return nil, err
	}
	return decData, nil
}

type password struct {
	defCost int
}

func NewPassword() *password {
	return &password{defCost: 10}
}

func (enc *password) EncodePassword(password []byte) string {
	resBytes, _ := bcrypt.GenerateFromPassword(password, enc.defCost)
	return string(resBytes)
}

func (enc *password) VerifyPassword(password []byte, cryptedPass string) bool {
	if e := bcrypt.CompareHashAndPassword([]byte(cryptedPass), password); e != nil {
		return false
	}
	return true
}

/*type password struct {
	key []byte
}

func NewPassword(key []byte) *password {
	if len(key) == 0 {
		return &password{key: []byte("wgo^2099_abc1234")}
	}

	return &password{key: key}
}

func (enc *password) EncodePassword(password []byte) string {
	h := hmac.New(sha256.New, enc.key)
	h.Write(password)
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

func (enc *password) VerifyPassword(password []byte, cryptedPass string) bool {
	h := hmac.New(sha256.New, enc.key)
	h.Write(password)
	target := h.Sum(nil)
	src, _ := base64.RawURLEncoding.DecodeString(cryptedPass)
	return hmac.Equal(src, target)
}*/

func CreateUUID(name string) string {
	var ns = uuid.New()
	if name == "" {
		return strings.Replace(ns.String(), "-", "", 4)
	}
	return strings.Replace(uuid.NewSHA1(ns, []byte(name)).String(), "-", "", 4)
}

func IsDuplicateError(e error) bool {
	return -1 != strings.Index(e.Error(), "Error 1062: Duplicate entry")
}

func IsMobilePhone(phone string) bool {
	var yes bool
	if len(phone) == 11 && phone[0] == 49 {
		yes = true
	}
	return yes
}

func DateFormat(sec int64, layout string) string {
	if sec == 0 {
		sec = time.Now().Unix()
	}
	if layout == "" {
		layout = DATE_LAYOUT_0
	}
	return time.Unix(sec, 0).In(LOCATION).Format(layout)
}

func TimeFormat(sec int64, layout string) string {
	if sec == 0 {
		sec = time.Now().Unix()
	}
	if layout == "" {
		layout = DATETIME_LAYOUT_0
	}
	return time.Unix(sec, 0).In(LOCATION).Format(layout)
}

func TimeParse(datetime, layout string) (time.Time, error) {
	return time.ParseInLocation(layout, datetime, LOCATION)
}

func WeekDay(t time.Time) int64 {
	d := t.Weekday()
	s := []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday, time.Sunday}
	for k, w := range s {
		if w == d {
			return int64(k)
		}
	}
	return -1
}

func CurrentWeekDay() int64 {
	return WeekDay(time.Now().In(LOCATION))
}

func MonthFirstTime(year, month int) time.Time {
	var (
		y int
		m time.Month
	)
	if year == 0 || month == 0 {
		y, m, _ = time.Now().Date()
	} else {
		t, _ := time.ParseInLocation(DATETIME_LAYOUT, fmt.Sprintf("%d-%d-1 15:04:05", year, month), LOCATION)
		y, m, _ = t.Date()
	}
	return time.Date(y, m, 1, 0, 0, 0, 0, LOCATION)
}

func MonthLastTime(year, month int) time.Time {
	var (
		y int
		m time.Month
	)
	if year == 0 || month == 0 {
		y, m, _ = time.Now().Date()
	} else {
		t, _ := time.ParseInLocation(DATETIME_LAYOUT, fmt.Sprintf("%d-%d-1 15:04:05", year, month), LOCATION)
		y, m, _ = t.Date()
	}

	ft := time.Date(y, m, 1, 23, 59, 59, 0, LOCATION)
	return ft.AddDate(0, 1, -1)
}

func InetNtoA(ip int64) string {
	return fmt.Sprintf("%d.%d.%d.%d",
		byte(ip>>24), byte(ip>>16), byte(ip>>8), byte(ip))
}

func InetAtoN(ip string) int64 {
	ret := big.NewInt(0)
	ret.SetBytes(net.ParseIP(ip).To4())
	return ret.Int64()
}

func RandomInt(min int, max int, randint int64) int {
	rand.Seed(time.Now().UnixNano() + randint)
	return min + rand.Intn(max-min)
}

func FormatMoney(money float64) float64 {
	val, _ := strconv.ParseFloat(strconv.FormatFloat(money, 'f', 2, 64), 64)
	return val
}

func NonceStr(len int) string {
	tmp := make([]string, len)
	str := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	for i := 0; i < len; i++ {
		k := rand.Intn(62)
		tmp[i] = str[k : k+1]
	}

	return strings.Join(tmp, "")
}

func CreateSN(prefix string) string {
	var (
		now     = time.Now().In(LOCATION)
		nowsec  = strconv.FormatInt(now.Unix(), 10)
		nownano = strconv.FormatInt(now.UnixNano(), 10)
		nanolen = len(nownano)
		y, _    = strconv.ParseInt(now.Format("2006"), 10, 64)
	)
	y = y % 26

	return fmt.Sprintf("%s%s%s%s%s%02d", prefix, string(y+65),
		now.Format("0102"),
		nowsec[len(nowsec)-5:],
		nownano[nanolen-6:nanolen-2],
		rand.Intn(99))
}

func CreateJSONBody(data interface{}, escapeHtml bool) (string, error) {
	var (
		buf = bytes.NewBuffer([]byte{})
		enc = json.NewEncoder(buf)
	)
	enc.SetEscapeHTML(escapeHtml)
	if e := enc.Encode(data); e != nil {
		return "", e
	}

	return strings.TrimRight(buf.String(), "\n"), nil
}

func ContainString(ss []string, s string) bool {
	if !sort.StringsAreSorted(ss) {
		sort.Strings(ss)
	}

	return InSortedString(ss, s)
}

func InSortedString(sortedStr []string, s string) bool {
	var pos = sort.SearchStrings(sortedStr, s)
	if pos == len(sortedStr) || sortedStr[pos] != s {
		return false
	}
	return true
}

func ContainInt(is []int, x int) bool {
	if !sort.IntsAreSorted(is) {
		sort.Ints(is)
	}

	return InSortedInts(is, x)
}

func InSortedInts(sortedInts []int, x int) bool {
	var pos = sort.SearchInts(sortedInts, x)
	if pos == len(sortedInts) || sortedInts[pos] != x {
		return false
	}
	return true
}

func DiffIntSlice(newIds, oldIds []int64) (n_ids, d_ids []int64) {
	for _, newId := range newIds {
		var f bool
		for _, oldId := range oldIds {
			if oldId == newId {
				f = true
				break
			}
		}
		if !f {
			n_ids = append(n_ids, newId)
		}
	}
	for _, oldId := range oldIds {
		var f bool
		for _, newId := range newIds {
			if oldId == newId {
				f = true
				break
			}
		}
		if !f {
			d_ids = append(d_ids, oldId)
		}
	}
	return
}

func Descartes(src [][]string) [][]string {
	var (
		num = len(src)
		ret [][]string
	)
	for _, val := range src[0] {
		ret = append(ret, []string{val})
	}

	for i := 0; i < num-1; i++ {
		var arr [][]string
		for _, vals := range ret {
			for _, val := range src[i+1] {
				var tmp []string
				tmp = append(tmp, vals...)
				tmp = append(tmp, val)
				arr = append(arr, tmp)
			}
		}
		ret = arr
	}
	return ret
}

func FileExist(file string) bool {
	if _, err := os.Stat(file); nil != err {
		return os.IsExist(err)
	}
	return true
}

func UploadFile(file multipart.File, header *multipart.FileHeader, dir, filename string, imgOnly bool) (string, error) {
	var ext string
	typ := header.Header.Get("Content-Type")
	switch typ {
	case "image/png":
		ext = "png"
	case "image/jpeg", "image/jpg":
		ext = "jpg"
	case "image/gif":
		ext = "gif"
	case "image/webp":
		ext = "webp"
	case "image/bmp":
		ext = "bmp"
	default:
		if imgOnly {
			return "", fmt.Errorf("not support file format %s", typ)
		}
		pos := strings.LastIndexByte(header.Filename, '.')
		if pos != -1 {
			ext = header.Filename[pos+1:]
		}
	}

	if dir == "" {
		dir = fmt.Sprintf("./web/upload/%s", DateFormat(0, "200601"))
	} else {
		dir = fmt.Sprintf("./web/%s", strings.Trim(dir, "/"))
	}

	if e := os.MkdirAll(dir, os.ModePerm); e != nil {
		return "", fmt.Errorf("目录创建失败, %s", e.Error())
	}

	var dst string
	if filename == "" {
		dst = fmt.Sprintf("%s/W%d.%s", dir, time.Now().UnixNano()/1000, ext)
	} else {
		dst = fmt.Sprintf("%s/%s.%s", dir, filename, ext)
	}

	f, e := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY, 0644)
	if e != nil {
		return "", e
	}
	defer f.Close()

	file.Seek(0, 0)
	buf := make([]byte, 10240)
	for {
		n, e := file.Read(buf)
		if n > 0 {
			if l, e := f.Write(buf[:n]); e != nil {
				panic(e)
			} else if l != n {
				panic("写入失败")
			}
		}
		if e != nil {
			if e == io.EOF {
				break
			}
			panic(e)
		}
	}

	return dst[5:], nil
}

func ResizeImage(file string, width uint, override bool) (string, uint, uint, error) {
	fd, err := os.OpenFile(file, os.O_RDONLY, 0644)
	if err != nil {
		return "", 0, 0, err
	}
	defer fd.Close()

	typ, err := tool.ImageFormatCheck(file)
	if err != nil {
		return "", 0, 0, err
	}

	var (
		i image.Image
		e error
	)
	switch typ {
	case "jpg":
		i, e = jpeg.Decode(fd)
	case "png":
		i, e = png.Decode(fd)
	case "gif":
		i, e = gif.Decode(fd)
	default:
		return "", 0, 0, fmt.Errorf("not support image format %s", typ)
	}
	if e != nil {
		return "", 0, 0, e
	}

	size := i.Bounds().Size()
	if uint(size.X) <= width {
		return file, uint(size.X), uint(size.Y), nil
	}

	var (
		h       = uint(size.Y) * width / uint(size.X)
		pos     = strings.LastIndexByte(file, '.')
		newfile string
	)
	if pos != -1 {
		newfile = fmt.Sprintf("%s_%dx%d.%s", file[:pos], width, h, typ)
	} else {
		newfile = fmt.Sprintf("%s_%dx%d.%s", file, width, h, typ)
	}

	out := resize.Resize(width, h, i, resize.Lanczos3)

	outfile, err := os.Create(newfile)
	if err != nil {
		return "", 0, 0, err
	}
	defer outfile.Close()

	switch typ {
	case "jpg":
		e = jpeg.Encode(outfile, out, &jpeg.Options{Quality: 100})
	case "png":
		e = png.Encode(outfile, out)
	case "gif":
		e = gif.Encode(outfile, out, &gif.Options{NumColors: 256})
	}
	if e != nil {
		return "", 0, 0, e
	}

	if override {
		fd.Close()
		tmpfile := file + ".tmp"
		if e := os.Rename(file, tmpfile); e != nil {
			os.Remove(newfile)
			return "", 0, 0, e
		}
		if e := os.Rename(newfile, file); e != nil {
			os.Remove(newfile)
			os.Rename(tmpfile, file)
			return "", 0, 0, e
		}
		if e := os.Remove(tmpfile); e != nil {
			os.Remove(file)
			os.Rename(tmpfile, file)
			return "", 0, 0, e
		}
		return file, width, h, nil
	}

	return newfile, width, h, nil
}
`
	if err := ioutil.WriteFile(this.dirXutil+"/util.go", []byte(code), os.ModePerm); nil != err {
		return err
	}
	return nil
}

func (this *projectGenerater) genProjectLevelDBFile() error {
	code := `package leveldb

import (
	"encoding/binary"
	"fmt"
	"github.com/syndtr/goleveldb/leveldb"
	"gopkg.in/yaml.v2"
	"log"
	"sync"
	"time"
)

var (
	ldbInstance *WgoLevelDB
	onceNewLDB  sync.Once
	ErrNoUse    = fmt.Errorf("leveldb no used")
	ErrExpired  = fmt.Errorf("cache expired")
)

type ldb struct {
	dbname string
	db     *leveldb.DB
	life   int64
	err    error
}

type WgoLevelDB struct {
	nodb *ldb
	ldbs []*ldb
	use  bool
}

func NewLevelDB(use bool, lifeMap map[string]int64) *WgoLevelDB {
	onceNewLDB.Do(func() {
		ldbInstance = &WgoLevelDB{use: use}
		if !use {
			ldbInstance.nodb = &ldb{err: ErrNoUse}
			return
		}

		for dbname, life := range lifeMap {
			if life <= 0 {
				life = 0
			} else if life < 60 {
				life = 60
			}

			db, err := leveldb.OpenFile("ldb/"+dbname, nil)
			if err != nil {
				log.Printf("leveldb open ldb/%s failed, %s", dbname, err.Error())
				continue
			}

			ldbInstance.ldbs = append(ldbInstance.ldbs, &ldb{
				dbname: dbname,
				db:     db,
				life:   life,
				err:    nil,
			})
		}
	})

	return ldbInstance
}

func (d *WgoLevelDB) GetDB(dbname string) *ldb {
	if !d.use {
		return d.nodb
	}

	for _, db := range d.ldbs {
		if dbname == db.dbname {
			return db
		}
	}

	log.Printf("no leveldb ldb/%s", dbname)
	return d.nodb
}

func (d *WgoLevelDB) Close() {
	for _, ldb := range d.ldbs {
		ldb.db.Close()
	}
}

func (l *ldb) DB() *leveldb.DB {
	return l.db
}

func (l *ldb) Get(key []byte) ([]byte, error) {
	if l.err != nil {
		return nil, l.err
	}

	r, e := l.db.Get(key, nil)
	if e != nil {
		return nil, e
	}

	return l.getDataBytes(r)
}

func (l *ldb) GetStruct(key []byte, target any) error {
	if l.err != nil {
		return l.err
	}

	r, e := l.db.Get(key, nil)
	if e != nil {
		return e
	}

	d, e := l.getDataBytes(r)
	if e != nil {
		return e
	}
	return yaml.Unmarshal(d, target)
}

func (l *ldb) Put(key, value []byte) error {
	if l.err != nil {
		return nil
	}

	d := l.setDataBytes(value)
	return l.db.Put(key, d, nil)
}

func (l *ldb) PutStruct(key []byte, target any) error {
	if l.err != nil {
		return nil
	}

	j, err := yaml.Marshal(target)
	if err != nil {
		return err
	}

	d := l.setDataBytes(j)
	return l.db.Put(key, d, nil)
}

func (l *ldb) Delete(key []byte) error {
	if l.err != nil {
		return nil
	}

	return l.db.Delete(key, nil)
}

func (l *ldb) Has(key []byte) (bool, error) {
	if l.err != nil {
		return false, nil
	}

	return l.db.Has(key, nil)
}

func (l *ldb) setDataBytes(target []byte) []byte {
	d := make([]byte, len(target)+8)
	if l.life > 0 {
		copy(d[0:8], l.uint64tobytes(uint64(time.Now().Unix()+l.life)))
	}
	copy(d[8:], target)

	return d
}

func (l *ldb) getDataBytes(data []byte) ([]byte, error) {
	if l.life > 0 && time.Now().Unix() > l.bytes2int64(data[0:8]) {
		return nil, ErrExpired
	}

	return data[8:], nil
}

func (l *ldb) uint64tobytes(i uint64) []byte {
	s := make([]byte, 8)
	binary.BigEndian.PutUint64(s, i)
	return s
}

func (l *ldb) bytes2int64(b []byte) int64 {
	return int64(binary.BigEndian.Uint64(b))
}
`
	if err := ioutil.WriteFile(this.dirLeveldb+"/leveldb.go", []byte(code), os.ModePerm); nil != err {
		return err
	}
	return nil
}

func (this *projectGenerater) genProjectAppJsonFile() error {
	code := `{
  "site": "` + this.projectName + `",
  "debug": true,
  "log_outer": 1,
  "http": {
    "addr": "127.0.0.1",
    "port": 8888,
    "use_websocket": false
  },
  "inet_real_ip": "8.8.8.8",
  "file_host": {
    "img_host": "https://imghost.example.com/",
    "oss_host": "https://imghost.oss-cn-beijing.aliyuncs.com/",
    "upf_host": "https://api.example.com"
  },
  "leveldb": {
    "use": false,
    "life": {
      "wxdata_vo": 600,
      "wechat_certificate": 30000,
      "util_dataset": 0
    }
  },
  "db_test_ping": false,
  "database": {
    "host_db_name": "default",
    "driver": "mysql",
    "host": "127.0.0.1",
    "port": 3306,
    "user": "root",
    "pass": "123456",
    "dbname": "` + this.projectName + `",
    "max_open_conns": 100,
    "max_idle_conns": 10,
    "conn_max_lifetime": 30
  },
  "open_tasker": false
}
`
	if err := ioutil.WriteFile(this.projectName+"/app.json", []byte(code), os.ModePerm); nil != err {
		return err
	}
	return nil
}
