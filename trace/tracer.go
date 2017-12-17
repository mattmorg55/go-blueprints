package trace

import (
	"fmt"
	"io"
)

// Tracer is an interfac that describes a tracer
type Tracer interface {
	Trace(...interface{}) //Zero or more arguments of any type
}

//Basic implementation
type tracer struct {
	out io.Writer
}

func (t *tracer) Trace(a ...interface{}) {
	t.out.Write([]byte(fmt.Sprint(a...)))
	t.out.Write([]byte("\n"))
}

// New returns new standard Tracer implementation
func New(w io.Writer) Tracer {
	return &tracer{out: w}
}

//Off implementation
type nilTracer struct{}

func (t *nilTracer) Trace(a ...interface{}) {}

//Off returns new no-op tracer
func Off() Tracer {
	return &nilTracer{}
}
