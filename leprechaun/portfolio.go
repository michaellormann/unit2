package leprechaun

import (
	"context"
	"fmt"
	"time"

	"github.com/luno/luno-go"
)

type Order int
type EntryStatus int
type SIGNAL int

const (
	SignalLong SIGNAL = iota
	SignalShort
	SignalWait
)

const (
	OpenLongTrade Order = iota
	OpenShortTrade
	CloseLongTrade
	CloseShortTrade
)

const (
	Open EntryStatus = iota
	Closed
)

var (
	BITCOIN          = &Asset{name: "BITCOIN", code: "XBT"}
	ETHEREUM         = &Asset{name: "ETHEREUM", code: "ETH"}
	LITECOIN         = &Asset{name: "LITECOIN", code: "LTC"}
	RIPPLE           = &Asset{name: "RIPPLE", code: "XRP"}
	BCASH            = &Asset{name: "BITCOIN CASH", code: "BCH"}
	DEFAULT_ASSETS   = []*Asset{BITCOIN, ETHEREUM, LITECOIN, RIPPLE}
	DEFAULT_CURRENCY = "NGN"
)

// Asset holds all details for a specific currency pair.
type Asset struct {
	name           string
	code           string
	Pair           string
	accountID      string
	fiatAccountID  string
	assetBalance   float64
	fiatBalance    float64
	sessionBalance float64 // Fiat balance locked to complete short orders
	sessionVolume  float64 // Volume of asset locked to complete long orders
	currency       string
	spread         float64 // Bid-Ask spread
	minOrderVol    float64 // Minimum volume that can be traded on the exchange
}

type Entry struct {
	Asset          string
	PurchaseCost   float64
	SaleCost       float64
	ID             string
	PurchasePrice  float64
	SalePrice      float64
	SaleID         string
	Status         int64
	Timestamp      string
	PurchaseVolume float64
	SaleVolume     float64
	Profit         float64
	Type           Order
	TriggerPrice   float64
	Updated        bool // order details have been updated with server side values

	// Update legder code first to reflect new struct fields.
	LunoAssetFee float64
	LunoFiatFee  float64
	// PPercent  float64 // Profit Percentage
}

// IsRipe checks whether a record is ready for sale per the user specified proift margin,.
func (rec Entry) IsRipe(currentPrice float64, updateProfitMargin bool) bool {
	// checks whether an asset is ready for sale
	if rec.Type == OpenLongTrade {
		// to be sold at a higher price than it was purchased
		if updateProfitMargin {
			// user may have changed desired profitMargin. Recalculate
			rec.TriggerPrice = rec.PurchasePrice + (rec.PurchasePrice * globalConfig.ProfitMargin)
		}
		return currentPrice >= rec.TriggerPrice
	} else if rec.Type == OpenShortTrade {
		// to be repurchased at a lower price than it was sold
		if updateProfitMargin {
			// user may have changed desired profitMargin. Recalculate
			rec.TriggerPrice = rec.PurchasePrice - (rec.PurchasePrice * globalConfig.ProfitMargin)
		}
		return currentPrice >= rec.TriggerPrice
	}
	return false
}

type Portfolio struct {
	assets       map[string]ExchangeHandler
	config       *Configuration
	ledger       *Ledger2
	signalChan   chan SIGNAL
	errChan      chan error
	debugChan    chan string
	waitLock     chan struct{}
	waitInterval time.Duration
	ctx          context.Context
}

func GetPortfolio(ctx context.Context) *Portfolio {
	return &Portfolio{
		assets:     make(map[string]ExchangeHandler),
		config:     globalConfig,
		signalChan: make(chan SIGNAL),
		waitLock:   make(chan struct{}, 1),
		ctx:        ctx,
	}
}

func (pf *Portfolio) Init() (err error) {
	// this initializes a new luno client for each asset pair
	if len(pf.config.APIKeyID) == 0 || len(pf.config.APIKeySecret) == 0 {
		return ErrInvalidAPICredentials
	}
	for _, asset := range DEFAULT_ASSETS { // TODO: LET USER DETERMINE ASSETS TO BE TRADED
		asset.Pair = asset.code + DEFAULT_CURRENCY // E.g. XBTNGN
		client := luno.NewClient()
		client.SetAuth(pf.config.APIKeyID, pf.config.APIKeySecret)
		if asset.code == "XRP" {
			asset.minOrderVol = 1
		} else {
			asset.minOrderVol = 0.0005
		}
		if err != nil {
			return
		}
		pf.assets[asset.name] = NewLunoExchangeHandler(client, asset, pf.ctx)
	}
	// init waitlock to allow initial round
	pf.waitLock <- struct{}{}
	return nil
}

func (pf *Portfolio) analyzeMarkets() {
	// for asset, handler := range pf.assets {
	// 	currentPrice, err := handler.CurrentPrice()
	// 	if err != nil {
	// 		raise(err)
	// 		continue
	// 	}
	// 	historicPrices, err := handler.PreviousPrices(108, M45)

	// }
	testSigs := []SIGNAL{SignalLong, SignalShort, SignalWait, SignalWait, SignalShort, SignalLong}
	for _, sig := range testSigs {
		pf.signalChan <- sig
		time.Sleep(15 * time.Second)
	}
}

func (pf *Portfolio) acquireWaitLock() {
	time.Sleep(pf.waitInterval)
	pf.waitLock <- struct{}{}
}

