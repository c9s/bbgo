package elliottwave

import (
	"bytes"
	"io"
	"strconv"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/dynamic"
	"github.com/c9s/bbgo/pkg/interact"
	"github.com/c9s/bbgo/pkg/style"
	"github.com/jedib0t/go-pretty/v6/table"
)

func (s *Strategy) initOutputCommands() {
	bbgo.RegisterCommand("/config", "Show latest config", func(reply interact.Reply) {
		var buffer bytes.Buffer
		s.Print(&buffer, false)
		reply.Message(buffer.String())
	})
	bbgo.RegisterCommand("/dump", "Dump internal params", func(reply interact.Reply) {
		reply.Message("Please enter series output length:")
	}).Next(func(length string, reply interact.Reply) {
		var buffer bytes.Buffer
		l, err := strconv.Atoi(length)
		if err != nil {
			dynamic.ParamDump(s, &buffer)
		} else {
			dynamic.ParamDump(s, &buffer, l)
		}
		reply.Message(buffer.String())
	})

	bbgo.RegisterModifier(s)
}

func (s *Strategy) Print(f io.Writer, pretty bool, withColor ...bool) {
	var tableStyle *table.Style
	if pretty {
		tableStyle = style.NewDefaultTableStyle()
	}
	dynamic.PrintConfig(s, f, tableStyle, len(withColor) > 0 && withColor[0], dynamic.DefaultWhiteList()...)
}
