// Package reflection provides a gRPC server reflection client and is an example
// of using a streams.StreamAdapter to simplify the use of a full-duplex
// bidirectional stream.
package reflection

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/jhump/grpctunnel/streams"
)

// Client is a client for the gRPC server reflection service. It provides a nicer to use API
// than dealing with the raw streaming client that is generated from the protobuf source.
type Client struct {
	adapter     streams.StreamAdapter[*grpc_reflection_v1alpha.ServerReflectionRequest, *grpc_reflection_v1alpha.ServerReflectionResponse]
	cachedFiles map[string]*descriptorpb.FileDescriptorProto
}

// NewClient creates a new reflection service client using the given stub. The given context
// is used to create a stream over which reflection information is downloaded.
func NewClient(ctx context.Context, stub grpc_reflection_v1alpha.ServerReflectionClient) (*Client, error) {
	stream, err := stub.ServerReflectionInfo(ctx)
	if err != nil {
		return nil, err
	}
	adapter := streams.NewStreamAdapter[*grpc_reflection_v1alpha.ServerReflectionRequest, *grpc_reflection_v1alpha.ServerReflectionResponse](stream)
	return &Client{
		adapter:     adapter,
		cachedFiles: map[string]*descriptorpb.FileDescriptorProto{},
	}, nil
}

func (c *Client) ListServices(ctx context.Context) ([]protoreflect.FullName, error) {
	resp, err := c.adapter.Call(ctx, &grpc_reflection_v1alpha.ServerReflectionRequest{
		MessageRequest: &grpc_reflection_v1alpha.ServerReflectionRequest_ListServices{
			ListServices: "*",
		},
	})
	if err != nil {
		return nil, err
	}
	switch r := resp.MessageResponse.(type) {
	case *grpc_reflection_v1alpha.ServerReflectionResponse_ListServicesResponse:
		svcs := r.ListServicesResponse.GetService()
		svcNames := make([]protoreflect.FullName, len(svcs))
		for i := range svcs {
			svcNames[i] = protoreflect.FullName(svcs[i].GetName())
		}
		return svcNames, nil
	case *grpc_reflection_v1alpha.ServerReflectionResponse_ErrorResponse:
		return nil, status.Error(codes.Code(r.ErrorResponse.GetErrorCode()), r.ErrorResponse.GetErrorMessage())
	default:
		return nil, status.Errorf(codes.Internal, "wrong response type: %T", r)
	}
}

func (c *Client) AllExtensionNumbersOfType(ctx context.Context, messageName protoreflect.FullName) ([]protoreflect.FieldNumber, error) {
	resp, err := c.adapter.Call(ctx, &grpc_reflection_v1alpha.ServerReflectionRequest{
		MessageRequest: &grpc_reflection_v1alpha.ServerReflectionRequest_AllExtensionNumbersOfType{
			AllExtensionNumbersOfType: string(messageName),
		},
	})
	if err != nil {
		return nil, err
	}
	switch r := resp.MessageResponse.(type) {
	case *grpc_reflection_v1alpha.ServerReflectionResponse_AllExtensionNumbersResponse:
		exts := r.AllExtensionNumbersResponse.GetExtensionNumber()
		extNums := make([]protoreflect.FieldNumber, len(exts))
		for i := range exts {
			extNums[i] = protoreflect.FieldNumber(exts[i])
		}
		return extNums, nil
	case *grpc_reflection_v1alpha.ServerReflectionResponse_ErrorResponse:
		return nil, status.Error(codes.Code(r.ErrorResponse.GetErrorCode()), r.ErrorResponse.GetErrorMessage())
	default:
		return nil, status.Errorf(codes.Internal, "wrong response type: %T", r)
	}
}

func (c *Client) FileByFilename(ctx context.Context, filename string) (*descriptorpb.FileDescriptorProto, error) {
	fd := c.cachedFiles[filename]
	if fd != nil {
		return fd, nil
	}
	fds, err := c.fileDescriptor(ctx, &grpc_reflection_v1alpha.ServerReflectionRequest{
		MessageRequest: &grpc_reflection_v1alpha.ServerReflectionRequest_FileByFilename{
			FileByFilename: filename,
		},
	})
	if err != nil {
		return nil, err
	}
	return findFileDescriptor(fds, func(fd *descriptorpb.FileDescriptorProto) bool {
		return fd.GetName() == filename
	})
}

func (c *Client) FileContainingSymbol(ctx context.Context, symbol protoreflect.FullName) (*descriptorpb.FileDescriptorProto, error) {
	fds, err := c.fileDescriptor(ctx, &grpc_reflection_v1alpha.ServerReflectionRequest{
		MessageRequest: &grpc_reflection_v1alpha.ServerReflectionRequest_FileContainingSymbol{
			FileContainingSymbol: string(symbol),
		},
	})
	if err != nil {
		return nil, err
	}
	return findFileDescriptor(fds, func(fd *descriptorpb.FileDescriptorProto) bool {
		return containsSymbol(fd, symbol)
	})
}

