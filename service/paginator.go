package service

import (
	"github.com/xiaocairen/wgo/mdb"
	"github.com/xiaocairen/wgo/mdb/msql"
)

type Paginator struct {
	Conn      *mdb.Conn
	Selection *msql.Select
	CurPage   uint64
	PerSize   uint64
	maxPage   uint64
	results   [][]interface{}
}

func (p *Paginator) GetMaxPage() uint64 {
	if p.CurPage > 1 && p.maxPage > 0 {
		return p.maxPage
	}

	query := p.Selection.BuildCountQuery()
	if err := p.Conn.QueryRow(query.Sql, query.Params...).Scan(&p.maxPage); err != nil {
		return 0
	}
	return p.maxPage
}

// target must be ptr to struct,
// return []interface{} will be a targets slice.
func (p *Paginator) GetCurPageTargets(target interface{}) ([]interface{}, error) {
	p.Selection.Limit = msql.LimitOffset(p.calcOffset(), p.PerSize)
	return p.Conn.Select(*p.Selection).Query().ScanStructAll(target)
}

func (p *Paginator) QueryResults() *rows {
	p.Selection.Limit = msql.LimitOffset(p.calcOffset(), p.PerSize)
	rs := p.Conn.Select(*p.Selection).Query()
	return &rows{rows:rs}
}

func (p *Paginator) calcOffset() uint64 {
	return (p.CurPage - 1) * p.PerSize
}

type rows struct {
	rows *mdb.Rows
}

func (rs *rows) Next() bool {
	return rs.rows.Next()
}

func (rs *rows) Scan(dest ...interface{}) error {
	return rs.rows.Scan(dest...)
}

func (rs *rows) ScanStruct(target interface{}) error {
	return rs.ScanStruct(target)
}