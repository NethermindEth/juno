// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.6
// 	protoc        (unknown)
// source: p2p/proto/sync/common.proto

package common

import (
	common "github.com/starknet-io/starknet-p2pspecs/p2p/proto/common"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Iteration_Direction int32

const (
	Iteration_Forward  Iteration_Direction = 0
	Iteration_Backward Iteration_Direction = 1
)

// Enum value maps for Iteration_Direction.
var (
	Iteration_Direction_name = map[int32]string{
		0: "Forward",
		1: "Backward",
	}
	Iteration_Direction_value = map[string]int32{
		"Forward":  0,
		"Backward": 1,
	}
)

func (x Iteration_Direction) Enum() *Iteration_Direction {
	p := new(Iteration_Direction)
	*p = x
	return p
}

func (x Iteration_Direction) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Iteration_Direction) Descriptor() protoreflect.EnumDescriptor {
	return file_p2p_proto_sync_common_proto_enumTypes[0].Descriptor()
}

func (Iteration_Direction) Type() protoreflect.EnumType {
	return &file_p2p_proto_sync_common_proto_enumTypes[0]
}

func (x Iteration_Direction) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Iteration_Direction.Descriptor instead.
func (Iteration_Direction) EnumDescriptor() ([]byte, []int) {
	return file_p2p_proto_sync_common_proto_rawDescGZIP(), []int{1, 0}
}

type StateDiffCommitment struct {
	state           protoimpl.MessageState `protogen:"open.v1"`
	StateDiffLength uint64                 `protobuf:"varint,1,opt,name=state_diff_length,json=stateDiffLength,proto3" json:"state_diff_length,omitempty"`
	Root            *common.Hash           `protobuf:"bytes,2,opt,name=root,proto3" json:"root,omitempty"`
	unknownFields   protoimpl.UnknownFields
	sizeCache       protoimpl.SizeCache
}

func (x *StateDiffCommitment) Reset() {
	*x = StateDiffCommitment{}
	mi := &file_p2p_proto_sync_common_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *StateDiffCommitment) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StateDiffCommitment) ProtoMessage() {}

func (x *StateDiffCommitment) ProtoReflect() protoreflect.Message {
	mi := &file_p2p_proto_sync_common_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StateDiffCommitment.ProtoReflect.Descriptor instead.
func (*StateDiffCommitment) Descriptor() ([]byte, []int) {
	return file_p2p_proto_sync_common_proto_rawDescGZIP(), []int{0}
}

func (x *StateDiffCommitment) GetStateDiffLength() uint64 {
	if x != nil {
		return x.StateDiffLength
	}
	return 0
}

func (x *StateDiffCommitment) GetRoot() *common.Hash {
	if x != nil {
		return x.Root
	}
	return nil
}

type Iteration struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Types that are valid to be assigned to Start:
	//
	//	*Iteration_BlockNumber
	//	*Iteration_Header
	Start         isIteration_Start   `protobuf_oneof:"start"`
	Direction     Iteration_Direction `protobuf:"varint,3,opt,name=direction,proto3,enum=Iteration_Direction" json:"direction,omitempty"`
	Limit         uint64              `protobuf:"varint,4,opt,name=limit,proto3" json:"limit,omitempty"`
	Step          uint64              `protobuf:"varint,5,opt,name=step,proto3" json:"step,omitempty"` // to allow interleaving from several nodes
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Iteration) Reset() {
	*x = Iteration{}
	mi := &file_p2p_proto_sync_common_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Iteration) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Iteration) ProtoMessage() {}

