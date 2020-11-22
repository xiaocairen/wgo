package mdb

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/xiaocairen/wgo/mdb/msql"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	READ_WRITE = 0
	ONLY_READ  = 1
	ONLY_WRITE = 2
	STRUCT_TAG = "mdb"
)

var (
	dbInstance *DB
	onceNewDB  sync.Once
)

type DBConfig struct {
	HostDBName      string `json:"host_db_name"`
	Driver          string `json:"driver"`
	Host            string `json:"host"`
	Port            int    `json:"port"`
	User            string `json:"user"`
	Pass            string `json:"pass"`
	Dbname          string `json:"dbname"`
	MaxOpenConns    int    `json:"max_open_conns"`
	MaxIdleConns    int    `json:"max_idle_conns"`
	ConnMaxLifetime int    `json:"conn_max_lifetime"`
	ReadOrWrite     int    `json:"read_or_write"`
}

type dbres struct {
	db     *sql.DB
	dbname string
	dsn    string
}

type DB struct {
	empty   bool
	alone   bool
	dres    *dbres
	rwres   []*dbres
	rres    []*dbres
	wres    []*dbres
	rwlen   int
	rlen    int
	wlen    int
	dynamic sync.Map
}

func NewDB(dbcs []*DBConfig, testPing bool) (*DB, error) {
	var err error
	onceNewDB.Do(func() {
		if nil == dbcs || 0 == len(dbcs) {
			dbInstance = &DB{empty: true}
			return
		}

		var (
			dres  *dbres
			rwres []*dbres
			rres  []*dbres
			wres  []*dbres
		)

		if 1 == len(dbcs) {
			db, dsn, e := openDB(dbcs[0], testPing)
			if e != nil {
				dbInstance = &DB{empty: true}
				err = e
				return
			}
			dres = &dbres{
				db:     db,
				dbname: dbcs[0].HostDBName,
				dsn:    dsn,
			}

			dbInstance = &DB{
				alone: true,
				dres:  dres,
				rwres: nil,
				rres:  nil,
				wres:  nil,
				rwlen: 0,
				rlen:  0,
				wlen:  0,
			}
		} else {
			for _, dbc := range dbcs {
				db, dsn, e := openDB(dbc, testPing)
				if e != nil {
					dbInstance = &DB{empty: true}
					err = e
					return
				}
				switch dbc.ReadOrWrite {
				case READ_WRITE:
					rwres = append(rwres, &dbres{
						db:     db,
						dbname: dbc.HostDBName,
						dsn:    dsn,
					})
				case ONLY_READ:
					rres = append(rres, &dbres{
						db:     db,
						dbname: dbc.HostDBName,
						dsn:    dsn,
					})
				case ONLY_WRITE:
					wres = append(wres, &dbres{
						db:     db,
						dbname: dbc.HostDBName,
						dsn:    dsn,
					})
				default:
					dbInstance = &DB{empty: true}
					err = fmt.Errorf("database tag read_or_write must be 0, 1, 2; 0:rw, 1:r, 2:w")
					return
				}
			}

			var (
				rwlen = len(rwres)
				wlen  = len(wres)
				rlen  = len(rres)
			)
			if rwlen > 0 {
				dres = rwres[0]
			} else if wlen > 0 {
				dres = wres[0]
			} else {
				dres = rres[0]
			}

			dbInstance = &DB{
				alone: false,
				dres:  dres,
				rwres: rwres,
				rres:  rres,
				wres:  wres,
				rwlen: rwlen,
				rlen:  rlen,
				wlen:  wlen,
			}
		}
	})
	return dbInstance, err
}

func openDB(dbc *DBConfig, testPing bool) (db *sql.DB, dsn string, err error) {
	if "" == dbc.Host || dbc.Port <= 0 {
		err = fmt.Errorf("db host empty or port '%d' invalid", dbc.Port)
		return
	}

	db, err = sql.Open(dbc.Driver, fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", dbc.User, dbc.Pass, dbc.Host, dbc.Port, dbc.Dbname))
	if err != nil {
		return
	}

	if testPing {
		if err = db.Ping(); err != nil {
			return
		}
	}

	if dbc.MaxOpenConns > 0 {
		db.SetMaxOpenConns(dbc.MaxOpenConns)
	}
	if dbc.MaxIdleConns > 0 {
		db.SetMaxIdleConns(dbc.MaxIdleConns)
	}
	if dbc.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(time.Duration(dbc.ConnMaxLifetime) * time.Second)
	}

	dsn = fmt.Sprintf("%s@tcp(%s:%d)/%s", dbc.User, dbc.Host, dbc.Port, dbc.Dbname)
	err = nil
	return
}

func (db *DB) selectRes(rw int, dbname string) *dbres {
	if db.alone {
		return db.dres
	}

	if "" == dbname {
		ts := int(time.Now().UnixNano() / 1000)
		if db.rwlen > 0 {
			return db.rwres[ts%db.rwlen]
		}
		switch rw {
		case ONLY_READ:
			if db.rlen > 0 {
				return db.rres[ts%db.rlen]
			}
			fallthrough
		case ONLY_WRITE:
			if db.wlen > 0 {
				return db.wres[ts%db.wlen]
			}
			return db.dres
		}
	} else {
		for _, r := range db.rwres {
			if r.dbname == dbname {
				return r
			}
		}

		switch rw {
		case ONLY_READ:
			for _, r := range db.rres {
				if r.dbname == dbname {
					return r
				}
			}
			fallthrough
		case ONLY_WRITE:
			for _, r := range db.wres {
				if r.dbname == dbname {
					return r
				}
			}
		}
	}
	return db.dres
}

