// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.0
// 	protoc        v3.19.4
// source: block.proto

package block

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Block struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Hash             []byte   `protobuf:"bytes,1,opt,name=hash,proto3" json:"hash,omitempty"`
	BlockNumber      uint64   `protobuf:"varint,2,opt,name=blockNumber,proto3" json:"blockNumber,omitempty"`
	ParentBlockHash  []byte   `protobuf:"bytes,3,opt,name=parentBlockHash,proto3" json:"parentBlockHash,omitempty"`
	Status           string   `protobuf:"bytes,4,opt,name=status,proto3" json:"status,omitempty"`
	SequencerAddress []byte   `protobuf:"bytes,5,opt,name=sequencerAddress,proto3" json:"sequencerAddress,omitempty"`
	GlobalStateRoot  []byte   `protobuf:"bytes,6,opt,name=globalStateRoot,proto3" json:"globalStateRoot,omitempty"`
	OldRoot          []byte   `protobuf:"bytes,7,opt,name=oldRoot,proto3" json:"oldRoot,omitempty"`
	AcceptedTime     int64    `protobuf:"varint,8,opt,name=acceptedTime,proto3" json:"acceptedTime,omitempty"`
	TimeStamp        int64    `protobuf:"varint,9,opt,name=timeStamp,proto3" json:"timeStamp,omitempty"`
	TxCount          uint64   `protobuf:"varint,10,opt,name=txCount,proto3" json:"txCount,omitempty"`
	TxCommitment     []byte   `protobuf:"bytes,11,opt,name=txCommitment,proto3" json:"txCommitment,omitempty"`
	EventCount       uint64   `protobuf:"varint,12,opt,name=eventCount,proto3" json:"eventCount,omitempty"`
	EventCommitment  []byte   `protobuf:"bytes,13,opt,name=eventCommitment,proto3" json:"eventCommitment,omitempty"`
	TxHashes         [][]byte `protobuf:"bytes,14,rep,name=TxHashes,proto3" json:"TxHashes,omitempty"`
}

