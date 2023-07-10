package google

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

var log = logrus.WithField("service", "google")

type SpreadSheetService struct {
	SpreadsheetID string
	TokenFile     string

	service *sheets.Service
}

func NewSpreadSheetService(ctx context.Context, tokenFile string, spreadsheetID string) *SpreadSheetService {
	if len(tokenFile) == 0 {
		log.Panicf("google.SpreadSheetService: jsonTokenFile is not set")
	}

	srv, err := sheets.NewService(ctx,
		option.WithCredentialsFile(tokenFile),
	)

	if err != nil {
		log.Panicf("google.SpreadSheetService: unable to initialize spreadsheet service: %v", err)
	}

	return &SpreadSheetService{
		SpreadsheetID: spreadsheetID,
		service:       srv,
	}
}

func ReadSheetValuesRange(srv *sheets.Service, spreadsheetId, readRange string) (*sheets.ValueRange, error) {
	log.Infof("ReadSheetValuesRange: %s", readRange)
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetId, readRange).Do()
	return resp, err
}

func AddNewSheet(srv *sheets.Service, spreadsheetId string, title string) (*sheets.BatchUpdateSpreadsheetResponse, error) {
	log.Infof("AddNewSheet: %s", title)
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
		case int:
			v := float64(typedValue)
			cells = append(cells, &sheets.CellData{UserEnteredValue: &sheets.ExtendedValue{NumberValue: &v}})
		case int64:
			v := float64(typedValue)
			cells = append(cells, &sheets.CellData{UserEnteredValue: &sheets.ExtendedValue{NumberValue: &v}})
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

func WriteStructHeader(srv *sheets.Service, spreadsheetId string, sheetId int64, structTag string, st interface{}) (*sheets.BatchUpdateSpreadsheetResponse, error) {
	typeOfSt := reflect.TypeOf(st)
	typeOfSt = typeOfSt.Elem()

	var headerTexts []interface{}
	for i := 0; i < typeOfSt.NumField(); i++ {
		tag := typeOfSt.Field(i).Tag
		tagValue := tag.Get(structTag)
		if len(tagValue) == 0 {
			continue
		}

		headerTexts = append(headerTexts, tagValue)
	}

	return AppendRow(srv, spreadsheetId, sheetId, headerTexts)
}

func WriteStructValues(srv *sheets.Service, spreadsheetId string, sheetId int64, structTag string, st interface{}) (*sheets.BatchUpdateSpreadsheetResponse, error) {
	typeOfSt := reflect.TypeOf(st)
	typeOfSt = typeOfSt.Elem()

	valueOfSt := reflect.ValueOf(st)
	valueOfSt = valueOfSt.Elem()

	var texts []interface{}
	for i := 0; i < typeOfSt.NumField(); i++ {
		tag := typeOfSt.Field(i).Tag
		tagValue := tag.Get(structTag)
		if len(tagValue) == 0 {
			continue
		}

		valueInf := valueOfSt.Field(i).Interface()

		switch typedValue := valueInf.(type) {
		case string:
			texts = append(texts, typedValue)
		case float64:
			texts = append(texts, typedValue)
		case int64:
			texts = append(texts, typedValue)
		case *float64:
			texts = append(texts, typedValue)
		case fixedpoint.Value:
			texts = append(texts, typedValue.String())
		case *fixedpoint.Value:
			texts = append(texts, typedValue.String())
		case time.Time:
			texts = append(texts, typedValue.Format(time.RFC3339))
		}
	}

	return AppendRow(srv, spreadsheetId, sheetId, texts)
}

func AppendRow(srv *sheets.Service, spreadsheetId string, sheetId int64, values []interface{}) (*sheets.BatchUpdateSpreadsheetResponse, error) {
	row := &sheets.RowData{}
	row.Values = ValuesToCellData(values)

	log.Infof("AppendRow: %+v", row.Values)
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
	log.Infof("BatchUpdateSpreadsheetResponse.SpreadsheetId: %+v", resp.SpreadsheetId)
	log.Infof("BatchUpdateSpreadsheetResponse.UpdatedSpreadsheet: %+v", resp.UpdatedSpreadsheet)
	log.Infof("BatchUpdateSpreadsheetResponse.Replies: %+v", resp.Replies)
}
