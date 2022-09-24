package main

import (
	"context"
	"fmt"
	"github.com/amir-the-h/okex"
	"github.com/amir-the-h/okex/api"
	"github.com/amir-the-h/okex/requests/rest/funding"
	"log"
)

func main() {
	apiKey := "5be64ef1-aefc-43f4-a730-f00a6b05bcb7"
	secretKey := "2AEDC9F0D0B626FA55B8A4E62A2B3C20"
	passphrase := "Weidan@2022"

	dest := okex.NormalServer // The main API server
	ctx := context.Background()
	client, err := api.NewClient(ctx, apiKey, secretKey, passphrase, dest)
	if err != nil {
		log.Fatalln(err)
	}

	response, err := client.Rest.Account.GetConfig()
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("Account Config %+v", response)

	a := funding.FundsTransfer{

		Ccy: "ETH",
		Amt: 0.1,

		From: 6,
		To:   18,
	}
	rep, _ := client.Rest.Funding.FundsTransfer(a)
	fmt.Println(rep, err)
	//errChan := make(chan *events.Error)
	//subChan := make(chan *events.Subscribe)
	//uSubChan := make(chan *events.Unsubscribe)
	//lCh := make(chan *events.Login)
	//oCh := make(chan *private.Order)
	//iCh := make(chan *public.Instruments)
	//sCh := make(chan *events.Success)
	//
	//// to receive unique events individually in separated channels
	//client.Ws.SetChannels(errChan, subChan, uSubChan, lCh, sCh)
	//
	//// subscribe into orders private channel
	//// it will do the login process and wait until authorization confirmed
	//err = client.Ws.Private.Order(ws_private_requests.Order{
	//	InstType: okex.SwapInstrument,
	//}, oCh)
	//if err != nil {
	//	log.Fatalln(err)
	//}
	//
	//// subscribe into instruments public channel
	//// it doesn't need any authorization
	//err = client.Ws.Public.Instruments(ws_public_requests.Instruments{
	//	InstType: okex.SwapInstrument,
	//}, iCh)
	//if err != nil {
	//	log.Fatalln("Instruments", err)
	//}
	//
	//// starting on listening
	//for {
	//	select {
	//	case <-lCh:
	//		log.Print("[Authorized]")
	//	case sub := <-subChan:
	//		channel, _ := sub.Arg.Get("channel")
	//		log.Printf("[Subscribed]\t%s", channel)
	//	case uSub := <-uSubChan:
	//		channel, _ := uSub.Arg.Get("channel")
	//		log.Printf("[Unsubscribed]\t%s", channel)
	//	case err := <-client.Ws.ErrChan:
	//		log.Printf("[Error]\t%+v", err)
	//	case o := <-oCh:
	//		log.Print("[Event]\tOrder")
	//		for _, p := range o.Orders {
	//			log.Printf("\t%+v", p)
	//		}
	//	case i := <-iCh:
	//		log.Print("[Event]\tInstrument")
	//		for _, p := range i.Instruments {
	//			log.Printf("\t%+v", p)
	//		}
	//	case e := <-client.Ws.StructuredEventChan:
	//		log.Printf("[Event] STRUCTED:\t%+v", e)
	//		v := reflect.TypeOf(e)
	//		switch v {
	//		case reflect.TypeOf(events.Error{}):
	//			log.Printf("[Error] STRUCTED:\t%+v", e)
	//		case reflect.TypeOf(events.Subscribe{}):
	//			log.Printf("[Subscribed] STRUCTED:\t%+v", e)
	//		case reflect.TypeOf(events.Unsubscribe{}):
	//			log.Printf("[Unsubscribed] STRUCTED:\t%+v", e)
	//		}
	//	case e := <-client.Ws.RawEventChan:
	//		log.Printf("[Event] RAW:\t%+v", e)
	//	case b := <-client.Ws.DoneChan:
	//		log.Printf("[End]:\t%v", b)
	//		return
	//	}
	//}
}
