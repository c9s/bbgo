package util

import (
	"bytes"
	"github.com/sirupsen/logrus"
	"text/template"
)

func Render(tpl string, args interface{}) string {
	var buf = bytes.NewBuffer(nil)
	tmpl, err := template.New("tmp").Parse(tpl)
	if err != nil {
		logrus.WithError(err).Error("template error")
		return ""
	}
	err = tmpl.Execute(buf, args)
	if err != nil {
		logrus.WithError(err).Error("template error")
		return ""
	}
	return buf.String()
}

