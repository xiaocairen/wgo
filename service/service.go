package service

import (
	"fmt"
	"github.com/xiaocairen/wgo/mdb"
	"github.com/xiaocairen/wgo/mdb/msql"
	"reflect"
	"strings"
	"sync"
)

var (
	svcInstance    *Service
	onceNewService sync.Once
)

type dbTable struct {
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

func NewService(db *mdb.DB, tc TableCollection) *Service {
	onceNewService.Do(func() {
		svcInstance = &Service{db: db}
		if nil != tc {
			tc.call(&TableRegister{service: svcInstance})
		}
	})
	return svcInstance
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
		target:     target,
		table:      dbt,
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
		target:     target,
		table:      dbt,
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

type dbService struct {
	conn       *mdb.Conn
	target     interface{}
	table      *dbTable
	targetType reflect.Type
	newErr     error
}

func (ds *dbService) Create() (id int64, err error) {
	if ds.newErr != nil {
		err = ds.newErr
		return
	}

	fv, rv, _ := ds.getFieldValues()
	res, err := ds.conn.Insert(msql.Insert{
		Into:       ds.table.tableName,
		FieldValue: fv,
	}).Exec()
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

	fv, _, pv := ds.getFieldValues()
	res, err := ds.conn.Update(msql.Update{
		Table:     msql.Table{Table: ds.table.tableName},
		SetValues: fv,
		Where:     msql.Where(ds.table.primaryKey, "=", pv),
	}).Exec()

	ar, _ := res.RowsAffected()
	return ar, err
}

func (ds *dbService) UpdateByPrimaryKey(value int64, data map[string]interface{}) (int64, error) {
	if ds.newErr != nil {
		return 0, ds.newErr
	}

	res, err := ds.conn.Update(msql.Update{
		Table:     msql.Table{Table: ds.table.tableName},
		SetValues: data,
		Where:     msql.Where(ds.table.primaryKey, "=", value),
	}).Exec()

	ar, _ := res.RowsAffected()
	return ar, err
}

func (ds *dbService) UpdateByPrimaryKeys(values []interface{}, data map[string]interface{}) (int64, error) {
	if ds.newErr != nil {
		return 0, ds.newErr
	}

	res, err := ds.conn.Update(msql.Update{
		Table:     msql.Table{Table: ds.table.tableName},
		SetValues: data,
		Where:     msql.In(ds.table.primaryKey, values),
	}).Exec()

	ar, _ := res.RowsAffected()
	return ar, err
}

func (ds *dbService) UpdateByField(field string, value interface{}, data map[string]interface{}) (int64, error) {
	if ds.newErr != nil {
		return 0, ds.newErr
	}

	res, err := ds.conn.Update(msql.Update{
		Table:     msql.Table{Table: ds.table.tableName},
		SetValues: data,
		Where:     msql.Where(field, "=", value),
	}).Exec()

	ar, _ := res.RowsAffected()
	return ar, err
}

func (ds *dbService) Delete() (int64, error) {
	if ds.newErr != nil {
		return 0, ds.newErr
	}

	pv := ds.GetPrimaryVal()
	res, err := ds.conn.Delete(msql.Delete{
		From:  ds.table.tableName,
		Where: msql.Where(ds.table.primaryKey, "=", pv),
	}).Exec()

	ar, _ := res.RowsAffected()
	return ar, err
}

func (ds *dbService) DeleteByPrimaryKey(value interface{}) (int64, error) {
	if ds.newErr != nil {
		return 0, ds.newErr
	}

	res, err := ds.conn.Delete(msql.Delete{
		From:  ds.table.tableName,
		Where: msql.Where(ds.table.primaryKey, "=", value),
	}).Exec()

	ar, _ := res.RowsAffected()
	return ar, err
}

func (ds *dbService) DeleteByPrimaryKeys(values []interface{}) (int64, error) {
	if ds.newErr != nil {
		return 0, ds.newErr
	}

	res, err := ds.conn.Delete(msql.Delete{
		From:  ds.table.tableName,
		Where: msql.In(ds.table.primaryKey, values),
	}).Exec()

	ar, _ := res.RowsAffected()
	return ar, err
}

func (ds *dbService) DeleteByField(field string, value interface{}) (int64, error) {
	if ds.newErr != nil {
		return 0, ds.newErr
	}

	res, err := ds.conn.Delete(msql.Delete{
		From:  ds.table.tableName,
		Where: msql.Where(field, "=", value),
	}).Exec()

	ar, _ := res.RowsAffected()
	return ar, err
}

func (ds *dbService) Load() error {
	if ds.newErr != nil {
		return ds.newErr
	}

	pv := ds.GetPrimaryVal()
	return ds.conn.Select(msql.Select{
		Select: msql.Fields(ds.table.tableFields...),
		From:   msql.Table{Table: ds.table.tableName},
		Where:  msql.Where(ds.table.primaryKey, "=", pv),
	}).QueryRow().ScanStruct(ds.target)
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

func (ds *dbService) LoadPaginator(curPage, pageSize uint64, selection *msql.Select, loadAssist bool) (*Paginator, error) {
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
