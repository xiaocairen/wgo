package service

import (
	"database/sql"
	"fmt"
	"github.com/xiaocairen/wgo/mdb"
	"github.com/xiaocairen/wgo/mdb/msql"
	"reflect"
	"strings"
	"sync"
)

const (
	MKEY        = "mkey"
	FKEY        = "fkey"
	ASSOC_TABLE = "table"
	ASSOC_ORDER = "order"
)

var (
	servicerInst    *Servicer
	onceNewServicer sync.Once
)

type table struct {
	target       interface{}
	targetType   reflect.Type
	structName   string
	structFields []reflect.StructField
	tableName    string
	tableFields  []string
	primaryKey   string
	primaryField reflect.StructField
}

// Servicer
type Servicer struct {
	db     *mdb.DB
	tables []*table
}

func NewServicer(db *mdb.DB) *Servicer {
	var first = false
	onceNewServicer.Do(func() {
		first = true
		servicerInst = &Servicer{db: db}
	})

	if first {
		return servicerInst
	} else {
		return nil
	}
}

func (s *Servicer) Registe(tc TableCollection) {
	if len(s.tables) == 0 && nil != tc {
		tc.call(&TableRegister{svcer: s})
	}
}

func (s *Servicer) New() *Service {
	c, e := s.db.GetConn()
	return &Service{
		db:     s.db,
		conn:   c,
		tx:     nil,
		in:     false,
		tables: s.tables,
		err:    e,
	}
}

// Service
type Service struct {
	db     *mdb.DB
	conn   *mdb.Conn
	tx     *mdb.Tx
	in     bool
	tables []*table
	err    error
}

func (s *Service) NewService() *Service {
	c, e := s.db.GetConn()
	return &Service{
		db:     s.db,
		conn:   c,
		tx:     nil,
		in:     false,
		tables: s.tables,
		err:    e,
	}
}

func (s *Service) NewServiceByHostname(hostDbame string) *Service {
	c, e := s.db.GetConnByName(hostDbame)
	return &Service{
		db:     s.db,
		conn:   c,
		tx:     nil,
		in:     false,
		tables: s.tables,
		err:    e,
	}
}

func (s *Service) NewConnService(config *mdb.DBConfig) *Service {
	c, e := s.db.NewConn(config)
	return &Service{
		db:     s.db,
		conn:   c,
		tx:     nil,
		in:     false,
		tables: s.tables,
		err:    e,
	}
}

func (s *Service) Conn() *mdb.Conn {
	if s.err != nil {
		panic(s.err)
	}
	return s.conn
}

func (s *Service) SelectDbHost(hostname string) {
	s.conn, s.err = s.db.GetConnByName(hostname)
}

func (s *Service) Begin() {
	if s.err != nil || s.in {
		return
	}

	s.tx = s.conn.Begin()
	s.in = true
}

func (s *Service) InTransaction() bool {
	return s.in
}

func (s *Service) Commit() error {
	if !s.in {
		return nil
	}

	err := s.tx.Commit()
	s.in = false
	s.tx = nil
	return err
}

func (s *Service) Rollback() error {
	if !s.in {
		return nil
	}

	err := s.tx.Rollback()
	s.in = false
	s.tx = nil
	return err
}

// only LoadPaginator has param selection and call LoadPageTarget,
// the target can be nil; otherwise target must be ptr to struct.
func (s *Service) New(target interface{}) *svc {
	if s.err != nil {
		return &svc{err: s.err}
	}

	var (
		dbt *table
		typ reflect.Type
		err error
	)
	if nil != target {
		dbt, typ, err = s.loadTableByTarget(target)
		if nil != err {
			return &svc{err: err}
		}
	}

	return &svc{
		conn:       s.conn,
		service:    s,
		table:      dbt,
		target:     target,
		targetType: typ,
		err:        nil,
	}
}

func (s *Service) loadTableByTarget(target interface{}) (*table, reflect.Type, error) {
	var (
		targetType = reflect.TypeOf(target)
		structName = targetType.String()
		dbtable    = s.loadTableByName(structName)
	)
	if dbtable == nil {
		return nil, nil, fmt.Errorf("not found target '%s' in table register", structName)
	}
	return dbtable, targetType.Elem(), nil
}

func (s *Service) loadTableByName(structName string) *table {
	for _, t := range s.tables {
		if t.structName == structName {
			return t
		}
	}
	return nil
}