// notest
func (x *Block) Reset() {
	*x = Block{}
	if protoimpl.UnsafeEnabled {
		mi := &file_block_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

// notest
func (x *Block) String() string {
	return protoimpl.X.MessageStringOf(x)
}

// notest
func (*Block) ProtoMessage() {}

// notest
func (x *Block) ProtoReflect() protoreflect.Message {
	mi := &file_block_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Block.ProtoReflect.Descriptor instead.
// notest
func (*Block) Descriptor() ([]byte, []int) {
	return file_block_proto_rawDescGZIP(), []int{0}
}

// notest
func (x *Block) GetHash() []byte {
	if x != nil {
		return x.Hash
	}
	return nil
}

// notest
func (x *Block) GetBlockNumber() uint64 {
	if x != nil {
		return x.BlockNumber
	}
	return 0
}

// notest
func (x *Block) GetParentBlockHash() []byte {
	if x != nil {
		return x.ParentBlockHash
	}
	return nil
}

// notest
func (x *Block) GetStatus() string {
	if x != nil {
		return x.Status
	}
	return ""
}

// notest
func (x *Block) GetSequencerAddress() []byte {
	if x != nil {
		return x.SequencerAddress
	}
	return nil
}

// notest
func (x *Block) GetGlobalStateRoot() []byte {
	if x != nil {
		return x.GlobalStateRoot
	}
	return nil
}

// notest
func (x *Block) GetOldRoot() []byte {
	if x != nil {
		return x.OldRoot
	}
	return nil
}

// notest
func (x *Block) GetAcceptedTime() int64 {
	if x != nil {
		return x.AcceptedTime
	}
	return 0
}

// notest
func (x *Block) GetTimeStamp() int64 {
	if x != nil {
		return x.TimeStamp
	}
	return 0
}

// notest
func (x *Block) GetTxCount() uint64 {
	if x != nil {
		return x.TxCount
	}
	return 0
}

// notest
func (x *Block) GetTxCommitment() []byte {
	if x != nil {
		return x.TxCommitment
	}
	return nil
}

// notest
func (x *Block) GetEventCount() uint64 {
	if x != nil {
		return x.EventCount
	}
	return 0
}

// notest
func (x *Block) GetEventCommitment() []byte {
	if x != nil {
		return x.EventCommitment
	}
	return nil
}

// notest
func (x *Block) GetTxHashes() [][]byte {
	if x != nil {
		return x.TxHashes
	}
	return nil
}

var File_block_proto protoreflect.FileDescriptor

var file_block_proto_rawDesc = []byte{
	0x0a, 0x0b, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xd5, 0x03,
	0x0a, 0x05, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x12, 0x12, 0x0a, 0x04, 0x68, 0x61, 0x73, 0x68, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x04, 0x68, 0x61, 0x73, 0x68, 0x12, 0x20, 0x0a, 0x0b, 0x62,
	0x6c, 0x6f, 0x63, 0x6b, 0x4e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x04,
	0x52, 0x0b, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x4e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x12, 0x28, 0x0a,
	0x0f, 0x70, 0x61, 0x72, 0x65, 0x6e, 0x74, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x48, 0x61, 0x73, 0x68,
	0x18, 0x03, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x0f, 0x70, 0x61, 0x72, 0x65, 0x6e, 0x74, 0x42, 0x6c,
	0x6f, 0x63, 0x6b, 0x48, 0x61, 0x73, 0x68, 0x12, 0x16, 0x0a, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75,
	0x73, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12,
	0x2a, 0x0a, 0x10, 0x73, 0x65, 0x71, 0x75, 0x65, 0x6e, 0x63, 0x65, 0x72, 0x41, 0x64, 0x64, 0x72,
	0x65, 0x73, 0x73, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x10, 0x73, 0x65, 0x71, 0x75, 0x65,
	0x6e, 0x63, 0x65, 0x72, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x12, 0x28, 0x0a, 0x0f, 0x67,
	0x6c, 0x6f, 0x62, 0x61, 0x6c, 0x53, 0x74, 0x61, 0x74, 0x65, 0x52, 0x6f, 0x6f, 0x74, 0x18, 0x06,
	0x20, 0x01, 0x28, 0x0c, 0x52, 0x0f, 0x67, 0x6c, 0x6f, 0x62, 0x61, 0x6c, 0x53, 0x74, 0x61, 0x74,
	0x65, 0x52, 0x6f, 0x6f, 0x74, 0x12, 0x18, 0x0a, 0x07, 0x6f, 0x6c, 0x64, 0x52, 0x6f, 0x6f, 0x74,
	0x18, 0x07, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x07, 0x6f, 0x6c, 0x64, 0x52, 0x6f, 0x6f, 0x74, 0x12,
	0x22, 0x0a, 0x0c, 0x61, 0x63, 0x63, 0x65, 0x70, 0x74, 0x65, 0x64, 0x54, 0x69, 0x6d, 0x65, 0x18,
	0x08, 0x20, 0x01, 0x28, 0x03, 0x52, 0x0c, 0x61, 0x63, 0x63, 0x65, 0x70, 0x74, 0x65, 0x64, 0x54,
	0x69, 0x6d, 0x65, 0x12, 0x1c, 0x0a, 0x09, 0x74, 0x69, 0x6d, 0x65, 0x53, 0x74, 0x61, 0x6d, 0x70,
	0x18, 0x09, 0x20, 0x01, 0x28, 0x03, 0x52, 0x09, 0x74, 0x69, 0x6d, 0x65, 0x53, 0x74, 0x61, 0x6d,
	0x70, 0x12, 0x18, 0x0a, 0x07, 0x74, 0x78, 0x43, 0x6f, 0x75, 0x6e, 0x74, 0x18, 0x0a, 0x20, 0x01,
	0x28, 0x04, 0x52, 0x07, 0x74, 0x78, 0x43, 0x6f, 0x75, 0x6e, 0x74, 0x12, 0x22, 0x0a, 0x0c, 0x74,
	0x78, 0x43, 0x6f, 0x6d, 0x6d, 0x69, 0x74, 0x6d, 0x65, 0x6e, 0x74, 0x18, 0x0b, 0x20, 0x01, 0x28,
	0x0c, 0x52, 0x0c, 0x74, 0x78, 0x43, 0x6f, 0x6d, 0x6d, 0x69, 0x74, 0x6d, 0x65, 0x6e, 0x74, 0x12,
	0x1e, 0x0a, 0x0a, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x43, 0x6f, 0x75, 0x6e, 0x74, 0x18, 0x0c, 0x20,
	0x01, 0x28, 0x04, 0x52, 0x0a, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x43, 0x6f, 0x75, 0x6e, 0x74, 0x12,
	0x28, 0x0a, 0x0f, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x43, 0x6f, 0x6d, 0x6d, 0x69, 0x74, 0x6d, 0x65,
	0x6e, 0x74, 0x18, 0x0d, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x0f, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x43,
	0x6f, 0x6d, 0x6d, 0x69, 0x74, 0x6d, 0x65, 0x6e, 0x74, 0x12, 0x1a, 0x0a, 0x08, 0x54, 0x78, 0x48,
	0x61, 0x73, 0x68, 0x65, 0x73, 0x18, 0x0e, 0x20, 0x03, 0x28, 0x0c, 0x52, 0x08, 0x54, 0x78, 0x48,
	0x61, 0x73, 0x68, 0x65, 0x73, 0x42, 0x2e, 0x5a, 0x2c, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e,
	0x63, 0x6f, 0x6d, 0x2f, 0x4e, 0x65, 0x74, 0x68, 0x65, 0x72, 0x6d, 0x69, 0x6e, 0x64, 0x45, 0x74,
	0x68, 0x2f, 0x6a, 0x75, 0x6e, 0x6f, 0x2f, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f,
	0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_block_proto_rawDescOnce sync.Once
	file_block_proto_rawDescData = file_block_proto_rawDesc
)

// notest
func file_block_proto_rawDescGZIP() []byte {
// notest
	file_block_proto_rawDescOnce.Do(func() {
		file_block_proto_rawDescData = protoimpl.X.CompressGZIP(file_block_proto_rawDescData)
	})
	return file_block_proto_rawDescData
}

var file_block_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_block_proto_goTypes = []interface{}{
	(*Block)(nil), // 0: Block
}
var file_block_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

// notest
func init() { file_block_proto_init() }
// notest
func file_block_proto_init() {
	if File_block_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
// notest
		file_block_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Block); i {
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
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_block_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_block_proto_goTypes,
		DependencyIndexes: file_block_proto_depIdxs,
		MessageInfos:      file_block_proto_msgTypes,
	}.Build()
	File_block_proto = out.File
	file_block_proto_rawDesc = nil
	file_block_proto_goTypes = nil
	file_block_proto_depIdxs = nil
}
