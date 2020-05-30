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
	MKEY = "mkey"
	FKEY = "fkey"
)

var (
	svcerInst  *Servicer
	onceNewSvc sync.Once
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

type Servicer struct {
	db     *mdb.DB
	tables []*table
}

func NewServicer(db *mdb.DB) *Servicer {
	var first = false
	onceNewSvc.Do(func() {
		first = true
		svcerInst = &Servicer{db: db}
	})

	if first {
		return svcerInst
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
	return &Service{
		db:     s.db,
		conn:   s.db.GetConn(),
		tx:     nil,
		in:     false,
		tables: s.tables,
	}
}

type Service struct {
	db     *mdb.DB
	conn   *mdb.Conn
	tx     *mdb.Tx
	in     bool
	tables []*table
}

func (s *Service) Conn() *mdb.Conn {
	return s.conn
}

func (s *Service) SelectDbHost(hostname string) {
	s.conn = s.db.GetConnByName(hostname)
}

func (s *Service) Begin() {
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

// target must be ptr to struct
func (s *Service) New(target interface{}) *svc {
	dbt, typ, err := s.loadTableByTarget(target)
	if err != nil {
		return &svc{newErr: err}
	}

	return &svc{
		conn:       s.conn,
		service:    s,
		table:      dbt,
		target:     target,
		targetType: typ,
		newErr:     nil,
	}
}

func (s *Service) loadTableByTarget(target interface{}) (*table, reflect.Type, error) {
	targetType := reflect.TypeOf(target)
	structName := targetType.String()
	if targetType.Kind() != reflect.Ptr {
		return nil, nil, fmt.Errorf("target '%s' must be ptr to struct", structName)
	}
	targetTypeElem := targetType.Elem()
	if targetTypeElem.Kind() != reflect.Struct {
		return nil, nil, fmt.Errorf("target '%s' must be ptr to struct", structName)
	}

	var dbt *table
	for _, t := range s.tables {
		if t.structName == structName {
			dbt = t
			break
		}
	}
	if dbt == nil {
		return nil, nil, fmt.Errorf("not found target '%s' in table register", structName)
	}
	return dbt, targetTypeElem, nil
}

func (s *Service) loadTableByName(structName string) *table {
	for _, dbt := range s.tables {
		if dbt.structName == structName {
			return dbt
		}
	}
	return nil
}

type svc struct {
	conn       *mdb.Conn
	service    *Service
	table      *table
	target     interface{}
	targetType reflect.Type
	newErr     error
}

func (s *svc) Create() (int64, error) {
	if s.newErr != nil {
		return 0, s.newErr
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
		return 0, err
	}

	id, _ := res.LastInsertId()
	rv.FieldByName(s.table.primaryField.Name).Set(reflect.ValueOf(id))
	return id, nil
}

func (s *svc) CreateMulti(data []map[string]interface{}) (num int, err error) {
	if s.newErr != nil {
		err = s.newErr
		return
	}

	num = len(data)
	if nil == data || 0 == num {
		err = fmt.Errorf("param data of CreateMulti() is empty")
		return
	}

	var fields []string
	var placeholder []string
	for k, _ := range data[0] {
		fields = append(fields, k)
		placeholder = append(placeholder, "?")
	}
	query := fmt.Sprintf("INSERT INTO `%s` (`%s`) VALUES (%s)", s.table.tableName, strings.Join(fields, "`, `"), strings.Join(placeholder, ", "))

	if s.service.in {
		stmt := s.service.tx.Prepare(query)
		for _, m := range data {
			var values []interface{}
			for _, f := range fields {
				values = append(values, m[f])
			}

			if _, err = stmt.Exec(values...); err != nil {
				return
			}
		}
	} else {
		tx := s.conn.Begin()
		stmt := tx.Prepare(query)
		for _, m := range data {
			var values []interface{}
			for _, f := range fields {
				values = append(values, m[f])
			}

			if _, err = stmt.Exec(values...); err != nil {
				tx.Rollback()
				return
			}
		}
		tx.Commit()
	}

	return
}

func (s *svc) Update() (int64, error) {
	if s.newErr != nil {
		return 0, s.newErr
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
		return 0, err
	}

	return res.RowsAffected()
}

func (s *svc) UpdateByPrimaryKey(value int64, data map[string]interface{}) (int64, error) {
	if s.newErr != nil {
		return 0, s.newErr
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
		return 0, err
	}

	return res.RowsAffected()
}

func (s *svc) UpdateByPrimaryKeys(values []int64, data map[string]interface{}) (int64, error) {
	if s.newErr != nil {
		return 0, s.newErr
	}

	vs := make([]interface{}, len(values))
	for k, v := range values {
		vs[k] = v
	}

	var (
		res    sql.Result
		err    error
		update = msql.Update{
			Table:     msql.Table{Table: s.table.tableName},
			SetValues: data,
			Where:     msql.In(s.table.primaryKey, vs),
		}
	)
	if s.service.in {
		res, err = s.service.tx.Update(update).Exec()
	} else {
		res, err = s.conn.Update(update).Exec()
	}
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}

func (s *svc) UpdateByField(field string, value interface{}, data map[string]interface{}) (int64, error) {
	if s.newErr != nil {
		return 0, s.newErr
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
		return 0, err
	}

	return res.RowsAffected()
}

func (s *svc) Delete() (int64, error) {
	if s.newErr != nil {
		return 0, s.newErr
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
		return 0, err
	}

	return res.RowsAffected()
}

func (s *svc) DeleteByPrimaryKey(value interface{}) (int64, error) {
	if s.newErr != nil {
		return 0, s.newErr
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
		return 0, err
	}

	return res.RowsAffected()
}

func (s *svc) DeleteByPrimaryKeys(values []interface{}) (int64, error) {
	if s.newErr != nil {
		return 0, s.newErr
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
		return 0, err
	}

	return res.RowsAffected()
}

func (s *svc) DeleteByField(field string, value interface{}) (int64, error) {
	if s.newErr != nil {
		return 0, s.newErr
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
		return 0, err
	}

	return res.RowsAffected()
}

func (s *svc) Load(primaryVal interface{}, with ...string) error {
	if s.newErr != nil {
		return s.newErr
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
	if s.newErr != nil {
		return s.newErr
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

		var (
			kind   = field.Type.Kind()
			ivalue = rvalue.FieldByName(mfield.Name).Interface()
		)
		if reflect.Ptr == kind {
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
		} else if reflect.Slice == kind {
			target := s.service.loadTableByName(field.Type.String()[2:])
			if nil == target {
				return fmt.Errorf("not found relation table struct '%s'", field.Type.String()[2:])
			}

			all, err := s.conn.Select(msql.Select{
				Select: msql.Fields(target.tableFields...),
				From:   msql.Table{Table: target.tableName},
				Where:  msql.Where(fkey, "=", ivalue),
			}).Query().ScanStructAll(target.target)
			if err != nil {
				fmt.Println(err)
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
	if s.newErr != nil {
		return nil, s.newErr
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
func (s *svc) LoadBy(where *msql.WhereCondition, orderBy []string, limit, offset uint64) ([]interface{}, error) {
	if s.newErr != nil {
		return nil, s.newErr
	}

	return s.conn.Select(msql.Select{
		Select:  msql.Fields(s.table.tableFields...),
		From:    msql.Table{Table: s.table.tableName},
		Where:   where,
		OrderBy: orderBy,
		Limit:   msql.LimitOffset(offset, limit),
	}).Query().ScanStructAll(s.target)
}

func (s *svc) LoadPaginator(selection *msql.Select, curPage, pageSize int64) (*Paginator, error) {
	if s.newErr != nil {
		return nil, s.newErr
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
			fieldValues[tt.Tag.Get(mdb.STRUCT_TAG)] = fv.Interface()
		}
	}
	return
}
