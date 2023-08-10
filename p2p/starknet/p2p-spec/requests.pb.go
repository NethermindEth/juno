// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.31.0
// 	protoc        v3.21.12
// source: p2p/proto/requests.proto

package p2p_spec

import (
	reflect "reflect"
	sync "sync"

	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Request struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Types that are assignable to Req:
	//
	//	*Request_GetBlocks
	//	*Request_GetSignatures
	//	*Request_GetEvents
	//	*Request_GetReceipts
	//	*Request_GetTransactions
	Req isRequest_Req `protobuf_oneof:"req"`
}

func (x *Request) Reset() {
	*x = Request{}
	if protoimpl.UnsafeEnabled {
		mi := &file_p2p_proto_requests_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Request) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Request) ProtoMessage() {}

func (x *Request) ProtoReflect() protoreflect.Message {
	mi := &file_p2p_proto_requests_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Request.ProtoReflect.Descriptor instead.
func (*Request) Descriptor() ([]byte, []int) {
	return file_p2p_proto_requests_proto_rawDescGZIP(), []int{0}
}

func (m *Request) GetReq() isRequest_Req {
	if m != nil {
		return m.Req
	}
	return nil
}

func (x *Request) GetGetBlocks() *GetBlocks {
	if x, ok := x.GetReq().(*Request_GetBlocks); ok {
		return x.GetBlocks
	}
	return nil
}

func (x *Request) GetGetSignatures() *GetSignatures {
	if x, ok := x.GetReq().(*Request_GetSignatures); ok {
		return x.GetSignatures
	}
	return nil
}

func (x *Request) GetGetEvents() *GetEvents {
	if x, ok := x.GetReq().(*Request_GetEvents); ok {
		return x.GetEvents
	}
	return nil
}

func (x *Request) GetGetReceipts() *GetReceipts {
	if x, ok := x.GetReq().(*Request_GetReceipts); ok {
		return x.GetReceipts
	}
	return nil
}

func (x *Request) GetGetTransactions() *GetTransactions {
	if x, ok := x.GetReq().(*Request_GetTransactions); ok {
		return x.GetTransactions
	}
	return nil
}

type isRequest_Req interface {
	isRequest_Req()
}

type Request_GetBlocks struct {
	GetBlocks *GetBlocks `protobuf:"bytes,1,opt,name=get_blocks,json=getBlocks,proto3,oneof"`
}

type Request_GetSignatures struct {
	GetSignatures *GetSignatures `protobuf:"bytes,2,opt,name=get_signatures,json=getSignatures,proto3,oneof"`
}

type Request_GetEvents struct {
	GetEvents *GetEvents `protobuf:"bytes,4,opt,name=get_events,json=getEvents,proto3,oneof"`
}

type Request_GetReceipts struct {
	GetReceipts *GetReceipts `protobuf:"bytes,5,opt,name=get_receipts,json=getReceipts,proto3,oneof"`
}

type Request_GetTransactions struct {
	GetTransactions *GetTransactions `protobuf:"bytes,6,opt,name=get_transactions,json=getTransactions,proto3,oneof"`
}

func (*Request_GetBlocks) isRequest_Req() {}

func (*Request_GetSignatures) isRequest_Req() {}

func (*Request_GetEvents) isRequest_Req() {}

func (*Request_GetReceipts) isRequest_Req() {}

func (*Request_GetTransactions) isRequest_Req() {}

type GetBlocksResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Blocks []*HeaderAndStateDiff `protobuf:"bytes,1,rep,name=blocks,proto3" json:"blocks,omitempty"`
}

func (x *GetBlocksResponse) Reset() {
	*x = GetBlocksResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_p2p_proto_requests_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetBlocksResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetBlocksResponse) ProtoMessage() {}

func (x *GetBlocksResponse) ProtoReflect() protoreflect.Message {
	mi := &file_p2p_proto_requests_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetBlocksResponse.ProtoReflect.Descriptor instead.
func (*GetBlocksResponse) Descriptor() ([]byte, []int) {
	return file_p2p_proto_requests_proto_rawDescGZIP(), []int{1}
}

func (x *GetBlocksResponse) GetBlocks() []*HeaderAndStateDiff {
	if x != nil {
		return x.Blocks
	}
	return nil
}

type HeaderAndStateDiff struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Header    *BlockHeader `protobuf:"bytes,1,opt,name=header,proto3" json:"header,omitempty"`
	StateDiff *StateDiff   `protobuf:"bytes,2,opt,name=state_diff,json=stateDiff,proto3" json:"state_diff,omitempty"`
}

