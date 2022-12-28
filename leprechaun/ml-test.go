package leprechaun

import (
	"github.com/gonum/stat"
	luno "github.com/luno/luno-go"
)

type Cols struct {
	luno.Candle
	mean, sd float64
}

type Rows struct {
	rows []Cols
}

func cize() *Cols {
	c := &Cols{}
	cn := luno.Candle{}
	c.Low = cn.Low
	c.High = cn.High
	c.Close = cn.Close
	c.Open = cn.Open
	c.Volume = cn.Volume
	// c.wmean, c.sd = stat.MeanStdDev(data, nil)
	return c
}

type NN struct {
}
