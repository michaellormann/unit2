package leprechaun

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
	"github.com/gonum/stat"

	luno "github.com/luno/luno-go"
	luno_decimal "github.com/luno/luno-go/decimal"
)

var (
	timeFormat = "15:08:14"
)

// LunoExchangeHandler
type LunoExchangeHandler struct {
	asset          *Asset
	client         *luno.Client
	assets         map[string]*Asset
	config         *Configuration
	sessionVolume  float64
	sessionBalance float64
	currency       string
	spread         float64
	retries        int64
	signalChan     chan SIGNAL
	debugChan      chan string
	ctx            context.Context
}

func NewLunoExchangeHandler(client *luno.Client, asset *Asset, ctx context.Context) *LunoExchangeHandler {
	return &LunoExchangeHandler{
		asset:      asset,
		client:     client,
		signalChan: make(chan SIGNAL),
		debugChan:  make(chan string),
		ctx:        ctx}
}

func (handler *LunoExchangeHandler) String() string {
	return handler.asset.name
}

func (handler *LunoExchangeHandler) handle429(e error) (retry bool) {
	if e.Error() == "luno: too many requests" {
		retry = true
		time.Sleep(1 * time.Second) // wait a bit
	}
	return
}

func (handler *LunoExchangeHandler) debug(v ...interface{}) {
	// write to stdout
	go func() { log.Println(v...) }()
}

func (handler *LunoExchangeHandler) debugf(format string, v ...interface{}) {
	// write to stdout
	go func() { log.Printf(format, v...) }()
}

func (handler *LunoExchangeHandler) profitAndLoss(open, close *Entry) {

}

// bid places an order to buys a specified amount of an asset on the exchange
// It executes immediately.
func (handler *LunoExchangeHandler) bid(price float64, volume float64) (orderID string, err error) {
	sleep() // Error 429 safety
	cost := price * volume
	handler.debugf("Placing bid order for NGN %.2f worth of %s (approx. %.2f %s) on the exchange...\n", cost, handler.asset.name, volume, handler.asset.code)
	//Place bid order on the exchange
	req := luno.PostMarketOrderRequest{Pair: handler.asset.Pair, Type: luno.OrderTypeBuy,
		BaseAccountId: stringToInt(handler.asset.accountID), CounterAccountId: stringToInt(handler.asset.fiatAccountID),
		CounterVolume: decimal(cost)}
	res, err := handler.client.PostMarketOrder(handler.ctx, &req)
	if err != nil {
		return
	}
	orderID = res.OrderId
	handler.debugf("Bid order for %.4f %s has been placed on the exchange.\n", volume, handler.asset.name)
	return
}

// ask places a bid order on the excahnge to sell `volume` worth of Client.asset in exhange for fiat currency.
func (handler *LunoExchangeHandler) ask(price, volume float64) (orderID string, err error) {
	sleep() // Error 429 safety
	cost := price * volume
	//Place ask order on the exchange
	log.Printf("Placing ask order for ~NGN %.2f worth of %s on the exchange...\n", cost, handler.asset.name)
	log.Printf("Current price is %4f\n", price)
	log.Printf("Order Volume: %v", volume)
	req := luno.PostMarketOrderRequest{Pair: handler.asset.Pair, Type: luno.OrderTypeSell,
		BaseAccountId: stringToInt(handler.asset.accountID), BaseVolume: decimal(volume),
		CounterAccountId: stringToInt(handler.asset.fiatAccountID)}
	res, err := handler.client.PostMarketOrder(handler.ctx, &req)
	if err != nil {
		log.Printf("(in `Client.ask`) %v", err.Error())
		return
	}
	orderID = res.OrderId
	log.Printf("Ask order for %.4f %s has been placed on the exchange.\n", volume, handler.asset.code)
	return
}

