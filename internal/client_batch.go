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
	"google.golang.org/protobuf/types/known/emptypb"
)

// SendRPCs uses five goroutines to send batches of RPCs of all types (unary,
// client-, server-, and bidi-streaming) using the given client. One of them
// sends only empty messages.
func SendRPCs(ctx context.Context, client grpchantesting.TestServiceClient) error {
	var done atomic.Bool
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	grp, ctx := errgroup.WithContext(ctx)
	type action func(context.Context, grpchantesting.TestServiceClient) error
	for _, fn := range []action{doUnary, doClientStream, doServerStream, doBidiStream, doEmpty} {
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
	for range 10 {
		err := stream.Send(&grpchantesting.Message{
			Count:   10,
			Payload: bytes.Repeat([]byte{0, 1, 2, 3}, 10000),
		})
		if errors.Is(err, io.EOF) {
			// The stream has ended. The actual status comes from CloseAndRecv.
			break
		}
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
		for range 10 {
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

// doEmpty sends empty request messages and receives empty response messages.
// Empty messages consume flow control window only in protocol revision two and
// later.
func doEmpty(ctx context.Context, client grpchantesting.TestServiceClient) error {
	stream, err := client.ClientStream(ctx)
	if err != nil {
		return err
	}
	for range 100 {
		err := stream.Send(&grpchantesting.Message{})
		if errors.Is(err, io.EOF) {
			// The stream has ended. The actual status comes from CloseAndRecv.
			break
		}
		if err != nil {
			return err
		}
	}
	if _, err := stream.CloseAndRecv(); err != nil {
		return err
	}
	for range 10 {
		if _, err := client.UseExternalMessageTwice(ctx, &emptypb.Empty{}); err != nil {
			return err
		}
	}
	return nil
}