// svc
type svc struct {
	conn       *mdb.Conn
	service    *Service
	table      *table
	target     interface{}
	targetType reflect.Type
	err        error
}

func (s *svc) Create() error {
	if s.err != nil {
		s.service.Rollback()
		return s.err
	}

	var (
		fv, rv, _ = s.getFieldValues()
		res       sql.Result
		err       error
		insert    = msql.Insert{
			Into:       s.table.tableName,
			FieldValue: fv,
		}
	)
	if s.service.in {
		res, err = s.service.tx.Insert(insert).Exec()
	} else {
		res, err = s.conn.Insert(insert).Exec()
	}
	if err != nil {
		s.service.Rollback()
		return err
	}

	id, _ := res.LastInsertId()
	rv.FieldByName(s.table.primaryField.Name).Set(reflect.ValueOf(id))
	return nil
}

func (s *svc) CreateOnDupkey(dupkey map[string]string) error {
	if s.err != nil {
		s.service.Rollback()
		return s.err
	}

	var (
		fv, rv, _ = s.getFieldValues()
		res       sql.Result
		err       error
		insert    = msql.Insert{
			Into:       s.table.tableName,
			FieldValue: fv,
			OnDKUpdate: dupkey,
		}
	)
	if s.service.in {
		res, err = s.service.tx.Insert(insert).Exec()
	} else {
		res, err = s.conn.Insert(insert).Exec()
	}
	if err != nil {
		s.service.Rollback()
		return err
	}

	id, _ := res.LastInsertId()
	rv.FieldByName(s.table.primaryField.Name).Set(reflect.ValueOf(id))
	return nil
}

func (s *svc) CreateMulti(data []map[string]interface{}) (num int64, err error) {
	if s.err != nil {
		s.service.Rollback()
		err = s.err
		return
	}

	num = int64(len(data))
	if nil == data || 0 == num {
		s.service.Rollback()
		err = fmt.Errorf("param data of CreateMulti() is empty")
		return
	}

	var fields []string
	for k, _ := range data[0] {
		fields = append(fields, k)
	}

	var (
		query     = fmt.Sprintf("INSERT INTO `%s` (`%s`) VALUES ", s.table.tableName, strings.Join(fields, "`, `"))
		allValues = make([]string, num)
	)
	for k, m := range data {
		var (
			holder []string
			values []interface{}
		)
		for _, f := range fields {
			var val = m[f]
			if sv, yes := val.(string); yes {
				values = append(values, strings.ReplaceAll(strings.ReplaceAll(sv, "\\", "\\\\"), "'", "\\'"))
			} else {
				values = append(values, m[f])
			}
			holder = append(holder, "'%v'")
		}

		allValues[k] = fmt.Sprintf("("+strings.Join(holder, ",")+")", values...)
	}
	query = query + strings.Join(allValues, ",")

	if s.service.in {
		_, err = s.service.tx.Exec(query)
	} else {
		_, err = s.conn.Exec(query)
	}
	if err != nil {
		s.service.Rollback()
		return 0, err
	}
	return
}

func (s *svc) CreateMultiOnDupkey(data []map[string]interface{}, dupkey map[string]string) (num int64, err error) {
	if s.err != nil {
		s.service.Rollback()
		err = s.err
		return
	}

	num = int64(len(data))
	if nil == data || 0 == num {
		s.service.Rollback()
		err = fmt.Errorf("param data of CreateMulti() is empty")
		return
	}

	var fields []string
	for k, _ := range data[0] {
		fields = append(fields, k)
	}

	var (
		query     = fmt.Sprintf("INSERT INTO `%s` (`%s`) VALUES ", s.table.tableName, strings.Join(fields, "`, `"))
		allValues = make([]string, num)
	)
	for k, m := range data {
		var (
			holder []string
			values []interface{}
		)
		for _, f := range fields {
			var val = m[f]
			if sv, yes := val.(string); yes {
				values = append(values, strings.ReplaceAll(strings.ReplaceAll(sv, "\\", "\\\\"), "'", "\\'"))
			} else {
				values = append(values, m[f])
			}
			holder = append(holder, "'%v'")
		}

		allValues[k] = fmt.Sprintf("("+strings.Join(holder, ",")+")", values...)
	}
	query = query + strings.Join(allValues, ",")

	if nil != dupkey {
		var udk []string
		for k, v := range dupkey {
			udk = append(udk, fmt.Sprintf("%s=%s", k, strings.ReplaceAll(strings.ReplaceAll(v, "\\", "\\\\"), "'", "\\'")))
		}
		query = query + fmt.Sprintf(" ON DUPLICATE KEY UPDATE %s", strings.Join(udk, ","))
	}

	if s.service.in {
		_, err = s.service.tx.Exec(query)
	} else {
		_, err = s.conn.Exec(query)
	}
	if err != nil {
		s.service.Rollback()
		return 0, err
	}
	return
}

