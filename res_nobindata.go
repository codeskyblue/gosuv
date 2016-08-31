// +build !bindata

package main

import (
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"path/filepath"
)

func init() {
	fs := http.FileServer(http.Dir(templateDir))
	http.Handle("/res/", http.StripPrefix("/res/", fs))
}

func executeTemplate(wr io.Writer, name string, data interface{}) {
	path := filepath.Join(templateDir, name+".html")
	body, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	template.Must(template.New("t").Delims("[[", "]]").Parse(string(body))).Execute(wr, data)
}
