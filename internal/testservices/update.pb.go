// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.26.0
// 	protoc        (unknown)
// source: update.proto

package testservices

import (
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

type Operation struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	VariableName string `protobuf:"bytes,1,opt,name=variable_name,json=variableName,proto3" json:"variable_name,omitempty"`
	// Types that are assignable to Request:
	//
	//	*Operation_Load
	//	*Operation_Save
	//	*Operation_SetValue
	//	*Operation_Add
	//	*Operation_Multiply
	//	*Operation_Divide
	//	*Operation_Exp
	//	*Operation_Sin
	//	*Operation_Cos
	//	*Operation_Ln
	//	*Operation_Log
	Request isOperation_Request `protobuf_oneof:"request"`
}

func (x *Operation) Reset() {
	*x = Operation{}
	if protoimpl.UnsafeEnabled {
		mi := &file_update_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Operation) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Operation) ProtoMessage() {}

func (x *Operation) ProtoReflect() protoreflect.Message {
	mi := &file_update_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Operation.ProtoReflect.Descriptor instead.
func (*Operation) Descriptor() ([]byte, []int) {
	return file_update_proto_rawDescGZIP(), []int{0}
}

func (x *Operation) GetVariableName() string {
	if x != nil {
		return x.VariableName
	}
	return ""
}

func (m *Operation) GetRequest() isOperation_Request {
	if m != nil {
		return m.Request
	}
	return nil
}

func (x *Operation) GetLoad() string {
	if x, ok := x.GetRequest().(*Operation_Load); ok {
		return x.Load
	}
	return ""
}

func (x *Operation) GetSave() *emptypb.Empty {
	if x, ok := x.GetRequest().(*Operation_Save); ok {
		return x.Save
	}
	return nil
}

func (x *Operation) GetSetValue() float64 {
	if x, ok := x.GetRequest().(*Operation_SetValue); ok {
		return x.SetValue
	}
	return 0
}

func (x *Operation) GetAdd() float64 {
	if x, ok := x.GetRequest().(*Operation_Add); ok {
		return x.Add
	}
	return 0
}

func (x *Operation) GetMultiply() float64 {
	if x, ok := x.GetRequest().(*Operation_Multiply); ok {
		return x.Multiply
	}
	return 0
}

func (x *Operation) GetDivide() float64 {
	if x, ok := x.GetRequest().(*Operation_Divide); ok {
		return x.Divide
	}
	return 0
}

func (x *Operation) GetExp() float64 {
	if x, ok := x.GetRequest().(*Operation_Exp); ok {
		return x.Exp
	}
	return 0
}

func (x *Operation) GetSin() *emptypb.Empty {
	if x, ok := x.GetRequest().(*Operation_Sin); ok {
		return x.Sin
	}
	return nil
}

func (x *Operation) GetCos() *emptypb.Empty {
	if x, ok := x.GetRequest().(*Operation_Cos); ok {
		return x.Cos
	}
	return nil
}

func (x *Operation) GetLn() *emptypb.Empty {
	if x, ok := x.GetRequest().(*Operation_Ln); ok {
		return x.Ln
	}
	return nil
}

func (x *Operation) GetLog() float64 {
	if x, ok := x.GetRequest().(*Operation_Log); ok {
		return x.Log
	}
	return 0
}

type isOperation_Request interface {
	isOperation_Request()
}

type Operation_Load struct {
	Load string `protobuf:"bytes,2,opt,name=load,proto3,oneof"`
}

type Operation_Save struct {
	Save *emptypb.Empty `protobuf:"bytes,3,opt,name=save,proto3,oneof"`
}

type Operation_SetValue struct {
	SetValue float64 `protobuf:"fixed64,4,opt,name=set_value,json=setValue,proto3,oneof"`
}

type Operation_Add struct {
	Add float64 `protobuf:"fixed64,5,opt,name=add,proto3,oneof"`
}

type Operation_Multiply struct {
	Multiply float64 `protobuf:"fixed64,6,opt,name=multiply,proto3,oneof"`
}

type Operation_Divide struct {
	Divide float64 `protobuf:"fixed64,7,opt,name=divide,proto3,oneof"`
}

type Operation_Exp struct {
	Exp float64 `protobuf:"fixed64,8,opt,name=exp,proto3,oneof"`
}

type Operation_Sin struct {
	Sin *emptypb.Empty `protobuf:"bytes,9,opt,name=sin,proto3,oneof"`
}

type Operation_Cos struct {
	Cos *emptypb.Empty `protobuf:"bytes,10,opt,name=cos,proto3,oneof"`
}

type Operation_Ln struct {
	Ln *emptypb.Empty `protobuf:"bytes,11,opt,name=ln,proto3,oneof"`
}

type Operation_Log struct {
	Log float64 `protobuf:"fixed64,12,opt,name=log,proto3,oneof"`
}

func (*Operation_Load) isOperation_Request() {}

func (*Operation_Save) isOperation_Request() {}

func (*Operation_SetValue) isOperation_Request() {}

func (*Operation_Add) isOperation_Request() {}

func (*Operation_Multiply) isOperation_Request() {}

func (*Operation_Divide) isOperation_Request() {}

func (*Operation_Exp) isOperation_Request() {}

func (*Operation_Sin) isOperation_Request() {}

func (*Operation_Cos) isOperation_Request() {}

func (*Operation_Ln) isOperation_Request() {}

func (*Operation_Log) isOperation_Request() {}

type Answer struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	VariableName string  `protobuf:"bytes,1,opt,name=variable_name,json=variableName,proto3" json:"variable_name,omitempty"`
	Result       float64 `protobuf:"fixed64,2,opt,name=result,proto3" json:"result,omitempty"`
}

