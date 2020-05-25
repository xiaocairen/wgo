package wgo

import "html/template"

var tplBuiltins = template.FuncMap{
	"url": tplUrl,
}

func tplUrl(args ...string) string {
	if len(args) != 3 {
		return "func url need 3 arguments"
	}
	r, e := appInstance.router.getRouter(args[0], args[1], args[2])
	if e != nil {
		return e.Error()
	}

	return r.Path
}
