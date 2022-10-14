// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.26.0
// 	protoc        (unknown)
// source: grpctunnel/v1/tunnel.proto

package tunnelpb

import (
	status "google.golang.org/genproto/googleapis/rpc/status"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// ClientToServer is the message a client sends to a server.
//
// For a single stream ID, the first such message must include the new_stream
// field. After that, there can be any number of requests sent, via the
// request_message field and additional messages thereafter that use the
// more_request_data field (for requests that are larger than 16kb). And
// finally, the RPC ends with either the half_close or cancel fields. If the
// half_close field is used, the RPC stream remains active so the server may
// continue to send response data. But, if the cancel field is used, the RPC
// stream is aborted and thus closed on both client and server ends. If a stream
// has been half-closed, the only allowed message from the client for that
// stream ID is one with the cancel field, to abort the remainder of the
// operation.
type ClientToServer struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The ID of the stream. Stream IDs must be used in increasing order and
	// cannot be re-used. Unlike in the HTTP/2 protocol, the stream ID is 64-bit
	// so overflow in a long-lived channel is excessively unlikely. (If the
	// channel were used for a stream every nanosecond, it would take close to
	// 300 years to exhaust every ID and reach an overflow situation.)
	StreamId int64 `protobuf:"varint,1,opt,name=stream_id,json=streamId,proto3" json:"stream_id,omitempty"`
	// Types that are assignable to Frame:
	//
	//	*ClientToServer_NewStream
	//	*ClientToServer_RequestMessage
	//	*ClientToServer_MoreRequestData
	//	*ClientToServer_HalfClose
	//	*ClientToServer_Cancel
	Frame isClientToServer_Frame `protobuf_oneof:"frame"`
}

func (x *ClientToServer) Reset() {
	*x = ClientToServer{}
	if protoimpl.UnsafeEnabled {
		mi := &file_grpctunnel_v1_tunnel_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ClientToServer) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClientToServer) ProtoMessage() {}