// GoLong buys an asset at a specific price with the intention that the asset will
// later be sold at a higher price to realize a profit.
func (handler *LunoExchangeHandler) GoLong(volume float64) (longOrder *OrderEntry, err error) {
	// goLong
	price, err := handler.CurrentPrice()
	if err != nil {
		return nil, err
	}
	ts := time.Now().Format(timeFormat)
	// Place market bid order.
	purchaseOrderID, err := handler.bid(price, volume)
	if err != nil {
		log.Printf("An error occured while going long!")
		return nil, err
	}

	handler.debug("New Long Trade Initiated. Order ID:", purchaseOrderID)
	handler.sessionVolume += volume

	return &OrderEntry{handler.asset.code, purchaseOrderID, ts, price, volume}, nil
}

// Stop Long closes a long order
func (handler *LunoExchangeHandler) StopLong(entry *Entry) (longOrder *StopOrderEntry, err error) {
	price, err := handler.CurrentPrice()
	if err != nil {
		return nil, err
	}
	ts := time.Now().Format(timeFormat)
	saleOrderID, err := handler.ask(price, entry.PurchaseVolume)
	if err != nil {
		log.Printf("An error occured while executing a stop long order! Reason: %s", err.Error())
		if strings.Contains(err.Error(), "ErrInsufficientBalance") {
			log.Printf("Your %s balance is insufficient to execute a short trade. Fund your account or specify a lower purchase unit.", handler.asset.name)
		}
		return nil, err
	}
	cost := price * entry.PurchaseVolume
	handler.sessionBalance += cost
	handler.debug("Order ID:", saleOrderID)

	return &StopOrderEntry{OrderEntry{handler.asset.name, saleOrderID, ts, price, entry.PurchaseVolume}}, nil
	// handler.debug(record.String())
}

// GoShort sells an asset at a certain price with the aim of repurchasing the same
// volume of asset sold at a lower price in the future to realize a profit.
// TODO XXX: Implement stoploss for short sold assets
// TODO: Make short-selling an  option
func (handler *LunoExchangeHandler) GoShort(volume float64) (shortOrder *OrderEntry, err error) {
	// goShort
	price, err := handler.CurrentPrice()
	if err != nil {
		log.Println("Could not retrieve price info from the exchange. (in `Client.GoShort`)")
		return nil, err
	}
	ts := time.Now().Format(timeFormat)
	saleOrderID, err := handler.ask(price, volume)
	if err != nil {
		log.Printf("An error occured while executing a short order! Reason: %s", err.Error())
		if strings.Contains(err.Error(), "ErrInsufficientBalance") {
			log.Printf("Your %s balance is insufficient to execute a short trade. Fund your account or specify a lower purchase unit.", handler.asset.name)
		}
		return nil, err
	}
	cost := price * volume
	handler.sessionBalance += cost
	handler.debug("Order ID:", saleOrderID)

	return &OrderEntry{handler.asset.name, saleOrderID, ts, price, volume}, nil
}

func (handler *LunoExchangeHandler) StopShort(entry *Entry) (*StopOrderEntry, error) {
	price, err := handler.CurrentPrice()
	if err != nil {
		return nil, err
	}
	ts := time.Now().Format(timeFormat)
	// Place market bid order.
	purchaseOrderID, err := handler.bid(price, entry.SaleVolume)
	if err != nil {
		log.Printf("An error occured (handler.StopShort)")
		return nil, err
	}

	handler.debug("Order ID:", purchaseOrderID)
	handler.sessionVolume += entry.SaleVolume

	return &StopOrderEntry{OrderEntry{handler.asset.name, purchaseOrderID, ts, entry.SaleVolume, price}}, nil
}

// CheckOrder tries to confirm if an order is still pending or not
func (handler *LunoExchangeHandler) GetOrderDetails(orderID string) (orderDetails *luno.GetOrderResponse, err error) {
	sleep() // Error 429 safety
	req := luno.GetOrderRequest{Id: orderID}
	orderDetails, err = handler.client.GetOrder(handler.ctx, &req)
	if err != nil {
		handler.debug(err)
		return orderDetails, err
	}
	if orderDetails.State == luno.OrderStatePending {
		return &luno.GetOrderResponse{}, errors.New("Order is still pending")
	}
	return
}

