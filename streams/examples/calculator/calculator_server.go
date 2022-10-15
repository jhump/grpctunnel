package calculator

import (
	"context"
	"math"
	"sync"

	"github.com/jhump/grpctunnel/internal/testservices"
	"github.com/jhump/grpctunnel/streams"
)

type Server struct {
	testservices.UnimplementedCalculatorServiceServer
	mu   sync.Mutex
	vars varCache
}

func (s *Server) Calculate(stream testservices.CalculatorService_CalculateServer) error {
	var streamCache varCache
	return streams.HandleServerStreamWithCorrelationKey[*testservices.Operation, *testservices.Answer, string](
		stream,
		func(_ context.Context, op *testservices.Operation) *testservices.Answer {
			return s.handle(&streamCache, op)
		},
		func(req *testservices.Operation) string {
			return req.VariableName
		},
	)
}

func (s *Server) handle(cache *varCache, op *testservices.Operation) *testservices.Answer {
	variable := op.VariableName
	var val float64
	defer func() {
		cache.save(variable, val)
	}()
	switch op := op.Request.(type) {
	case *testservices.Operation_Load:
		val = s.vars.load(op.Load)
	case *testservices.Operation_SetValue:
		val = cache.load(variable)
	}

	switch op := op.Request.(type) {
	case *testservices.Operation_Save:
		s.vars.save(variable, val)
	case *testservices.Operation_Add:
		val += op.Add
	case *testservices.Operation_Multiply:
		val *= op.Multiply
	case *testservices.Operation_Divide:
		val /= op.Divide
	case *testservices.Operation_Exp:
		val = math.Pow(val, op.Exp)
	case *testservices.Operation_Sin:
		val = math.Sin(val)
	case *testservices.Operation_Cos:
		val = math.Cos(val)
	case *testservices.Operation_Ln:
		val = math.Log(val)
	case *testservices.Operation_Log:
		val = math.Log(val) / math.Log(op.Log)
	}

	return &testservices.Answer{VariableName: op.VariableName, Result: val}
}

type varCache struct {
	mu     sync.Mutex
	values map[string]float64
}

func (s *varCache) load(name string) float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.values[name]
}

func (s *varCache) save(name string, val float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.values == nil {
		s.values = map[string]float64{}
	}
	s.values[name] = val
}