func (s *svc) CreateMultiAny(data []interface{}) (num int64, err error) {
	if s.err != nil {
		s.service.Rollback()
		err = s.err
		return
	}

	num = int64(len(data))
	if nil == data || 0 == num {
		s.service.Rollback()
		err = fmt.Errorf("param data of CreateMultiAny() is empty")
		return
	}

	var (
		fields    = s.GetTableFields()
		query     = fmt.Sprintf("INSERT INTO `%s` (`%s`) VALUES ", s.table.tableName, strings.Join(fields, "`, `"))
		allValues = make([]string, num)
	)
	for k, m := range data {
		var (
			holder []string
			values []interface{}
			vref   = reflect.ValueOf(m)
		)
		if vref.Type().Kind() == reflect.Ptr {
			vref = vref.Elem()
			if vref.Type().Kind() != reflect.Struct {
				s.service.Rollback()
				return 0, fmt.Errorf("CreateMultiAny param data element must be struct or ptr to struct")
			}
		}

		for i := 0; i < vref.NumField(); i++ {
			var val = vref.Field(i).Interface()
			if sv, yes := val.(string); yes {
				values = append(values, strings.ReplaceAll(strings.ReplaceAll(sv, "\\", "\\\\"), "'", "\\'"))
			} else {
				values = append(values, val)
			}
			holder = append(holder, "'%v'")
		}

		allValues[k] = fmt.Sprintf("("+strings.Join(holder, ",")+")", values...)
	}

	query = query + strings.Join(allValues, ",")

	if s.service.in {
		_, err = s.service.tx.Exec(query)
	} else {
		_, err = s.conn.Exec(query)
	}
	if err != nil {
		s.service.Rollback()
		return 0, err
	}
	return
}

func (s *svc) CreateMultiAnyOnDupkey(data []interface{}, dupkey map[string]string) (num int64, err error) {
	if s.err != nil {
		s.service.Rollback()
		err = s.err
		return
	}

	num = int64(len(data))
	if nil == data || 0 == num {
		s.service.Rollback()
		err = fmt.Errorf("param data of CreateMultiAny() is empty")
		return
	}

	var (
		fields    = s.GetTableFields()
		query     = fmt.Sprintf("INSERT INTO `%s` (`%s`) VALUES ", s.table.tableName, strings.Join(fields, "`, `"))
		allValues = make([]string, num)
	)
	for k, m := range data {
		var (
			holder []string
			values []interface{}
			vref   = reflect.ValueOf(m)
		)
		if vref.Type().Kind() == reflect.Ptr {
			vref = vref.Elem()
			if vref.Type().Kind() != reflect.Struct {
				s.service.Rollback()
				return 0, fmt.Errorf("CreateMultiAny param data element must be struct or ptr to struct")
			}
		}

		for i := 0; i < vref.NumField(); i++ {
			var val = vref.Field(i).Interface()
			if sv, yes := val.(string); yes {
				values = append(values, strings.ReplaceAll(strings.ReplaceAll(sv, "\\", "\\\\"), "'", "\\'"))
			} else {
				values = append(values, val)
			}
			holder = append(holder, "'%v'")
		}

		allValues[k] = fmt.Sprintf("("+strings.Join(holder, ",")+")", values...)
	}

	query = query + strings.Join(allValues, ",")

	if nil != dupkey {
		var udk []string
		for k, v := range dupkey {
			udk = append(udk, fmt.Sprintf("%s=%s", k, strings.ReplaceAll(strings.ReplaceAll(v, "\\", "\\\\"), "'", "\\'")))
		}
		query = query + fmt.Sprintf(" ON DUPLICATE KEY UPDATE %s", strings.Join(udk, ","))
	}
	println(query)
	if s.service.in {
		_, err = s.service.tx.Exec(query)
	} else {
		_, err = s.conn.Exec(query)
	}
	if err != nil {
		s.service.Rollback()
		return 0, err
	}
	return
}

