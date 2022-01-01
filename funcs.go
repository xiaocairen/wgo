package wgo

import "html/template"

var tplBuiltins = template.FuncMap{
	"url": tplUrl,
}

func tplUrl(args ...string) template.URL {
	if len(args) != 3 {
		return "func url need 3 arguments"
	}
	r, e := appinst.router.getRouter(args[0], args[1], args[2])
	if e != nil {
		return template.URL(e.Error())
	}

	return template.URL(r.Path)
}