// ConfirmOrder checks if an order placed on the exchange has been executed
func (handler *LunoExchangeHandler) ConfirmOrder(rec *Entry) (done bool, err error) {
	// Make this method a goroutine
	if rec.Status == 0 {
		sleep() // Error 429 safety
		req := luno.GetOrderRequest{Id: rec.SaleID}
		res, err := handler.client.GetOrder(handler.ctx, &req)
		if err != nil {
			handler.debug("Error! Could not confirm order: ", rec.SaleID)
			handler.debug("Please check your network connectivity")
			handler.debug(err.Error())
		}
		if res.State == luno.OrderStateComplete {
			rec.Status = 1
		}
		done = true
		// Note other details of the response object should be used to update sale history and calculate profit.
		// Should be implemented by update_ledger function.
	}
	return
}

func (handler *LunoExchangeHandler) GetBalance(asset *Asset) (balance float64, err error) {
	sleep() // Error 429 safety
	assetBalanceReq := luno.GetBalancesRequest{Assets: []string{asset.Pair}}
	assetBalance, err := handler.client.GetBalances(handler.ctx, &assetBalanceReq)
	if err != nil {
		return balance, err
	}
	if assetBalance != nil && len(assetBalance.Balance) > 0 {
		for _, astBal := range assetBalance.Balance {
			if astBal.Asset == handler.asset.name {
				handler.asset.accountID = astBal.AccountId
				asset.assetBalance = astBal.Balance.Float64()
			}
			if astBal.Asset == handler.currency {
				handler.asset.fiatAccountID = astBal.AccountId
				handler.asset.fiatBalance = astBal.Balance.Float64()
			}
		}
	}
	log.Printf("%#v \n", assetBalance)
	handler.debug(handler.asset.fiatBalance)
	err = nil
	return
}

// CheckBalanceSufficiency determines whether the client has purchasing power
func (handler *LunoExchangeHandler) CheckBalanceSufficiency(asset *Asset) (canPurchase bool, err error) {
	// Luno charges a 1% taker fee
	purchaseUnit := globalConfig.AdjustedPurchaseUnit
	if handler.asset.fiatBalance <= 0.0 {
		handler.GetBalance(asset)
	}
	if handler.asset.fiatBalance < purchaseUnit {
		// `AdjustedPurchaseUnit` is more than available balance (NGN)
		canPurchase = false
	} else {
		canPurchase = true
	}
	return
}

// StopPendingOrder tries to remove a pending order from the order book
func (handler *LunoExchangeHandler) StopPendingOrder(orderID string) (ok bool) {
	sleep() // Error 429 safety
	req := luno.StopOrderRequest{OrderId: orderID}
	res, err := handler.client.StopOrder(handler.ctx, &req)
	if err != nil {
		handler.debug(err)
		return false
	}
	if res.Success {
		return true
	}
	return
}

// CurrentPrice retrieves the ask price for the client's asset.
func (handler *LunoExchangeHandler) CurrentPrice() (price float64, err error) {
	sleep() // Error 429 safety
	// TODO: UPDATE PRICE AUTOMATICALLY EVERY 180 SECS and return that value to any callers until the next update.
	// No need to connect everytime
	req := luno.GetTickerRequest{Pair: handler.asset.Pair}
	res, err := handler.client.GetTicker(handler.ctx, &req)
	if err != nil {
		return
	}
	price = res.Ask.Float64()
	handler.spread = res.Ask.Float64() - res.Bid.Float64()
	return
}

type mDate struct {
	day   int
	month time.Month
	year  int
	str   string
}

func (d *mDate) newDate(year int, month time.Month, day int) string {
	return fmt.Sprintf("%v-%d-%v", year, month, day)
}

type Hour4Trades struct {
	start, end time.Time
	candles    []luno.Candle
}

