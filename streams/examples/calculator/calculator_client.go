package calculator

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/jhump/grpctunnel/internal/testservices"
	"github.com/jhump/grpctunnel/streams"
)

type Client struct {
	adapter streams.StreamAdapter[*testservices.Operation, *testservices.Answer]
}

func NewClient(ctx context.Context, stub testservices.CalculatorServiceClient) (*Client, error) {
	stream, err := stub.Calculate(ctx)
	if err != nil {
		return nil, err
	}
	adapter := streams.NewStreamAdapterWithCorrelationKey[*testservices.Operation, *testservices.Answer](
		stream,
		func(req *testservices.Operation) string {
			return req.VariableName
		},
		func(req *testservices.Answer) string {
			return req.VariableName
		},
	)
	return &Client{adapter: adapter}, nil
}

//	*Operation_Sin
//	*Operation_Cos
//	*Operation_Ln
//	*Operation_Log

func (c *Client) Load(ctx context.Context, variable, fromOtherVariable string) (float64, error) {
	resp, err := c.adapter.Call(ctx, &testservices.Operation{
		VariableName: variable,
		Request: &testservices.Operation_Load{
			Load: fromOtherVariable,
		},
	})
	if err != nil {
		return 0, err
	}
	return resp.Result, nil
}

func (c *Client) Save(ctx context.Context, variable string) (float64, error) {
	resp, err := c.adapter.Call(ctx, &testservices.Operation{
		VariableName: variable,
		Request: &testservices.Operation_Save{
			Save: &emptypb.Empty{},
		},
	})
	if err != nil {
		return 0, err
	}
	return resp.Result, nil
}

func (c *Client) Set(ctx context.Context, variable string, val float64) (float64, error) {
	resp, err := c.adapter.Call(ctx, &testservices.Operation{
		VariableName: variable,
		Request: &testservices.Operation_SetValue{
			SetValue: val,
		},
	})
	if err != nil {
		return 0, err
	}
	return resp.Result, nil
}

func (c *Client) Add(ctx context.Context, variable string, val float64) (float64, error) {
	resp, err := c.adapter.Call(ctx, &testservices.Operation{
		VariableName: variable,
		Request: &testservices.Operation_Add{
			Add: val,
		},
	})
	if err != nil {
		return 0, err
	}
	return resp.Result, nil
}

func (c *Client) Multiple(ctx context.Context, variable string, val float64) (float64, error) {
	resp, err := c.adapter.Call(ctx, &testservices.Operation{
		VariableName: variable,
		Request: &testservices.Operation_Multiply{
			Multiply: val,
		},
	})
	if err != nil {
		return 0, err
	}
	return resp.Result, nil
}

func (c *Client) Divide(ctx context.Context, variable string, val float64) (float64, error) {
	resp, err := c.adapter.Call(ctx, &testservices.Operation{
		VariableName: variable,
		Request: &testservices.Operation_Divide{
			Divide: val,
		},
	})
	if err != nil {
		return 0, err
	}
	return resp.Result, nil
}

func (c *Client) Exp(ctx context.Context, variable string, val float64) (float64, error) {
	resp, err := c.adapter.Call(ctx, &testservices.Operation{
		VariableName: variable,
		Request: &testservices.Operation_Exp{
			Exp: val,
		},
	})
	if err != nil {
		return 0, err
	}
	return resp.Result, nil
}

func (c *Client) Sin(ctx context.Context, variable string) (float64, error) {
	resp, err := c.adapter.Call(ctx, &testservices.Operation{
		VariableName: variable,
		Request: &testservices.Operation_Sin{
			Sin: &emptypb.Empty{},
		},
	})
	if err != nil {
		return 0, err
	}
	return resp.Result, nil
}

func (c *Client) Cos(ctx context.Context, variable string) (float64, error) {
	resp, err := c.adapter.Call(ctx, &testservices.Operation{
		VariableName: variable,
		Request: &testservices.Operation_Cos{
			Cos: &emptypb.Empty{},
		},
	})
	if err != nil {
		return 0, err
	}
	return resp.Result, nil
}

func (c *Client) Ln(ctx context.Context, variable string) (float64, error) {
	resp, err := c.adapter.Call(ctx, &testservices.Operation{
		VariableName: variable,
		Request: &testservices.Operation_Ln{
			Ln: &emptypb.Empty{},
		},
	})
	if err != nil {
		return 0, err
	}
	return resp.Result, nil
}

func (c *Client) Log(ctx context.Context, variable string, val float64) (float64, error) {
	resp, err := c.adapter.Call(ctx, &testservices.Operation{
		VariableName: variable,
		Request: &testservices.Operation_Log{
			Log: val,
		},
	})
	if err != nil {
		return 0, err
	}
	return resp.Result, nil
}
