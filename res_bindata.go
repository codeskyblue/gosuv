// +build bindata

package main

import (
	"html/template"
	"io"
	"log"
	"net/http"
	"path/filepath"
)

var tmpl *template.Template

func parseTemplate(name string, content string) {
	if tmpl == nil {
		tmpl = template.New(name)
	}
	var t *template.Template
	if tmpl.Name() == name {
		t = tmpl
	} else {
		t = tmpl.New(name)
	}
	template.Must(t.New(name).Delims("[[", "]]").Parse(content))
}

func init() {
	http.Handle("/res/", http.StripPrefix("/res/", http.FileServer(assetFS())))
}

func executeTemplate(wr io.Writer, name string, data interface{}) {
	if tmpl == nil || tmpl.Lookup(name) == nil {
		path := filepath.Join(templateDir, name+".html")
		data, err := Asset(path)
		if err != nil {
			log.Fatal(err)
		}
		parseTemplate(name, string(data))
	}
	tmpl.ExecuteTemplate(wr, name, data)
}
