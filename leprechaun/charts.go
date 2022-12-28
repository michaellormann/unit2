package leprechaun

/* This file is part of Leprechaun.
*  @author: Michael Lormann
*  `analyzer.go` holds the [basic] technical analysis logic for Leprechaun.
 */

import (
	"errors"
	"fmt"
	"math"
	"time"
)

// Analyzer defines the interface for an arbitrary analysis pipeline.
// the `SetClosingPrices()` and `SetOHLC()` function takes in a list of historical prices of any asset,
// the analysis is done on the price data. The data is retrieved from the exchange, each time.
//
//	and the `Emit` function returns the signal based on the analysis done.
type Analyzer interface {
	// Emit returns the final market signal based on the analysis done by the analyzer plugin.
	Emit() (SIGNAL, error)
	// GetClosingPrices recieves the closing prices over a time period from the bot.
	SetClosingPrices(prices []float64) error
	// GetOHLC receives the OHLC data of trades from the bot. The number of data points and time range is
	// specified by the PriceDimensions() function.
	SetOHLC(candles []OHLC) error
	// SetCurrentPrice passes the current ask price of the asset to the analysis plugin
	SetCurrentPrice(float64) error
	// SetOptions recieves the bots preferred analyzer configuration
	SetOptions(opts *AnalysisOptions) error
	// Description returns a short explanation of the plugins functionality.
	Description() string
}

type timeInterval time.Duration

const (
	// M15 - 15 Minutes
	M15 = 15 * time.Minute
	// M30 - 30 Minutes
	M30 = 30 * time.Minute
	// M45 - 45 Minutes
	M45 = 45 * time.Minute
	// H1 - 1 Hour
	H1 = 1 * time.Hour
	// H2 - 2 Hours
	H2 = 2 * time.Hour
	// H3 - 3 Hours
	H3 = H1 + H2
	// H4 - 4 Hours
	H4 = H2 * 2
	// H6 - 6 Hours
	H6 = H3 * 2
	// H12 - 12 Hours
	H12 = H6 * 2
	// H18 - 18 Hours
	H18 = H6 * 3
	// H24 - 24 Hours
	H24 = H12 * 2
	// H48 - 48 Hours
	H48 = H24 * 2
	// H72 - 72 Hours
	H72 = H24 * 3
)

// AnalysisOptions are configuration information passed from the bot to the analyzer.
type AnalysisOptions struct {
	// AnalysisPeriod is the period for which the analysis is carried out.
	//  e.g. 24 hours to analyze past price data for the past day.
	AnalysisPeriod time.Duration
	// Interval is the period between each data point.
	// e.g. One hour to retrieve hourly data.
	Interval time.Duration
	// Mode is the trading mode for each
	Mode TradeMode
}

// TradeMode specifies the manner an upward or downward price trend is interpreted by Leprechaun.
// TradeMode only deals with price trend and hence it should not be the only indicator used in price
// technical analysis.
type TradeMode uint

const (
	// Contrarian Mode assumes that a price trend in any direction will be followed
	// by a reversal in the opposite direction. For example, if the price of an asset has been
	// steadily falling for a period of time, Contrarian mode lets the bot buy the asset, with
	// the hope of selling it at a higher price when the trend reverses. Conversely, an asset
	// on an uptrend is sold to hedge against losses when the price trend reverses.
	Contrarian TradeMode = iota
	// TrendFollowing mode assumes that a price movement in any direction (up or down)
	// will tend to continue in that manner. For example, if the price of an asset, say Bitcoin,
	// has been rising steadily for the past few hours, this mode assumes that the price will
	// continue to rise even further, this means the bot buys an asset on the rise with the hope
	// of selling it at an even higher price.
	TrendFollowing
)

// PricePosition indicates whether the current price is above or below an indicator e.g. the moving average.
type PricePosition struct {
	Above, Below, Stable bool
	Margin               float64
}

// MovingAverage ...
type MovingAverage struct {
	Period int // Number of datapoints considered.
	Window int
}

// LineChart is a chart that uses the closing prices of an asset over a specific period of time as data points.
type LineChart struct {
	Prices        []float64
	Trend         ChartTrend
	Start, Stop   time.Time
	Interval      time.Duration
	MovingAverage map[string]int
	LinesData     [3]float64
}