func (s *svc) Update() (int64, error) {
	if s.err != nil {
		s.service.Rollback()
		return 0, s.err
	}

	var (
		fv, _, pv = s.getFieldValues()
		res       sql.Result
		err       error
		update    = msql.Update{
			Table:     msql.Table{Table: s.table.tableName},
			SetValues: fv,
			Where:     msql.Where(s.table.primaryKey, "=", pv),
		}
	)
	if s.service.in {
		res, err = s.service.tx.Update(update).Exec()
	} else {
		res, err = s.conn.Update(update).Exec()
	}
	if err != nil {
		s.service.Rollback()
		return 0, err
	}

	if n, e := res.RowsAffected(); e != nil {
		s.service.Rollback()
		return n, e
	} else {
		return n, nil
	}
}

func (s *svc) UpdateByPrimaryKey(value interface{}, data map[string]interface{}) (int64, error) {
	if s.err != nil {
		s.service.Rollback()
		return 0, s.err
	}

	var (
		res    sql.Result
		err    error
		update = msql.Update{
			Table:     msql.Table{Table: s.table.tableName},
			SetValues: data,
			Where:     msql.Where(s.table.primaryKey, "=", value),
		}
	)
	if s.service.in {
		res, err = s.service.tx.Update(update).Exec()
	} else {
		res, err = s.conn.Update(update).Exec()
	}
	if err != nil {
		s.service.Rollback()
		return 0, err
	}

	if n, e := res.RowsAffected(); e != nil {
		s.service.Rollback()
		return n, e
	} else {
		return n, nil
	}
}

func (s *svc) UpdateByPrimaryKeys(values []interface{}, data map[string]interface{}) (int64, error) {
	if s.err != nil {
		s.service.Rollback()
		return 0, s.err
	}

	var (
		res    sql.Result
		err    error
		update = msql.Update{
			Table:     msql.Table{Table: s.table.tableName},
			SetValues: data,
			Where:     msql.In(s.table.primaryKey, values),
		}
	)
	if s.service.in {
		res, err = s.service.tx.Update(update).Exec()
	} else {
		res, err = s.conn.Update(update).Exec()
	}
	if err != nil {
		s.service.Rollback()
		return 0, err
	}

	if n, e := res.RowsAffected(); e != nil {
		s.service.Rollback()
		return n, e
	} else {
		return n, nil
	}
}

func (s *svc) UpdateByField(field string, value interface{}, data map[string]interface{}) (int64, error) {
	if s.err != nil {
		s.service.Rollback()
		return 0, s.err
	}

	var (
		res    sql.Result
		err    error
		update = msql.Update{
			Table:     msql.Table{Table: s.table.tableName},
			SetValues: data,
			Where:     msql.Where(field, "=", value),
		}
	)
	if s.service.in {
		res, err = s.service.tx.Update(update).Exec()
	} else {
		res, err = s.conn.Update(update).Exec()
	}
	if err != nil {
		s.service.Rollback()
		return 0, err
	}

	if n, e := res.RowsAffected(); e != nil {
		s.service.Rollback()
		return n, e
	} else {
		return n, nil
	}
}

func (s *svc) UpdateByWhere(where *msql.WhereCondition, data map[string]interface{}) (int64, error) {
	if s.err != nil {
		s.service.Rollback()
		return 0, s.err
	}

	var (
		res    sql.Result
		err    error
		update = msql.Update{
			Table:     msql.Table{Table: s.table.tableName},
			SetValues: data,
			Where:     where,
		}
	)
	if s.service.in {
		res, err = s.service.tx.Update(update).Exec()
	} else {
		res, err = s.conn.Update(update).Exec()
	}
	if err != nil {
		s.service.Rollback()
		return 0, err
	}

	if n, e := res.RowsAffected(); e != nil {
		s.service.Rollback()
		return n, e
	} else {
		return n, nil
	}
}

func (s *svc) Delete() (int64, error) {
	if s.err != nil {
		s.service.Rollback()
		return 0, s.err
	}

	var (
		res sql.Result
		err error
		del = msql.Delete{
			From:  s.table.tableName,
			Where: msql.Where(s.table.primaryKey, "=", s.GetPrimaryVal()),
		}
	)
	if s.service.in {
		res, err = s.service.tx.Delete(del).Exec()
	} else {
		res, err = s.conn.Delete(del).Exec()
	}
	if err != nil {
		s.service.Rollback()
		return 0, err
	}

	if n, e := res.RowsAffected(); e != nil {
		s.service.Rollback()
		return n, e
	} else {
		return n, nil
	}
}