// PreviousTrades retreives past trades/prices from the exchange. Trades are grouped at specified intervals.
// It is targeted for use in a candlestick chart. It is important to note that the data is
// returned in reverse form. i.e. The most recent price is last in the list and the earliest is first.
func (handler *LunoExchangeHandler) PreviousTrades(numDays int64) (data map[luno.Time][]luno.Candle, err error) {
	now := time.Now()
	// numDays = 3
	midnight := toMidnight(now)
	seconds := 28800 // 8 hours
	var D = mDate{}
	var dates = map[luno.Time]string{}
	var startTimes = []luno.Time{}
	var dailyTrades = map[luno.Time][]luno.Candle{}
	var Trades = map[int64][]map[luno.Time][]luno.Candle{}

	for h := 0.0; h <= float64(8*numDays); h += 8 {
		t := luno.Time(midnight.Add(time.Duration(-h) * time.Hour))
		startTimes = append(startTimes, t)
		dailyTrades[t] = []luno.Candle{}
		if dates[t] == "" {
			dates[t] = D.newDate(time.Time(t).Date())
		}
	}
	for i := int64(0); i < numDays; i++ {
		Trades[i] = []map[luno.Time][]luno.Candle{}
	}
	// Reverse the order of the timestamps. The earliest should be first in the list and the latest should come last.
	reverseTimestamps(startTimes)
	log.Println("STARTTIMES", startTimes)
	// log.Println("DAILYTRADES", dailyTrades)
	// log.Println("DATES", dates)
	// Retrieve past trades from the exchange.
	for _, start := range startTimes {
		sleep2()
		req := luno.GetCandlesRequest{Pair: handler.asset.Pair, Since: start, Duration: int64(seconds)}
		res, err := handler.client.GetCandles(handler.ctx, &req)
		if err != nil {
			log.Fatal(handler.asset.Pair, err)
		}
		dailyTrades[start] = append(dailyTrades[start], res.Candles...)
	}
	return dailyTrades, nil
}

// FeeInfo retrieves taker/maker fee information for this client
func (handler *LunoExchangeHandler) FeeInfo() (info luno.GetFeeInfoResponse, err error) {
	sleep() // Error 429 safety
	req := luno.GetFeeInfoRequest{Pair: handler.asset.Pair}
	res, err := handler.client.GetFeeInfo(handler.ctx, &req)
	if err != nil {
		return
	}
	info = *res
	return
}

// TopOrders retrieves the top ask and bid orders on the exchange
func (handler *LunoExchangeHandler) TopOrders() (orders map[string]luno.OrderBookEntry) {
	sleep() // Error 429 safety
	req := luno.GetOrderBookRequest{Pair: handler.asset.Pair}
	orderBook, err := handler.client.GetOrderBook(handler.ctx, &req)
	if err != nil {
		handler.debug(err)
	}
	topAsk := orderBook.Asks[0]
	topBid := orderBook.Bids[0]
	orders["ask"] = topAsk
	orders["bid"] = topBid
	return
}

// PendingOrders retrieves unexecuted orders still in the order book.
func (handler *LunoExchangeHandler) PendingOrders() (pendingOrders interface{}) {
	sleep() // Error 429 safety
	accID := stringToInt(handler.asset.fiatAccountID)
	req := luno.ListPendingTransactionsRequest{Id: accID}
	res, err := handler.client.ListPendingTransactions(handler.ctx, &req)
	if err != nil {
		handler.debug(err)
	}
	pending := res.Pending
	numPending := len(pending)
	if numPending == 0 {
		handler.debug("There are no pending transactions associated with", handler)
		pendingOrders = []string{}
	}
	handler.debug("There are", numPending, "transactions associated with", handler)

	pendingOrders = pending
	return
}

// Decimal converts a float64 value to a Decimal representation of scale 10
func decimal(val float64) (dec luno_decimal.Decimal) {
	dec = luno_decimal.NewFromFloat64(val, 4)
	return
}

func reverseTimestamps(stamps []luno.Time) {
	for i, j := 0, len(stamps)-1; i < j; i, j = i+1, j-1 {
		stamps[i], stamps[j] = stamps[j], stamps[i]
	}
}

func reverseSlice(slice []luno.PublicTrade) {
	for i, j := 0, len(slice)-1; i < j; i, j = i+1, j-1 {
		slice[i], slice[j] = slice[j], slice[i]
	}
}