// NewLineChart creates a price chart with the closing price of each time interval
// used as each individual data point.
func NewLineChart(prices []float64) LineChart {
	chart := LineChart{
		MovingAverage: map[string]int{"PERIOD": 20, "WINDOW": 2},
	}
	chart.Prices = prices
	// chart.DetectTrend()
	return chart
}

// DetectTrend tries to detect the overall sentiment of the chart.
// If the price at any point is higher than its next price it
// signifies a drop in price, and vice versa.
// If the score is positive, there has been a relative uptrend in price movement
// if the score is negative, price movement has been downward
func (chart LineChart) DetectTrend() {
	score := 0
	for x := 0; x < len(chart.Prices)-1; x++ {
		if chart.Prices[x] > chart.Prices[x+1] {
			// a datapoint is less than the one before it. Indicates a reduction in price
			score--
		} else if chart.Prices[x] < chart.Prices[x+1] {
			// a datapoint is higher than the one before it. Indicates an increase in price
			score++
		}
	}
	if score > 0 {
		chart.Trend = Bullish
	} else if score < 0 {
		chart.Trend = Bearish
	} else {
		chart.Trend = Indifferent
	}

}

// OHLC holds the Open-High-Low-Close data for a range of prices
type OHLC struct {
	Open                 float64              // Opening Price
	High                 float64              // Highest Price
	Low                  float64              // Lowest Price
	Close                float64              // Closing Price
	Range                float64              // Difference between Opening and Closing prices
	percentChange        float64              // Percent change in price
	Period               time.Duration        // unit of time being represented
	Time                 time.Time            // start time for this specific candle
	Trend                ChartTrend           // Overall Price trend
	Prices               *[]float64           // A pointer to the price list
	TotalVolume          float64              // Total traded volume of the period in question
	Patterns             []CandlestickPattern // Patterns that the most recent candles in the chart form.
	UpperTail, LowerTail float64
	ID                   int // A unique number that identifies a candle in a series
}

// doOHLC to extract OHLC info from a list of prices for a given time range
func doOHLC(startTime time.Time, prices []float64, volume float64) OHLC {
	candle := OHLC{Prices: &prices, TotalVolume: volume, Time: startTime.Truncate(time.Hour).Truncate(time.Minute), Period: time.Hour}
	candle.Close = prices[len(prices)-1]
	candle.Open = prices[0]
	candle.High = Max64(prices)
	candle.Low = Min64(prices)
	candle.Range = candle.Close - candle.Open
	candle.percentChange = (candle.Range * 100) / candle.Open
	if candle.Range < 1.0 {
		// Negative price movement
		candle.Trend = Bearish
	} else {
		// Positive price movement
		candle.Trend = Bullish
	}
	switch candle.Trend {
	case Bullish:
		candle.UpperTail = candle.High - candle.Close
		candle.LowerTail = candle.Open - candle.Low
	case Bearish:
		candle.UpperTail = candle.High - candle.Open
		candle.LowerTail = candle.Close - candle.Low
	}
	// candle.Period = time.Hour
	return candle
}

// BB calculates the bollinger bands for a time series
func BB(prices float64, SMA, deviation int64) {
	// Calculate the simple moving average
	// window = 1
}

// IsBullish returns true if the candle closes at a higher price than its open price.
func (candle OHLC) IsBullish() bool {
	return candle.Trend == Bullish
}

// IsBearish returns true if the candle closes at a lower price than its open price.
func (candle OHLC) IsBearish() bool {
	return candle.Trend == Bearish
}

// IsDoji returns true if a candles opening price is virtually the same with its closing price.
// See `https://www.investopedia.com/terms/d/doji.asp`
func (candle OHLC) IsDoji() bool {
	return math.Floor(candle.Open) == math.Floor(candle.Close)
}

// IsHammer returns true if the candle is a hammer.
// i.e. A black or white candlestick that consists of a small body near the high with little or no upper shadow and a long lower tail.
// Considered a bullish pattern during a downtrend. See https://en.wikipedia.org/wiki/Hammer_(candlestick_pattern)
func (candle OHLC) IsHammer() bool {
	if candle.IsBullish() {
		// The lower tail/shadow is at least twice as long as the upper tail.
		if candle.LowerTail > (2 * candle.UpperTail) {
			// if (candle.Open - candle.Low) > (candle.Close-candle.High)*2 {
			return true
		}
	}
	return false
}

