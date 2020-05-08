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
	svcInstance    *Service
	onceNewService sync.Once
)

type dbTable struct {
	target       interface{}
	targetType   reflect.Type
	structName   string
	structFields []reflect.StructField
	tableName    string
	tableFields  []string
	primaryKey   string
	primaryField reflect.StructField
}

type Service struct {
	db       *mdb.DB
	dbTables []*dbTable
}

func NewService(db *mdb.DB) *Service {
	onceNewService.Do(func() {
		svcInstance = &Service{db: db}
	})
	return svcInstance
}

func (svc *Service) Registe(tc TableCollection) {
	if len(svc.dbTables) == 0 && nil != tc {
		tc.call(&TableRegister{service: svc})
	}
}

func (svc *Service) DB() *mdb.DB {
	return svc.db
}

// target must be ptr to struct
func (svc *Service) NewDBService(target interface{}) *dbService {
	dbt, typ, err := svc.loadTableByTarget(target)
	if err != nil {
		return &dbService{newErr: err}
	}

	return &dbService{
		conn:       svc.db.GetConn(),
		service:    svc,
		table:      dbt,
		target:     target,
		targetType: typ,
		newErr:     nil,
	}
}

// target must be ptr to struct
// hostDbName is field host_db_name of db config in app.json
func (svc *Service) NewDBServiceByHostDbName(target interface{}, hostDbName string) *dbService {
	dbt, typ, err := svc.loadTableByTarget(target)
	if err != nil {
		return &dbService{newErr: err}
	}

	return &dbService{
		conn:       svc.db.GetConnByName(hostDbName),
		service:    svc,
		table:      dbt,
		target:     target,
		targetType: typ,
		newErr:     nil,
	}
}

