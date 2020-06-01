package service

import (
	"math"
	"github.com/xiaocairen/wgo/mdb"
	"github.com/xiaocairen/wgo/mdb/msql"
)

type Paginator struct {
	Conn      *mdb.Conn
	Selection *msql.Select
	target    interface{}
	CurPage   int64
	PerSize   int64
	maxPage   int64
	total     int64
	results   [][]interface{}
}

func (p *Paginator) GetMaxPage() int64 {
	if p.CurPage > 1 && p.maxPage > 0 {
		return p.maxPage
	}

	total := p.GetTotal()
	p.maxPage = int64(math.Ceil(float64(total) / float64(p.PerSize)))
	return p.maxPage
}

func (p *Paginator) GetTotal() int64 {
	if p.CurPage > 1 && p.total > 0 {
		return p.total
	}

	query := p.Selection.BuildCountQuery()
	if err := p.Conn.QueryRow(query.Sql, query.Params...).Scan(&p.total); err != nil {
		return 0
	}
	return p.total
}

// target must be ptr to struct,
// return []interface{} will be a targets slice.
func (p *Paginator) GetCurPageTargets() ([]interface{}, error) {
	p.Selection.Limit = msql.LimitOffset(uint64(p.calcOffset()), uint64(p.PerSize))
	return p.Conn.Select(*p.Selection).Query().ScanStructAll(p.target)
}

func (p *Paginator) QueryResults() *rows {
	p.Selection.Limit = msql.LimitOffset(uint64(p.calcOffset()), uint64(p.PerSize))
	rs := p.Conn.Select(*p.Selection).Query()
	return &rows{rows: rs}
}

func (p *Paginator) calcOffset() int64 {
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
