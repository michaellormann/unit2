package leprechaun

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
)

var (
	globalConfig             *Configuration
	ErrInvalidAPICredentials error = errors.New("invalid api uid")
)

// Session defines parameters for a single trading session
type Session struct {
	startTime    time.Time
	ledger       *Ledger2
	elapsed      time.Duration
	sold         float64
	purchased    float64
	profit       float64
	portfolio    *Portfolio
	config       *Configuration
	exc          *Exchange
	analysisFunc *Analyzer
	debugChan    chan string
	errChan      chan error
	done         chan struct{}
}

func NewSession(ctx context.Context) *Session {
	globalConfig = new(Configuration)
	globalConfig.TestConfig(".") // test
	session := &Session{
		portfolio: GetPortfolio(ctx),
		config:    globalConfig,
	}
	session.errChan = make(chan error)
	session.debugChan = make(chan string)
	session.portfolio.errChan = session.errChan
	session.portfolio.debugChan = session.debugChan
	return session
}

func (s *Session) Initialize() (err error) {
	// this initializes a new luno client for each asset pair
	if len(s.config.APIKeyID) == 0 || len(s.config.APIKeySecret) == 0 {
		return ErrInvalidAPICredentials
	}
	s.ledger = GetLedger2()
	s.portfolio.ledger = s.ledger

	err = s.portfolio.Init()
	if err != nil {
		// Exchange API rejected API key.
		if strings.Contains(err.Error(), "ErrAPIKeyNotFound") {
			log.Print("Incorrect API KEY!")
			return err
		}
		// API Key has been revoked.
		if strings.Contains(err.Error(), "ErrAPIKeyRevoked") {
			log.Print("The API Key you has been revoked!")
			return err
		}
		// Could not connect to remote host.
		if strings.Contains(err.Error(), "no such host") || strings.Contains(err.Error(), "No address associated with hostname") {
			log.Println("Network error!")
			return err
		}
		if strings.Contains(err.Error(), "context deadline exceeded") {
			log.Println("time out.")
			return err
		}
		log.Println("Could not initialize client. Reason: ", err)
		return err
	}
	return nil
}

func (s *Session) Start() {
	s.startTime = time.Now()
	go s.portfolio.analyzeMarkets()
	go s.portfolio.Trade()
	go s.portfolio.CloseLongPositions()
	go s.portfolio.CloseShortPositions()
	<-s.done
	s.elapsed = time.Since(s.startTime)
	fmt.Printf("Session duration: %s/n", s.elapsed)
	fmt.Printf("Total sold: %.2f/n", s.sold)
	fmt.Printf("Total purchased: %.2f/n", s.purchased)
}

func (s *Session) Stop() {
	s.done <- struct{}{}
}

func (s *Session) debug(v ...interface{}) {
	fmt.Println(v...)
}

func (s *Session) GetPrices() {
	for asset, handler := range s.portfolio.assets {
		data, err := handler.PreviousTrades(5)
		if err != nil {
			log.Printf("%v", err)
			continue
		}
		fmt.Println("OHLC DATA FOR ", asset)
		fmt.Println(data)
	}
}

func raise(err error) {
	fmt.Println("ERROR::", err)
}
