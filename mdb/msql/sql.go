package msql

import (
	"fmt"
	"strings"
)

type SqlStatement struct {
	Sql    string
	Params []interface{}
	Err    error
}

type Select struct {
	Select  []string
	From    Table
	Where   *WhereCondition
	GroupBy []string
	Having  *WhereCondition
	OrderBy []string
	Limit   []uint64
}

func (r Select) Build() SqlStatement {
	var (
		fields  string
		where   string
		groupBy string
		having  string
		orderBy string
		limit   string
		params  []interface{}
	)

	if nil != r.Where {
		if nil != r.Where.err {
			return SqlStatement{Err: r.Where.err}
		}
		if len(r.Where.param) > 0 {
			where = " WHERE " + r.Where.where
			params = append(params, r.Where.param...)
		}
	}

	if 0 == len(r.Select) {
		fields = "*"
	} else {
		fields = strings.Join(r.Select, ",")
	}

	if len(r.GroupBy) > 0 {
		groupBy = " GROUP BY " + strings.Join(r.GroupBy, ",")
	}
	if nil != r.Having && len(r.Having.param) > 0 {
		having = " HAVING " + r.Having.where
		params = append(params, r.Having.param...)
	}
	if len(r.OrderBy) > 0 {
		orderBy = " ORDER BY " + strings.Join(r.OrderBy, ",")
	}

	n := len(r.Limit)
	if 1 == n {
		limit = fmt.Sprintf(" LIMIT %d", r.Limit[0])
	} else if n >= 2 {
		limit = fmt.Sprintf(" LIMIT %d, %d", r.Limit[0], r.Limit[1])
	}

	sql := fmt.Sprintf("SELECT %s FROM %s %s%s%s%s%s", fields, r.From.String(), where, groupBy, having, orderBy, limit)
	return SqlStatement{
		Sql:    sql,
		Params: params,
		Err:    nil,
	}
}

func (r Select) BuildCountQuery() SqlStatement {
	var (
		where   string
		groupBy string
		having  string
		params  []interface{}
	)

	if nil != r.Where {
		if nil != r.Where.err {
			return SqlStatement{Err: r.Where.err}
		}
		if len(r.Where.param) > 0 {
			where = " WHERE " + r.Where.where
			params = append(params, r.Where.param...)
		}
	}

	if len(r.GroupBy) > 0 {
		groupBy = " GROUP BY " + strings.Join(r.GroupBy, ",")
	}
	if nil != r.Having && len(r.Having.param) > 0 {
		having = " HAVING " + r.Having.where
		params = append(params, r.Having.param...)
	}

	sql := fmt.Sprintf("SELECT COUNT(*) FROM %s %s%s%s", r.From.String(), where, groupBy, having)
	return SqlStatement{
		Sql:    sql,
		Params: params,
		Err:    nil,
	}
}

type Insert struct {
	Into       string
	FieldValue map[string]interface{}
	OnDKUpdate map[string]string
}

func (c Insert) Build() SqlStatement {
	var (
		sql    string
		set    []string
		dku    []string
		params []interface{}
	)
	for k, v := range c.FieldValue {
		set = append(set, "`"+k+"`=?")
		params = append(params, v)
	}
	for k, v := range c.OnDKUpdate {
		dku = append(dku, fmt.Sprintf("`%s`=%s", k, v))
	}

	if 0 == len(dku) {
		sql = fmt.Sprintf("INSERT `%s` SET %s", c.Into, strings.Join(set, ","))
	} else {
		sql = fmt.Sprintf("INSERT `%s` SET %s ON DUPLICATE KEY UPDATE %s", c.Into, strings.Join(set, ","), strings.Join(dku, ","))
	}
	return SqlStatement{
		Sql:    sql,
		Params: params,
		Err:    nil,
	}
}

type Update struct {
	Table     Table
	SetValues map[string]interface{}
	Where     *WhereCondition
	OrderBy   []string
	Limit     uint64
}