func (s *svc) DeleteByPrimaryKey(value interface{}) (int64, error) {
	if s.err != nil {
		s.service.Rollback()
		return 0, s.err
	}

	var (
		res sql.Result
		err error
		del = msql.Delete{
			From:  s.table.tableName,
			Where: msql.Where(s.table.primaryKey, "=", value),
		}
	)
	if s.service.in {
		res, err = s.service.tx.Delete(del).Exec()
	} else {
		res, err = s.conn.Delete(del).Exec()
	}
	if err != nil {
		s.service.Rollback()
		return 0, err
	}

	if n, e := res.RowsAffected(); e != nil {
		s.service.Rollback()
		return n, e
	} else {
		return n, nil
	}
}

func (s *svc) DeleteByPrimaryKeys(values []interface{}) (int64, error) {
	if s.err != nil {
		s.service.Rollback()
		return 0, s.err
	}

	var (
		res sql.Result
		err error
		del = msql.Delete{
			From:  s.table.tableName,
			Where: msql.In(s.table.primaryKey, values),
		}
	)
	if s.service.in {
		res, err = s.service.tx.Delete(del).Exec()
	} else {
		res, err = s.conn.Delete(del).Exec()
	}
	if err != nil {
		s.service.Rollback()
		return 0, err
	}

	if n, e := res.RowsAffected(); e != nil {
		s.service.Rollback()
		return n, e
	} else {
		return n, nil
	}
}

func (s *svc) DeleteByField(field string, value interface{}) (int64, error) {
	if s.err != nil {
		s.service.Rollback()
		return 0, s.err
	}

	var (
		res sql.Result
		err error
		del = msql.Delete{
			From:  s.table.tableName,
			Where: msql.Where(field, "=", value),
		}
	)
	if s.service.in {
		res, err = s.service.tx.Delete(del).Exec()
	} else {
		res, err = s.conn.Delete(del).Exec()
	}
	if err != nil {
		s.service.Rollback()
		return 0, err
	}

	if n, e := res.RowsAffected(); e != nil {
		s.service.Rollback()
		return n, e
	} else {
		return n, nil
	}
}

func (s *svc) DeleteByWhere(where *msql.WhereCondition) (int64, error) {
	if s.err != nil {
		s.service.Rollback()
		return 0, s.err
	}

	var (
		res sql.Result
		err error
		del = msql.Delete{
			From:  s.table.tableName,
			Where: where,
		}
	)
	if s.service.in {
		res, err = s.service.tx.Delete(del).Exec()
	} else {
		res, err = s.conn.Delete(del).Exec()
	}
	if err != nil {
		s.service.Rollback()
		return 0, err
	}

	if n, e := res.RowsAffected(); e != nil {
		s.service.Rollback()
		return n, e
	} else {
		return n, nil
	}
}

func (s *svc) Count(where *msql.WhereCondition, groupBy []string) (int64, error) {
	if s.err != nil {
		return 0, s.err
	}

	var (
		total     int64
		selection = msql.Select{
			Select:  msql.Fields("COUNT(*)"),
			From:    msql.Table{Table: s.table.tableName},
			Where:   where,
			GroupBy: groupBy,
		}
	)
	if err := s.conn.Select(selection).QueryRow().Scan(&total); nil != err {
		return 0, err
	}

	return total, nil
}

func (s *svc) Has(where *msql.WhereCondition, groupBy []string) (bool, error) {
	if total, err := s.Count(where, groupBy); err != nil {
		return false, err
	} else if total == 0 {
		return false, nil
	}
	return true, nil
}

func (s *svc) HasByPrimary(primaryVal interface{}) (bool, error) {
	if s.err != nil {
		return false, s.err
	}

	var (
		total     int64
		selection = msql.Select{
			Select: msql.Fields("COUNT(*)"),
			From:   msql.Table{Table: s.table.tableName},
			Where:  msql.Where(s.table.primaryKey, "=", primaryVal),
		}
	)
	if err := s.conn.Select(selection).QueryRow().Scan(&total); nil != err {
		return false, err
	}
	if total == 0 {
		return false, nil
	}
	return true, nil
}

func (s *svc) Load(primaryVal interface{}, with ...string) error {
	if s.err != nil {
		return s.err
	}

	err := s.conn.Select(msql.Select{
		Select: msql.Fields(s.table.tableFields...),
		From:   msql.Table{Table: s.table.tableName},
		Where:  msql.Where(s.table.primaryKey, "=", primaryVal),
	}).QueryRow().ScanStruct(s.target)
	if nil != err {
		return err
	}

	return s.loadWith(with...)
}