func (x *ClientToServer) ProtoReflect() protoreflect.Message {
	mi := &file_grpctunnel_v1_tunnel_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ClientToServer.ProtoReflect.Descriptor instead.
func (*ClientToServer) Descriptor() ([]byte, []int) {
	return file_grpctunnel_v1_tunnel_proto_rawDescGZIP(), []int{0}
}

func (x *ClientToServer) GetStreamId() int64 {
	if x != nil {
		return x.StreamId
	}
	return 0
}

func (m *ClientToServer) GetFrame() isClientToServer_Frame {
	if m != nil {
		return m.Frame
	}
	return nil
}

func (x *ClientToServer) GetNewStream() *NewStream {
	if x, ok := x.GetFrame().(*ClientToServer_NewStream); ok {
		return x.NewStream
	}
	return nil
}

func (x *ClientToServer) GetRequestMessage() *MessageData {
	if x, ok := x.GetFrame().(*ClientToServer_RequestMessage); ok {
		return x.RequestMessage
	}
	return nil
}

func (x *ClientToServer) GetMoreRequestData() []byte {
	if x, ok := x.GetFrame().(*ClientToServer_MoreRequestData); ok {
		return x.MoreRequestData
	}
	return nil
}

func (x *ClientToServer) GetHalfClose() *emptypb.Empty {
	if x, ok := x.GetFrame().(*ClientToServer_HalfClose); ok {
		return x.HalfClose
	}
	return nil
}

func (x *ClientToServer) GetCancel() *emptypb.Empty {
	if x, ok := x.GetFrame().(*ClientToServer_Cancel); ok {
		return x.Cancel
	}
	return nil
}

type isClientToServer_Frame interface {
	isClientToServer_Frame()
}

type ClientToServer_NewStream struct {
	// Creates a new RPC stream, which includes request header metadata. The
	// stream ID must not be an already active stream.
	NewStream *NewStream `protobuf:"bytes,2,opt,name=new_stream,json=newStream,proto3,oneof"`
}

type ClientToServer_RequestMessage struct {
	// Sends a message on the RPC stream. If the message is larger than 16k,
	// the rest of the message should be sent in chunks using the
	// more_request_data field (up to 16kb of data in each chunk).
	RequestMessage *MessageData `protobuf:"bytes,3,opt,name=request_message,json=requestMessage,proto3,oneof"`
}

type ClientToServer_MoreRequestData struct {
	// Sends a chunk of request data, for a request message that could not
	// wholly fit in a request_message field (e.g. > 16kb).
	MoreRequestData []byte `protobuf:"bytes,4,opt,name=more_request_data,json=moreRequestData,proto3,oneof"`
}

type ClientToServer_HalfClose struct {
	// Half-closes the stream, signaling that no more request messages will
	// be sent. No other messages, other than one with the cancel field set,
	// should be sent for this stream (at least not until it is terminated
	// by the server, after which the ID can be re-used).
	HalfClose *emptypb.Empty `protobuf:"bytes,5,opt,name=half_close,json=halfClose,proto3,oneof"`
}

type ClientToServer_Cancel struct {
	// Aborts the stream. No other messages should be sent for this stream
	// (unless the ID is being re-used after the stream is terminated on the
	// server side).
	Cancel *emptypb.Empty `protobuf:"bytes,6,opt,name=cancel,proto3,oneof"`
}

func (*ClientToServer_NewStream) isClientToServer_Frame() {}

func (*ClientToServer_RequestMessage) isClientToServer_Frame() {}

func (*ClientToServer_MoreRequestData) isClientToServer_Frame() {}

func (*ClientToServer_HalfClose) isClientToServer_Frame() {}

func (*ClientToServer_Cancel) isClientToServer_Frame() {}

// ServerToClient is the message a server sends to a client.
//
// For a single stream ID, the first such message should include the
// response_headers field unless no headers are to be sent. After the headers,
// the server can send any number of responses, via the response_message field
// and additional messages thereafter that use the more_response_data field (for
// responses that are larger than 16kb). A message with the close_stream field
// concludes the stream, whether it terminates successfully or with an error.
type ServerToClient struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The ID of the stream. Stream IDs are defined by the client and should be
	// used in monotonically increasing order. They cannot be re-used. Unlike
	// HTTP/2, the ID is 64-bit, so overflow/re-use should not be an issue. (If
	// the channel were used for a stream every nanosecond, it would take close
	// to 300 years to exhaust every ID and reach an overflow situation.)
	StreamId int64 `protobuf:"varint,1,opt,name=stream_id,json=streamId,proto3" json:"stream_id,omitempty"`
	// Types that are assignable to Frame:
	//
	//	*ServerToClient_ResponseHeaders
	//	*ServerToClient_ResponseMessage
	//	*ServerToClient_MoreResponseData
	//	*ServerToClient_CloseStream
	Frame isServerToClient_Frame `protobuf_oneof:"frame"`
}

func (x *ServerToClient) Reset() {
	*x = ServerToClient{}
	if protoimpl.UnsafeEnabled {
		mi := &file_grpctunnel_v1_tunnel_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ServerToClient) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ServerToClient) ProtoMessage() {}

