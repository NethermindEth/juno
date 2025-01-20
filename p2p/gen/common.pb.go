// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.3
// 	protoc        (unknown)
// source: common.proto

package gen

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

type L1DataAvailabilityMode int32

const (
	L1DataAvailabilityMode_Calldata L1DataAvailabilityMode = 0
	L1DataAvailabilityMode_Blob     L1DataAvailabilityMode = 1
)

// Enum value maps for L1DataAvailabilityMode.
var (
	L1DataAvailabilityMode_name = map[int32]string{
		0: "Calldata",
		1: "Blob",
	}
	L1DataAvailabilityMode_value = map[string]int32{
		"Calldata": 0,
		"Blob":     1,
	}
)

func (x L1DataAvailabilityMode) Enum() *L1DataAvailabilityMode {
	p := new(L1DataAvailabilityMode)
	*p = x
	return p
}

func (x L1DataAvailabilityMode) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (L1DataAvailabilityMode) Descriptor() protoreflect.EnumDescriptor {
	return file_common_proto_enumTypes[0].Descriptor()
}

func (L1DataAvailabilityMode) Type() protoreflect.EnumType {
	return &file_common_proto_enumTypes[0]
}

func (x L1DataAvailabilityMode) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use L1DataAvailabilityMode.Descriptor instead.
func (L1DataAvailabilityMode) EnumDescriptor() ([]byte, []int) {
	return file_common_proto_rawDescGZIP(), []int{0}
}

type VolitionDomain int32

const (
	VolitionDomain_L1 VolitionDomain = 0
	VolitionDomain_L2 VolitionDomain = 1
)

// Enum value maps for VolitionDomain.
var (
	VolitionDomain_name = map[int32]string{
		0: "L1",
		1: "L2",
	}
	VolitionDomain_value = map[string]int32{
		"L1": 0,
		"L2": 1,
	}
)

func (x VolitionDomain) Enum() *VolitionDomain {
	p := new(VolitionDomain)
	*p = x
	return p
}

func (x VolitionDomain) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (VolitionDomain) Descriptor() protoreflect.EnumDescriptor {
	return file_common_proto_enumTypes[1].Descriptor()
}

func (VolitionDomain) Type() protoreflect.EnumType {
	return &file_common_proto_enumTypes[1]
}

func (x VolitionDomain) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use VolitionDomain.Descriptor instead.
func (VolitionDomain) EnumDescriptor() ([]byte, []int) {
	return file_common_proto_rawDescGZIP(), []int{1}
}

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
	return file_common_proto_enumTypes[2].Descriptor()
}

func (Iteration_Direction) Type() protoreflect.EnumType {
	return &file_common_proto_enumTypes[2]
}

func (x Iteration_Direction) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Iteration_Direction.Descriptor instead.
func (Iteration_Direction) EnumDescriptor() ([]byte, []int) {
	return file_common_proto_rawDescGZIP(), []int{11, 0}
}

type Felt252 struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Elements      []byte                 `protobuf:"bytes,1,opt,name=elements,proto3" json:"elements,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Felt252) Reset() {
	*x = Felt252{}
	mi := &file_common_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Felt252) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Felt252) ProtoMessage() {}

func (x *Felt252) ProtoReflect() protoreflect.Message {
	mi := &file_common_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Felt252.ProtoReflect.Descriptor instead.
func (*Felt252) Descriptor() ([]byte, []int) {
	return file_common_proto_rawDescGZIP(), []int{0}
}

func (x *Felt252) GetElements() []byte {
	if x != nil {
		return x.Elements
	}
	return nil
}

// A hash value representable as a Felt252
type Hash struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Elements      []byte                 `protobuf:"bytes,1,opt,name=elements,proto3" json:"elements,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Hash) Reset() {
	*x = Hash{}
	mi := &file_common_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Hash) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Hash) ProtoMessage() {}

