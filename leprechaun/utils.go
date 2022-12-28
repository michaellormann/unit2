package leprechaun

/* This file is part of Leprechaun.
*  @author: Michael Lormann
 */

import (
	"fmt"
	"math"
	"os"
	"time"
)

var (
	stringToIntDict = map[rune]int64{'0': 0, '1': 1, '2': 2, '3': 3, '4': 4, '5': 5, '6': 6,
		'7': 7, '8': 8, '9': 9}
)

// Channels for communicating with the UI.
type Channels struct {
	// Log sends messages of Leprechaun's activities from the bot to the UI.
	LogChan chan string
	// Cancel delivers a signal from the user (through the UI) to stop the traing loop.
	CancelChan chan struct{}
	// Stopped sends a signal from the bot to the UI after it has stopped.
	// To facilitate graceful exiting.
	StoppedChan chan struct{}
	// Error is for sending bot-side errors to the UI.
	ErrorChan chan error
	// PurchaseChan channel notifies the UI that a purchase has been made so it can update its displayed records
	PurchaseChan chan struct{}
	// SaleChan channel notifies the UI that a sale has been made so it can update its displayed records.
	SaleChan chan struct{}
}

// Log sets the log channel
func (c *Channels) Log(channel chan string) {
	c.LogChan = channel
}

// Cancel sets the channel with which to stop the bot
func (c *Channels) Cancel(channel chan struct{}) {
	c.CancelChan = channel
}

// BotStopped set the cahnnel through which the bot informs the UI it has stopped.
func (c *Channels) BotStopped(channel chan struct{}) {
	c.StoppedChan = channel
}

// Error sets the channel through which the bot sends error messages to the UI.
func (c *Channels) Error(channel chan error) {
	c.ErrorChan = channel
}

// Purchase sets the channel through which the bot alerts the UI to new Purchase events.
func (c *Channels) Purchase(channel chan struct{}) {
	c.PurchaseChan = channel
}
func spinner(delay time.Duration) {
	for {
		for _, r := range `\|/` {
			fmt.Printf("\r%c", r)
			time.Sleep(delay)
		}
	}
}

// Sale sets the channel through which the bot alerts the UI to new Sale events.
func (c *Channels) Sale(channel chan struct{}) {
	c.SaleChan = channel
}

// exists returns true if `path` exists, otherwise false.
func exists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

// sleep delays the bot between each request in order to avoid exceeding the rate limit.
func sleep() {
	time.Sleep(600 * time.Millisecond)
}

// sleep2 delays the bot for slightly longer than sleep b/c sometimes sleep still triggers Error 429.
func sleep2() {
	time.Sleep(700 * time.Millisecond)
}

// stringToInt converts a string of numbers to its numerical value
// without loss of precision or conversion errors up until math.MaxInt64
func stringToInt(s string) (num int64) {
	for i, v := range s {
		n := stringToIntDict[v]
		x := len(s) - i
		c := math.Pow(1e1, float64(x-1))
		num += int64(n) * int64(c)
	}
	return
}

func toMidnight(t0 time.Time) time.Time {
	return time.Date(t0.Year(), t0.Month(), t0.Day(), 0, 0, 0, 0, t0.Location())
}