func (x *ServerToClient) ProtoReflect() protoreflect.Message {
	mi := &file_grpctunnel_v1_tunnel_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ServerToClient.ProtoReflect.Descriptor instead.
func (*ServerToClient) Descriptor() ([]byte, []int) {
	return file_grpctunnel_v1_tunnel_proto_rawDescGZIP(), []int{1}
}

func (x *ServerToClient) GetStreamId() int64 {
	if x != nil {
		return x.StreamId
	}
	return 0
}

func (m *ServerToClient) GetFrame() isServerToClient_Frame {
	if m != nil {
		return m.Frame
	}
	return nil
}

func (x *ServerToClient) GetResponseHeaders() *Metadata {
	if x, ok := x.GetFrame().(*ServerToClient_ResponseHeaders); ok {
		return x.ResponseHeaders
	}
	return nil
}

func (x *ServerToClient) GetResponseMessage() *MessageData {
	if x, ok := x.GetFrame().(*ServerToClient_ResponseMessage); ok {
		return x.ResponseMessage
	}
	return nil
}

func (x *ServerToClient) GetMoreResponseData() []byte {
	if x, ok := x.GetFrame().(*ServerToClient_MoreResponseData); ok {
		return x.MoreResponseData
	}
	return nil
}

func (x *ServerToClient) GetCloseStream() *CloseStream {
	if x, ok := x.GetFrame().(*ServerToClient_CloseStream); ok {
		return x.CloseStream
	}
	return nil
}

type isServerToClient_Frame interface {
	isServerToClient_Frame()
}

type ServerToClient_ResponseHeaders struct {
	// Sends response headers for this stream. If headers are sent at all,
	// they must be sent before any response message data.
	ResponseHeaders *Metadata `protobuf:"bytes,2,opt,name=response_headers,json=responseHeaders,proto3,oneof"`
}

type ServerToClient_ResponseMessage struct {
	// Sends a message on the RPC stream. If the message is larger than 16k,
	// the rest of the message should be sent in chunks using the
	// more_response_data field (up to 16kb of data in each chunk).
	ResponseMessage *MessageData `protobuf:"bytes,3,opt,name=response_message,json=responseMessage,proto3,oneof"`
}

type ServerToClient_MoreResponseData struct {
	// Sends a chunk of response data, for a response message that could not
	// wholly fit in a response_message field (e.g. > 16kb).
	MoreResponseData []byte `protobuf:"bytes,4,opt,name=more_response_data,json=moreResponseData,proto3,oneof"`
}

type ServerToClient_CloseStream struct {
	// Terminates the stream and communicates the final disposition to the
	// client. After the stream is closed, no other messages should use the
	// given stream ID until the ID is re-used (e.g. a NewStream message is
	// received that creates another stream with the same ID).
	CloseStream *CloseStream `protobuf:"bytes,5,opt,name=close_stream,json=closeStream,proto3,oneof"`
}

func (*ServerToClient_ResponseHeaders) isServerToClient_Frame() {}

func (*ServerToClient_ResponseMessage) isServerToClient_Frame() {}

func (*ServerToClient_MoreResponseData) isServerToClient_Frame() {}

func (*ServerToClient_CloseStream) isServerToClient_Frame() {}

type NewStream struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	MethodName     string    `protobuf:"bytes,1,opt,name=method_name,json=methodName,proto3" json:"method_name,omitempty"`
	RequestHeaders *Metadata `protobuf:"bytes,2,opt,name=request_headers,json=requestHeaders,proto3" json:"request_headers,omitempty"`
}

func (x *NewStream) Reset() {
	*x = NewStream{}
	if protoimpl.UnsafeEnabled {
		mi := &file_grpctunnel_v1_tunnel_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *NewStream) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NewStream) ProtoMessage() {}

func (x *NewStream) ProtoReflect() protoreflect.Message {
	mi := &file_grpctunnel_v1_tunnel_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NewStream.ProtoReflect.Descriptor instead.
func (*NewStream) Descriptor() ([]byte, []int) {
	return file_grpctunnel_v1_tunnel_proto_rawDescGZIP(), []int{2}
}

func (x *NewStream) GetMethodName() string {
	if x != nil {
		return x.MethodName
	}
	return ""
}

func (x *NewStream) GetRequestHeaders() *Metadata {
	if x != nil {
		return x.RequestHeaders
	}
	return nil
}

type MessageData struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The full size of the message.
	Size int32 `protobuf:"varint,1,opt,name=size,proto3" json:"size,omitempty"`
	// The message data. This field should not be longer than 16kb (16,384
	// bytes). If the full size of the message is larger then it should be
	// split into multiple chunks. The chunking is done to allow multiple
	// access to the underlying gRPC stream by concurrent tunneled streams.
	// If very large messages were sent via a single chunk, it could cause
	// head-of-line blocking and starvation when multiple streams need to send
	// data on the one underlying gRPC stream.
	Data []byte `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`
}

func (x *MessageData) Reset() {
	*x = MessageData{}
	if protoimpl.UnsafeEnabled {
		mi := &file_grpctunnel_v1_tunnel_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *MessageData) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MessageData) ProtoMessage() {}

