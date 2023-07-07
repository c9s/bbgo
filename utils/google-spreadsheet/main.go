package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	googleservice "github.com/c9s/bbgo/pkg/service/google"
	"github.com/c9s/bbgo/pkg/types"
)

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tokFile := "token.json"
	if p, ok := os.LookupEnv("GOOGLE_AUTH_TOKEN_FILE"); ok {
		tokFile = p
	}

	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
	}
	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func main() {
	ctx := context.Background()

	tokenFile := os.Getenv("GOOGLE_AUTH_TOKEN_FILE")

	srv, err := sheets.NewService(ctx,
		option.WithCredentialsFile(tokenFile),
	)

	if err != nil {
		log.Fatalf("Unable to new google sheets service client: %v", err)
	}

	spreadsheetId := os.Getenv("GOOGLE_SPREADSHEET_ID")
	spreadsheetUrl := googleservice.GetSpreadSheetURL(spreadsheetId)

	logrus.Infoln(spreadsheetUrl)

	spreadsheet, err := srv.Spreadsheets.Get(spreadsheetId).Do()
	if err != nil {
		log.Fatalf("unable to get spreadsheet data: %v", err)
	}

	logrus.Infof("spreadsheet: %+v", spreadsheet)

	for i, sheet := range spreadsheet.Sheets {
		logrus.Infof("#%d sheetId: %d", i, sheet.Properties.SheetId)
		logrus.Infof("#%d sheetTitle: %s", i, sheet.Properties.Title)
	}

	batchUpdateResp, err := googleservice.AddNewSheet(srv, spreadsheetId, fmt.Sprintf("Test Auto Add Sheet #%d", len(spreadsheet.Sheets)+1))
	if err != nil {
		log.Fatal(err)
	}

	googleservice.DebugBatchUpdateSpreadsheetResponse(batchUpdateResp)

	stats := types.NewProfitStats(types.Market{
		Symbol:        "BTCUSDT",
		BaseCurrency:  "BTC",
		QuoteCurrency: "USDT",
	})
	stats.TodayNetProfit = fixedpoint.NewFromFloat(100.0)
	stats.TodayPnL = fixedpoint.NewFromFloat(100.0)
	stats.TodayGrossLoss = fixedpoint.NewFromFloat(-100.0)

	_, err = googleservice.WriteStructHeader(srv, spreadsheetId, 0, "json", stats)
	if err != nil {
		log.Fatal(err)
	}

	_, err = googleservice.WriteStructValues(srv, spreadsheetId, 0, "json", stats)
	if err != nil {
		log.Fatal(err)
	}

	readRange := "Sheet1!A2:E"
	resp, err := googleservice.ReadSheetValuesRange(srv, spreadsheetId, readRange)
	if err != nil {
		log.Fatalf("Unable to retrieve data from sheet: %v", err)
	}

	if len(resp.Values) == 0 {
		fmt.Println("No data found")
	} else {
		fmt.Println("Name, Major:")
		for _, row := range resp.Values {
			// Print columns A and E, which correspond to indices 0 and 4.
			fmt.Printf("%s, %s\n", row[0], row[4])
		}
	}
}
