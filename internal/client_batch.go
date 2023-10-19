package internal

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync/atomic"
	"time"

	"github.com/fullstorydev/grpchan/grpchantesting"
	"golang.org/x/sync/errgroup"
)

// SendRPCs uses four goroutines to send batches of RPCs of all types (unary,
// client-, server-, and bidi-streaaming) using the given client.
func SendRPCs(ctx context.Context, client grpchantesting.TestServiceClient) error {
	var done atomic.Bool
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	grp, ctx := errgroup.WithContext(ctx)
	type action func(context.Context, grpchantesting.TestServiceClient) error
	for _, fn := range []action{doUnary, doClientStream, doServerStream, doBidiStream} {
		fn := fn
		grp.Go(func() error {
			for {
				if done.Load() {
					return nil
				}
				if err := fn(ctx, client); err != nil {
					return err
				}
			}
		})
	}
	time.Sleep(5 * time.Second)
	done.Store(true)
	time.AfterFunc(time.Second, cancel)
	return grp.Wait()
}

func doUnary(ctx context.Context, client grpchantesting.TestServiceClient) error {
	_, err := client.Unary(ctx, &grpchantesting.Message{
		Count:   10,
		Payload: bytes.Repeat([]byte{0, 1, 2, 3}, 100),
	})
	return err
}

func doClientStream(ctx context.Context, client grpchantesting.TestServiceClient) error {
	stream, err := client.ClientStream(ctx)
	if err != nil {
		return err
	}
	for i := 0; i < 10; i++ {
		err := stream.Send(&grpchantesting.Message{
			Count:   10,
			Payload: bytes.Repeat([]byte{0, 1, 2, 3}, 10000),
		})
		if err != nil {
			return err
		}
	}
	_, err = stream.CloseAndRecv()
	return err
}

func doServerStream(ctx context.Context, client grpchantesting.TestServiceClient) error {
	stream, err := client.ServerStream(ctx, &grpchantesting.Message{
		Count:   10,
		Payload: bytes.Repeat([]byte{0, 1, 2, 3}, 1000),
	})
	if err != nil {
		return err
	}
	for {
		_, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func doBidiStream(ctx context.Context, client grpchantesting.TestServiceClient) error {
	stream, err := client.BidiStream(ctx)
	if err != nil {
		return err
	}
	go func() {
		for i := 0; i < 10; i++ {
			err := stream.Send(&grpchantesting.Message{
				Count:   10,
				Payload: bytes.Repeat([]byte{0, 1, 2, 3}, 1000),
			})
			if err != nil {
				return
			}
		}
		_ = stream.CloseSend()
	}()
	for {
		_, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