func (s *svc) LoadOne(where *msql.WhereCondition, orderBy []string, with ...string) error {
	if s.err != nil {
		return s.err
	}

	err := s.conn.Select(msql.Select{
		Select:  msql.Fields(s.table.tableFields...),
		From:    msql.Table{Table: s.table.tableName},
		Where:   where,
		OrderBy: orderBy,
		Limit:   msql.Limit(1),
	}).QueryRow().ScanStruct(s.target)
	if nil != err {
		return err
	}

	return s.loadWith(with...)
}

func (s *svc) LoadTarget(target interface{}, primaryVal interface{}, with ...string) error {
	if s.err != nil {
		return s.err
	}

	fields, err := s.getTargetFields(target)
	if err != nil {
		return err
	}

	err = s.conn.Select(msql.Select{
		Select: msql.Fields(fields...),
		From:   msql.Table{Table: s.table.tableName},
		Where:  msql.Where(s.table.primaryKey, "=", primaryVal),
	}).QueryRow().ScanStruct(target)
	if nil != err {
		return err
	}

	return s.loadTargetWith(target, with...)
}

func (s *svc) LoadOneTarget(target interface{}, where *msql.WhereCondition, orderBy []string, with ...string) error {
	if s.err != nil {
		return s.err
	}

	fields, err := s.getTargetFields(target)
	if err != nil {
		return err
	}

	err = s.conn.Select(msql.Select{
		Select:  msql.Fields(fields...),
		From:    msql.Table{Table: s.table.tableName},
		Where:   where,
		OrderBy: orderBy,
		Limit:   msql.Limit(1),
	}).QueryRow().ScanStruct(target)
	if nil != err {
		return err
	}

	return s.loadTargetWith(target, with...)
}

func (s *svc) loadWith(with ...string) error {
	if len(with) == 0 {
		return nil
	}

	rvalue := reflect.ValueOf(s.target).Elem()
	for _, sn := range with {
		var field *reflect.StructField
		for _, f := range s.table.structFields {
			if sn == f.Name {
				field = &f
				break
			}
		}
		if nil == field {
			return fmt.Errorf("unkown field '%s' in %s", sn, s.table.structName)
		}

		var (
			mkey   = field.Tag.Get(MKEY)
			fkey   = field.Tag.Get(FKEY)
			mfield *reflect.StructField
		)
		if "" == mkey {
			return fmt.Errorf("not found mkey tag in field '%s' of %s", sn, s.table.structName)
		}
		if "" == fkey {
			return fmt.Errorf("not found fkey tag in field '%s' of %s", sn, s.table.structName)
		}
		for _, f := range s.table.structFields {
			if mkey == f.Tag.Get(mdb.STRUCT_TAG) {
				mfield = &f
				break
			}
		}
		if nil == mfield {
			return fmt.Errorf("not found mkey tag '%s' relate field of %s", mkey, s.table.structName)
		}

		ivalue := rvalue.FieldByName(mfield.Name).Interface()
		switch field.Type.Kind() {
		case reflect.Ptr:
			target := s.service.loadTableByName(field.Type.String())
			if nil == target {
				return fmt.Errorf("not found relation table struct '%s'", field.Type.String())
			}

			out := reflect.New(field.Type.Elem()).Interface()
			if e := s.conn.Select(msql.Select{
				Select: msql.Fields(target.tableFields...),
				From:   msql.Table{Table: target.tableName},
				Where:  msql.Where(fkey, "=", ivalue),
			}).QueryRow().ScanStruct(out); e != nil {
				return e
			}

			rvalue.FieldByName(field.Name).Set(reflect.ValueOf(out))

		case reflect.Slice:
			target := s.service.loadTableByName(field.Type.String()[2:])
			if nil == target {
				return fmt.Errorf("not found relation table struct '%s'", field.Type.String()[2:])
			}

			var (
				orderBy    []string
				assocOrder = field.Tag.Get(ASSOC_ORDER)
			)
			if len(assocOrder) > 0 {
				orderBy = msql.OrderBy(assocOrder)
			} else {
				orderBy = msql.OrderBy(target.primaryKey + " DESC")
			}

			all, err := s.conn.Select(msql.Select{
				Select:  msql.Fields(target.tableFields...),
				From:    msql.Table{Table: target.tableName},
				Where:   msql.Where(fkey, "=", ivalue),
				OrderBy: orderBy,
			}).Query().ScanStructAll(target.target)
			if err != nil {
				return err
			}

			n := len(all)
			if n > 0 {
				s := reflect.MakeSlice(field.Type, n, n)
				for i, iface := range all {
					v := s.Index(i)
					v.Set(reflect.ValueOf(iface))
				}
				rvalue.FieldByName(field.Name).Set(s)
			}
		}
	}
	return nil
}