func (u Update) Build() SqlStatement {
	var (
		set     []string
		where   string
		params  []interface{}
		orderBy string
		limit   string
	)

	if nil != u.Where && nil != u.Where.err {
		return SqlStatement{Err: u.Where.err}
	}

	for k, v := range u.SetValues {
		set = append(set, "`"+k+"`=?")
		params = append(params, v)
	}

	if nil != u.Where && len(u.Where.param) > 0 {
		where = " WHERE " + u.Where.where
		params = append(params, u.Where.param...)
	}

	if len(u.OrderBy) > 0 {
		orderBy = " ORDER BY " + strings.Join(u.OrderBy, ",")
	}
	if u.Limit > 0 {
		limit = fmt.Sprintf(" LIMIT %d", u.Limit)
	}

	sql := fmt.Sprintf("UPDATE %s SET %s%s%s%s", u.Table.String(), strings.Join(set, ","), where, orderBy, limit)
	return SqlStatement{
		Sql:    sql,
		Params: params,
		Err:    nil,
	}
}

type Delete struct {
	From    string
	Using   Table
	Where   *WhereCondition
	OrderBy []string
	Limit   uint64
}

func (d Delete) Build() SqlStatement {
	var (
		using   string
		where   string
		params  []interface{}
		orderBy string
		limit   string
	)
	if nil != d.Where && nil != d.Where.err {
		return SqlStatement{Err: d.Where.err}
	}

	t := d.Using.String()
	if len(t) > 0 {
		using = " USING " + t
	}

	if nil != d.Where && len(d.Where.param) > 0 {
		where = " WHERE " + d.Where.where
		params = d.Where.param
	}
	if len(d.OrderBy) > 0 {
		orderBy = " ORDER BY " + strings.Join(d.OrderBy, ",")
	}
	if d.Limit > 0 {
		limit = fmt.Sprintf(" LIMIT %d", d.Limit)
	}

	sql := fmt.Sprintf("DELETE FROM `%s`%s%s%s%s", d.From, using, where, orderBy, limit)
	return SqlStatement{
		Sql:    sql,
		Params: params,
		Err:    nil,
	}
}

type Table struct {
	Table    string
	As       string
	Join     []Join
	LeftJoin []LeftJoin
}

func (t Table) String() string {
	var (
		as        = ""
		innerJoin = ""
		leftJoin  = ""
	)
	if len(t.Table) == 0 {
		return ""
	}

	if len(t.As) > 0 {
		as = " AS " + t.As
	}

	for _, s := range t.Join {
		innerJoin += s.String()
	}
	for _, s := range t.LeftJoin {
		leftJoin += s.String()
	}

	return fmt.Sprintf(" `%s`%s%s%s", t.Table, as, innerJoin, leftJoin)
}

type Join struct {
	Table string
	As    string
	On    string
}

func (j Join) String() string {
	var as = ""
	if "" != j.As {
		as = "AS " + j.As
	}
	return fmt.Sprintf(" JOIN `%s` %s ON %s", j.Table, as, j.On)
}

type LeftJoin struct {
	Table string
	As    string
	On    string
}

func (lj LeftJoin) String() string {
	var as = ""
	if "" != lj.As {
		as = "AS " + lj.As
	}
	return fmt.Sprintf(" LEFT JOIN `%s` %s ON %s", lj.Table, as, lj.On)
}

type WhereCondition struct {
	where string
	param []interface{}
	err   error
}

func Where(field string, opr string, value interface{}) *WhereCondition {
	return &WhereCondition{
		where: fmt.Sprintf("`%s` %s ?", field, opr),
		param: []interface{}{value},
	}
}

func Having(field string, opr string, value interface{}) *WhereCondition {
	return &WhereCondition{
		where: fmt.Sprintf("`%s` %s ?", field, opr),
		param: []interface{}{value},
	}
}

