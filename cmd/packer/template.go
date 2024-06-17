package packer

import (
	"html/template"
	"strings"
)

const cssTemplate = `
div {
	display: none
}

h1, h2 {
	text-align: center
}

img {
	margin: 0;
	padding: 0;
	display: block;
	vertical-align: baseline;
}
`

var htmlTemplate = template.Must(template.New("page").Parse(`<div>.</div><img src="kindle:embed:{{ . }}?mime=image/jpeg">`))

func templateStr(tpl *template.Template, data interface{}) string {
	buf := new(strings.Builder)

	if err := tpl.Execute(buf, data); err != nil {
		panic(err)
	}

	return buf.String()
}