func (x *MessageData) ProtoReflect() protoreflect.Message {
	mi := &file_grpctunnel_v1_tunnel_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use MessageData.ProtoReflect.Descriptor instead.
func (*MessageData) Descriptor() ([]byte, []int) {
	return file_grpctunnel_v1_tunnel_proto_rawDescGZIP(), []int{3}
}

func (x *MessageData) GetSize() int32 {
	if x != nil {
		return x.Size
	}
	return 0
}

func (x *MessageData) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

type CloseStream struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ResponseTrailers *Metadata      `protobuf:"bytes,1,opt,name=response_trailers,json=responseTrailers,proto3" json:"response_trailers,omitempty"`
	Status           *status.Status `protobuf:"bytes,2,opt,name=status,proto3" json:"status,omitempty"`
}

func (x *CloseStream) Reset() {
	*x = CloseStream{}
	if protoimpl.UnsafeEnabled {
		mi := &file_grpctunnel_v1_tunnel_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CloseStream) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CloseStream) ProtoMessage() {}

func (x *CloseStream) ProtoReflect() protoreflect.Message {
	mi := &file_grpctunnel_v1_tunnel_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CloseStream.ProtoReflect.Descriptor instead.
func (*CloseStream) Descriptor() ([]byte, []int) {
	return file_grpctunnel_v1_tunnel_proto_rawDescGZIP(), []int{4}
}

func (x *CloseStream) GetResponseTrailers() *Metadata {
	if x != nil {
		return x.ResponseTrailers
	}
	return nil
}

func (x *CloseStream) GetStatus() *status.Status {
	if x != nil {
		return x.Status
	}
	return nil
}

type Metadata struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Md map[string]*Metadata_Values `protobuf:"bytes,1,rep,name=md,proto3" json:"md,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
}

func (x *Metadata) Reset() {
	*x = Metadata{}
	if protoimpl.UnsafeEnabled {
		mi := &file_grpctunnel_v1_tunnel_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Metadata) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Metadata) ProtoMessage() {}

func (x *Metadata) ProtoReflect() protoreflect.Message {
	mi := &file_grpctunnel_v1_tunnel_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Metadata.ProtoReflect.Descriptor instead.
func (*Metadata) Descriptor() ([]byte, []int) {
	return file_grpctunnel_v1_tunnel_proto_rawDescGZIP(), []int{5}
}

func (x *Metadata) GetMd() map[string]*Metadata_Values {
	if x != nil {
		return x.Md
	}
	return nil
}

type Metadata_Values struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Val []string `protobuf:"bytes,1,rep,name=val,proto3" json:"val,omitempty"`
}

func (x *Metadata_Values) Reset() {
	*x = Metadata_Values{}
	if protoimpl.UnsafeEnabled {
		mi := &file_grpctunnel_v1_tunnel_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Metadata_Values) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Metadata_Values) ProtoMessage() {}

func (x *Metadata_Values) ProtoReflect() protoreflect.Message {
	mi := &file_grpctunnel_v1_tunnel_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Metadata_Values.ProtoReflect.Descriptor instead.
func (*Metadata_Values) Descriptor() ([]byte, []int) {
	return file_grpctunnel_v1_tunnel_proto_rawDescGZIP(), []int{5, 0}
}

func (x *Metadata_Values) GetVal() []string {
	if x != nil {
		return x.Val
	}
	return nil
}

var File_grpctunnel_v1_tunnel_proto protoreflect.FileDescriptor

var file_grpctunnel_v1_tunnel_proto_rawDesc = []byte{
	0x0a, 0x1a, 0x67, 0x72, 0x70, 0x63, 0x74, 0x75, 0x6e, 0x6e, 0x65, 0x6c, 0x2f, 0x76, 0x31, 0x2f,
	0x74, 0x75, 0x6e, 0x6e, 0x65, 0x6c, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0d, 0x67, 0x72,
	0x70, 0x63, 0x74, 0x75, 0x6e, 0x6e, 0x65, 0x6c, 0x2e, 0x76, 0x31, 0x1a, 0x1b, 0x67, 0x6f, 0x6f,
	0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x65, 0x6d, 0x70,
	0x74, 0x79, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x17, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
	0x2f, 0x72, 0x70, 0x63, 0x2f, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x22, 0xd1, 0x02, 0x0a, 0x0e, 0x43, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x54, 0x6f, 0x53, 0x65,
	0x72, 0x76, 0x65, 0x72, 0x12, 0x1b, 0x0a, 0x09, 0x73, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x5f, 0x69,
	0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x08, 0x73, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x49,
	0x64, 0x12, 0x39, 0x0a, 0x0a, 0x6e, 0x65, 0x77, 0x5f, 0x73, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x18, 0x2e, 0x67, 0x72, 0x70, 0x63, 0x74, 0x75, 0x6e, 0x6e,
	0x65, 0x6c, 0x2e, 0x76, 0x31, 0x2e, 0x4e, 0x65, 0x77, 0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x48,
	0x00, 0x52, 0x09, 0x6e, 0x65, 0x77, 0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x12, 0x45, 0x0a, 0x0f,
	0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x5f, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x72, 0x70, 0x63, 0x74, 0x75, 0x6e, 0x6e,
	0x65, 0x6c, 0x2e, 0x76, 0x31, 0x2e, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x44, 0x61, 0x74,
	0x61, 0x48, 0x00, 0x52, 0x0e, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x4d, 0x65, 0x73, 0x73,
	0x61, 0x67, 0x65, 0x12, 0x2c, 0x0a, 0x11, 0x6d, 0x6f, 0x72, 0x65, 0x5f, 0x72, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x5f, 0x64, 0x61, 0x74, 0x61, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0c, 0x48, 0x00,
	0x52, 0x0f, 0x6d, 0x6f, 0x72, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x44, 0x61, 0x74,
	0x61, 0x12, 0x37, 0x0a, 0x0a, 0x68, 0x61, 0x6c, 0x66, 0x5f, 0x63, 0x6c, 0x6f, 0x73, 0x65, 0x18,
	0x05, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x48, 0x00, 0x52,
	0x09, 0x68, 0x61, 0x6c, 0x66, 0x43, 0x6c, 0x6f, 0x73, 0x65, 0x12, 0x30, 0x0a, 0x06, 0x63, 0x61,
	0x6e, 0x63, 0x65, 0x6c, 0x18, 0x06, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x16, 0x2e, 0x67, 0x6f, 0x6f,
	0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70,
	0x74, 0x79, 0x48, 0x00, 0x52, 0x06, 0x63, 0x61, 0x6e, 0x63, 0x65, 0x6c, 0x42, 0x07, 0x0a, 0x05,
	0x66, 0x72, 0x61, 0x6d, 0x65, 0x22, 0xb6, 0x02, 0x0a, 0x0e, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72,
	0x54, 0x6f, 0x43, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x12, 0x1b, 0x0a, 0x09, 0x73, 0x74, 0x72, 0x65,
	0x61, 0x6d, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x08, 0x73, 0x74, 0x72,
	0x65, 0x61, 0x6d, 0x49, 0x64, 0x12, 0x44, 0x0a, 0x10, 0x72, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x5f, 0x68, 0x65, 0x61, 0x64, 0x65, 0x72, 0x73, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x17, 0x2e, 0x67, 0x72, 0x70, 0x63, 0x74, 0x75, 0x6e, 0x6e, 0x65, 0x6c, 0x2e, 0x76, 0x31, 0x2e,
	0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0x48, 0x00, 0x52, 0x0f, 0x72, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x73, 0x12, 0x47, 0x0a, 0x10, 0x72,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x5f, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x72, 0x70, 0x63, 0x74, 0x75, 0x6e, 0x6e,
	0x65, 0x6c, 0x2e, 0x76, 0x31, 0x2e, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x44, 0x61, 0x74,
	0x61, 0x48, 0x00, 0x52, 0x0f, 0x72, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x4d, 0x65, 0x73,
	0x73, 0x61, 0x67, 0x65, 0x12, 0x2e, 0x0a, 0x12, 0x6d, 0x6f, 0x72, 0x65, 0x5f, 0x72, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x5f, 0x64, 0x61, 0x74, 0x61, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0c,
	0x48, 0x00, 0x52, 0x10, 0x6d, 0x6f, 0x72, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x44, 0x61, 0x74, 0x61, 0x12, 0x3f, 0x0a, 0x0c, 0x63, 0x6c, 0x6f, 0x73, 0x65, 0x5f, 0x73, 0x74,
	0x72, 0x65, 0x61, 0x6d, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x72, 0x70,
	0x63, 0x74, 0x75, 0x6e, 0x6e, 0x65, 0x6c, 0x2e, 0x76, 0x31, 0x2e, 0x43, 0x6c, 0x6f, 0x73, 0x65,
	0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x48, 0x00, 0x52, 0x0b, 0x63, 0x6c, 0x6f, 0x73, 0x65, 0x53,
	0x74, 0x72, 0x65, 0x61, 0x6d, 0x42, 0x07, 0x0a, 0x05, 0x66, 0x72, 0x61, 0x6d, 0x65, 0x22, 0x6e,
	0x0a, 0x09, 0x4e, 0x65, 0x77, 0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x12, 0x1f, 0x0a, 0x0b, 0x6d,
	0x65, 0x74, 0x68, 0x6f, 0x64, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x0a, 0x6d, 0x65, 0x74, 0x68, 0x6f, 0x64, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x40, 0x0a, 0x0f,
	0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x5f, 0x68, 0x65, 0x61, 0x64, 0x65, 0x72, 0x73, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x17, 0x2e, 0x67, 0x72, 0x70, 0x63, 0x74, 0x75, 0x6e, 0x6e,
	0x65, 0x6c, 0x2e, 0x76, 0x31, 0x2e, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0x52, 0x0e,
	0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x73, 0x22, 0x35,
	0x0a, 0x0b, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x44, 0x61, 0x74, 0x61, 0x12, 0x12, 0x0a,
	0x04, 0x73, 0x69, 0x7a, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x05, 0x52, 0x04, 0x73, 0x69, 0x7a,
	0x65, 0x12, 0x12, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52,
	0x04, 0x64, 0x61, 0x74, 0x61, 0x22, 0x7f, 0x0a, 0x0b, 0x43, 0x6c, 0x6f, 0x73, 0x65, 0x53, 0x74,
	0x72, 0x65, 0x61, 0x6d, 0x12, 0x44, 0x0a, 0x11, 0x72, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x5f, 0x74, 0x72, 0x61, 0x69, 0x6c, 0x65, 0x72, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x17, 0x2e, 0x67, 0x72, 0x70, 0x63, 0x74, 0x75, 0x6e, 0x6e, 0x65, 0x6c, 0x2e, 0x76, 0x31, 0x2e,
	0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0x52, 0x10, 0x72, 0x65, 0x73, 0x70, 0x6f, 0x6e,
	0x73, 0x65, 0x54, 0x72, 0x61, 0x69, 0x6c, 0x65, 0x72, 0x73, 0x12, 0x2a, 0x0a, 0x06, 0x73, 0x74,
	0x61, 0x74, 0x75, 0x73, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x12, 0x2e, 0x67, 0x6f, 0x6f,
	0x67, 0x6c, 0x65, 0x2e, 0x72, 0x70, 0x63, 0x2e, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x52, 0x06,
	0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x22, 0xae, 0x01, 0x0a, 0x08, 0x4d, 0x65, 0x74, 0x61, 0x64,
	0x61, 0x74, 0x61, 0x12, 0x2f, 0x0a, 0x02, 0x6d, 0x64, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32,
	0x1f, 0x2e, 0x67, 0x72, 0x70, 0x63, 0x74, 0x75, 0x6e, 0x6e, 0x65, 0x6c, 0x2e, 0x76, 0x31, 0x2e,
	0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0x2e, 0x4d, 0x64, 0x45, 0x6e, 0x74, 0x72, 0x79,
	0x52, 0x02, 0x6d, 0x64, 0x1a, 0x1a, 0x0a, 0x06, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x73, 0x12, 0x10,
	0x0a, 0x03, 0x76, 0x61, 0x6c, 0x18, 0x01, 0x20, 0x03, 0x28, 0x09, 0x52, 0x03, 0x76, 0x61, 0x6c,
	0x1a, 0x55, 0x0a, 0x07, 0x4d, 0x64, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b,
	0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x34, 0x0a,
	0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1e, 0x2e, 0x67,
	0x72, 0x70, 0x63, 0x74, 0x75, 0x6e, 0x6e, 0x65, 0x6c, 0x2e, 0x76, 0x31, 0x2e, 0x4d, 0x65, 0x74,
	0x61, 0x64, 0x61, 0x74, 0x61, 0x2e, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x73, 0x52, 0x05, 0x76, 0x61,
	0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x32, 0xb6, 0x01, 0x0a, 0x0d, 0x54, 0x75, 0x6e, 0x6e,
	0x65, 0x6c, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x4e, 0x0a, 0x0a, 0x4f, 0x70, 0x65,
	0x6e, 0x54, 0x75, 0x6e, 0x6e, 0x65, 0x6c, 0x12, 0x1d, 0x2e, 0x67, 0x72, 0x70, 0x63, 0x74, 0x75,
	0x6e, 0x6e, 0x65, 0x6c, 0x2e, 0x76, 0x31, 0x2e, 0x43, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x54, 0x6f,
	0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x1a, 0x1d, 0x2e, 0x67, 0x72, 0x70, 0x63, 0x74, 0x75, 0x6e,
	0x6e, 0x65, 0x6c, 0x2e, 0x76, 0x31, 0x2e, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x54, 0x6f, 0x43,
	0x6c, 0x69, 0x65, 0x6e, 0x74, 0x28, 0x01, 0x30, 0x01, 0x12, 0x55, 0x0a, 0x11, 0x4f, 0x70, 0x65,
	0x6e, 0x52, 0x65, 0x76, 0x65, 0x72, 0x73, 0x65, 0x54, 0x75, 0x6e, 0x6e, 0x65, 0x6c, 0x12, 0x1d,
	0x2e, 0x67, 0x72, 0x70, 0x63, 0x74, 0x75, 0x6e, 0x6e, 0x65, 0x6c, 0x2e, 0x76, 0x31, 0x2e, 0x53,
	0x65, 0x72, 0x76, 0x65, 0x72, 0x54, 0x6f, 0x43, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x1a, 0x1d, 0x2e,
	0x67, 0x72, 0x70, 0x63, 0x74, 0x75, 0x6e, 0x6e, 0x65, 0x6c, 0x2e, 0x76, 0x31, 0x2e, 0x43, 0x6c,
	0x69, 0x65, 0x6e, 0x74, 0x54, 0x6f, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x28, 0x01, 0x30, 0x01,
	0x42, 0x26, 0x5a, 0x24, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x6a,
	0x68, 0x75, 0x6d, 0x70, 0x2f, 0x67, 0x72, 0x70, 0x63, 0x74, 0x75, 0x6e, 0x6e, 0x65, 0x6c, 0x2f,
	0x74, 0x75, 0x6e, 0x6e, 0x65, 0x6c, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_grpctunnel_v1_tunnel_proto_rawDescOnce sync.Once
	file_grpctunnel_v1_tunnel_proto_rawDescData = file_grpctunnel_v1_tunnel_proto_rawDesc
)

func file_grpctunnel_v1_tunnel_proto_rawDescGZIP() []byte {
	file_grpctunnel_v1_tunnel_proto_rawDescOnce.Do(func() {
		file_grpctunnel_v1_tunnel_proto_rawDescData = protoimpl.X.CompressGZIP(file_grpctunnel_v1_tunnel_proto_rawDescData)
	})
	return file_grpctunnel_v1_tunnel_proto_rawDescData
}

var file_grpctunnel_v1_tunnel_proto_msgTypes = make([]protoimpl.MessageInfo, 8)
var file_grpctunnel_v1_tunnel_proto_goTypes = []interface{}{
	(*ClientToServer)(nil),  // 0: grpctunnel.v1.ClientToServer
	(*ServerToClient)(nil),  // 1: grpctunnel.v1.ServerToClient
	(*NewStream)(nil),       // 2: grpctunnel.v1.NewStream
	(*MessageData)(nil),     // 3: grpctunnel.v1.MessageData
	(*CloseStream)(nil),     // 4: grpctunnel.v1.CloseStream
	(*Metadata)(nil),        // 5: grpctunnel.v1.Metadata
	(*Metadata_Values)(nil), // 6: grpctunnel.v1.Metadata.Values
	nil,                     // 7: grpctunnel.v1.Metadata.MdEntry
	(*emptypb.Empty)(nil),   // 8: google.protobuf.Empty
	(*status.Status)(nil),   // 9: google.rpc.Status
}
var file_grpctunnel_v1_tunnel_proto_depIdxs = []int32{
	2,  // 0: grpctunnel.v1.ClientToServer.new_stream:type_name -> grpctunnel.v1.NewStream
	3,  // 1: grpctunnel.v1.ClientToServer.request_message:type_name -> grpctunnel.v1.MessageData
	8,  // 2: grpctunnel.v1.ClientToServer.half_close:type_name -> google.protobuf.Empty
	8,  // 3: grpctunnel.v1.ClientToServer.cancel:type_name -> google.protobuf.Empty
	5,  // 4: grpctunnel.v1.ServerToClient.response_headers:type_name -> grpctunnel.v1.Metadata
	3,  // 5: grpctunnel.v1.ServerToClient.response_message:type_name -> grpctunnel.v1.MessageData
	4,  // 6: grpctunnel.v1.ServerToClient.close_stream:type_name -> grpctunnel.v1.CloseStream
	5,  // 7: grpctunnel.v1.NewStream.request_headers:type_name -> grpctunnel.v1.Metadata
	5,  // 8: grpctunnel.v1.CloseStream.response_trailers:type_name -> grpctunnel.v1.Metadata
	9,  // 9: grpctunnel.v1.CloseStream.status:type_name -> google.rpc.Status
	7,  // 10: grpctunnel.v1.Metadata.md:type_name -> grpctunnel.v1.Metadata.MdEntry
	6,  // 11: grpctunnel.v1.Metadata.MdEntry.value:type_name -> grpctunnel.v1.Metadata.Values
	0,  // 12: grpctunnel.v1.TunnelService.OpenTunnel:input_type -> grpctunnel.v1.ClientToServer
	1,  // 13: grpctunnel.v1.TunnelService.OpenReverseTunnel:input_type -> grpctunnel.v1.ServerToClient
	1,  // 14: grpctunnel.v1.TunnelService.OpenTunnel:output_type -> grpctunnel.v1.ServerToClient
	0,  // 15: grpctunnel.v1.TunnelService.OpenReverseTunnel:output_type -> grpctunnel.v1.ClientToServer
	14, // [14:16] is the sub-list for method output_type
	12, // [12:14] is the sub-list for method input_type
	12, // [12:12] is the sub-list for extension type_name
	12, // [12:12] is the sub-list for extension extendee
	0,  // [0:12] is the sub-list for field type_name
}

func init() { file_grpctunnel_v1_tunnel_proto_init() }
func file_grpctunnel_v1_tunnel_proto_init() {
	if File_grpctunnel_v1_tunnel_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_grpctunnel_v1_tunnel_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ClientToServer); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_grpctunnel_v1_tunnel_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ServerToClient); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_grpctunnel_v1_tunnel_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*NewStream); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_grpctunnel_v1_tunnel_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*MessageData); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_grpctunnel_v1_tunnel_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CloseStream); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_grpctunnel_v1_tunnel_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Metadata); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_grpctunnel_v1_tunnel_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Metadata_Values); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	file_grpctunnel_v1_tunnel_proto_msgTypes[0].OneofWrappers = []interface{}{
		(*ClientToServer_NewStream)(nil),
		(*ClientToServer_RequestMessage)(nil),
		(*ClientToServer_MoreRequestData)(nil),
		(*ClientToServer_HalfClose)(nil),
		(*ClientToServer_Cancel)(nil),
	}
	file_grpctunnel_v1_tunnel_proto_msgTypes[1].OneofWrappers = []interface{}{
		(*ServerToClient_ResponseHeaders)(nil),
		(*ServerToClient_ResponseMessage)(nil),
		(*ServerToClient_MoreResponseData)(nil),
		(*ServerToClient_CloseStream)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_grpctunnel_v1_tunnel_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   8,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_grpctunnel_v1_tunnel_proto_goTypes,
		DependencyIndexes: file_grpctunnel_v1_tunnel_proto_depIdxs,
		MessageInfos:      file_grpctunnel_v1_tunnel_proto_msgTypes,
	}.Build()
	File_grpctunnel_v1_tunnel_proto = out.File
	file_grpctunnel_v1_tunnel_proto_rawDesc = nil
	file_grpctunnel_v1_tunnel_proto_goTypes = nil
	file_grpctunnel_v1_tunnel_proto_depIdxs = nil
}