package main

import (
	"context"
	"fmt"
	"log"
	"unit2/leprechaun"

	"github.com/pkg/errors"

	gg "gorgonia.org/gorgonia"
	"gorgonia.org/tensor"
)

func main() {
	// ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(12*time.Second))
	// defer cancel()
	ctx := context.Background()
	sess := leprechaun.NewSession(ctx)
	sess.Initialize()
	sess.GetPrices()
	sess.Start()
	fmt.Printf("%#v/n", sess)
}

type Brain struct {
	g      *gg.ExprGraph
	w0, w1 *gg.Node

	out     *gg.Node
	predVal gg.Value
}

func Init(g *gg.ExprGraph) *Brain {
	// Create node for w/weight
	w0 := gg.NewMatrix(g, tensor.Float64, gg.WithShape(784, 300), gg.WithName("w0"), gg.WithInit(gg.GlorotN(1.0)))
	w1 := gg.NewMatrix(g, tensor.Float64, gg.WithShape(300, 10), gg.WithName("w1"), gg.WithInit(gg.GlorotN(1.0)))
	return &Brain{
		g:  g,
		w0: w0,
		w1: w1,
	}
}

func (b *Brain) learnables() gg.Nodes {
	return gg.Nodes{b.w0, b.w1}
}

func (b *Brain) fwd(x *gg.Node) (err error) {
	var l0, l1 *gg.Node
	var l0dot *gg.Node

	// Set first layer to be copy of input
	l0 = x

	// Dot product of l0 and w0, use as input for ReLU
	if l0dot, err = gg.Mul(l0, b.w0); err != nil {
		return errors.Wrap(err, "Unable to multiply l0 and w0")
	}

	// l0dot := gg.Must(gg.Mul(l0, m.w0))

	// Build hidden layer out of result
	l1 = gg.Must(gg.Rectify(l0dot))

	var out *gg.Node
	if out, err = gg.Mul(l1, b.w1); err != nil {
		return errors.Wrapf(err, "Unable to multiply l1 and w1")
	}

	b.out, err = gg.SoftMax(out)
	gg.Read(b.out, &b.predVal)
	return

}

func main1() {
	graph := gg.NewGraph()
	var x, y, z *gg.Node
	var err error

	x = gg.NewScalar(graph, gg.Float64, gg.WithName("x"))
	y = gg.NewScalar(graph, gg.Float64, gg.WithName("y"))
	if z, err = gg.Add(x, y); err != nil {
		log.Fatal(err)
	}

	vm := gg.NewTapeMachine(graph)
	defer vm.Close()

	gg.Let(x, 612.21)
	gg.Let(y, 218.34)

	if err = vm.RunAll(); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%v", z.Value())
}