func (x *Hash) ProtoReflect() protoreflect.Message {
	mi := &file_common_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Hash.ProtoReflect.Descriptor instead.
func (*Hash) Descriptor() ([]byte, []int) {
	return file_common_proto_rawDescGZIP(), []int{1}
}

func (x *Hash) GetElements() []byte {
	if x != nil {
		return x.Elements
	}
	return nil
}

// A 256 bit hash value (like Keccak256)
type Hash256 struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Required to be 32 bytes long
	Elements      []byte `protobuf:"bytes,1,opt,name=elements,proto3" json:"elements,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Hash256) Reset() {
	*x = Hash256{}
	mi := &file_common_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Hash256) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Hash256) ProtoMessage() {}

func (x *Hash256) ProtoReflect() protoreflect.Message {
	mi := &file_common_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Hash256.ProtoReflect.Descriptor instead.
func (*Hash256) Descriptor() ([]byte, []int) {
	return file_common_proto_rawDescGZIP(), []int{2}
}

func (x *Hash256) GetElements() []byte {
	if x != nil {
		return x.Elements
	}
	return nil
}

type Hashes struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Items         []*Hash                `protobuf:"bytes,1,rep,name=items,proto3" json:"items,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Hashes) Reset() {
	*x = Hashes{}
	mi := &file_common_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Hashes) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Hashes) ProtoMessage() {}

func (x *Hashes) ProtoReflect() protoreflect.Message {
	mi := &file_common_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Hashes.ProtoReflect.Descriptor instead.
func (*Hashes) Descriptor() ([]byte, []int) {
	return file_common_proto_rawDescGZIP(), []int{3}
}

func (x *Hashes) GetItems() []*Hash {
	if x != nil {
		return x.Items
	}
	return nil
}

type Address struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Elements      []byte                 `protobuf:"bytes,1,opt,name=elements,proto3" json:"elements,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Address) Reset() {
	*x = Address{}
	mi := &file_common_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Address) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Address) ProtoMessage() {}

func (x *Address) ProtoReflect() protoreflect.Message {
	mi := &file_common_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Address.ProtoReflect.Descriptor instead.
func (*Address) Descriptor() ([]byte, []int) {
	return file_common_proto_rawDescGZIP(), []int{4}
}

func (x *Address) GetElements() []byte {
	if x != nil {
		return x.Elements
	}
	return nil
}

type PeerID struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Id            []byte                 `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *PeerID) Reset() {
	*x = PeerID{}
	mi := &file_common_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *PeerID) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PeerID) ProtoMessage() {}

func (x *PeerID) ProtoReflect() protoreflect.Message {
	mi := &file_common_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PeerID.ProtoReflect.Descriptor instead.
func (*PeerID) Descriptor() ([]byte, []int) {
	return file_common_proto_rawDescGZIP(), []int{5}
}

func (x *PeerID) GetId() []byte {
	if x != nil {
		return x.Id
	}
	return nil
}

type Uint128 struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Low           uint64                 `protobuf:"varint,1,opt,name=low,proto3" json:"low,omitempty"`
	High          uint64                 `protobuf:"varint,2,opt,name=high,proto3" json:"high,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Uint128) Reset() {
	*x = Uint128{}
	mi := &file_common_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Uint128) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Uint128) ProtoMessage() {}

func (x *Uint128) ProtoReflect() protoreflect.Message {
	mi := &file_common_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Uint128.ProtoReflect.Descriptor instead.
func (*Uint128) Descriptor() ([]byte, []int) {
	return file_common_proto_rawDescGZIP(), []int{6}
}

func (x *Uint128) GetLow() uint64 {
	if x != nil {
		return x.Low
	}
	return 0
}

func (x *Uint128) GetHigh() uint64 {
	if x != nil {
		return x.High
	}
	return 0
}

type ConsensusSignature struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	R             *Felt252               `protobuf:"bytes,1,opt,name=r,proto3" json:"r,omitempty"`
	S             *Felt252               `protobuf:"bytes,2,opt,name=s,proto3" json:"s,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ConsensusSignature) Reset() {
	*x = ConsensusSignature{}
	mi := &file_common_proto_msgTypes[7]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ConsensusSignature) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ConsensusSignature) ProtoMessage() {}