// where and statement.
// use:  And("name", "=", "wb", "age", ">", "18")
//       And("age", ">", "18", Or("name", "=", "wb", "name", "=", "lm"))
//       And("city", "=", "zz", "age", ">", "18", In("name", []interface{}{"wb", "lm"}))
func And(and ...interface{}) *WhereCondition {
	var (
		err   error
		where []string
		param []interface{}
		n     = len(and)
	)
	if 0 == n {
		return &WhereCondition{err: fmt.Errorf("param of 'and' sql is empty")}
	}

	for i := 0; i < n; {
		if v, ok := and[i].(*WhereCondition); ok {
			where = append(where, "("+v.where+")")
			param = append(param, v.param...)
			i++
		} else if f, ok := and[i].(string); ok && i+2 < n {
			if opr, ok1 := and[i+1].(string); ok1 {
				where = append(where, fmt.Sprintf("`%s` %s ?", f, opr))
				param = append(param, and[i+2])
			} else {
				err = fmt.Errorf("'or' sql operator must be string, '%t'", and[i+1])
			}
			i += 3
		} else {
			err = fmt.Errorf("'or' sql field must be string or struct WhereCondition{}, '%t'", and[i])
		}
	}
	return &WhereCondition{
		where: strings.Join(where, " OR "),
		param: param,
		err:   err,
	}
}

// where or []interface{}.
// usage see And()
func Or(or ...interface{}) *WhereCondition {
	var (
		err   error
		where []string
		param []interface{}
		n     = len(or)
	)
	if 0 == n {
		return &WhereCondition{err: fmt.Errorf("param of 'or' sql is empty")}
	}

	for i := 0; i < n; {
		if v, ok := or[i].(*WhereCondition); ok {
			where = append(where, "("+v.where+")")
			param = append(param, v.param...)
			i++
		} else if f, ok := or[i].(string); ok && i+2 < n {
			if opr, ok1 := or[i+1].(string); ok1 {
				where = append(where, fmt.Sprintf("`%s` %s ?", f, opr))
				param = append(param, or[i+2])
			} else {
				err = fmt.Errorf("'or' sql operator must be string, '%t'", or[i+1])
				break
			}
			i += 3
		} else {
			err = fmt.Errorf("'or' sql field must be string or struct WhereCondition{}, '%t'", or[i])
			break
		}
	}
	return &WhereCondition{
		where: strings.Join(where, " AND "),
		param: param,
		err:   err,
	}
}

func In(field string, values []interface{}) *WhereCondition {
	n := len(values)
	placeholder := make([]string, n)
	for i := 0; i < n; i++ {
		placeholder[i] = "?"
	}
	return &WhereCondition{
		where: fmt.Sprintf("`%s` IN (%s)", field, strings.Join(placeholder, ",")),
		param: values,
	}
}

func NotIn(field string, values []interface{}) *WhereCondition {
	n := len(values)
	placeholder := make([]string, n)
	for i := 0; i < n; i++ {
		placeholder[i] = "?"
	}
	return &WhereCondition{
		where: fmt.Sprintf("`%s` NOT IN (%s)", field, strings.Join(placeholder, ",")),
		param: values,
	}
}

func Between(field string, start interface{}, end interface{}) *WhereCondition {
	return &WhereCondition{
		where: fmt.Sprintf("`%s` BETWEEN ? AND ?", field),
		param: []interface{}{start, end},
	}
}

func NotBetween(field string, start interface{}, end interface{}) *WhereCondition {
	return &WhereCondition{
		where: fmt.Sprintf("`%s` NOT BETWEEN ? AND ?", field),
		param: []interface{}{start, end},
	}
}

func Fields(f ...string) []string {
	return f
}

func Joins(j ...Join) []Join {
	return j
}

func LeftJoins(lj ...LeftJoin) []LeftJoin {
	return lj
}

func GroupBy(gby ...string) []string {
	return gby
}

func OrderBy(oby ...string) []string {
	return oby
}

func Limit(count uint64) []uint64 {
	return []uint64{count}
}

func LimitOffset(offset uint64, count uint64) []uint64 {
	return []uint64{offset, count}
}