func (pf *Portfolio) Trade() {
	for {
		<-pf.waitLock

		for _, handler := range pf.assets {
			signal := <-pf.signalChan
			fmt.Printf("Received signal: %v\n", signal)
			switch signal {
			case SignalLong:
				purchase, err := handler.GoLong(pf.config.AdjustedPurchaseUnit)
				if err != nil {
					// TODO: HANDLE ERRORS BETTER
					fmt.Printf("Trading error: %s. Will skip\n", err)
					continue
				}
				pf.openTrade(purchase, OpenLongTrade)
			case SignalShort:
				sale, err := handler.GoShort(pf.config.AdjustedPurchaseUnit)
				if err != nil {
					// TODO: HANDLE ERRORS BETTER
					fmt.Printf("Trading error: %s. Will skip\n", err)
					continue
				}
				pf.openTrade(sale, OpenShortTrade)
			case SignalWait:
				go pf.acquireWaitLock()

			}
		}
	}
}

func (pf *Portfolio) openTrade(order *OrderEntry, orderType Order) (entry Entry) {
	switch orderType {
	case OpenLongTrade:
		// new position. added to ledger
		entry.PurchasePrice = order.Price
		entry.PurchaseCost = order.Price * order.Volume
		entry.PurchaseVolume = order.Volume
		entry.TriggerPrice = order.Price + (order.Price * globalConfig.ProfitMargin)
		// save to ledger

	case OpenShortTrade:
		// new postion. add to ledger
		entry.SalePrice = order.Price
		entry.SaleVolume = order.Volume
		entry.SaleCost = order.Price * order.Volume
		entry.TriggerPrice = order.Price - (order.Price * globalConfig.ProfitMargin)
	}

	if !entry.Updated {
	}
	pf.updateOrderDetails(&entry)
	if !pf.ledger.isOpen {
		pf.ledger.loadDatabase()
	}
	defer pf.ledger.Save()
	pf.ledger.AddRecord(entry)

	return entry
}

func (pf *Portfolio) closeTrade(entry *Entry, asset string, price float64, timestamp string, volume float64, id string, orderType Order) {
	switch orderType {
	case CloseLongTrade:
		entry.SalePrice = price
		entry.SaleVolume = volume
		entry.SaleCost = price * volume
		entry.Profit = entry.PurchaseCost - entry.SaleCost
		entry.Status = 1

	case CloseShortTrade:
		entry.PurchasePrice = price
		entry.PurchaseVolume = volume
		entry.PurchaseCost = price * volume
		entry.Profit = entry.PurchaseCost - entry.SaleCost

	}
	if !pf.ledger.isOpen {
		pf.ledger.loadDatabase()
	}
	defer pf.ledger.Save()
	pf.ledger.AddRecord(*entry)
}

func (pf *Portfolio) CloseLongPositions() (err error) {
	// TODO: Make async i.e. an infinite loop. sleep between each round
	for asset, handler := range pf.assets {
		longOrders, err := pf.ledger.GetRecordsByType(asset, OpenLongTrade)
		if err != nil {
			return err
		}
		for _, order := range longOrders {
			currentPrice, err := handler.CurrentPrice()
			if err != nil {
				return err
			}
			if order.IsRipe(currentPrice, true) {
				// Sell Long Assets
				handler.StopLong(&order)
			}
		}
	}
	return nil
}

func (pf *Portfolio) CloseShortPositions() (err error) {
	for asset, handler := range pf.assets {
		longOrders, err := pf.ledger.GetRecordsByType(asset, OpenShortTrade)
		if err != nil {
			return err
		}
		for _, order := range longOrders {
			currentPrice, err := handler.CurrentPrice()
			if err != nil {
				return err
			}
			if order.IsRipe(currentPrice, true) {
				// Sell Long Assets
				handler.StopLong(&order)
			}
		}
	}
	return nil
}

// UpdateOrderDetails updates order details
func (pf *Portfolio) updateOrderDetails(entry *Entry) (updated bool) {
	handler := pf.assets[entry.Asset]
	orderDetails, err := handler.GetOrderDetails(entry.ID)
	if err != nil {
		// return record unchanged
		return false
	}
	copy := *entry
	switch entry.Type {
	case OpenLongTrade:
		copy.LunoFiatFee = orderDetails.FeeCounter.Float64()
		copy.PurchaseCost = orderDetails.Counter.Float64()
		copy.PurchaseVolume = orderDetails.Base.Float64()
		copy.PurchasePrice = entry.PurchaseCost / entry.PurchaseVolume
		copy.LunoAssetFee = orderDetails.FeeBase.Float64()
		copy.Timestamp = orderDetails.CompletedTimestamp.String()
	case OpenShortTrade:
		copy.LunoFiatFee = orderDetails.FeeCounter.Float64()
		copy.SaleCost = orderDetails.Counter.Float64()
		copy.SaleVolume = orderDetails.Base.Float64()
		copy.SalePrice = entry.SaleCost / entry.SaleVolume
		copy.LunoAssetFee = orderDetails.FeeBase.Float64()
		copy.Timestamp = orderDetails.CompletedTimestamp.String()

	case CloseLongTrade:

	case CloseShortTrade:

	}
	fmt.Println("Record updated from: ")
	fmt.Printf("%#v\n", entry)
	fmt.Println("To:")
	fmt.Printf("%#v\n", copy)
	entry = &copy
	entry.Updated = true
	return
}

func (pf *Portfolio) compileReport() {
	// collate profit/loss/hodl data accross all asset classes
}

// helper fuction
func (pf *Portfolio) recordNewProfit(asset *Asset) {

}
