package main

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
)

//go:embed templates/admin/*.tmpl admin_static/*
var adminAssetsFS embed.FS

type adminTemplateRenderer struct {
	env string
}

func newAdminTemplateRenderer(env string) *adminTemplateRenderer {
	return &adminTemplateRenderer{
		env: env,
	}
}

func (r *adminTemplateRenderer) templatesForRender(contentTemplatePath string) (*template.Template, error) {
	var sourceFS fs.FS
	if r.env == "development" {
		sourceFS = os.DirFS(".")
	} else {
		sourceFS = adminAssetsFS
	}

	templates, err := template.New("layout.tmpl").Funcs(template.FuncMap{
		"safe": func(s string) template.HTML {
			return template.HTML(s)
		},
	}).ParseFS(sourceFS, "templates/admin/layout.tmpl", contentTemplatePath)
	if err != nil {
		return nil, fmt.Errorf("parse admin templates: %w", err)
	}
	return templates, nil
}

func adminStaticFileSystem(env string) (http.FileSystem, error) {
	if env == "development" {
		return http.Dir("admin_static"), nil
	}

	sub, err := fs.Sub(adminAssetsFS, "admin_static")
	if err != nil {
		return nil, fmt.Errorf("admin static fs: %w", err)
	}
	return http.FS(sub), nil
}