// Engulfs checks if a candle (i.e. candleTwo)
func (candle OHLC) Engulfs(candleTwo OHLC) bool {
	if candle.High > candleTwo.High && candle.Low < candleTwo.Low {
		return true
	}
	return false
}

// AllBearish returns true if all candles in the slice are bearish, returns false otherwise
func (cht CandleChart) AllBearish(candles []OHLC) bool {
	for _, candle := range candles {
		if candle.IsBullish() {
			return false
		}
	}
	return true
}

// AllBullish returns true if all candles in the slice are bullish, returns false otherwise.
func (cht CandleChart) AllBullish(candles []OHLC) bool {
	for _, candle := range candles {
		if candle.IsBearish() {
			return false
		}
	}
	return true
}

// ChartTrend represents the general price movement of a given OHLC unit. It may be bullish or bearish.
type ChartTrend string

const (
	// Bullish indicates a positive price move where the closing price is higher than the opening price
	Bullish ChartTrend = "Bullish"
	// Bearish indicates a negative price move where the opening price is higher than the closing price
	Bearish ChartTrend = "Bearish"
	// Indifferent indicates a balanced trading trend in both direction or no change at all.
	Indifferent ChartTrend = "Indifferent"
)

// IsBullish ...
func (tr ChartTrend) IsBullish() bool {
	return tr == "Bullish"
}

// IsBearish ...
func (tr ChartTrend) IsBearish() bool {
	return tr == "Bearish"
}

// IsIndifferent ...
func (tr ChartTrend) IsIndifferent() bool {
	return tr == "Indifferent"
}

// CandlestickPattern is a specific pattern for a set of candles.
// The most recent candles in a candle chart are examined to see if they
// match any of the patterns described. Basic candles chart patterns are included.
// See `https://www.investopedia.com/trading/candlestick-charting-what-is-it/`
type CandlestickPattern uint

type (
	// BearishCandlestickPattern is a bearish candlestick pattern
	BearishCandlestickPattern CandlestickPattern
	// BullishCandlestickPattern is a bullish candlestick pattern
	BullishCandlestickPattern CandlestickPattern
)

const (
	// BullishEngulfingPattern takes place when buyers outpace sellers.
	// This is reflected in the chart by a long green real body engulfing a small red real body.
	// With bulls having established some control, the price could head higher.
	BullishEngulfingPattern BullishCandlestickPattern = iota
	// BullishMorningStar Consists of a large black body candlestick followed by a small body (red or green) that occurs below the large red body candlestick.
	// On the following day, a third white body candlestick is formed that closes well into the black body candlestick.
	// It is considered a major reversal signal when it appears at the bottom
	BullishMorningStar
	// MorningDojiStar Consists of a large black body candlestick followed by a Doji that occurred below the preceding candlestick.
	// On the following day, a third white body candlestick is formed that closes well into the black body candlestick which appeared before the Doji.
	// It is considered a major reversal signal that is more bullish than the regular morning star pattern because of the existence of the Doji.
	MorningDojiStar
	// BullishHarami is the opposite of the upside down bearish harami.
	// A downtrend is in play, and a small real body (green) occurs inside the large real body (red) of the previous day.
	// This tells the technician that the trend is pausing. If it is followed by another up day, more upside could be forthcoming.
	BullishHarami
	// BullishHaramiCross occurs in a downtrend, where a down candle is followed by a doji.
	// The doji is within the real body of the prior session.
	// The implications are the same as the bullish harami.
	BullishHaramiCross
	// BullishRisingThree pattern starts out with what is called a "long white day."
	// Then, on the second, third, and fourth trading sessions, small real bodies move the price lower,
	// but they still stay within the price range of the long white day (day one in the pattern).
	// The fifth and last day of the pattern is another long white day.
	// Even though the pattern shows us that the price is falling for three straight days,
	// a new low is not seen, and the bull traders prepare for the next move up
	BullishRisingThree
	// BullishRisingTwo is similar to the rising three patterns but with two small bearish candles instead of three.
	BullishRisingTwo
	// BullishKeyReversal is a key reversal in a downtrend occurs when the price opens below the prior bar's close,
	// makes a new low, and then closes above the prior bar's high.
	// This indicates a strong shift to the upside, warning of a potential rally.
	BullishKeyReversal
	// BullishGenericPattern is a pattern that is formed by subsequently higher closes of the candles in question.
	// It is intended for use in the event the common patterns defined above are not detected.
	BullishGenericPattern
)