func (x *Answer) Reset() {
	*x = Answer{}
	if protoimpl.UnsafeEnabled {
		mi := &file_update_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Answer) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Answer) ProtoMessage() {}

func (x *Answer) ProtoReflect() protoreflect.Message {
	mi := &file_update_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Answer.ProtoReflect.Descriptor instead.
func (*Answer) Descriptor() ([]byte, []int) {
	return file_update_proto_rawDescGZIP(), []int{1}
}

func (x *Answer) GetVariableName() string {
	if x != nil {
		return x.VariableName
	}
	return ""
}

func (x *Answer) GetResult() float64 {
	if x != nil {
		return x.Result
	}
	return 0
}

var File_update_proto protoreflect.FileDescriptor

var file_update_proto_rawDesc = []byte{
	0x0a, 0x0c, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x12,
	0x67, 0x72, 0x70, 0x63, 0x74, 0x75, 0x6e, 0x6e, 0x65, 0x6c, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x69,
	0x6e, 0x67, 0x1a, 0x1b, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x62, 0x75, 0x66, 0x2f, 0x65, 0x6d, 0x70, 0x74, 0x79, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22,
	0x94, 0x03, 0x0a, 0x09, 0x4f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x23, 0x0a,
	0x0d, 0x76, 0x61, 0x72, 0x69, 0x61, 0x62, 0x6c, 0x65, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x0c, 0x76, 0x61, 0x72, 0x69, 0x61, 0x62, 0x6c, 0x65, 0x4e, 0x61,
	0x6d, 0x65, 0x12, 0x14, 0x0a, 0x04, 0x6c, 0x6f, 0x61, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09,
	0x48, 0x00, 0x52, 0x04, 0x6c, 0x6f, 0x61, 0x64, 0x12, 0x2c, 0x0a, 0x04, 0x73, 0x61, 0x76, 0x65,
	0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x48, 0x00,
	0x52, 0x04, 0x73, 0x61, 0x76, 0x65, 0x12, 0x1d, 0x0a, 0x09, 0x73, 0x65, 0x74, 0x5f, 0x76, 0x61,
	0x6c, 0x75, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x01, 0x48, 0x00, 0x52, 0x08, 0x73, 0x65, 0x74,
	0x56, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x12, 0x0a, 0x03, 0x61, 0x64, 0x64, 0x18, 0x05, 0x20, 0x01,
	0x28, 0x01, 0x48, 0x00, 0x52, 0x03, 0x61, 0x64, 0x64, 0x12, 0x1c, 0x0a, 0x08, 0x6d, 0x75, 0x6c,
	0x74, 0x69, 0x70, 0x6c, 0x79, 0x18, 0x06, 0x20, 0x01, 0x28, 0x01, 0x48, 0x00, 0x52, 0x08, 0x6d,
	0x75, 0x6c, 0x74, 0x69, 0x70, 0x6c, 0x79, 0x12, 0x18, 0x0a, 0x06, 0x64, 0x69, 0x76, 0x69, 0x64,
	0x65, 0x18, 0x07, 0x20, 0x01, 0x28, 0x01, 0x48, 0x00, 0x52, 0x06, 0x64, 0x69, 0x76, 0x69, 0x64,
	0x65, 0x12, 0x12, 0x0a, 0x03, 0x65, 0x78, 0x70, 0x18, 0x08, 0x20, 0x01, 0x28, 0x01, 0x48, 0x00,
	0x52, 0x03, 0x65, 0x78, 0x70, 0x12, 0x2a, 0x0a, 0x03, 0x73, 0x69, 0x6e, 0x18, 0x09, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x48, 0x00, 0x52, 0x03, 0x73, 0x69,
	0x6e, 0x12, 0x2a, 0x0a, 0x03, 0x63, 0x6f, 0x73, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x16,
	0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66,
	0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x48, 0x00, 0x52, 0x03, 0x63, 0x6f, 0x73, 0x12, 0x28, 0x0a,
	0x02, 0x6c, 0x6e, 0x18, 0x0b, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67,
	0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70, 0x74,
	0x79, 0x48, 0x00, 0x52, 0x02, 0x6c, 0x6e, 0x12, 0x12, 0x0a, 0x03, 0x6c, 0x6f, 0x67, 0x18, 0x0c,
	0x20, 0x01, 0x28, 0x01, 0x48, 0x00, 0x52, 0x03, 0x6c, 0x6f, 0x67, 0x42, 0x09, 0x0a, 0x07, 0x72,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x22, 0x45, 0x0a, 0x06, 0x41, 0x6e, 0x73, 0x77, 0x65, 0x72,
	0x12, 0x23, 0x0a, 0x0d, 0x76, 0x61, 0x72, 0x69, 0x61, 0x62, 0x6c, 0x65, 0x5f, 0x6e, 0x61, 0x6d,
	0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0c, 0x76, 0x61, 0x72, 0x69, 0x61, 0x62, 0x6c,
	0x65, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x16, 0x0a, 0x06, 0x72, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x01, 0x52, 0x06, 0x72, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x32, 0x5f, 0x0a,
	0x11, 0x43, 0x61, 0x6c, 0x63, 0x75, 0x6c, 0x61, 0x74, 0x6f, 0x72, 0x53, 0x65, 0x72, 0x76, 0x69,
	0x63, 0x65, 0x12, 0x4a, 0x0a, 0x09, 0x43, 0x61, 0x6c, 0x63, 0x75, 0x6c, 0x61, 0x74, 0x65, 0x12,
	0x1d, 0x2e, 0x67, 0x72, 0x70, 0x63, 0x74, 0x75, 0x6e, 0x6e, 0x65, 0x6c, 0x2e, 0x74, 0x65, 0x73,
	0x74, 0x69, 0x6e, 0x67, 0x2e, 0x4f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x1a, 0x1a,
	0x2e, 0x67, 0x72, 0x70, 0x63, 0x74, 0x75, 0x6e, 0x6e, 0x65, 0x6c, 0x2e, 0x74, 0x65, 0x73, 0x74,
	0x69, 0x6e, 0x67, 0x2e, 0x41, 0x6e, 0x73, 0x77, 0x65, 0x72, 0x28, 0x01, 0x30, 0x01, 0x42, 0x33,
	0x5a, 0x31, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x6a, 0x68, 0x75,
	0x6d, 0x70, 0x2f, 0x67, 0x72, 0x70, 0x63, 0x74, 0x75, 0x6e, 0x6e, 0x65, 0x6c, 0x2f, 0x69, 0x6e,
	0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f, 0x74, 0x65, 0x73, 0x74, 0x73, 0x65, 0x72, 0x76, 0x69,
	0x63, 0x65, 0x73, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_update_proto_rawDescOnce sync.Once
	file_update_proto_rawDescData = file_update_proto_rawDesc
)

func file_update_proto_rawDescGZIP() []byte {
	file_update_proto_rawDescOnce.Do(func() {
		file_update_proto_rawDescData = protoimpl.X.CompressGZIP(file_update_proto_rawDescData)
	})
	return file_update_proto_rawDescData
}

var file_update_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_update_proto_goTypes = []interface{}{
	(*Operation)(nil),     // 0: grpctunnel.testing.Operation
	(*Answer)(nil),        // 1: grpctunnel.testing.Answer
	(*emptypb.Empty)(nil), // 2: google.protobuf.Empty
}
var file_update_proto_depIdxs = []int32{
	2, // 0: grpctunnel.testing.Operation.save:type_name -> google.protobuf.Empty
	2, // 1: grpctunnel.testing.Operation.sin:type_name -> google.protobuf.Empty
	2, // 2: grpctunnel.testing.Operation.cos:type_name -> google.protobuf.Empty
	2, // 3: grpctunnel.testing.Operation.ln:type_name -> google.protobuf.Empty
	0, // 4: grpctunnel.testing.CalculatorService.Calculate:input_type -> grpctunnel.testing.Operation
	1, // 5: grpctunnel.testing.CalculatorService.Calculate:output_type -> grpctunnel.testing.Answer
	5, // [5:6] is the sub-list for method output_type
	4, // [4:5] is the sub-list for method input_type
	4, // [4:4] is the sub-list for extension type_name
	4, // [4:4] is the sub-list for extension extendee
	0, // [0:4] is the sub-list for field type_name
}

func init() { file_update_proto_init() }
func file_update_proto_init() {
	if File_update_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_update_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Operation); i {
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
		file_update_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Answer); i {
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
	file_update_proto_msgTypes[0].OneofWrappers = []interface{}{
		(*Operation_Load)(nil),
		(*Operation_Save)(nil),
		(*Operation_SetValue)(nil),
		(*Operation_Add)(nil),
		(*Operation_Multiply)(nil),
		(*Operation_Divide)(nil),
		(*Operation_Exp)(nil),
		(*Operation_Sin)(nil),
		(*Operation_Cos)(nil),
		(*Operation_Ln)(nil),
		(*Operation_Log)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_update_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_update_proto_goTypes,
		DependencyIndexes: file_update_proto_depIdxs,
		MessageInfos:      file_update_proto_msgTypes,
	}.Build()
	File_update_proto = out.File
	file_update_proto_rawDesc = nil
	file_update_proto_goTypes = nil
	file_update_proto_depIdxs = nil
}