func (db *DB) GetConn() (*Conn, error) {
	if db.empty {
		return nil, fmt.Errorf("no database connection resource")
	}

	if db.alone {
		return &Conn{
			rdb: db.dres,
			wdb: db.dres,
			one: true,
		}, nil
	} else {
		return &Conn{
			rdb: db.selectRes(ONLY_READ, ""),
			wdb: db.selectRes(ONLY_WRITE, ""),
		}, nil
	}
}

func (db *DB) GetConnByName(hostDBName string) (*Conn, error) {
	if db.empty {
		return nil, fmt.Errorf("no database connection resource")
	}

	if db.alone {
		return &Conn{
			rdb: db.dres,
			wdb: db.dres,
			one: true,
		}, nil
	} else {
		return &Conn{
			rdb: db.selectRes(ONLY_READ, hostDBName),
			wdb: db.selectRes(ONLY_WRITE, hostDBName),
		}, nil
	}
}

func (db *DB) NewConn(dbc *DBConfig) (*Conn, error) {
	if c, b := db.dynamic.Load(dbc.HostDBName); b {
		conn, _ := c.(*Conn)
		return conn, nil
	} else {
		sqlDB, dsn, err := openDB(dbc, true)
		if err != nil {
			return nil, err
		}

		var (
			r = &dbres{
				db:     sqlDB,
				dbname: dbc.HostDBName,
				dsn:    dsn,
			}
			c = &Conn{
				rdb: r,
				wdb: r,
				one: true,
			}
		)
		db.dynamic.Store(dbc.HostDBName, c)
		return c, nil
	}
}

// wrap select, insert, update, delete query
type selectQuery struct {
	res *dbres
	sql msql.SqlStatement
}

func (s selectQuery) Query() *Rows {
	if s.sql.Err != nil {
		return &Rows{lerr: s.sql.Err}
	}

	rows, err := s.res.db.Query(s.sql.Sql, s.sql.Params...)
	return &Rows{rows: rows, lerr: err}
}

func (s selectQuery) QueryRow() *Row {
	if s.sql.Err != nil {
		return &Row{rows: &Rows{lerr: s.sql.Err}}
	}

	rows, err := s.res.db.Query(s.sql.Sql, s.sql.Params...)
	return &Row{rows: &Rows{rows: rows, lerr: err}}
}

type modifyQuery struct {
	res *dbres
	sql msql.SqlStatement
}

func (m modifyQuery) Exec() (sql.Result, error) {
	if m.sql.Err != nil {
		return nil, m.sql.Err
	}

	return m.res.db.Exec(m.sql.Sql, m.sql.Params...)
}

type Conn struct {
	rdb *dbres
	wdb *dbres
	one bool
}

func (dc *Conn) Begin() *Tx {
	tx, err := dc.wdb.db.Begin()
	return &Tx{
		tx:   tx,
		lerr: err,
	}
}

func (dc *Conn) Select(sql msql.Select) *selectQuery {
	return &selectQuery{
		res: dc.rdb,
		sql: sql.Build(),
	}
}

func (dc *Conn) Insert(sql msql.Insert) *modifyQuery {
	return &modifyQuery{
		res: dc.wdb,
		sql: sql.Build(),
	}
}

func (dc *Conn) Update(sql msql.Update) *modifyQuery {
	return &modifyQuery{
		res: dc.wdb,
		sql: sql.Build(),
	}
}

func (dc *Conn) Delete(sql msql.Delete) *modifyQuery {
	return &modifyQuery{
		res: dc.wdb,
		sql: sql.Build(),
	}
}

func (dc *Conn) Exec(query string, args ...interface{}) (sql.Result, error) {
	return dc.wdb.db.Exec(query, args...)
}

func (dc *Conn) Prepare(query string) *dbStmt {
	if len(query) < 6 {
		return &dbStmt{lerr: fmt.Errorf("prepare sql too short '%s'", query)}
	}

	var (
		stmt *sql.Stmt
		err  error
	)
	if strings.Contains(strings.ToUpper(query[0:6]), "select") {
		stmt, err = dc.rdb.db.Prepare(query)
	} else {
		stmt, err = dc.wdb.db.Prepare(query)
	}
	return &dbStmt{stmt: stmt, lerr: err}
}

func (dc *Conn) Query(query string, args ...interface{}) *Rows {
	rows, err := dc.rdb.db.Query(query, args...)
	return &Rows{rows: rows, lerr: err}
}

func (dc *Conn) QueryRow(query string, args ...interface{}) *Row {
	rows, err := dc.rdb.db.Query(query, args...)
	return &Row{rows: &Rows{rows: rows, lerr: err}}
}

func (dc *Conn) GetDSN() string {
	if dc.one {
		return dc.wdb.dsn
	} else {
		return fmt.Sprintf("write=%s, read=%s", dc.wdb.dsn, dc.rdb.dsn)
	}
}