const (
	// BearishEngulfingPattern develops in an uptrend when sellers outnumber buyers.
	// This action is reflected by a long red real body engulfing a small green real body.
	// The pattern indicates that sellers are back in control and that the price could continue to decline.
	BearishEngulfingPattern BearishCandlestickPattern = iota
	// BearishEveningStar is a topping pattern.
	// It is identified by the last candle in the pattern opening below the previous day's small real body.
	// The small real body can be either red or green. The last candle closes deep into the real body of the candle two days prior.
	// The pattern shows a stalling of the buyers and then the sellers taking control. More selling could develop.
	BearishEveningStar
	// EveningDojiStar Consists of three candlesticks.
	// First is a large white body candlestick followed by a Doji that gaps above the white body.
	// The third candlestick is a black body that closes well into the white body.
	// When it appears at the top it is considered a reversal signal.
	// It signals a more bearish trend than the evening star pattern because of the Doji that has appeared between the two bodies.
	EveningDojiStar
	// BearishHarami pattern is a small real body (red) completely inside the previous day's real body.
	// This is not so much a pattern to act on, but it could be one to watch.
	// The pattern shows indecision on the part of the buyers.
	// If the price continues higher afterward, all may still be well with the uptrend,
	// but a down candle following this pattern indicates a further slide.
	BearishHarami
	// BearishHaramiCross occurs in an uptrend, where an up candle is followed by a doji—the session where the candlestick has a virtually equal open and close.
	// The doji is within the real body of the prior session. The implications are the same as the bearish harami
	BearishHaramiCross
	// BearishFallingThree pattern starts out with a strong down day.
	// This is followed by three small real bodies that make upward progress but stay within the range of the first big down day.
	// The pattern completes when the fifth day makes another large downward move.
	// It shows that sellers are back in control and that the price could head lower.
	BearishFallingThree
	// BearishFallingTwo is the same as BearishFallingthree but has two small bullish bodies between the bearish candles.
	BearishFallingTwo
	// BearishKeyReversal is a key reversal in an uptrend and occurs when the price opens above the prior bar's close,
	// makes a new high, and then closes below the prior bar's low.
	// It shows a strong shift in momentum which could indicate a pullback is starting.
	BearishKeyReversal
	// BearishGenericPattern is a pattern that is formed by subsequently lower closes of the candles in question.
	// It is intended for use in the event the common patterns defined above are not detected.
	// Its score should be dependent on the number of candles that form the longest chain.
	BearishGenericPattern
)

var (
	// ErrLastCandle is returned while trying to trasverse the last candle in a chart. See `CandleChart.nextCandle` and `CandleChart.previousCandle`
	ErrLastCandle = errors.New("there are no more candles in the chart. this is the last one")
)

// BullishChartPattern is a bullish candlestick pattern detected in the chart
type BullishChartPattern struct {
	Pattern         BullishCandlestickPattern
	PreceedingTrend ChartTrend
}

// BearishChartPattern is a bearish candlestick pattern detected in the chart
type BearishChartPattern struct {
	Pattern         BearishCandlestickPattern
	PreceedingTrend ChartTrend
}

// CandleChart is a chart that holds the OHLC data against time
type CandleChart struct {
	Candles           []OHLC
	Length            int // Number of candles in the chart
	Start, Stop       time.Time
	Interval          time.Duration
	MovingAverage     map[string]int
	MA30              float64
	MA90              float64
	Lines             [3]float64
	MaxPatternCandles int // Maximum number of most recent candles to check for common candlestick patterns.
	BullishPatterns   []BullishChartPattern
	BearishPatterns   []BearishChartPattern // These are the bearish patterns that have been detected in the most recent candles of the chart.
}

// NewCandleChart returns a candlestick chart initialized with the provided values.
func NewCandleChart(candles []OHLC) CandleChart {
	c := CandleChart{
		Candles:           []OHLC{},
		MaxPatternCandles: 5,
		BearishPatterns:   []BearishChartPattern{},
		BullishPatterns:   []BullishChartPattern{},
	}
	for i, candle := range candles {
		candle.ID = i
		c.Candles = append(c.Candles, candle)
	}
	return c
}

