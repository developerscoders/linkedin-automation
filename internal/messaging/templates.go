package messaging

import (
	"bytes"
	"fmt"
	"text/template"
)

type Template struct {
	Name    string
	Content string
}

type TemplateEngine struct {
	templates map[string]*template.Template
}

func NewTemplateEngine() *TemplateEngine {
	return &TemplateEngine{
		templates: make(map[string]*template.Template),
	}
}

func (e *TemplateEngine) AddTemplate(name, content string) error {
	tmpl, err := template.New(name).Parse(content)
	if err != nil {
		return fmt.Errorf("failed to parse template %s: %w", name, err)
	}
	e.templates[name] = tmpl
	return nil
}

func (e *TemplateEngine) Render(name string, data map[string]string) (string, error) {
	tmpl, ok := e.templates[name]
	if !ok {
		return "", fmt.Errorf("template %s not found", name)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}