type dbStmt struct {
	stmt *sql.Stmt
	lerr error
}

func (s *dbStmt) Close() error {
	if s.lerr != nil {
		return s.lerr
	}
	return s.stmt.Close()
}

func (s *dbStmt) Exec(args ...interface{}) (sql.Result, error) {
	if s.lerr != nil {
		return nil, s.lerr
	}
	return s.stmt.Exec(args...)
}

func (s *dbStmt) Query(args ...interface{}) *Rows {
	if s.lerr != nil {
		return &Rows{lerr: s.lerr}
	}

	rows, err := s.stmt.Query(args...)
	return &Rows{rows: rows, lerr: err}
}

func (s *dbStmt) QueryRow(args ...interface{}) *Row {
	if s.lerr != nil {
		return &Row{rows: &Rows{lerr: s.lerr}}
	}

	rows, err := s.stmt.Query(args...)
	return &Row{rows: &Rows{rows: rows, lerr: err}}
}

type Row struct {
	rows *Rows
}

func (r *Row) Scan(dest ...interface{}) error {
	if r.rows.rows != nil {
		defer r.rows.rows.Close()
	}
	if r.rows.lerr != nil {
		return r.rows.lerr
	}

	if !r.rows.rows.Next() {
		if err := r.rows.rows.Err(); err != nil {
			return err
		}
		return sql.ErrNoRows
	}

	return r.rows.rows.Scan(dest...)
}

func (r *Row) ScanStruct(out interface{}) error {
	if r.rows.rows != nil {
		defer r.rows.rows.Close()
	}
	if r.rows.lerr != nil {
		return r.rows.lerr
	}

	if !r.rows.rows.Next() {
		if err := r.rows.rows.Err(); err != nil {
			return err
		}
		return sql.ErrNoRows
	}

	return r.rows.ScanStruct(out)
}

type Rows struct {
	rows *sql.Rows
	lerr error
}

func (r *Rows) Next() bool {
	if r.lerr != nil {
		return false
	}
	return r.rows.Next()
}

func (r *Rows) NextResultSet() bool {
	if r.lerr != nil {
		return false
	}
	return r.rows.NextResultSet()
}

func (r *Rows) Err() error {
	if r.lerr != nil {
		return r.lerr
	}
	return r.rows.Err()
}

func (r *Rows) Close() error {
	if r.lerr != nil {
		return r.lerr
	}
	return r.rows.Close()
}

func (r *Rows) Columns() ([]string, error) {
	if r.lerr != nil {
		return nil, r.lerr
	}
	return r.rows.Columns()
}

func (r *Rows) ColumnTypes() ([]*sql.ColumnType, error) {
	if r.lerr != nil {
		return nil, r.lerr
	}
	return r.rows.ColumnTypes()
}

func (r *Rows) Scan(dest ...interface{}) error {
	if r.lerr != nil {
		if r.rows != nil {
			r.rows.Close()
		}
		return r.lerr
	}
	return r.rows.Scan(dest...)
}

func (r *Rows) ScanStruct(out interface{}) error {
	if r.lerr != nil {
		if r.rows != nil {
			r.rows.Close()
		}
		return r.lerr
	}

	coltype, err := r.rows.ColumnTypes()
	if err != nil {
		r.rows.Close()
		return err
	}

	var (
		n      = len(coltype)
		ot     = reflect.TypeOf(out)
		ov     = reflect.ValueOf(out)
		ote    = ot.Elem()
		ove    = ov.Elem()
		fields = make([]reflect.StructField, n)
	)

	if ot.Kind() != reflect.Ptr || ote.Kind() != reflect.Struct {
		r.rows.Close()
		return fmt.Errorf("out must be ptr to struct")
	}
	if ov.IsNil() {
		r.rows.Close()
		return fmt.Errorf("out is nil")
	}

	for k, ct := range coltype {
		if sf, found := ote.FieldByName(ct.Name()); !found {
			for i := 0; i < ote.NumField(); i++ {
				f := ote.Field(i)
				if ct.Name() == f.Tag.Get(STRUCT_TAG) {
					fields[k] = f
					found = true
					break
				}
			}
			/*if !found {
				r.rows.Close()
				return fmt.Errorf("not found '%s' in struct", ct.Name())
			}*/
		} else {
			fields[k] = sf
		}

		if !ove.FieldByName(fields[k].Name).CanSet() {
			r.rows.Close()
			return fmt.Errorf("field '%s' in struct can't be set", fields[k].Name)
		}
	}

	return r.scanStruct(ote, ove, coltype, fields)
}