func (x *ConsensusSignature) ProtoReflect() protoreflect.Message {
	mi := &file_common_proto_msgTypes[7]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ConsensusSignature.ProtoReflect.Descriptor instead.
func (*ConsensusSignature) Descriptor() ([]byte, []int) {
	return file_common_proto_rawDescGZIP(), []int{7}
}

func (x *ConsensusSignature) GetR() *Felt252 {
	if x != nil {
		return x.R
	}
	return nil
}

func (x *ConsensusSignature) GetS() *Felt252 {
	if x != nil {
		return x.S
	}
	return nil
}

type Patricia struct {
	state   protoimpl.MessageState `protogen:"open.v1"`
	NLeaves uint64                 `protobuf:"varint,1,opt,name=n_leaves,json=nLeaves,proto3" json:"n_leaves,omitempty"` // needed to know the height, so as to how many nodes to expect in a proof.
	// and also when receiving all leaves, how many to expect
	Root          *Hash `protobuf:"bytes,2,opt,name=root,proto3" json:"root,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Patricia) Reset() {
	*x = Patricia{}
	mi := &file_common_proto_msgTypes[8]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Patricia) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Patricia) ProtoMessage() {}

func (x *Patricia) ProtoReflect() protoreflect.Message {
	mi := &file_common_proto_msgTypes[8]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Patricia.ProtoReflect.Descriptor instead.
func (*Patricia) Descriptor() ([]byte, []int) {
	return file_common_proto_rawDescGZIP(), []int{8}
}

func (x *Patricia) GetNLeaves() uint64 {
	if x != nil {
		return x.NLeaves
	}
	return 0
}

func (x *Patricia) GetRoot() *Hash {
	if x != nil {
		return x.Root
	}
	return nil
}

type StateDiffCommitment struct {
	state           protoimpl.MessageState `protogen:"open.v1"`
	StateDiffLength uint64                 `protobuf:"varint,1,opt,name=state_diff_length,json=stateDiffLength,proto3" json:"state_diff_length,omitempty"`
	Root            *Hash                  `protobuf:"bytes,2,opt,name=root,proto3" json:"root,omitempty"`
	unknownFields   protoimpl.UnknownFields
	sizeCache       protoimpl.SizeCache
}

func (x *StateDiffCommitment) Reset() {
	*x = StateDiffCommitment{}
	mi := &file_common_proto_msgTypes[9]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *StateDiffCommitment) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StateDiffCommitment) ProtoMessage() {}

func (x *StateDiffCommitment) ProtoReflect() protoreflect.Message {
	mi := &file_common_proto_msgTypes[9]
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
	return file_common_proto_rawDescGZIP(), []int{9}
}

func (x *StateDiffCommitment) GetStateDiffLength() uint64 {
	if x != nil {
		return x.StateDiffLength
	}
	return 0
}

func (x *StateDiffCommitment) GetRoot() *Hash {
	if x != nil {
		return x.Root
	}
	return nil
}

type BlockID struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Number        uint64                 `protobuf:"varint,1,opt,name=number,proto3" json:"number,omitempty"`
	Header        *Hash                  `protobuf:"bytes,2,opt,name=header,proto3" json:"header,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *BlockID) Reset() {
	*x = BlockID{}
	mi := &file_common_proto_msgTypes[10]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *BlockID) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BlockID) ProtoMessage() {}

func (x *BlockID) ProtoReflect() protoreflect.Message {
	mi := &file_common_proto_msgTypes[10]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BlockID.ProtoReflect.Descriptor instead.
func (*BlockID) Descriptor() ([]byte, []int) {
	return file_common_proto_rawDescGZIP(), []int{10}
}

func (x *BlockID) GetNumber() uint64 {
	if x != nil {
		return x.Number
	}
	return 0
}

func (x *BlockID) GetHeader() *Hash {
	if x != nil {
		return x.Header
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
	mi := &file_common_proto_msgTypes[11]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Iteration) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Iteration) ProtoMessage() {}

func (x *Iteration) ProtoReflect() protoreflect.Message {
	mi := &file_common_proto_msgTypes[11]
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
	return file_common_proto_rawDescGZIP(), []int{11}
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

func (x *Iteration) GetHeader() *Hash {
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
	Header *Hash `protobuf:"bytes,2,opt,name=header,proto3,oneof"`
}

func (*Iteration_BlockNumber) isIteration_Start() {}

func (*Iteration_Header) isIteration_Start() {}

// mark the end of a stream of messages
// TBD: may not be required if we open a stream per request.
type Fin struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Fin) Reset() {
	*x = Fin{}
	mi := &file_common_proto_msgTypes[12]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Fin) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Fin) ProtoMessage() {}