func containsSymbol(fd *descriptorpb.FileDescriptorProto, symbol protoreflect.FullName) bool {
	name := string(symbol)
	var prefix string
	if fd.GetPackage() != "" {
		prefix = fd.GetPackage() + "."
	}
	for _, md := range fd.GetMessageType() {
		if msgContainsSymbol(prefix, md, name) {
			return true
		}
	}
	for _, ed := range fd.GetEnumType() {
		if enumContainsSymbol(prefix, ed, name) {
			return true
		}
	}
	for _, extd := range fd.GetExtension() {
		if prefix+extd.GetName() == name {
			return true
		}
	}
	for _, sd := range fd.GetService() {
		if prefix+sd.GetName() == name {
			return true
		}
		for _, mtd := range sd.GetMethod() {
			if prefix+mtd.GetName() == name {
				return true
			}
		}
	}
	return false
}

func msgContainsSymbol(prefix string, md *descriptorpb.DescriptorProto, name string) bool {
	if prefix+md.GetName() == name {
		return true
	}
	prefix = prefix + md.GetName() + "."
	for _, fd := range md.GetField() {
		if prefix+fd.GetName() == name {
			return true
		}
	}
	for _, ood := range md.GetOneofDecl() {
		if prefix+ood.GetName() == name {
			return true
		}
	}
	for _, ed := range md.GetEnumType() {
		if enumContainsSymbol(prefix, ed, name) {
			return true
		}
	}
	for _, extd := range md.GetExtension() {
		if prefix+extd.GetName() == name {
			return true
		}
	}
	for _, nmd := range md.GetNestedType() {
		if msgContainsSymbol(prefix, nmd, name) {
			return true
		}
	}
	return false
}

func enumContainsSymbol(prefix string, ed *descriptorpb.EnumDescriptorProto, name string) bool {
	if prefix+ed.GetName() == name {
		return true
	}
	for _, evd := range ed.GetValue() {
		if prefix+evd.GetName() == name {
			return true
		}
	}
	return false
}

func (c *Client) FileContainingExtension(ctx context.Context, messageType protoreflect.FullName, extNumber protoreflect.FieldNumber) (*descriptorpb.FileDescriptorProto, error) {
	fds, err := c.fileDescriptor(ctx, &grpc_reflection_v1alpha.ServerReflectionRequest{
		MessageRequest: &grpc_reflection_v1alpha.ServerReflectionRequest_FileContainingExtension{
			FileContainingExtension: &grpc_reflection_v1alpha.ExtensionRequest{
				ContainingType:  string(messageType),
				ExtensionNumber: int32(extNumber),
			},
		},
	})
	if err != nil {
		return nil, err
	}
	return findFileDescriptor(fds, func(fd *descriptorpb.FileDescriptorProto) bool {
		return containsExtension(fd, messageType, extNumber)
	})
}

func containsExtension(fd *descriptorpb.FileDescriptorProto, messageType protoreflect.FullName, extNum protoreflect.FieldNumber) bool {
	extendee := strings.TrimPrefix(string(messageType), ".")
	num := int32(extNum)

	for _, extd := range fd.GetExtension() {
		if strings.TrimPrefix(extd.GetExtendee(), ".") == extendee && extd.GetNumber() == num {
			return true
		}
	}
	for _, md := range fd.GetMessageType() {
		if msgContainsExtension(md, extendee, num) {
			return true
		}
	}
	return false
}

func msgContainsExtension(md *descriptorpb.DescriptorProto, extendee string, num int32) bool {
	for _, extd := range md.GetExtension() {
		if strings.TrimPrefix(extd.GetExtendee(), ".") == extendee && extd.GetNumber() == num {
			return true
		}
	}
	for _, nmd := range md.GetNestedType() {
		if msgContainsExtension(nmd, extendee, num) {
			return true
		}
	}
	return false
}

func findFileDescriptor(fds []*descriptorpb.FileDescriptorProto, predicate func(fd *descriptorpb.FileDescriptorProto) bool) (*descriptorpb.FileDescriptorProto, error) {
	for _, fd := range fds {
		if predicate(fd) {
			return fd, nil
		}
	}
	return nil, status.Error(codes.Internal, "response did not include requested file")
}

func (c *Client) fileDescriptor(ctx context.Context, req *grpc_reflection_v1alpha.ServerReflectionRequest) ([]*descriptorpb.FileDescriptorProto, error) {
	resp, err := c.adapter.Call(ctx, req)
	if err != nil {
		return nil, err
	}
	switch r := resp.MessageResponse.(type) {
	case *grpc_reflection_v1alpha.ServerReflectionResponse_FileDescriptorResponse:
		files := r.FileDescriptorResponse.GetFileDescriptorProto()
		results := make([]*descriptorpb.FileDescriptorProto, len(files))
		for i, fd := range files {
			var fdProto descriptorpb.FileDescriptorProto
			if err := proto.Unmarshal(fd, &fdProto); err != nil {
				return nil, err
			}
			c.cachedFiles[fdProto.GetName()] = &fdProto
			results[i] = &fdProto
		}
		return results, nil
	case *grpc_reflection_v1alpha.ServerReflectionResponse_ErrorResponse:
		return nil, status.Error(codes.Code(r.ErrorResponse.GetErrorCode()), r.ErrorResponse.GetErrorMessage())
	default:
		return nil, status.Errorf(codes.Internal, "wrong response type: %T", r)
	}
}
