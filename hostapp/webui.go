package main

import (
	"github.com/labstack/echo/v4"
	"html/template"
	"io"
	"net/http"
	"time"
)

func isOlder(date1, date2 time.Time) bool {
	return date1.Before(date2)
}

type Template struct {
	templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func serveHTTP(c echo.Context) error {
	devices, err := getDevicesAndNodes()
	if err != nil {
		return c.String(http.StatusInternalServerError, "Cannot fetch devices")
	}
	return c.Render(http.StatusOK, "index.html", devices)
}

func startWebUI() {

	e := echo.New()
	e.Static("/static", "static")
	funcMap := template.FuncMap{
		"isOlder": isOlder,
	}

	t := &Template{
		templates: template.Must(template.New("base").Funcs(funcMap).ParseGlob("templates/*.html")),
	}

	e.Renderer = t
	e.GET("/", serveHTTP)
	e.Logger.Fatal(e.Start(":8080"))

}
