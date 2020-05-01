package service

import (
	"fmt"
	"github.com/xiaocairen/wgo/mdb"
	"github.com/xiaocairen/wgo/tool"
	"reflect"
	"strings"
)

const TABLE_KEY_TAG = "pk"

type TableCollection func(tr *TableRegister)

func (fn TableCollection) call(tr *TableRegister) {
	fn(tr)
}

type TableRegister struct {
	service *Service
}

func (sr *TableRegister) RegisteTables(tables []interface{}) {
	for _, t := range tables {
		sr.service.dbTables = append(sr.service.dbTables, sr.registe(t))
	}
}

func (tr *TableRegister) registe(service interface{}) *dbTable {
	t := reflect.TypeOf(service)
	if t.Kind() != reflect.Ptr {
		panic(fmt.Sprintf("service '%s' must be ptr to struct", t.String()))
	}
	e := t.Elem()
	if e.Kind() != reflect.Struct {
		panic(fmt.Sprintf("service '%s' must be ptr to struct", t.String()))
	}

	name := t.String()
	dotPos := strings.LastIndex(name, ".")
	tableName := tool.Camel2Underline(name[dotPos+1:])

	dbt := &dbTable{structName: t.String(), tableName: tableName}
	var (
		sf []reflect.StructField
		tf []string
		pk string
		pf reflect.StructField
		v  = reflect.ValueOf(service).Elem()
	)
	for i := 0; i < e.NumField(); i++ {
		field := e.Field(i)
		dbTag := field.Tag.Get(mdb.STRUCT_TAG)
		if len(dbTag) > 0 {
			tf = append(tf, dbTag)
			fv := v.FieldByName(field.Name)
			if !fv.CanSet() {
				panic(fmt.Errorf("field '%s' of '%s' with tag 'mdb' must be visible", field.Name, name))
			}
			if "yes" == field.Tag.Get(TABLE_KEY_TAG) {
				if len(pk) != 0 {
					panic(fmt.Errorf("struct '%s' primary key must be only one", name))
				}
				if fv.Type().Kind() != reflect.Int64 {
					panic(fmt.Errorf("struct '%s' primary key type must be int64", name))
				}
				pk = dbTag
				pf = field
			}
		}
		sf = append(sf, field)
	}
	if len(pk) == 0 {
		panic(fmt.Errorf("struct '%s' must be have a primary key", name))
	}

	dbt.structFields = sf
	dbt.tableFields = tf
	dbt.primaryKey = pk
	dbt.primaryField = pf

	return dbt
}