func (x *HeaderAndStateDiff) Reset() {
	*x = HeaderAndStateDiff{}
	if protoimpl.UnsafeEnabled {
		mi := &file_p2p_proto_requests_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *HeaderAndStateDiff) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*HeaderAndStateDiff) ProtoMessage() {}

func (x *HeaderAndStateDiff) ProtoReflect() protoreflect.Message {
	mi := &file_p2p_proto_requests_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use HeaderAndStateDiff.ProtoReflect.Descriptor instead.
func (*HeaderAndStateDiff) Descriptor() ([]byte, []int) {
	return file_p2p_proto_requests_proto_rawDescGZIP(), []int{2}
}

func (x *HeaderAndStateDiff) GetHeader() *BlockHeader {
	if x != nil {
		return x.Header
	}
	return nil
}

func (x *HeaderAndStateDiff) GetStateDiff() *StateDiff {
	if x != nil {
		return x.StateDiff
	}
	return nil
}

var File_p2p_proto_requests_proto protoreflect.FileDescriptor

var file_p2p_proto_requests_proto_rawDesc = []byte{
	0x0a, 0x18, 0x70, 0x32, 0x70, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x72, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x15, 0x70, 0x32, 0x70, 0x2f,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x1a, 0x15, 0x70, 0x32, 0x70, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x65, 0x76, 0x65,
	0x6e, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x17, 0x70, 0x32, 0x70, 0x2f, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x2f, 0x72, 0x65, 0x63, 0x65, 0x69, 0x70, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x1a, 0x1b, 0x70, 0x32, 0x70, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x74, 0x72, 0x61,
	0x6e, 0x73, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x15,
	0x70, 0x32, 0x70, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x73, 0x74, 0x61, 0x74, 0x65, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x95, 0x02, 0x0a, 0x07, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x12, 0x2b, 0x0a, 0x0a, 0x67, 0x65, 0x74, 0x5f, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0a, 0x2e, 0x47, 0x65, 0x74, 0x42, 0x6c, 0x6f, 0x63, 0x6b,
	0x73, 0x48, 0x00, 0x52, 0x09, 0x67, 0x65, 0x74, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x12, 0x37,
	0x0a, 0x0e, 0x67, 0x65, 0x74, 0x5f, 0x73, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x73,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0e, 0x2e, 0x47, 0x65, 0x74, 0x53, 0x69, 0x67, 0x6e,
	0x61, 0x74, 0x75, 0x72, 0x65, 0x73, 0x48, 0x00, 0x52, 0x0d, 0x67, 0x65, 0x74, 0x53, 0x69, 0x67,
	0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x73, 0x12, 0x2b, 0x0a, 0x0a, 0x67, 0x65, 0x74, 0x5f, 0x65,
	0x76, 0x65, 0x6e, 0x74, 0x73, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0a, 0x2e, 0x47, 0x65,
	0x74, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x73, 0x48, 0x00, 0x52, 0x09, 0x67, 0x65, 0x74, 0x45, 0x76,
	0x65, 0x6e, 0x74, 0x73, 0x12, 0x31, 0x0a, 0x0c, 0x67, 0x65, 0x74, 0x5f, 0x72, 0x65, 0x63, 0x65,
	0x69, 0x70, 0x74, 0x73, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0c, 0x2e, 0x47, 0x65, 0x74,
	0x52, 0x65, 0x63, 0x65, 0x69, 0x70, 0x74, 0x73, 0x48, 0x00, 0x52, 0x0b, 0x67, 0x65, 0x74, 0x52,
	0x65, 0x63, 0x65, 0x69, 0x70, 0x74, 0x73, 0x12, 0x3d, 0x0a, 0x10, 0x67, 0x65, 0x74, 0x5f, 0x74,
	0x72, 0x61, 0x6e, 0x73, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x06, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x10, 0x2e, 0x47, 0x65, 0x74, 0x54, 0x72, 0x61, 0x6e, 0x73, 0x61, 0x63, 0x74, 0x69,
	0x6f, 0x6e, 0x73, 0x48, 0x00, 0x52, 0x0f, 0x67, 0x65, 0x74, 0x54, 0x72, 0x61, 0x6e, 0x73, 0x61,
	0x63, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x42, 0x05, 0x0a, 0x03, 0x72, 0x65, 0x71, 0x22, 0x40, 0x0a,
	0x11, 0x47, 0x65, 0x74, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e,
	0x73, 0x65, 0x12, 0x2b, 0x0a, 0x06, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x18, 0x01, 0x20, 0x03,
	0x28, 0x0b, 0x32, 0x13, 0x2e, 0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x41, 0x6e, 0x64, 0x53, 0x74,
	0x61, 0x74, 0x65, 0x44, 0x69, 0x66, 0x66, 0x52, 0x06, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x22,
	0x65, 0x0a, 0x12, 0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x41, 0x6e, 0x64, 0x53, 0x74, 0x61, 0x74,
	0x65, 0x44, 0x69, 0x66, 0x66, 0x12, 0x24, 0x0a, 0x06, 0x68, 0x65, 0x61, 0x64, 0x65, 0x72, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0c, 0x2e, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x48, 0x65, 0x61,
	0x64, 0x65, 0x72, 0x52, 0x06, 0x68, 0x65, 0x61, 0x64, 0x65, 0x72, 0x12, 0x29, 0x0a, 0x0a, 0x73,
	0x74, 0x61, 0x74, 0x65, 0x5f, 0x64, 0x69, 0x66, 0x66, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x0a, 0x2e, 0x53, 0x74, 0x61, 0x74, 0x65, 0x44, 0x69, 0x66, 0x66, 0x52, 0x09, 0x73, 0x74, 0x61,
	0x74, 0x65, 0x44, 0x69, 0x66, 0x66, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_p2p_proto_requests_proto_rawDescOnce sync.Once
	file_p2p_proto_requests_proto_rawDescData = file_p2p_proto_requests_proto_rawDesc
)

func file_p2p_proto_requests_proto_rawDescGZIP() []byte {
	file_p2p_proto_requests_proto_rawDescOnce.Do(func() {
		file_p2p_proto_requests_proto_rawDescData = protoimpl.X.CompressGZIP(file_p2p_proto_requests_proto_rawDescData)
	})
	return file_p2p_proto_requests_proto_rawDescData
}

var (
	file_p2p_proto_requests_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
	file_p2p_proto_requests_proto_goTypes  = []interface{}{
		(*Request)(nil),            // 0: Request
		(*GetBlocksResponse)(nil),  // 1: GetBlocksResponse
		(*HeaderAndStateDiff)(nil), // 2: HeaderAndStateDiff
		(*GetBlocks)(nil),          // 3: GetBlocks
		(*GetSignatures)(nil),      // 4: GetSignatures
		(*GetEvents)(nil),          // 5: GetEvents
		(*GetReceipts)(nil),        // 6: GetReceipts
		(*GetTransactions)(nil),    // 7: GetTransactions
		(*BlockHeader)(nil),        // 8: BlockHeader
		(*StateDiff)(nil),          // 9: StateDiff
	}
)
var file_p2p_proto_requests_proto_depIdxs = []int32{
	3, // 0: Request.get_blocks:type_name -> GetBlocks
	4, // 1: Request.get_signatures:type_name -> GetSignatures
	5, // 2: Request.get_events:type_name -> GetEvents
	6, // 3: Request.get_receipts:type_name -> GetReceipts
	7, // 4: Request.get_transactions:type_name -> GetTransactions
	2, // 5: GetBlocksResponse.blocks:type_name -> HeaderAndStateDiff
	8, // 6: HeaderAndStateDiff.header:type_name -> BlockHeader
	9, // 7: HeaderAndStateDiff.state_diff:type_name -> StateDiff
	8, // [8:8] is the sub-list for method output_type
	8, // [8:8] is the sub-list for method input_type
	8, // [8:8] is the sub-list for extension type_name
	8, // [8:8] is the sub-list for extension extendee
	0, // [0:8] is the sub-list for field type_name
}

func init() { file_p2p_proto_requests_proto_init() }
func file_p2p_proto_requests_proto_init() {
	if File_p2p_proto_requests_proto != nil {
		return
	}
	file_p2p_proto_block_proto_init()
	file_p2p_proto_event_proto_init()
	file_p2p_proto_receipt_proto_init()
	file_p2p_proto_transaction_proto_init()
	file_p2p_proto_state_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_p2p_proto_requests_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Request); i {
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
		file_p2p_proto_requests_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetBlocksResponse); i {
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
		file_p2p_proto_requests_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*HeaderAndStateDiff); i {
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
	file_p2p_proto_requests_proto_msgTypes[0].OneofWrappers = []interface{}{
		(*Request_GetBlocks)(nil),
		(*Request_GetSignatures)(nil),
		(*Request_GetEvents)(nil),
		(*Request_GetReceipts)(nil),
		(*Request_GetTransactions)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_p2p_proto_requests_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_p2p_proto_requests_proto_goTypes,
		DependencyIndexes: file_p2p_proto_requests_proto_depIdxs,
		MessageInfos:      file_p2p_proto_requests_proto_msgTypes,
	}.Build()
	File_p2p_proto_requests_proto = out.File
	file_p2p_proto_requests_proto_rawDesc = nil
	file_p2p_proto_requests_proto_goTypes = nil
	file_p2p_proto_requests_proto_depIdxs = nil
}