func (x *Fin) ProtoReflect() protoreflect.Message {
	mi := &file_common_proto_msgTypes[12]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Fin.ProtoReflect.Descriptor instead.
func (*Fin) Descriptor() ([]byte, []int) {
	return file_common_proto_rawDescGZIP(), []int{12}
}

var File_common_proto protoreflect.FileDescriptor

var file_common_proto_rawDesc = []byte{
	0x0a, 0x0c, 0x63, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x25,
	0x0a, 0x07, 0x46, 0x65, 0x6c, 0x74, 0x32, 0x35, 0x32, 0x12, 0x1a, 0x0a, 0x08, 0x65, 0x6c, 0x65,
	0x6d, 0x65, 0x6e, 0x74, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x08, 0x65, 0x6c, 0x65,
	0x6d, 0x65, 0x6e, 0x74, 0x73, 0x22, 0x22, 0x0a, 0x04, 0x48, 0x61, 0x73, 0x68, 0x12, 0x1a, 0x0a,
	0x08, 0x65, 0x6c, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52,
	0x08, 0x65, 0x6c, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x73, 0x22, 0x25, 0x0a, 0x07, 0x48, 0x61, 0x73,
	0x68, 0x32, 0x35, 0x36, 0x12, 0x1a, 0x0a, 0x08, 0x65, 0x6c, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x73,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x08, 0x65, 0x6c, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x73,
	0x22, 0x25, 0x0a, 0x06, 0x48, 0x61, 0x73, 0x68, 0x65, 0x73, 0x12, 0x1b, 0x0a, 0x05, 0x69, 0x74,
	0x65, 0x6d, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x05, 0x2e, 0x48, 0x61, 0x73, 0x68,
	0x52, 0x05, 0x69, 0x74, 0x65, 0x6d, 0x73, 0x22, 0x25, 0x0a, 0x07, 0x41, 0x64, 0x64, 0x72, 0x65,
	0x73, 0x73, 0x12, 0x1a, 0x0a, 0x08, 0x65, 0x6c, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x73, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x0c, 0x52, 0x08, 0x65, 0x6c, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x73, 0x22, 0x18,
	0x0a, 0x06, 0x50, 0x65, 0x65, 0x72, 0x49, 0x44, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x0c, 0x52, 0x02, 0x69, 0x64, 0x22, 0x2f, 0x0a, 0x07, 0x55, 0x69, 0x6e, 0x74,
	0x31, 0x32, 0x38, 0x12, 0x10, 0x0a, 0x03, 0x6c, 0x6f, 0x77, 0x18, 0x01, 0x20, 0x01, 0x28, 0x04,
	0x52, 0x03, 0x6c, 0x6f, 0x77, 0x12, 0x12, 0x0a, 0x04, 0x68, 0x69, 0x67, 0x68, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x04, 0x52, 0x04, 0x68, 0x69, 0x67, 0x68, 0x22, 0x44, 0x0a, 0x12, 0x43, 0x6f, 0x6e,
	0x73, 0x65, 0x6e, 0x73, 0x75, 0x73, 0x53, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x12,
	0x16, 0x0a, 0x01, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x08, 0x2e, 0x46, 0x65, 0x6c,
	0x74, 0x32, 0x35, 0x32, 0x52, 0x01, 0x72, 0x12, 0x16, 0x0a, 0x01, 0x73, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x08, 0x2e, 0x46, 0x65, 0x6c, 0x74, 0x32, 0x35, 0x32, 0x52, 0x01, 0x73, 0x22,
	0x40, 0x0a, 0x08, 0x50, 0x61, 0x74, 0x72, 0x69, 0x63, 0x69, 0x61, 0x12, 0x19, 0x0a, 0x08, 0x6e,
	0x5f, 0x6c, 0x65, 0x61, 0x76, 0x65, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x07, 0x6e,
	0x4c, 0x65, 0x61, 0x76, 0x65, 0x73, 0x12, 0x19, 0x0a, 0x04, 0x72, 0x6f, 0x6f, 0x74, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x05, 0x2e, 0x48, 0x61, 0x73, 0x68, 0x52, 0x04, 0x72, 0x6f, 0x6f,
	0x74, 0x22, 0x5c, 0x0a, 0x13, 0x53, 0x74, 0x61, 0x74, 0x65, 0x44, 0x69, 0x66, 0x66, 0x43, 0x6f,
	0x6d, 0x6d, 0x69, 0x74, 0x6d, 0x65, 0x6e, 0x74, 0x12, 0x2a, 0x0a, 0x11, 0x73, 0x74, 0x61, 0x74,
	0x65, 0x5f, 0x64, 0x69, 0x66, 0x66, 0x5f, 0x6c, 0x65, 0x6e, 0x67, 0x74, 0x68, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x04, 0x52, 0x0f, 0x73, 0x74, 0x61, 0x74, 0x65, 0x44, 0x69, 0x66, 0x66, 0x4c, 0x65,
	0x6e, 0x67, 0x74, 0x68, 0x12, 0x19, 0x0a, 0x04, 0x72, 0x6f, 0x6f, 0x74, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x05, 0x2e, 0x48, 0x61, 0x73, 0x68, 0x52, 0x04, 0x72, 0x6f, 0x6f, 0x74, 0x22,
	0x40, 0x0a, 0x07, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x49, 0x44, 0x12, 0x16, 0x0a, 0x06, 0x6e, 0x75,
	0x6d, 0x62, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x06, 0x6e, 0x75, 0x6d, 0x62,
	0x65, 0x72, 0x12, 0x1d, 0x0a, 0x06, 0x68, 0x65, 0x61, 0x64, 0x65, 0x72, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x05, 0x2e, 0x48, 0x61, 0x73, 0x68, 0x52, 0x06, 0x68, 0x65, 0x61, 0x64, 0x65,
	0x72, 0x22, 0xe0, 0x01, 0x0a, 0x09, 0x49, 0x74, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x12,
	0x23, 0x0a, 0x0c, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x5f, 0x6e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x04, 0x48, 0x00, 0x52, 0x0b, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x4e, 0x75,
	0x6d, 0x62, 0x65, 0x72, 0x12, 0x1f, 0x0a, 0x06, 0x68, 0x65, 0x61, 0x64, 0x65, 0x72, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x05, 0x2e, 0x48, 0x61, 0x73, 0x68, 0x48, 0x00, 0x52, 0x06, 0x68,
	0x65, 0x61, 0x64, 0x65, 0x72, 0x12, 0x32, 0x0a, 0x09, 0x64, 0x69, 0x72, 0x65, 0x63, 0x74, 0x69,
	0x6f, 0x6e, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x14, 0x2e, 0x49, 0x74, 0x65, 0x72, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x44, 0x69, 0x72, 0x65, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x09,
	0x64, 0x69, 0x72, 0x65, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x14, 0x0a, 0x05, 0x6c, 0x69, 0x6d,
	0x69, 0x74, 0x18, 0x04, 0x20, 0x01, 0x28, 0x04, 0x52, 0x05, 0x6c, 0x69, 0x6d, 0x69, 0x74, 0x12,
	0x12, 0x0a, 0x04, 0x73, 0x74, 0x65, 0x70, 0x18, 0x05, 0x20, 0x01, 0x28, 0x04, 0x52, 0x04, 0x73,
	0x74, 0x65, 0x70, 0x22, 0x26, 0x0a, 0x09, 0x44, 0x69, 0x72, 0x65, 0x63, 0x74, 0x69, 0x6f, 0x6e,
	0x12, 0x0b, 0x0a, 0x07, 0x46, 0x6f, 0x72, 0x77, 0x61, 0x72, 0x64, 0x10, 0x00, 0x12, 0x0c, 0x0a,
	0x08, 0x42, 0x61, 0x63, 0x6b, 0x77, 0x61, 0x72, 0x64, 0x10, 0x01, 0x42, 0x07, 0x0a, 0x05, 0x73,
	0x74, 0x61, 0x72, 0x74, 0x22, 0x05, 0x0a, 0x03, 0x46, 0x69, 0x6e, 0x2a, 0x30, 0x0a, 0x16, 0x4c,
	0x31, 0x44, 0x61, 0x74, 0x61, 0x41, 0x76, 0x61, 0x69, 0x6c, 0x61, 0x62, 0x69, 0x6c, 0x69, 0x74,
	0x79, 0x4d, 0x6f, 0x64, 0x65, 0x12, 0x0c, 0x0a, 0x08, 0x43, 0x61, 0x6c, 0x6c, 0x64, 0x61, 0x74,
	0x61, 0x10, 0x00, 0x12, 0x08, 0x0a, 0x04, 0x42, 0x6c, 0x6f, 0x62, 0x10, 0x01, 0x2a, 0x20, 0x0a,
	0x0e, 0x56, 0x6f, 0x6c, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x44, 0x6f, 0x6d, 0x61, 0x69, 0x6e, 0x12,
	0x06, 0x0a, 0x02, 0x4c, 0x31, 0x10, 0x00, 0x12, 0x06, 0x0a, 0x02, 0x4c, 0x32, 0x10, 0x01, 0x42,
	0x27, 0x5a, 0x25, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x4e, 0x65,
	0x74, 0x68, 0x65, 0x72, 0x6d, 0x69, 0x6e, 0x64, 0x45, 0x74, 0x68, 0x2f, 0x6a, 0x75, 0x6e, 0x6f,
	0x2f, 0x70, 0x32, 0x70, 0x2f, 0x67, 0x65, 0x6e, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_common_proto_rawDescOnce sync.Once
	file_common_proto_rawDescData = file_common_proto_rawDesc
)

func file_common_proto_rawDescGZIP() []byte {
	file_common_proto_rawDescOnce.Do(func() {
		file_common_proto_rawDescData = protoimpl.X.CompressGZIP(file_common_proto_rawDescData)
	})
	return file_common_proto_rawDescData
}

var file_common_proto_enumTypes = make([]protoimpl.EnumInfo, 3)
var file_common_proto_msgTypes = make([]protoimpl.MessageInfo, 13)
var file_common_proto_goTypes = []any{
	(L1DataAvailabilityMode)(0), // 0: L1DataAvailabilityMode
	(VolitionDomain)(0),         // 1: VolitionDomain
	(Iteration_Direction)(0),    // 2: Iteration.Direction
	(*Felt252)(nil),             // 3: Felt252
	(*Hash)(nil),                // 4: Hash
	(*Hash256)(nil),             // 5: Hash256
	(*Hashes)(nil),              // 6: Hashes
	(*Address)(nil),             // 7: Address
	(*PeerID)(nil),              // 8: PeerID
	(*Uint128)(nil),             // 9: Uint128
	(*ConsensusSignature)(nil),  // 10: ConsensusSignature
	(*Patricia)(nil),            // 11: Patricia
	(*StateDiffCommitment)(nil), // 12: StateDiffCommitment
	(*BlockID)(nil),             // 13: BlockID
	(*Iteration)(nil),           // 14: Iteration
	(*Fin)(nil),                 // 15: Fin
}
var file_common_proto_depIdxs = []int32{
	4, // 0: Hashes.items:type_name -> Hash
	3, // 1: ConsensusSignature.r:type_name -> Felt252
	3, // 2: ConsensusSignature.s:type_name -> Felt252
	4, // 3: Patricia.root:type_name -> Hash
	4, // 4: StateDiffCommitment.root:type_name -> Hash
	4, // 5: BlockID.header:type_name -> Hash
	4, // 6: Iteration.header:type_name -> Hash
	2, // 7: Iteration.direction:type_name -> Iteration.Direction
	8, // [8:8] is the sub-list for method output_type
	8, // [8:8] is the sub-list for method input_type
	8, // [8:8] is the sub-list for extension type_name
	8, // [8:8] is the sub-list for extension extendee
	0, // [0:8] is the sub-list for field type_name
}

func init() { file_common_proto_init() }
func file_common_proto_init() {
	if File_common_proto != nil {
		return
	}
	file_common_proto_msgTypes[11].OneofWrappers = []any{
		(*Iteration_BlockNumber)(nil),
		(*Iteration_Header)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_common_proto_rawDesc,
			NumEnums:      3,
			NumMessages:   13,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_common_proto_goTypes,
		DependencyIndexes: file_common_proto_depIdxs,
		EnumInfos:         file_common_proto_enumTypes,
		MessageInfos:      file_common_proto_msgTypes,
	}.Build()
	File_common_proto = out.File
	file_common_proto_rawDesc = nil
	file_common_proto_goTypes = nil
	file_common_proto_depIdxs = nil
}