func (s *svc) loadTargetWith(target interface{}, with ...string) error {
	if len(with) == 0 {
		return nil
	}

	var (
		targetTyp    = reflect.TypeOf(target)
		targetVal    = reflect.ValueOf(target)
		rvalue       = targetVal.Elem()
		rtype        = targetTyp.Elem()
		numField     = rtype.NumField()
		targetFields = make([]reflect.StructField, numField)
	)
	for i := 0; i < numField; i++ {
		field := rtype.Field(i)
		if field.Anonymous {
			continue
		}

		targetFields[i] = field
	}

	for _, sn := range with {
		var field *reflect.StructField
		for _, f := range targetFields {
			if sn == f.Name {
				field = &f
				break
			}
		}
		if nil == field {
			return fmt.Errorf("unkown field '%s' in %s", sn, s.table.structName)
		}

		var (
			mkey   = field.Tag.Get(MKEY)
			fkey   = field.Tag.Get(FKEY)
			mfield *reflect.StructField
		)
		if "" == mkey {
			return fmt.Errorf("not found mkey tag in field '%s' of %s", sn, s.table.structName)
		}
		if "" == fkey {
			return fmt.Errorf("not found fkey tag in field '%s' of %s", sn, s.table.structName)
		}
		for _, f := range targetFields {
			if mkey == f.Tag.Get(mdb.STRUCT_TAG) {
				mfield = &f
				break
			}
		}
		if nil == mfield {
			return fmt.Errorf("not found mkey tag '%s' relate field of %s", mkey, s.table.structName)
		}

		ivalue := rvalue.FieldByName(mfield.Name).Interface()
		switch field.Type.Kind() {
		case reflect.Ptr:
			target := s.service.loadTableByName(field.Type.String())
			if nil == target {
				return fmt.Errorf("not found relation table struct '%s'", field.Type.String())
			}

			out := reflect.New(field.Type.Elem()).Interface()
			if e := s.conn.Select(msql.Select{
				Select: msql.Fields(target.tableFields...),
				From:   msql.Table{Table: target.tableName},
				Where:  msql.Where(fkey, "=", ivalue),
			}).QueryRow().ScanStruct(out); e != nil {
				return e
			}

			rvalue.FieldByName(field.Name).Set(reflect.ValueOf(out))

		case reflect.Slice:
			target := s.service.loadTableByName(field.Type.String()[2:])
			if nil == target {
				return fmt.Errorf("not found relation table struct '%s'", field.Type.String()[2:])
			}

			var (
				orderBy    []string
				assocOrder = field.Tag.Get(ASSOC_ORDER)
			)
			if len(assocOrder) > 0 {
				orderBy = msql.OrderBy(assocOrder)
			} else {
				orderBy = msql.OrderBy(target.primaryKey + " DESC")
			}

			all, err := s.conn.Select(msql.Select{
				Select:  msql.Fields(target.tableFields...),
				From:    msql.Table{Table: target.tableName},
				Where:   msql.Where(fkey, "=", ivalue),
				OrderBy: orderBy,
			}).Query().ScanStructAll(target.target)
			if err != nil {
				return err
			}

			n := len(all)
			if n > 0 {
				s := reflect.MakeSlice(field.Type, n, n)
				for i, iface := range all {
					v := s.Index(i)
					v.Set(reflect.ValueOf(iface))
				}
				rvalue.FieldByName(field.Name).Set(s)
			}
		}
	}
	return nil
}

// param where use func msql.Where, msql.And, msql.Or, msql.In, msql.NotIn,
// msql.Between, msql.NotBetween to generate.
// or use nil mean no WhereCondition
func (s *svc) LoadAll(where *msql.WhereCondition, orderBy []string) ([]interface{}, error) {
	if s.err != nil {
		return nil, s.err
	}

	if nil == orderBy {
		orderBy = msql.OrderBy(s.table.primaryKey + " DESC")
	}

	return s.conn.Select(msql.Select{
		Select:  msql.Fields(s.table.tableFields...),
		From:    msql.Table{Table: s.table.tableName},
		Where:   where,
		OrderBy: orderBy,
	}).Query().ScanStructAll(s.target)
}

