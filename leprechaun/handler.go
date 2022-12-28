package leprechaun

import (
	luno "github.com/luno/luno-go"
)

type ExchangeHandler interface {
	GoLong(volume float64) (longorder *OrderEntry, err error)
	StopLong(rec *Entry) (longOrder *StopOrderEntry, err error)
	GoShort(volume float64) (shortOrder *OrderEntry, err error)
	StopShort(rec *Entry) (shortOrder *StopOrderEntry, err error)
	String() string
	CurrentPrice() (float64, error)
	GetBalance(asset *Asset) (float64, error)
	CheckBalanceSufficiency(asset *Asset) (canPurchase bool, err error)
	ConfirmOrder(rec *Entry) (done bool, err error)
	PreviousTrades(numDays int64) (data map[luno.Time][]luno.Candle, err error)
	GetOrderDetails(orderID string) (orderDetails *luno.GetOrderResponse, err error)
}

type Exchange struct {
	name string

	portfolio *Portfolio
}
