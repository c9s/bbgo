package google

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"google.golang.org/api/sheets/v4"
)

func ReadSheetValuesRange(srv *sheets.Service, spreadsheetId, readRange string) (*sheets.ValueRange, error) {
	logrus.Infof("ReadSheetValuesRange: %s", readRange)
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetId, readRange).Do()
	return resp, err
}

func AddNewSheet(srv *sheets.Service, spreadsheetId string, title string) (*sheets.BatchUpdateSpreadsheetResponse, error) {
	logrus.Infof("AddNewSheet: %s", title)
	return srv.Spreadsheets.BatchUpdate(spreadsheetId, &sheets.BatchUpdateSpreadsheetRequest{
		IncludeSpreadsheetInResponse: false,
		Requests: []*sheets.Request{
			{
				AddSheet: &sheets.AddSheetRequest{
					Properties: &sheets.SheetProperties{
						Hidden:        false,
						TabColor:      nil,
						TabColorStyle: nil,
						Title:         title,
					},
				},
			},
		},
	}).Do()
}

func ValuesToCellData(values []interface{}) (cells []*sheets.CellData) {
	for _, anyValue := range values {
		switch typedValue := anyValue.(type) {
		case string:
			cells = append(cells, &sheets.CellData{
				UserEnteredValue: &sheets.ExtendedValue{StringValue: &typedValue},
			})
		case float64:
			cells = append(cells, &sheets.CellData{
				UserEnteredValue: &sheets.ExtendedValue{NumberValue: &typedValue},
			})
		case bool:
			cells = append(cells, &sheets.CellData{
				UserEnteredValue: &sheets.ExtendedValue{BoolValue: &typedValue},
			})
		}
	}

	return cells
}

func GetSpreadSheetURL(spreadsheetId string) string {
	return fmt.Sprintf("https://docs.google.com/spreadsheets/d/%s/edit#gid=0", spreadsheetId)
}

func AppendRow(srv *sheets.Service, spreadsheetId string, sheetId int64, values []interface{}) (*sheets.BatchUpdateSpreadsheetResponse, error) {
	row := &sheets.RowData{}
	row.Values = ValuesToCellData(values)

	logrus.Infof("AppendRow: %+v", row.Values)
	return srv.Spreadsheets.BatchUpdate(spreadsheetId, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{
			{
				AppendCells: &sheets.AppendCellsRequest{
					Fields:  "*",
					Rows:    []*sheets.RowData{row},
					SheetId: sheetId,
				},
			},
		},
	}).Do()
}

func DebugBatchUpdateSpreadsheetResponse(resp *sheets.BatchUpdateSpreadsheetResponse) {
	logrus.Infof("BatchUpdateSpreadsheetResponse.SpreadsheetId: %+v", resp.SpreadsheetId)
	logrus.Infof("BatchUpdateSpreadsheetResponse.UpdatedSpreadsheet: %+v", resp.UpdatedSpreadsheet)
	logrus.Infof("BatchUpdateSpreadsheetResponse.Replies: %+v", resp.Replies)
}