func (cht CandleChart) nextCandle(current OHLC) (candle OHLC, err error) {
	if len(cht.Candles) >= current.ID+1 {
		return OHLC{}, ErrLastCandle
	}
	return cht.Candles[current.ID+1], nil
}

func (cht CandleChart) nextCandles(num int, current OHLC) (candles []OHLC, err error) {
	if len(cht.Candles) >= current.ID+1 {
		return nil, ErrLastCandle
	}
	for i := 1; i == num; i++ {
		candles = append(candles, cht.Candles[current.ID+i])
	}
	return
}

func (cht CandleChart) previousCandle(current OHLC) (candle OHLC, err error) {
	if current.ID == 0 {
		return OHLC{}, ErrLastCandle
	}
	return cht.Candles[current.ID-1], nil
}

func (cht CandleChart) previousCandles(num int, current OHLC) (candles []OHLC, err error) {
	if current.ID == 0 {
		return nil, ErrLastCandle
	}
	for i := 1; i <= num; i++ {
		candles = append(candles, cht.Candles[current.ID-i])
	}
	return
}

// AddBearishPattern adds a detected bearish pattern to the chart struct as well as the trend
// of the candles preceeding the detect pattern.
func (cht CandleChart) AddBearishPattern(earliestCandle OHLC, pattern BearishCandlestickPattern) {
	if previousThreeCandles, err := cht.previousCandles(3, earliestCandle); err != ErrLastCandle {
		cht.BearishPatterns = append(cht.BearishPatterns, BearishChartPattern{Pattern: pattern,
			PreceedingTrend: cht.DetectTrend(previousThreeCandles)})
	}
}

// AddBullishPattern adds a detected bullish pattern to the chart struct as well as the trend
// of the candles preceeding the detected pattern.
func (cht CandleChart) AddBullishPattern(earliestCandle OHLC, pattern BullishCandlestickPattern) {
	if previousThreeCandles, err := cht.previousCandles(3, earliestCandle); err != ErrLastCandle {
		cht.BullishPatterns = append(cht.BullishPatterns, BullishChartPattern{Pattern: pattern,
			PreceedingTrend: cht.DetectTrend(previousThreeCandles)})
	}
}

// DetectTrend tries to score the overall trend of a group of candles that typically follow each other.
// It is best but not necessary to provide an odd number of candles for a certain score.
func (cht CandleChart) DetectTrend(candles []OHLC) ChartTrend {
	// TODO: add constraint to ensure only an odd number of candles are checked
	bullishScore, bearishScore := 0, 0
	for _, candle := range candles {
		if candle.IsBearish() {
			bearishScore++
		} else if candle.IsBullish() {
			bullishScore++
		}
	}
	if bullishScore > bearishScore {
		return Bullish
	} else if bearishScore > bullishScore {
		return Bearish
	} else if bearishScore == bullishScore {
		return Indifferent
	}
	return Indifferent
}

