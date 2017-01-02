package main

import (
	"io"
	"text/template"
)

type Printer interface {
	Print(io.Writer, Events) error
}

type PrinterFunc func(io.Writer, Events) error

func (p PrinterFunc) Print(w io.Writer, evs Events) error {
	return p(w, evs)
}

type TemplateExecuter interface {
	Execute(w io.Writer, data interface{}) error
}
type TemplatePrinter struct {
	template TemplateExecuter
}

func (t TemplatePrinter) Print(w io.Writer, evs Events) error {
	return t.template.Execute(w, evs)
}

func NewTemplatePrinter(text string) (Printer, error) {
	var ret TemplatePrinter
	var err error
	ret.template, err = template.New("main").Parse(text)
	return ret, err
}