func (svc *Service) loadTableByTarget(target interface{}) (*dbTable, reflect.Type, error) {
	targetType := reflect.TypeOf(target)
	structName := targetType.String()
	if targetType.Kind() != reflect.Ptr {
		return nil, nil, fmt.Errorf("target '%s' must be ptr to struct", structName)
	}
	targetTypeElem := targetType.Elem()
	if targetTypeElem.Kind() != reflect.Struct {
		return nil, nil, fmt.Errorf("target '%s' must be ptr to struct", structName)
	}

	var dbt *dbTable
	for _, t := range svc.dbTables {
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

func (svc *Service) loadTableByName(structName string) *dbTable {
	for _, dbt := range svc.dbTables {
		if dbt.structName == structName {
			return dbt
		}
	}
	return nil
}

type dbService struct {
	conn       *mdb.Conn
	tx         *mdb.Tx
	intx       bool
	service    *Service
	table      *dbTable
	target     interface{}
	targetType reflect.Type
	newErr     error
}

func (ds *dbService) Begin() {
	ds.tx = ds.conn.Begin()
	ds.intx = true
}

func (ds *dbService) Rollback() error {
	err := ds.tx.Rollback()
	ds.tx = nil
	ds.intx = false
	return err
}

func (ds *dbService) Commit() error {
	err := ds.tx.Commit()
	ds.tx = nil
	ds.intx = false
	return err
}

func (ds *dbService) Create() (id int64, err error) {
	if ds.newErr != nil {
		err = ds.newErr
		return
	}

	var (
		fv, rv, _ = ds.getFieldValues()
		res       sql.Result
		insert    = msql.Insert{
			Into:       ds.table.tableName,
			FieldValue: fv,
		}
	)
	if ds.intx {
		res, err = ds.tx.Insert(insert).Exec()
	} else {
		res, err = ds.conn.Insert(insert).Exec()
	}
	if err != nil {
		return
	}

	id, _ = res.LastInsertId()
	rv.FieldByName(ds.table.primaryField.Name).Set(reflect.ValueOf(id))
	return
}

func (ds *dbService) CreateMulti(data []map[string]interface{}) (num int, err error) {
	if ds.newErr != nil {
		err = ds.newErr
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
	query := fmt.Sprintf("INSERT INTO `%s` (`%s`) VALUES (%s)", ds.table.tableName, strings.Join(fields, "`, `"), strings.Join(placeholder, ", "))

	tx := ds.conn.Begin()
	stmt := tx.Prepare(query)
	for _, m := range data {
		var values []interface{}
		for _, f := range fields {
			values = append(values, m[f])
		}

		_, err = stmt.Exec(values...)
		if err != nil {
			tx.Rollback()
			return
		}
	}
	tx.Commit()

	return
}

func (ds *dbService) Update() (int64, error) {
	if ds.newErr != nil {
		return 0, ds.newErr
	}

	var (
		fv, _, pv = ds.getFieldValues()
		res       sql.Result
		err       error
		update    = msql.Update{
			Table:     msql.Table{Table: ds.table.tableName},
			SetValues: fv,
			Where:     msql.Where(ds.table.primaryKey, "=", pv),
		}
	)
	if ds.intx {
		res, err = ds.tx.Update(update).Exec()
	} else {
		res, err = ds.conn.Update(update).Exec()
	}
	if err != nil {
		return 0, err
	}

	ar, _ := res.RowsAffected()
	return ar, err
}

func (ds *dbService) UpdateByPrimaryKey(value int64, data map[string]interface{}) (int64, error) {
	if ds.newErr != nil {
		return 0, ds.newErr
	}

	var (
		res    sql.Result
		err    error
		update = msql.Update{
			Table:     msql.Table{Table: ds.table.tableName},
			SetValues: data,
			Where:     msql.Where(ds.table.primaryKey, "=", value),
		}
	)
	if ds.intx {
		res, err = ds.tx.Update(update).Exec()
	} else {
		res, err = ds.conn.Update(update).Exec()
	}
	if err != nil {
		return 0, err
	}

	ar, _ := res.RowsAffected()
	return ar, err
}

func (ds *dbService) UpdateByPrimaryKeys(values []interface{}, data map[string]interface{}) (int64, error) {
	if ds.newErr != nil {
		return 0, ds.newErr
	}

	var (
		res    sql.Result
		err    error
		update = msql.Update{
			Table:     msql.Table{Table: ds.table.tableName},
			SetValues: data,
			Where:     msql.In(ds.table.primaryKey, values),
		}
	)
	if ds.intx {
		res, err = ds.tx.Update(update).Exec()
	} else {
		res, err = ds.conn.Update(update).Exec()
	}
	if err != nil {
		return 0, err
	}

	ar, _ := res.RowsAffected()
	return ar, err
}

func (ds *dbService) UpdateByField(field string, value interface{}, data map[string]interface{}) (int64, error) {
	if ds.newErr != nil {
		return 0, ds.newErr
	}

	var (
		res    sql.Result
		err    error
		update = msql.Update{
			Table:     msql.Table{Table: ds.table.tableName},
			SetValues: data,
			Where:     msql.Where(field, "=", value),
		}
	)
	if ds.intx {
		res, err = ds.tx.Update(update).Exec()
	} else {
		res, err = ds.conn.Update(update).Exec()
	}
	if err != nil {
		return 0, err
	}

	ar, _ := res.RowsAffected()
	return ar, err
}

func (ds *dbService) Delete() (int64, error) {
	if ds.newErr != nil {
		return 0, ds.newErr
	}

	var (
		pv  = ds.GetPrimaryVal()
		res sql.Result
		err error
		del = msql.Delete{
			From:  ds.table.tableName,
			Where: msql.Where(ds.table.primaryKey, "=", pv),
		}
	)
	if ds.intx {
		res, err = ds.tx.Delete(del).Exec()
	} else {
		res, err = ds.conn.Delete(del).Exec()
	}
	if err != nil {
		return 0, err
	}

	ar, _ := res.RowsAffected()
	return ar, err
}

func (ds *dbService) DeleteByPrimaryKey(value interface{}) (int64, error) {
	if ds.newErr != nil {
		return 0, ds.newErr
	}

	var (
		res sql.Result
		err error
		del = msql.Delete{
			From:  ds.table.tableName,
			Where: msql.Where(ds.table.primaryKey, "=", value),
		}
	)
	if ds.intx {
		res, err = ds.tx.Delete(del).Exec()
	} else {
		res, err = ds.conn.Delete(del).Exec()
	}
	if err != nil {
		return 0, err
	}

	ar, _ := res.RowsAffected()
	return ar, err
}

func (ds *dbService) DeleteByPrimaryKeys(values []interface{}) (int64, error) {
	if ds.newErr != nil {
		return 0, ds.newErr
	}

	var (
		res sql.Result
		err error
		del = msql.Delete{
			From:  ds.table.tableName,
			Where: msql.In(ds.table.primaryKey, values),
		}
	)
	if ds.intx {
		res, err = ds.tx.Delete(del).Exec()
	} else {
		res, err = ds.conn.Delete(del).Exec()
	}
	if err != nil {
		return 0, err
	}

	ar, _ := res.RowsAffected()
	return ar, err
}

func (ds *dbService) DeleteByField(field string, value interface{}) (int64, error) {
	if ds.newErr != nil {
		return 0, ds.newErr
	}

	var (
		res sql.Result
		err error
		del = msql.Delete{
			From:  ds.table.tableName,
			Where: msql.Where(field, "=", value),
		}
	)
	if ds.intx {
		res, err = ds.tx.Delete(del).Exec()
	} else {
		res, err = ds.conn.Delete(del).Exec()
	}
	if err != nil {
		return 0, err
	}

	ar, _ := res.RowsAffected()
	return ar, err
}

func (ds *dbService) Load(with ...string) error {
	if ds.newErr != nil {
		return ds.newErr
	}

	pv := ds.GetPrimaryVal()
	err := ds.conn.Select(msql.Select{
		Select: msql.Fields(ds.table.tableFields...),
		From:   msql.Table{Table: ds.table.tableName},
		Where:  msql.Where(ds.table.primaryKey, "=", pv),
	}).QueryRow().ScanStruct(ds.target)

	if nil != err {
		return err
	}

	return ds.loadWith(with...)
}

func (ds *dbService) loadWith(with ...string) error {
	if len(with) == 0 {
		return nil
	}

	rvalue := reflect.ValueOf(ds.target).Elem()
	for _, sn := range with {
		var field *reflect.StructField
		for _, f := range ds.table.structFields {
			if sn == f.Name {
				field = &f
				break
			}
		}
		if nil == field {
			return fmt.Errorf("unkown field '%s' in %s", sn, ds.table.structName)
		}

		var (
			mkey   = field.Tag.Get(MKEY)
			fkey   = field.Tag.Get(FKEY)
			mfield *reflect.StructField
		)
		if "" == mkey {
			return fmt.Errorf("not found mkey tag in field '%s' of %s", sn, ds.table.structName)
		}
		if "" == fkey {
			return fmt.Errorf("not found fkey tag in field '%s' of %s", sn, ds.table.structName)
		}
		for _, f := range ds.table.structFields {
			if mkey == f.Tag.Get(mdb.STRUCT_TAG) {
				mfield = &f
				break
			}
		}
		if nil == mfield {
			return fmt.Errorf("not found mkey tag '%s' relate field of %s", mkey, ds.table.structName)
		}

		var (
			kind   = field.Type.Kind()
			ivalue = rvalue.FieldByName(mfield.Name).Interface()
		)
		if reflect.Ptr == kind {
			target := ds.service.loadTableByName(field.Type.String())
			if nil == target {
				return fmt.Errorf("not found relation table struct '%s'", field.Type.String())
			}

			out := reflect.New(field.Type.Elem()).Interface()
			if e := ds.conn.Select(msql.Select{
				Select: msql.Fields(target.tableFields...),
				From:   msql.Table{Table: target.tableName},
				Where:  msql.Where(fkey, "=", ivalue),
			}).QueryRow().ScanStruct(out); e != nil {
				return e
			}

			rvalue.FieldByName(field.Name).Set(reflect.ValueOf(out))
		} else if reflect.Slice == kind {
			target := ds.service.loadTableByName(field.Type.String()[2:])
			if nil == target {
				return fmt.Errorf("not found relation table struct '%s'", field.Type.String()[2:])
			}

			all, err := ds.conn.Select(msql.Select{
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

func (ds *dbService) LoadOne(where *msql.WhereCondition, orderBy []string) error {
	if ds.newErr != nil {
		return ds.newErr
	}

	rows := ds.conn.Select(msql.Select{
		Select:  msql.Fields(ds.table.tableFields...),
		From:    msql.Table{Table: ds.table.tableName},
		Where:   where,
		OrderBy: orderBy,
		Limit:   msql.Limit(1),
	}).Query()
	defer rows.Close()

	return rows.ScanStruct(ds.target)
}

// param where use func msql.Where, msql.And, msql.Or, msql.In, msql.NotIn,
// msql.Between, msql.NotBetween to generate.
// or use nil mean no WhereCondition
func (ds *dbService) LoadAll(where *msql.WhereCondition, orderBy []string) ([]interface{}, error) {
	if ds.newErr != nil {
		return nil, ds.newErr
	}

	return ds.conn.Select(msql.Select{
		Select:  msql.Fields(ds.table.tableFields...),
		From:    msql.Table{Table: ds.table.tableName},
		Where:   where,
		OrderBy: orderBy,
	}).Query().ScanStructAll(ds.target)
}

// param where use func msql.Where, msql.And, msql.Or, msql.In, msql.NotIn,
// msql.Between, msql.NotBetween to generate.
// or use nil mean no WhereCondition
func (ds *dbService) LoadBy(where *msql.WhereCondition, orderBy []string, limit, offset uint64) ([]interface{}, error) {
	if ds.newErr != nil {
		return nil, ds.newErr
	}

	return ds.conn.Select(msql.Select{
		Select:  msql.Fields(ds.table.tableFields...),
		From:    msql.Table{Table: ds.table.tableName},
		Where:   where,
		OrderBy: orderBy,
		Limit:   msql.LimitOffset(offset, limit),
	}).Query().ScanStructAll(ds.target)
}

func (ds *dbService) LoadPaginator(selection *msql.Select, curPage, pageSize uint64) (*Paginator, error) {
	if ds.newErr != nil {
		return nil, ds.newErr
	}

	if nil == selection {
		selection = &msql.Select{
			Select:  msql.Fields(ds.table.tableFields...),
			From:    msql.Table{Table: ds.table.tableName},
			OrderBy: msql.OrderBy(ds.table.primaryKey + " DESC"),
		}
	}

	return &Paginator{
		Conn:      ds.conn,
		Selection: selection,
		CurPage:   curPage,
		PerSize:   pageSize,
	}, nil
}

func (ds *dbService) GetPrimaryKey() string {
	return ds.table.primaryKey
}

func (ds *dbService) GetPrimaryVal() interface{} {
	return reflect.ValueOf(ds.target).Elem().FieldByName(ds.table.primaryField.Name).Interface()
}

func (ds *dbService) GetTableName() string {
	return ds.table.tableName
}

func (ds *dbService) GetTableFields() []string {
	return ds.table.tableFields
}

func (ds *dbService) getFieldValues() (fieldValues map[string]interface{}, targetValue reflect.Value, primaryValue int64) {
	targetValue = reflect.ValueOf(ds.target).Elem()
	for i := 0; i < ds.targetType.NumField(); i++ {
		tt := ds.targetType.Field(i)
		fv := targetValue.FieldByName(tt.Name)
		if tt.Name == ds.table.primaryField.Name {
			primaryValue = reflect.Indirect(fv).Int()
		} else {
			fieldValues[tt.Tag.Get(mdb.STRUCT_TAG)] = fv.Interface()
		}
	}
	return
}