// DetectPatterns tries to match the most recent price data to common candlestick patterns
func (cht CandleChart) DetectPatterns() {
	fmt.Println(len(cht.Candles), cht.Candles)
	patternCandles := cht.Candles[len(cht.Candles)-cht.MaxPatternCandles : len(cht.Candles)]
	lastIdx := len(patternCandles) - 1
	lastCandle := patternCandles[lastIdx]
	// Check for patterns that end with a bearish candle, for example the bearish engulfing pattern
	if lastCandle.IsBearish() {
		if previousCandle, err := cht.previousCandle(lastCandle); err != ErrLastCandle {
			if previousCandle.IsBullish() {
				// Check for BearishEngulfingPattern. see https://www.investopedia.com/trading/candlestick-charting-what-is-it/ for more info
				if lastCandle.Engulfs(previousCandle) {
					// The last candle engulfs the preceeding one
					cht.AddBearishPattern(previousCandle, BearishEngulfingPattern)
				}
				// Check for bearish harami
				if previousCandle.Engulfs(lastCandle) {
					// the last candle is engulfed by the preceeding one
					cht.AddBearishPattern(previousCandle, BearishHarami)
				}
				// Check for bearish key reversal pattern
				if lastCandle.Open > previousCandle.Close && lastCandle.High > lastCandle.Open {
					if lastCandle.Close < previousCandle.Low {
						cht.AddBearishPattern(previousCandle, BearishKeyReversal)
					}
				}
			}
			// Check for bearish evening star
			if thirdCandle, err := cht.previousCandle(previousCandle); err != ErrLastCandle {
				if thirdCandle.IsBullish() {
					if previousCandle.IsDoji() {
						if previousCandle.Low > thirdCandle.Close && lastCandle.Open < previousCandle.Close {
							if lastCandle.Close > thirdCandle.Open {
								// conditions for an evening doji star has been met.
								cht.AddBearishPattern(thirdCandle, EveningDojiStar)
							}
						}
					} else { // Next to last candle is Not a doji
						// Previous candle is relatively small and gaps above the previous (third to last) candle
						if previousCandle.Range <= (lastCandle.Range/2) && previousCandle.Open > thirdCandle.Open {
							// Last candle opens below previous smaller candle and closes deep into the candle two periods before
							if lastCandle.Open > previousCandle.Close && lastCandle.Close > thirdCandle.Open {
								// Conditions for an evening star have been met
								cht.AddBearishPattern(thirdCandle, BearishEveningStar)
							}
						}
					}
				}
			}

		}
		// Check for bearish falling three (a.k.a Bearish 3-method formation)
		if previousThreeCandles, err := cht.previousCandles(3, lastCandle); err != ErrLastCandle {
			allThreeBullish := cht.AllBullish(previousThreeCandles)
			if allThreeBullish {
				if fifthCandle, err := cht.previousCandle(previousThreeCandles[len(previousThreeCandles)-1]); err != ErrLastCandle {
					// fifthCandle is the one that preceedes the three bullish candles and of course our bearish current candle
					if fifthCandle.IsBearish() {
						highestPrices := []float64{}
						for _, candle := range previousThreeCandles {
							highestPrices = append(highestPrices, candle.High)
						}
						if Max64(highestPrices) < fifthCandle.High {
							cht.AddBearishPattern(fifthCandle, BearishFallingThree)
						}
					}

				}
			}
		}
		// Check for bearish falling two
		if previousTwoCandles, err := cht.previousCandles(2, lastCandle); err != ErrLastCandle {
			bothBullish := cht.AllBullish(previousTwoCandles)
			if bothBullish {
				if fourthCandle, err := cht.previousCandle(previousTwoCandles[1]); err != ErrLastCandle {
					if fourthCandle.IsBearish() {
						highestPrices := []float64{}
						for _, candle := range previousTwoCandles {
							highestPrices = append(highestPrices, candle.High)
						}
						if Max64(highestPrices) < fourthCandle.High {
							cht.AddBearishPattern(fourthCandle, BearishFallingTwo)
						}
					}
				}
			}
		}

		// In the event no patterns have been detected check for a generic bearish pattern
		if previousThreeCandles, err := cht.previousCandles(3, lastCandle); err != ErrLastCandle {
			if cht.AllBearish(previousThreeCandles) {
				cht.AddBearishPattern(lastCandle, BearishGenericPattern)
			}
		}

	} else if lastCandle.IsBullish() { // Check for patterns that end in a bullish candle
		if previousCandle, err := cht.previousCandle(lastCandle); err != ErrLastCandle {
			if previousCandle.IsBearish() {
				// Check for BullishEngulfingPattern. see https://www.investopedia.com/trading/candlestick-charting-what-is-it/ for more info
				if lastCandle.Engulfs(previousCandle) {
					// The last candle engulfs the preceeding one
					cht.AddBullishPattern(previousCandle, BullishEngulfingPattern)
				}
				// Check for bullish harami
				if previousCandle.Engulfs(lastCandle) {
					// The last candle is engulfed by the preceeding one
					cht.AddBullishPattern(previousCandle, BullishHarami)
				}
				// Check for bullish key reversal
				if lastCandle.Open < previousCandle.Close && lastCandle.Low < lastCandle.Open {
					if lastCandle.Close > previousCandle.High {
						cht.AddBullishPattern(previousCandle, BullishKeyReversal)
					}
				}
			}
			// Check for bullish morning star
			if thirdCandle, err := cht.previousCandle(previousCandle); err != ErrLastCandle {
				if thirdCandle.IsBearish() {
					if previousCandle.IsDoji() { // Check for morning doji star
						if previousCandle.High < thirdCandle.Close && lastCandle.Open > previousCandle.Close {
							if lastCandle.Close < thirdCandle.Open {
								// conditions for an evening doji star has been met.
								cht.AddBearishPattern(thirdCandle, EveningDojiStar)
							}
						}
					} else {
						// Previous candle is relatively small and gaps below the previous (third to last) candle
						// if previousCandle.Range <= (lastCandle.Range/2) && previousCandle.Open < thirdCandle.Close {
						if previousCandle.Range <= (lastCandle.Range/2) && previousCandle.Close < thirdCandle.Close {
							// Last candle closes above previous smaller candle and oens deep into the candle two periods before
							// if lastCandle.Close > previousCandle.Open && lastCandle.Open < thirdCandle.Open {
							if lastCandle.Open > previousCandle.Close && lastCandle.Close < thirdCandle.Open {
								// Conditions for a morning star have been met
								cht.AddBullishPattern(thirdCandle, BullishMorningStar)
							}
						}
					}
				}
			}

		}
		// Check for bullish rising three (a.k.a Bullish 3-method formation)
		if previousThreeCandles, err := cht.previousCandles(3, lastCandle); err != ErrLastCandle {
			allThreeBearish := cht.AllBearish(previousThreeCandles)
			if allThreeBearish {
				if fifthCandle, err := cht.previousCandle(previousThreeCandles[len(previousThreeCandles)-1]); err != ErrLastCandle {
					// fifthCandle is the one that preceedes the three bearish candles and of course our bullish current candle
					if fifthCandle.IsBullish() {
						lowestPrices := []float64{}
						for _, candle := range previousThreeCandles {
							lowestPrices = append(lowestPrices, candle.High)
						}
						if Min64(lowestPrices) > fifthCandle.Low {
							cht.AddBullishPattern(fifthCandle, BullishRisingThree)
						}
					}
				}
			}
		}
		// Check for bullish rising two
		if previousTwoCandles, err := cht.previousCandles(2, lastCandle); err != ErrLastCandle {
			bothBearish := cht.AllBearish(previousTwoCandles)
			if bothBearish {
				if fourthCandle, err := cht.previousCandle(previousTwoCandles[1]); err != ErrLastCandle {
					if fourthCandle.IsBullish() {
						lowestPrices := []float64{}
						for _, candle := range previousTwoCandles {
							lowestPrices = append(lowestPrices, candle.Low)
						}
						if Max64(lowestPrices) > fourthCandle.Low {
							cht.AddBearishPattern(fourthCandle, BearishFallingTwo)
						}
					}
				}
			}
		}
		// In the event no patterns have been detected check for a generic bullsih pattern
		if previousThreeCandles, err := cht.previousCandles(3, lastCandle); err != ErrLastCandle {
			if cht.AllBullish(previousThreeCandles) {
				cht.AddBullishPattern(lastCandle, BullishGenericPattern)
			}
		}
	}

	// Check for patterns that end in  a doji
	if lastCandle.IsDoji() {
		// Check for bullish harami cross
		if previousCandle, err := cht.previousCandle(lastCandle); err != ErrLastCandle {
			if previousCandle.IsBearish() && lastCandle.High < previousCandle.High && lastCandle.Low > previousCandle.Low {
				cht.AddBullishPattern(previousCandle, BullishHaramiCross)
			}
		}
		// Check for bearish harami cross
		if previousCandle, err := cht.previousCandle(lastCandle); err != ErrLastCandle {
			if previousCandle.IsBullish() && lastCandle.High < previousCandle.High && lastCandle.Low > previousCandle.Low {
				cht.AddBearishPattern(previousCandle, BearishHaramiCross)
			}
		}
	}

}

// Min64 returns the smallest value in a float64 list
func Min64(a []float64) float64 {
	if len(a) == 0 {
		return 0
	}
	min := a[0]
	for _, v := range a {
		if v < min {
			min = v
		}
	}
	return min
}

// Max64 returns the largest value in a float64 list
func Max64(a []float64) float64 {
	if len(a) == 0 {
		return 0
	}
	max := a[0]
	for _, v := range a {
		if v > max {
			max = v
		}
	}
	return max
}
