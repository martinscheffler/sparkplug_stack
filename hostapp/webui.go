package main

import (
	"errors"
	"fmt"
	"github.com/labstack/echo/v4"
	"html/template"
	"io"
	"net/http"
	"time"
)

func isOlder(date1, date2 time.Time) bool {
	return date1.Before(date2)
}

// Template is a custom template renderer for echo
type Template struct {
	templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, _ echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func serveNodeList(c echo.Context) error {
	nodes, err := getNodes()
	if err != nil {
		return c.String(http.StatusInternalServerError, "Cannot fetch devices")
	}
	err = c.Render(http.StatusOK, "index.html", nodes)
	if err != nil {
		c.Logger().Error(err)
	}
	return err
}

func serveNodeInfo(c echo.Context) error {
	groupId := c.Param("groupId")
	nodeId := c.Param("nodeId")
	deviceId := c.Param("deviceId")
	node, err := getNodeInfo(groupId, nodeId, deviceId)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Cannot fetch device")
	}
	data := struct {
		Node      *NodeInfo
		DataTypes map[int]string
	}{
		Node:      node,
		DataTypes: DataTypes,
	}
	return c.Render(http.StatusOK, "node.html", data)
}

func customHTTPErrorHandler(err error, c echo.Context) {
	code := http.StatusInternalServerError
	var he *echo.HTTPError
	if errors.As(err, &he) {
		code = he.Code
	}
	c.Logger().Error(err)
	errorPage := fmt.Sprintf("%d.html", code)
	if err := c.File(errorPage); err != nil {
		c.Logger().Error(err)
	}
}

func startWebUI() {

	e := echo.New()

	e.HTTPErrorHandler = customHTTPErrorHandler

	// Set up static file handling
	e.Static("/static", "static")

	// Set up templates
	funcMap := template.FuncMap{
		"isOlder": isOlder,
	}
	t := &Template{
		templates: template.Must(template.New("base").Funcs(funcMap).ParseGlob("templates/*.html")),
	}
	e.Renderer = t

	// Define routes
	e.GET("/", serveNodeList)
	e.GET("/node/:groupId/:nodeId", serveNodeInfo)
	e.GET("/node/:groupId/:nodeId/:deviceId", serveNodeInfo)

	// Start http server
	e.Logger.Fatal(e.Start(":8080"))

}