func (r *Rows) ScanStructAll(in interface{}) ([]interface{}, error) {
	if r.lerr != nil {
		if r.rows != nil {
			r.rows.Close()
		}
		return nil, r.lerr
	}
	defer r.rows.Close()

	coltype, err := r.rows.ColumnTypes()
	if err != nil {
		return nil, err
	}

	var (
		n      = len(coltype)
		ot     = reflect.TypeOf(in)
		ov     = reflect.ValueOf(in)
		ote    = ot.Elem()
		ove    = ov.Elem()
		fields = make([]reflect.StructField, n)
	)
	if ot.Kind() != reflect.Ptr || ote.Kind() != reflect.Struct {
		return nil, fmt.Errorf("param in must be ptr to struct")
	}
	if ov.IsNil() {
		return nil, fmt.Errorf("param in is nil")
	}

	for k, ct := range coltype {
		if sf, found := ote.FieldByName(ct.Name()); !found {
			for i := 0; i < ote.NumField(); i++ {
				f := ote.Field(i)
				if ct.Name() == f.Tag.Get(STRUCT_TAG) {
					fields[k] = f
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("not found '%s' in struct", ct.Name())
			}
		} else {
			fields[k] = sf
		}

		if !ove.FieldByName(fields[k].Name).CanSet() {
			return nil, fmt.Errorf("field '%s' in struct can't be set", fields[k].Name)
		}
	}

	var out = make([]interface{}, 0)
	for r.rows.Next() {
		tmp := reflect.New(ote).Interface()
		ote := reflect.TypeOf(tmp).Elem()
		ove := reflect.ValueOf(tmp).Elem()
		if err := r.scanStruct(ote, ove, coltype, fields); err != nil {
			return nil, err
		}
		out = append(out, tmp)
	}
	if err := r.rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Rows) scanStruct(ote reflect.Type, ove reflect.Value, coltype []*sql.ColumnType, fields []reflect.StructField) error {
	values := make([]interface{}, len(coltype))
	for k, ct := range coltype {
		switch ct.ScanType().Kind() {
		case reflect.Uint:
			values[k] = new(uint)
		case reflect.Uint8:
			values[k] = new(uint8)
		case reflect.Uint16:
			values[k] = new(uint16)
		case reflect.Uint32:
			values[k] = new(uint32)
		case reflect.Uint64:
			values[k] = new(uint64)
		case reflect.Int:
			values[k] = new(int)
		case reflect.Int8:
			values[k] = new(int8)
		case reflect.Int16:
			values[k] = new(int16)
		case reflect.Int32:
			values[k] = new(int32)
		case reflect.Int64:
			values[k] = new(int64)
		case reflect.Float32:
			values[k] = new(float32)
		case reflect.Float64:
			values[k] = new(float64)
		case reflect.Slice:
			fallthrough
		case reflect.String:
			values[k] = new(string)
		default:
			switch ct.ScanType().Name() {
			case "NullInt32":
				values[k] = new(sql.NullInt32)
			case "NullInt64":
				values[k] = new(sql.NullInt64)
			case "NullFloat64":
				values[k] = new(sql.NullFloat64)
			case "NullString":
				values[k] = new(sql.NullString)
			case "NullBool":
				values[k] = new(sql.NullBool)
			case "NullTime":
				values[k] = new(sql.NullTime)
			}
		}
	}

	if err := r.rows.Scan(values...); err != nil {
		return err
	}

	for k, f := range fields {
		fillStruct(ote, ove, f, values[k])
	}

	return nil
}

// database transaction wrape *sql.Tx
type Tx struct {
	tx   *sql.Tx
	lerr error
}

func (dt *Tx) Insert(sql msql.Insert) *txModifyQuery {
	return &txModifyQuery{
		tx:  dt,
		sql: sql.Build(),
	}
}

func (dt *Tx) Update(sql msql.Update) *txModifyQuery {
	return &txModifyQuery{
		tx:  dt,
		sql: sql.Build(),
	}
}

func (dt *Tx) Delete(sql msql.Delete) *txModifyQuery {
	return &txModifyQuery{
		tx:  dt,
		sql: sql.Build(),
	}
}

func (dt *Tx) Prepare(query string) *dbStmt {
	stmt, err := dt.tx.Prepare(query)
	return &dbStmt{stmt: stmt, lerr: err}
}

func (dt *Tx) Commit() error {
	if dt.lerr != nil {
		return dt.lerr
	}
	return dt.tx.Commit()
}

func (dt *Tx) Rollback() error {
	if dt.lerr != nil {
		return dt.lerr
	}
	return dt.tx.Rollback()
}

type txModifyQuery struct {
	tx  *Tx
	sql msql.SqlStatement
}

func (m txModifyQuery) Exec() (sql.Result, error) {
	if nil != m.sql.Err {
		return nil, m.sql.Err
	}

	return m.tx.tx.Exec(m.sql.Sql, m.sql.Params...)
}

const (
	INT8_MIN   = -128
	INT8_MAX   = 127
	INT16_MIN  = -32768
	INT16_MAX  = 32767
	INT32_MIN  = -2147483648
	INT32_MAX  = 2147483647
	INT64_MIN  = -9223372036854775808
	INT64_MAX  = 9223372036854775807
	UINT8_MAX  = 255
	UINT16_MAX = 65535
	UINT32_MAX = 4294967295
	UINT64_MAX = 18446744073709551615
)

func fillStruct(ote reflect.Type, ove reflect.Value, field reflect.StructField, value interface{}) {
	org := ove.FieldByName(field.Name)
	val := reflect.ValueOf(value).Elem()
	orgType := org.Type()
	valType := val.Type()
	if valType.AssignableTo(orgType) {
		org.Set(val)
	} else {
		valKind := valType.Kind()
		switch orgType.Kind() {
		case reflect.Int:
			switch valKind {
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				org.Set(reflect.ValueOf(int(reflect.Indirect(reflect.ValueOf(value)).Uint())))
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				org.Set(reflect.ValueOf(int(reflect.Indirect(reflect.ValueOf(value)).Int())))
			case reflect.String:
				if s, ok := value.(*string); ok {
					if i, e := strconv.Atoi(*s); e == nil {
						org.Set(reflect.ValueOf(i))
					}
				}
			case reflect.Struct:
				switch valType.Name() {
				case "NullInt32":
					v, _ := value.(*sql.NullInt32)
					if v.Valid {
						org.Set(reflect.ValueOf(int(v.Int32)))
					}
				case "NullInt64":
					v, _ := value.(*sql.NullInt64)
					if v.Valid {
						org.Set(reflect.ValueOf(int(v.Int64)))
					}
				case "NullFloat64":
				case "NullString":
					v, _ := value.(*sql.NullString)
					if v.Valid {
						if i, e := strconv.Atoi(v.String); e == nil {
							org.Set(reflect.ValueOf(i))
						}
					}
				case "NullBool":
					v, _ := value.(*sql.NullBool)
					if v.Valid {
						if v.Bool {
							org.Set(reflect.ValueOf(1))
						} else {
							org.Set(reflect.ValueOf(0))
						}
					}
				}
			}

		case reflect.Int8:
			switch valKind {
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				i := reflect.Indirect(reflect.ValueOf(value)).Uint()
				if i <= INT8_MAX {
					org.Set(reflect.ValueOf(int8(i)))
				}
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				i := reflect.Indirect(reflect.ValueOf(value)).Int()
				if i >= INT8_MIN && i <= INT8_MAX {
					org.Set(reflect.ValueOf(int8(i)))
				}
			case reflect.String:
				if s, ok := value.(*string); ok {
					if i, e := strconv.Atoi(*s); e == nil && i >= INT8_MIN && i <= INT8_MAX {
						org.Set(reflect.ValueOf(int8(i)))
					}
				}
			case reflect.Struct:
				switch valType.Name() {
				case "NullInt32":
					v, _ := value.(*sql.NullInt32)
					if v.Valid {
						if v.Int32 >= INT8_MIN && v.Int32 <= INT8_MAX {
							org.Set(reflect.ValueOf(int8(v.Int32)))
						}
					}
				case "NullInt64":
					v, _ := value.(*sql.NullInt64)
					if v.Valid {
						if v.Int64 >= INT8_MIN && v.Int64 <= INT8_MAX {
							org.Set(reflect.ValueOf(int8(v.Int64)))
						}
					}
				case "NullFloat64":
				case "NullString":
					v, _ := value.(*sql.NullString)
					if v.Valid {
						if i, e := strconv.Atoi(v.String); e == nil && i >= INT8_MIN && i <= INT8_MAX {
							org.Set(reflect.ValueOf(int8(i)))
						}
					}
				case "NullBool":
					v, _ := value.(*sql.NullBool)
					if v.Valid {
						if v.Bool {
							org.Set(reflect.ValueOf(1))
						} else {
							org.Set(reflect.ValueOf(0))
						}
					}
				}
			}

		case reflect.Int16:
			switch valKind {
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				i := reflect.Indirect(reflect.ValueOf(value)).Uint()
				if i <= INT16_MAX {
					org.Set(reflect.ValueOf(int16(i)))
				}
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				i := reflect.Indirect(reflect.ValueOf(value)).Int()
				if i >= INT16_MIN && i <= INT16_MAX {
					org.Set(reflect.ValueOf(int16(i)))
				}
			case reflect.String:
				if s, ok := value.(*string); ok {
					if i, e := strconv.Atoi(*s); e == nil && i >= INT16_MIN && i <= INT16_MAX {
						org.Set(reflect.ValueOf(int16(i)))
					}
				}
			case reflect.Struct:
				switch valType.Name() {
				case "NullInt32":
					v, _ := value.(*sql.NullInt32)
					if v.Valid {
						if v.Int32 >= INT16_MIN && v.Int32 <= INT16_MAX {
							org.Set(reflect.ValueOf(int16(v.Int32)))
						}
					}
				case "NullInt64":
					v, _ := value.(*sql.NullInt64)
					if v.Valid {
						if v.Int64 >= INT16_MIN && v.Int64 <= INT16_MAX {
							org.Set(reflect.ValueOf(int16(v.Int64)))
						}
					}
				case "NullString":
					v, _ := value.(*sql.NullString)
					if v.Valid {
						if i, e := strconv.Atoi(v.String); e == nil && i >= INT16_MIN && i <= INT16_MAX {
							org.Set(reflect.ValueOf(int16(i)))
						}
					}
				case "NullBool":
					v, _ := value.(*sql.NullBool)
					if v.Valid {
						if v.Bool {
							org.Set(reflect.ValueOf(1))
						} else {
							org.Set(reflect.ValueOf(0))
						}
					}
				}
			}

		case reflect.Int32:
			switch valKind {
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				i := reflect.Indirect(reflect.ValueOf(value)).Uint()
				if i <= INT32_MAX {
					org.Set(reflect.ValueOf(int32(i)))
				}
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				i := reflect.Indirect(reflect.ValueOf(value)).Int()
				if i >= INT32_MIN && i <= INT32_MAX {
					org.Set(reflect.ValueOf(int32(i)))
				}
			case reflect.String:
				if s, ok := value.(*string); ok {
					if i, e := strconv.Atoi(*s); e == nil && i >= INT32_MIN && i <= INT32_MAX {
						org.Set(reflect.ValueOf(int32(i)))
					}
				}
			case reflect.Struct:
				switch valType.Name() {
				case "NullInt32":
					v, _ := value.(*sql.NullInt32)
					if v.Valid {
						org.Set(reflect.ValueOf(v.Int32))
					}
				case "NullInt64":
					v, _ := value.(*sql.NullInt64)
					if v.Valid {
						if v.Int64 >= INT32_MIN && v.Int64 <= INT32_MAX {
							org.Set(reflect.ValueOf(int32(v.Int64)))
						}
					}
				case "NullString":
					v, _ := value.(*sql.NullString)
					if v.Valid {
						if i, e := strconv.Atoi(v.String); e == nil {
							org.Set(reflect.ValueOf(int32(i)))
						}
					}
				case "NullBool":
					v, _ := value.(*sql.NullBool)
					if v.Valid {
						if v.Bool {
							org.Set(reflect.ValueOf(1))
						} else {
							org.Set(reflect.ValueOf(0))
						}
					}
				}
			}

		case reflect.Int64:
			switch valKind {
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				org.Set(reflect.ValueOf(int64(reflect.Indirect(reflect.ValueOf(value)).Uint())))
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				org.Set(reflect.ValueOf(reflect.Indirect(reflect.ValueOf(value)).Int()))
			case reflect.String:
				if s, ok := value.(*string); ok {
					if i, e := strconv.Atoi(*s); e == nil {
						org.Set(reflect.ValueOf(int64(i)))
					}
				}
			case reflect.Struct:
				switch valType.Name() {
				case "NullInt32":
					v, _ := value.(*sql.NullInt32)
					if v.Valid {
						org.Set(reflect.ValueOf(v.Int32))
					}
				case "NullInt64":
					v, _ := value.(*sql.NullInt64)
					if v.Valid {
						org.Set(reflect.ValueOf(v.Int64))
					}
				case "NullString":
					v, _ := value.(*sql.NullString)
					if v.Valid {
						if i, e := strconv.Atoi(v.String); e == nil {
							org.Set(reflect.ValueOf(i))
						}
					}
				case "NullBool":
					v, _ := value.(*sql.NullBool)
					if v.Valid {
						if v.Bool {
							org.Set(reflect.ValueOf(1))
						} else {
							org.Set(reflect.ValueOf(0))
						}
					}
				}
			}

		case reflect.Uint:
			switch valKind {
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				org.Set(reflect.ValueOf(uint(reflect.Indirect(reflect.ValueOf(value)).Uint())))
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				i := reflect.Indirect(reflect.ValueOf(value)).Int()
				if i >= 0 {
					org.Set(reflect.ValueOf(uint(i)))
				}
			case reflect.String:
				if s, ok := value.(*string); ok {
					if i, e := strconv.Atoi(*s); e == nil && i >= 0 {
						org.Set(reflect.ValueOf(uint(i)))
					}
				}
			case reflect.Struct:
				switch valType.Name() {
				case "NullInt32":
					v, _ := value.(*sql.NullInt32)
					if v.Valid && v.Int32 >= 0 {
						org.Set(reflect.ValueOf(uint(v.Int32)))
					}
				case "NullInt64":
					v, _ := value.(*sql.NullInt64)
					if v.Valid && v.Int64 >= 0 {
						org.Set(reflect.ValueOf(uint(v.Int64)))
					}
				case "NullString":
					v, _ := value.(*sql.NullString)
					if v.Valid {
						if i, e := strconv.Atoi(v.String); e == nil && i >= 0 {
							org.Set(reflect.ValueOf(uint(i)))
						}
					}
				case "NullBool":
					v, _ := value.(*sql.NullBool)
					if v.Valid {
						if v.Bool {
							org.Set(reflect.ValueOf(1))
						} else {
							org.Set(reflect.ValueOf(0))
						}
					}
				}
			}

		case reflect.Uint8:
			switch valKind {
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				i := reflect.Indirect(reflect.ValueOf(value)).Uint()
				if i <= UINT8_MAX {
					org.Set(reflect.ValueOf(uint8(i)))
				}
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				i := reflect.Indirect(reflect.ValueOf(value)).Int()
				if i >= 0 && i <= UINT8_MAX {
					org.Set(reflect.ValueOf(uint8(i)))
				}
			case reflect.String:
				if s, ok := value.(*string); ok {
					if i, e := strconv.Atoi(*s); e == nil && i >= 0 && i <= UINT8_MAX {
						org.Set(reflect.ValueOf(uint8(i)))
					}
				}
			case reflect.Struct:
				switch valType.Name() {
				case "NullInt32":
					v, _ := value.(*sql.NullInt32)
					if v.Valid && v.Int32 >= 0 && v.Int32 <= UINT8_MAX {
						org.Set(reflect.ValueOf(uint8(v.Int32)))
					}
				case "NullInt64":
					v, _ := value.(*sql.NullInt64)
					if v.Valid && v.Int64 >= 0 && v.Int64 <= UINT8_MAX {
						org.Set(reflect.ValueOf(uint8(v.Int64)))
					}
				case "NullString":
					v, _ := value.(*sql.NullString)
					if v.Valid {
						if i, e := strconv.Atoi(v.String); e == nil && i > 0 && i <= UINT8_MAX {
							org.Set(reflect.ValueOf(uint8(i)))
						}
					}
				case "NullBool":
					v, _ := value.(*sql.NullBool)
					if v.Valid {
						if v.Bool {
							org.Set(reflect.ValueOf(1))
						} else {
							org.Set(reflect.ValueOf(0))
						}
					}
				}
			}

		case reflect.Uint16:
			switch valKind {
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				i := reflect.Indirect(reflect.ValueOf(value)).Uint()
				if i <= UINT16_MAX {
					org.Set(reflect.ValueOf(uint16(i)))
				}
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				i := reflect.Indirect(reflect.ValueOf(value)).Int()
				if i >= 0 && i <= UINT16_MAX {
					org.Set(reflect.ValueOf(uint16(i)))
				}
			case reflect.String:
				if s, ok := value.(*string); ok {
					if i, e := strconv.Atoi(*s); e == nil && i >= 0 && i <= UINT16_MAX {
						org.Set(reflect.ValueOf(uint16(i)))
					}
				}
			case reflect.Struct:
				switch valType.Name() {
				case "NullInt32":
					v, _ := value.(*sql.NullInt32)
					if v.Valid && v.Int32 >= 0 && v.Int32 <= UINT16_MAX {
						org.Set(reflect.ValueOf(uint16(v.Int32)))
					}
				case "NullInt64":
					v, _ := value.(*sql.NullInt64)
					if v.Valid && v.Int64 >= 0 && v.Int64 <= UINT16_MAX {
						org.Set(reflect.ValueOf(uint16(v.Int64)))
					}
				case "NullString":
					v, _ := value.(*sql.NullString)
					if v.Valid {
						if i, e := strconv.Atoi(v.String); e == nil && i >= 0 && i <= UINT16_MAX {
							org.Set(reflect.ValueOf(uint16(i)))
						}
					}
				case "NullBool":
					v, _ := value.(*sql.NullBool)
					if v.Valid {
						if v.Bool {
							org.Set(reflect.ValueOf(1))
						} else {
							org.Set(reflect.ValueOf(0))
						}
					}
				}
			}

		case reflect.Uint32:
			switch valKind {
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				i := reflect.Indirect(reflect.ValueOf(value)).Uint()
				if i <= UINT32_MAX {
					org.Set(reflect.ValueOf(uint32(i)))
				}
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				i := reflect.Indirect(reflect.ValueOf(value)).Int()
				if i >= 0 && i <= UINT32_MAX {
					org.Set(reflect.ValueOf(uint32(i)))
				}
			case reflect.String:
				if s, ok := value.(*string); ok {
					if i, e := strconv.Atoi(*s); e == nil && i >= 0 && i <= UINT32_MAX {
						org.Set(reflect.ValueOf(uint32(i)))
					}
				}
			case reflect.Struct:
				switch valType.Name() {
				case "NullInt32":
					v, _ := value.(*sql.NullInt32)
					if v.Valid && v.Int32 >= 0 {
						org.Set(reflect.ValueOf(uint32(v.Int32)))
					}
				case "NullInt64":
					v, _ := value.(*sql.NullInt64)
					if v.Valid && v.Int64 >= 0 && v.Int64 <= UINT32_MAX {
						org.Set(reflect.ValueOf(uint32(v.Int64)))
					}
				case "NullFloat64":
				case "NullString":
					v, _ := value.(*sql.NullString)
					if v.Valid {
						if i, e := strconv.Atoi(v.String); e == nil && i >= 0 {
							org.Set(reflect.ValueOf(uint32(i)))
						}
					}
				case "NullBool":
					v, _ := value.(*sql.NullBool)
					if v.Valid {
						if v.Bool {
							org.Set(reflect.ValueOf(1))
						} else {
							org.Set(reflect.ValueOf(0))
						}
					}
				}
			}

		case reflect.Uint64:
			switch valKind {
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				org.Set(reflect.ValueOf(reflect.Indirect(reflect.ValueOf(value)).Uint()))
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				i := reflect.Indirect(reflect.ValueOf(value)).Int()
				if i >= 0 {
					org.Set(reflect.ValueOf(uint64(i)))
				}
			case reflect.String:
				if s, ok := value.(*string); ok {
					if i, e := strconv.Atoi(*s); e == nil {
						org.Set(reflect.ValueOf(uint64(i)))
					}
				}
			case reflect.Struct:
				switch valType.Name() {
				case "NullInt32":
					v, _ := value.(*sql.NullInt32)
					if v.Valid && v.Int32 >= 0 {
						org.Set(reflect.ValueOf(uint64(v.Int32)))
					}
				case "NullInt64":
					v, _ := value.(*sql.NullInt64)
					if v.Valid && v.Int64 >= 0 {
						org.Set(reflect.ValueOf(uint64(v.Int64)))
					}
				case "NullString":
					v, _ := value.(*sql.NullString)
					if v.Valid {
						if i, e := strconv.Atoi(v.String); e == nil && i >= 0 {
							org.Set(reflect.ValueOf(uint64(i)))
						}
					}
				case "NullBool":
					v, _ := value.(*sql.NullBool)
					if v.Valid {
						if v.Bool {
							org.Set(reflect.ValueOf(1))
						} else {
							org.Set(reflect.ValueOf(0))
						}
					}
				}
			}

		case reflect.Float32:
			switch valKind {
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				org.Set(reflect.ValueOf(float32(reflect.Indirect(reflect.ValueOf(value)).Uint())))
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				org.Set(reflect.ValueOf(float32(reflect.Indirect(reflect.ValueOf(value)).Int())))
			case reflect.Float32, reflect.Float64:
				org.Set(reflect.ValueOf(float32(reflect.Indirect(reflect.ValueOf(value)).Float())))
			case reflect.String:
				if s, ok := value.(*string); ok {
					if f, e := strconv.ParseFloat(*s, 32); e == nil {
						org.Set(reflect.ValueOf(f))
					}
				}
			case reflect.Struct:
				switch valType.Name() {
				case "NullInt32":
					v, _ := value.(*sql.NullInt32)
					if v.Valid {
						org.Set(reflect.ValueOf(float32(v.Int32)))
					}
				case "NullInt64":
					v, _ := value.(*sql.NullInt64)
					if v.Valid {
						org.Set(reflect.ValueOf(float32(v.Int64)))
					}
				case "NullFloat64":
					v, _ := value.(*sql.NullFloat64)
					if v.Valid {
						org.Set(reflect.ValueOf(float32(v.Float64)))
					}
				case "NullString":
					v, _ := value.(*sql.NullString)
					if v.Valid {
						if f, e := strconv.ParseFloat(v.String, 32); e == nil {
							org.Set(reflect.ValueOf(f))
						}
					}
				case "NullBool":
					v, _ := value.(*sql.NullBool)
					if v.Valid {
						if v.Bool {
							org.Set(reflect.ValueOf(1))
						} else {
							org.Set(reflect.ValueOf(0))
						}
					}
				}
			}

		case reflect.Float64:
			switch valKind {
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				org.Set(reflect.ValueOf(float64(reflect.Indirect(reflect.ValueOf(value)).Uint())))
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				org.Set(reflect.ValueOf(float64(reflect.Indirect(reflect.ValueOf(value)).Int())))
			case reflect.Float32, reflect.Float64:
				org.Set(reflect.ValueOf(reflect.Indirect(reflect.ValueOf(value)).Float()))
			case reflect.String:
				if s, ok := value.(*string); ok {
					if f, e := strconv.ParseFloat(*s, 64); e == nil {
						org.Set(reflect.ValueOf(f))
					}
				}
			case reflect.Struct:
				switch valType.Name() {
				case "NullInt32":
					v, _ := value.(*sql.NullInt32)
					if v.Valid {
						org.Set(reflect.ValueOf(float64(v.Int32)))
					}
				case "NullInt64":
					v, _ := value.(*sql.NullInt64)
					if v.Valid {
						org.Set(reflect.ValueOf(float64(v.Int64)))
					}
				case "NullFloat64":
					v, _ := value.(*sql.NullFloat64)
					if v.Valid {
						org.Set(reflect.ValueOf(v.Float64))
					}
				case "NullString":
					v, _ := value.(*sql.NullString)
					if v.Valid {
						if f, e := strconv.ParseFloat(v.String, 64); e == nil {
							org.Set(reflect.ValueOf(f))
						}
					}
				case "NullBool":
					v, _ := value.(*sql.NullBool)
					if v.Valid {
						if v.Bool {
							org.Set(reflect.ValueOf(1))
						} else {
							org.Set(reflect.ValueOf(0))
						}
					}
				}
			}

		case reflect.String:
			switch valKind {
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				org.Set(reflect.ValueOf(strconv.FormatUint(reflect.Indirect(reflect.ValueOf(value)).Uint(), 64)))
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				org.Set(reflect.ValueOf(strconv.FormatInt(reflect.Indirect(reflect.ValueOf(value)).Int(), 64)))
			case reflect.Float32, reflect.Float64:
				org.Set(reflect.ValueOf(strconv.FormatFloat(reflect.Indirect(reflect.ValueOf(value)).Float(), 'f', -1, 64)))
			case reflect.Struct:
				switch valType.Name() {
				case "NullInt32":
					v, _ := value.(*sql.NullInt32)
					if v.Valid {
						org.Set(reflect.ValueOf(strconv.Itoa(int(v.Int32))))
					}
				case "NullInt64":
					v, _ := value.(*sql.NullInt64)
					if v.Valid {
						org.Set(reflect.ValueOf(strconv.FormatInt(v.Int64, 10)))
					}
				case "NullFloat64":
					v, _ := value.(*sql.NullFloat64)
					if v.Valid {
						org.Set(reflect.ValueOf(strconv.FormatFloat(v.Float64, 'f', 6, 64)))
					}
				case "NullString":
					v, _ := value.(*sql.NullString)
					if v.Valid {
						org.Set(reflect.ValueOf(v.String))
					}
				case "NullBool":
					v, _ := value.(*sql.NullBool)
					if v.Valid {
						if v.Bool {
							org.Set(reflect.ValueOf("true"))
						} else {
							org.Set(reflect.ValueOf("false"))
						}
					}
				}
			}
		}
	}
}
