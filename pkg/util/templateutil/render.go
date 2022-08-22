package templateutil

import (
	"bytes"
	"text/template"

	"github.com/sirupsen/logrus"
)

func Render(tpl string, args interface{}) string {
	var buf = bytes.NewBuffer(nil)
	tmpl, err := template.New("tmp").Parse(tpl)
	if err != nil {
		logrus.WithError(err).Error("template parse error")
		return ""
	}

	err = tmpl.Execute(buf, args)
	if err != nil {
		logrus.WithError(err).Error("template execute error")
		return ""
	}

	return buf.String()
}