func (x *Iteration) ProtoReflect() protoreflect.Message {
	mi := &file_p2p_proto_sync_common_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Iteration.ProtoReflect.Descriptor instead.
func (*Iteration) Descriptor() ([]byte, []int) {
	return file_p2p_proto_sync_common_proto_rawDescGZIP(), []int{1}
}

func (x *Iteration) GetStart() isIteration_Start {
	if x != nil {
		return x.Start
	}
	return nil
}

func (x *Iteration) GetBlockNumber() uint64 {
	if x != nil {
		if x, ok := x.Start.(*Iteration_BlockNumber); ok {
			return x.BlockNumber
		}
	}
	return 0
}

func (x *Iteration) GetHeader() *common.Hash {
	if x != nil {
		if x, ok := x.Start.(*Iteration_Header); ok {
			return x.Header
		}
	}
	return nil
}

func (x *Iteration) GetDirection() Iteration_Direction {
	if x != nil {
		return x.Direction
	}
	return Iteration_Forward
}

func (x *Iteration) GetLimit() uint64 {
	if x != nil {
		return x.Limit
	}
	return 0
}

func (x *Iteration) GetStep() uint64 {
	if x != nil {
		return x.Step
	}
	return 0
}

type isIteration_Start interface {
	isIteration_Start()
}

type Iteration_BlockNumber struct {
	BlockNumber uint64 `protobuf:"varint,1,opt,name=block_number,json=blockNumber,proto3,oneof"`
}

type Iteration_Header struct {
	Header *common.Hash `protobuf:"bytes,2,opt,name=header,proto3,oneof"`
}

func (*Iteration_BlockNumber) isIteration_Start() {}

func (*Iteration_Header) isIteration_Start() {}

var File_p2p_proto_sync_common_proto protoreflect.FileDescriptor

const file_p2p_proto_sync_common_proto_rawDesc = "" +
	"\n" +
	"\x1bp2p/proto/sync/common.proto\x1a\x16p2p/proto/common.proto\"\\\n" +
	"\x13StateDiffCommitment\x12*\n" +
	"\x11state_diff_length\x18\x01 \x01(\x04R\x0fstateDiffLength\x12\x19\n" +
	"\x04root\x18\x02 \x01(\v2\x05.HashR\x04root\"\xe0\x01\n" +
	"\tIteration\x12#\n" +
	"\fblock_number\x18\x01 \x01(\x04H\x00R\vblockNumber\x12\x1f\n" +
	"\x06header\x18\x02 \x01(\v2\x05.HashH\x00R\x06header\x122\n" +
	"\tdirection\x18\x03 \x01(\x0e2\x14.Iteration.DirectionR\tdirection\x12\x14\n" +
	"\x05limit\x18\x04 \x01(\x04R\x05limit\x12\x12\n" +
	"\x04step\x18\x05 \x01(\x04R\x04step\"&\n" +
	"\tDirection\x12\v\n" +
	"\aForward\x10\x00\x12\f\n" +
	"\bBackward\x10\x01B\a\n" +
	"\x05startB@Z>github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/commonb\x06proto3"

var (
	file_p2p_proto_sync_common_proto_rawDescOnce sync.Once
	file_p2p_proto_sync_common_proto_rawDescData []byte
)

func file_p2p_proto_sync_common_proto_rawDescGZIP() []byte {
	file_p2p_proto_sync_common_proto_rawDescOnce.Do(func() {
		file_p2p_proto_sync_common_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_p2p_proto_sync_common_proto_rawDesc), len(file_p2p_proto_sync_common_proto_rawDesc)))
	})
	return file_p2p_proto_sync_common_proto_rawDescData
}

var file_p2p_proto_sync_common_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_p2p_proto_sync_common_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_p2p_proto_sync_common_proto_goTypes = []any{
	(Iteration_Direction)(0),    // 0: Iteration.Direction
	(*StateDiffCommitment)(nil), // 1: StateDiffCommitment
	(*Iteration)(nil),           // 2: Iteration
	(*common.Hash)(nil),         // 3: Hash
}
var file_p2p_proto_sync_common_proto_depIdxs = []int32{
	3, // 0: StateDiffCommitment.root:type_name -> Hash
	3, // 1: Iteration.header:type_name -> Hash
	0, // 2: Iteration.direction:type_name -> Iteration.Direction
	3, // [3:3] is the sub-list for method output_type
	3, // [3:3] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_p2p_proto_sync_common_proto_init() }
func file_p2p_proto_sync_common_proto_init() {
	if File_p2p_proto_sync_common_proto != nil {
		return
	}
	file_p2p_proto_sync_common_proto_msgTypes[1].OneofWrappers = []any{
		(*Iteration_BlockNumber)(nil),
		(*Iteration_Header)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_p2p_proto_sync_common_proto_rawDesc), len(file_p2p_proto_sync_common_proto_rawDesc)),
			NumEnums:      1,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_p2p_proto_sync_common_proto_goTypes,
		DependencyIndexes: file_p2p_proto_sync_common_proto_depIdxs,
		EnumInfos:         file_p2p_proto_sync_common_proto_enumTypes,
		MessageInfos:      file_p2p_proto_sync_common_proto_msgTypes,
	}.Build()
	File_p2p_proto_sync_common_proto = out.File
	file_p2p_proto_sync_common_proto_goTypes = nil
	file_p2p_proto_sync_common_proto_depIdxs = nil
}