// param where use func msql.Where, msql.And, msql.Or, msql.In, msql.NotIn,
// msql.Between, msql.NotBetween to generate.
// or use nil mean no WhereCondition
func (s *svc) LoadList(where *msql.WhereCondition, orderBy []string, limit, offset uint64) ([]interface{}, error) {
	if s.err != nil {
		return nil, s.err
	}

	if nil == orderBy {
		orderBy = msql.OrderBy(s.table.primaryKey + " DESC")
	}

	return s.conn.Select(msql.Select{
		Select:  msql.Fields(s.table.tableFields...),
		From:    msql.Table{Table: s.table.tableName},
		Where:   where,
		OrderBy: orderBy,
		Limit:   msql.LimitOffset(offset, limit),
	}).Query().ScanStructAll(s.target)
}

func (s *svc) LoadCount(where *msql.WhereCondition, groupBy []string, having *msql.WhereCondition) (int64, error) {
	if s.err != nil {
		return 0, s.err
	}

	var count int64
	e := s.conn.Select(msql.Select{
		Select:  msql.Fields("COUNT(*)"),
		From:    msql.Table{Table: s.table.tableName},
		Where:   where,
		GroupBy: groupBy,
		Having:  having,
	}).QueryRow().Scan(&count)
	if e != nil {
		return 0, e
	}

	return count, nil
}

func (s *svc) LoadPaginator(selection *msql.Select, curPage, pageSize int64) (*Paginator, error) {
	if s.err != nil {
		return nil, s.err
	}

	if nil == selection {
		selection = &msql.Select{
			Select:  msql.Fields(s.table.tableFields...),
			From:    msql.Table{Table: s.table.tableName},
			OrderBy: msql.OrderBy(s.table.primaryKey + " DESC"),
		}
	}

	return &Paginator{
		Conn:      s.conn,
		target:    s.target,
		Selection: selection,
		CurPage:   curPage,
		PerSize:   pageSize,
	}, nil
}

func (s *svc) LoadPaginatorByWhere(where *msql.WhereCondition, curPage, pageSize int64, orderBy []string) (*Paginator, error) {
	if s.err != nil {
		return nil, s.err
	}

	if nil == orderBy {
		orderBy = msql.OrderBy(s.table.primaryKey + " DESC")
	}
	var selection = &msql.Select{
		Select:  msql.Fields(s.table.tableFields...),
		From:    msql.Table{Table: s.table.tableName},
		Where:   where,
		OrderBy: orderBy,
	}

	return &Paginator{
		Conn:      s.conn,
		target:    s.target,
		Selection: selection,
		CurPage:   curPage,
		PerSize:   pageSize,
	}, nil
}

func (s *svc) GetPrimaryKey() string {
	return s.table.primaryKey
}

func (s *svc) GetPrimaryVal() interface{} {
	return reflect.ValueOf(s.target).Elem().FieldByName(s.table.primaryField.Name).Interface()
}

func (s *svc) GetTableName() string {
	return s.table.tableName
}

func (s *svc) GetTableFields() []string {
	return s.table.tableFields
}

func (s *svc) getFieldValues() (fieldValues map[string]interface{}, targetValue reflect.Value, primaryValue int64) {
	fieldValues = make(map[string]interface{})
	targetValue = reflect.ValueOf(s.target).Elem()
	for i := 0; i < s.targetType.NumField(); i++ {
		tt := s.targetType.Field(i)
		fv := targetValue.FieldByName(tt.Name)
		if tt.Name == s.table.primaryField.Name {
			primaryValue = reflect.Indirect(fv).Int()
		} else {
			dbtag := tt.Tag.Get(mdb.STRUCT_TAG)
			if dbtag != "" {
				fieldValues[dbtag] = fv.Interface()
			}
		}
	}
	return
}

func (s *svc) getTargetFields(target interface{}) (fields []string, err error) {
	t := reflect.TypeOf(target)
	if t.Kind() != reflect.Ptr {
		err = fmt.Errorf("target '%s' must be ptr to struct", t.String())
		return
	}
	e := t.Elem()
	if e.Kind() != reflect.Struct {
		err = fmt.Errorf("target '%s' must be ptr to struct", t.String())
		return
	}

	for i := 0; i < e.NumField(); i++ {
		f := e.Field(i)
		tag := f.Tag.Get(mdb.STRUCT_TAG)
		if tag != "" {
			fields = append(fields, tag)
		}
	}
	return
}
