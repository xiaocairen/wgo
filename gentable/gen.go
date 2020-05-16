package gentable


import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

var (
	db     *sql.DB
	user   string
	pass   string
	host   string
	port   string
	dbname string
	tag    string
)

func init() {
	user = GetCmdOption("--user")
	if user == "" {
		user = "root"
	}

	pass = GetCmdOption("--pass")
	if pass == "" {
		pass = "123456"
	}

	host = GetCmdOption("--host")
	if host == "" {
		host = "127.0.0.1"
	}

	port = GetCmdOption("--port")
	if port == "" {
		port = "3306"
	}

	dbname = GetCmdOption("--db")
	if dbname == "" {
		dbname = "nshop"
	}

	tag = GetCmdOption("--tag")
	if tag == "" {
		tag = "mdb"
	}

	db = GetDB()
}

/*func main() {
	GenTables()
}*/

func GenTables() {
	rows, err := db.Query("SHOW TABLES")
	if nil != err {
		panic(err)
	}

	for rows.Next() {
		var t string
		if err := rows.Scan(&t); nil != err {
			panic(err)
		}

		code := findTableFields(t)
		GenGoFile(t+".go", code)
	}
}

func GenGoFile(filename, code string) {
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

func findTableFields(table string) string {
	rows, err := db.Query("SHOW COLUMNS FROM " + table)
	if nil != err {
		panic(err)
	}

	structName := Underline2Camel(table)
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

		if err := rows.Scan(&Field, &Type, &Null, &Key, &Default, &Extra); nil != err {
			panic(err)
		}

		fs := GenField(table, Field, Type, Null, Key, Default, Extra)
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

func GenField(table, field, typ, null, key string, def interface{}, extra string) *fieldStruct {
	fs := &fieldStruct{}
	fs.Name = Underline2Camel(field)

	pos := strings.IndexByte(typ, '(')
	if -1 == pos {
		if strings.Contains(typ, "float") || strings.Contains(typ, "double") {
			fs.Type = "float64"
		}
	} else {
		switch typ[:pos] {
		case "char", "varchar", "tinytext", "mediumtext", "text", "longtext", "date", "datetime", "time":
			fs.Type = "string"
		case "tinyint", "smallint", "mediumint", "int", "bigint":
			/*if strings.Contains(typ, "unsigned") {
				fs.Type = "uint64"
			} else {
				fs.Type = "int64"
			}*/
			fs.Type = "int64"
		case "timestamp":
			fs.Type = "int64"
		case "decimal", "float", "double":
			fs.Type = "float64"
		case "enum":
			fs.Type = "string"
		default:
			panic(fmt.Sprintf("unkown field type '%s'", typ))
		}
	}

	if "PRI" == key {
		fs.Tag = fmt.Sprintf("`json:\"%s\" yaml:\"%s\" %s:\"%s\" pk:\"yes\"`", field, field, tag, field)
	} else {
		fs.Tag = fmt.Sprintf("`json:\"%s\" yaml:\"%s\" %s:\"%s\"`", field, field, tag, field)
	}

	fs.NameLen = len(fs.Name)
	fs.TypeLen = len(fs.Type)
	return fs
}

func GetDB() *sql.DB {
	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", user, pass, host, port, dbname))
	if nil != err {
		panic(err)
	}

	if err := db.Ping(); nil != err {
		panic(err)
	}

	return db
}

func Underline2Camel(underline string) string {
	var camel = make([]rune, len(underline))
	var i = 0
	for k, c := range underline {
		if k == 0 {
			if c >= 97 && c <= 122 {
				camel[i] = c - 32
			} else if c != '_' {
				camel[i] = c
			}
			i++
		} else if c >= 97 && c <= 122 {
			if underline[k-1] == '_' {
				camel[i] = c - 32
			} else {
				camel[i] = c
			}
			i++
		} else if c != '_' {
			camel[i] = c
			i++
		}
	}

	return string(camel[:i])
}

func GetCmdOption(k string) string {
	if len(os.Args) == 1 {
		return ""
	}

	alen := len(os.Args[1:])
	args := make([]string, alen)
	copy(args, os.Args[1:])

	klen := len(k)
	if klen == 1 {
		k = "-" + k
	} else if klen == 2 {
		if !strings.HasPrefix(k, "-") {
			k = "--" + k
		}
	} else if !strings.HasPrefix(k, "--") {
		k = "--" + k
	}

	for i := 0; i < alen; {
		if args[i] == k {
			if i < alen {
				return args[i+1]
			} else {
				return ""
			}
		}
		if strings.HasPrefix(args[i], k) && strings.Contains(args[i], "=") {
			ss := strings.SplitN(args[i], "=", 2)
			return strings.TrimSpace(ss[1])
		}
		i++
	}

	return ""
}

