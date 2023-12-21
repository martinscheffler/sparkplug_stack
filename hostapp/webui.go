package main

import (
	"errors"
	"fmt"
	"github.com/labstack/echo/v4"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func isOlder(date1, date2 time.Time) bool {
	return date1.Before(date2)
}

// TemplateRegistry is a custom template renderer for echo
type TemplateRegistry struct {
	templates map[string]*template.Template
}

func (t *TemplateRegistry) Render(w io.Writer, name string, data interface{}, _ echo.Context) error {
	tmpl, ok := t.templates[name]
	if !ok {
		err := errors.New("TemplateRegistry not found -> " + name)
		return err
	}
	return tmpl.ExecuteTemplate(w, "base.html", data)
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
	templates := make(map[string]*template.Template)

	pattern := "templates/*.html"
	files, err := filepath.Glob(pattern)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	base := "templates/base.html"
	for _, file := range files {
		fileName := filepath.Base(file)
		if fileName != "base.html" {
			templates[fileName] = template.Must(
				template.New(fileName).Funcs(funcMap).ParseFiles(file, base))
		}
	}

	e.Renderer = &TemplateRegistry{
		templates: templates,
	}

	// Define routes
	e.GET("/", serveNodeList)
	e.GET("/node/:groupId/:nodeId", serveNodeInfo)
	e.GET("/node/:groupId/:nodeId/:deviceId", serveNodeInfo)

	// Start http server
	e.Logger.Fatal(e.Start(":8080"))

}
