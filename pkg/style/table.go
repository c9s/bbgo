package style

import (
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

func NewDefaultTableStyle() *table.Style {
	style := table.Style{
		Name:    "StyleRounded",
		Box:     table.StyleBoxRounded,
		Format:  table.FormatOptionsDefault,
		HTML:    table.DefaultHTMLOptions,
		Options: table.OptionsDefault,
		Title:   table.TitleOptionsDefault,
		Color:   table.ColorOptionsYellowWhiteOnBlack,
	}
	style.Color.Row = text.Colors{text.FgHiYellow, text.BgHiBlack}
	style.Color.RowAlternate = text.Colors{text.FgYellow, text.BgBlack}
	return &style
}
