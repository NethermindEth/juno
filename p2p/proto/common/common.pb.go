// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.6
// 	protoc        (unknown)
// source: p2p/proto/common.proto

package common

import (
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
	return file_p2p_proto_common_proto_enumTypes[0].Descriptor()
}

func (L1DataAvailabilityMode) Type() protoreflect.EnumType {
	return &file_p2p_proto_common_proto_enumTypes[0]
}

func (x L1DataAvailabilityMode) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use L1DataAvailabilityMode.Descriptor instead.
func (L1DataAvailabilityMode) EnumDescriptor() ([]byte, []int) {
	return file_p2p_proto_common_proto_rawDescGZIP(), []int{0}
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
	return file_p2p_proto_common_proto_enumTypes[1].Descriptor()
}

func (VolitionDomain) Type() protoreflect.EnumType {
	return &file_p2p_proto_common_proto_enumTypes[1]
}

func (x VolitionDomain) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use VolitionDomain.Descriptor instead.
func (VolitionDomain) EnumDescriptor() ([]byte, []int) {
	return file_p2p_proto_common_proto_rawDescGZIP(), []int{1}
}

type Felt252 struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Elements      []byte                 `protobuf:"bytes,1,opt,name=elements,proto3" json:"elements,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Felt252) Reset() {
	*x = Felt252{}
	mi := &file_p2p_proto_common_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Felt252) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Felt252) ProtoMessage() {}

func (x *Felt252) ProtoReflect() protoreflect.Message {
	mi := &file_p2p_proto_common_proto_msgTypes[0]
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
	return file_p2p_proto_common_proto_rawDescGZIP(), []int{0}
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
	mi := &file_p2p_proto_common_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Hash) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Hash) ProtoMessage() {}

func (x *Hash) ProtoReflect() protoreflect.Message {
	mi := &file_p2p_proto_common_proto_msgTypes[1]
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
	return file_p2p_proto_common_proto_rawDescGZIP(), []int{1}
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
	mi := &file_p2p_proto_common_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Hash256) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Hash256) ProtoMessage() {}

func (x *Hash256) ProtoReflect() protoreflect.Message {
	mi := &file_p2p_proto_common_proto_msgTypes[2]
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
	return file_p2p_proto_common_proto_rawDescGZIP(), []int{2}
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
	mi := &file_p2p_proto_common_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Hashes) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Hashes) ProtoMessage() {}

func (x *Hashes) ProtoReflect() protoreflect.Message {
	mi := &file_p2p_proto_common_proto_msgTypes[3]
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
	return file_p2p_proto_common_proto_rawDescGZIP(), []int{3}
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
	mi := &file_p2p_proto_common_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Address) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Address) ProtoMessage() {}

func (x *Address) ProtoReflect() protoreflect.Message {
	mi := &file_p2p_proto_common_proto_msgTypes[4]
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
	return file_p2p_proto_common_proto_rawDescGZIP(), []int{4}
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
	mi := &file_p2p_proto_common_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *PeerID) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PeerID) ProtoMessage() {}

func (x *PeerID) ProtoReflect() protoreflect.Message {
	mi := &file_p2p_proto_common_proto_msgTypes[5]
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
	return file_p2p_proto_common_proto_rawDescGZIP(), []int{5}
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
	mi := &file_p2p_proto_common_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Uint128) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Uint128) ProtoMessage() {}

func (x *Uint128) ProtoReflect() protoreflect.Message {
	mi := &file_p2p_proto_common_proto_msgTypes[6]
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
	return file_p2p_proto_common_proto_rawDescGZIP(), []int{6}
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
	mi := &file_p2p_proto_common_proto_msgTypes[7]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ConsensusSignature) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ConsensusSignature) ProtoMessage() {}

func (x *ConsensusSignature) ProtoReflect() protoreflect.Message {
	mi := &file_p2p_proto_common_proto_msgTypes[7]
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
	return file_p2p_proto_common_proto_rawDescGZIP(), []int{7}
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
	mi := &file_p2p_proto_common_proto_msgTypes[8]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Patricia) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Patricia) ProtoMessage() {}

func (x *Patricia) ProtoReflect() protoreflect.Message {
	mi := &file_p2p_proto_common_proto_msgTypes[8]
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
	return file_p2p_proto_common_proto_rawDescGZIP(), []int{8}
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

type BlockID struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Number        uint64                 `protobuf:"varint,1,opt,name=number,proto3" json:"number,omitempty"`
	Header        *Hash                  `protobuf:"bytes,2,opt,name=header,proto3" json:"header,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *BlockID) Reset() {
	*x = BlockID{}
	mi := &file_p2p_proto_common_proto_msgTypes[9]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *BlockID) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BlockID) ProtoMessage() {}

func (x *BlockID) ProtoReflect() protoreflect.Message {
	mi := &file_p2p_proto_common_proto_msgTypes[9]
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
	return file_p2p_proto_common_proto_rawDescGZIP(), []int{9}
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

type BlockProof struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Proof         [][]byte               `protobuf:"bytes,1,rep,name=proof,proto3" json:"proof,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *BlockProof) Reset() {
	*x = BlockProof{}
	mi := &file_p2p_proto_common_proto_msgTypes[10]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *BlockProof) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BlockProof) ProtoMessage() {}

func (x *BlockProof) ProtoReflect() protoreflect.Message {
	mi := &file_p2p_proto_common_proto_msgTypes[10]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BlockProof.ProtoReflect.Descriptor instead.
func (*BlockProof) Descriptor() ([]byte, []int) {
	return file_p2p_proto_common_proto_rawDescGZIP(), []int{10}
}

func (x *BlockProof) GetProof() [][]byte {
	if x != nil {
		return x.Proof
	}
	return nil
}

// mark the end of a stream of messages
type Fin struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Fin) Reset() {
	*x = Fin{}
	mi := &file_p2p_proto_common_proto_msgTypes[11]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Fin) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Fin) ProtoMessage() {}

func (x *Fin) ProtoReflect() protoreflect.Message {
	mi := &file_p2p_proto_common_proto_msgTypes[11]
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
	return file_p2p_proto_common_proto_rawDescGZIP(), []int{11}
}

var File_p2p_proto_common_proto protoreflect.FileDescriptor

const file_p2p_proto_common_proto_rawDesc = "" +
	"\n" +
	"\x16p2p/proto/common.proto\"%\n" +
	"\aFelt252\x12\x1a\n" +
	"\belements\x18\x01 \x01(\fR\belements\"\"\n" +
	"\x04Hash\x12\x1a\n" +
	"\belements\x18\x01 \x01(\fR\belements\"%\n" +
	"\aHash256\x12\x1a\n" +
	"\belements\x18\x01 \x01(\fR\belements\"%\n" +
	"\x06Hashes\x12\x1b\n" +
	"\x05items\x18\x01 \x03(\v2\x05.HashR\x05items\"%\n" +
	"\aAddress\x12\x1a\n" +
	"\belements\x18\x01 \x01(\fR\belements\"\x18\n" +
	"\x06PeerID\x12\x0e\n" +
	"\x02id\x18\x01 \x01(\fR\x02id\"/\n" +
	"\aUint128\x12\x10\n" +
	"\x03low\x18\x01 \x01(\x04R\x03low\x12\x12\n" +
	"\x04high\x18\x02 \x01(\x04R\x04high\"D\n" +
	"\x12ConsensusSignature\x12\x16\n" +
	"\x01r\x18\x01 \x01(\v2\b.Felt252R\x01r\x12\x16\n" +
	"\x01s\x18\x02 \x01(\v2\b.Felt252R\x01s\"@\n" +
	"\bPatricia\x12\x19\n" +
	"\bn_leaves\x18\x01 \x01(\x04R\anLeaves\x12\x19\n" +
	"\x04root\x18\x02 \x01(\v2\x05.HashR\x04root\"@\n" +
	"\aBlockID\x12\x16\n" +
	"\x06number\x18\x01 \x01(\x04R\x06number\x12\x1d\n" +
	"\x06header\x18\x02 \x01(\v2\x05.HashR\x06header\"\"\n" +
	"\n" +
	"BlockProof\x12\x14\n" +
	"\x05proof\x18\x01 \x03(\fR\x05proof\"\x05\n" +
	"\x03Fin*0\n" +
	"\x16L1DataAvailabilityMode\x12\f\n" +
	"\bCalldata\x10\x00\x12\b\n" +
	"\x04Blob\x10\x01* \n" +
	"\x0eVolitionDomain\x12\x06\n" +
	"\x02L1\x10\x00\x12\x06\n" +
	"\x02L2\x10\x01B;Z9github.com/starknet-io/starknet-p2pspecs/p2p/proto/commonb\x06proto3"

var (
	file_p2p_proto_common_proto_rawDescOnce sync.Once
	file_p2p_proto_common_proto_rawDescData []byte
)

func file_p2p_proto_common_proto_rawDescGZIP() []byte {
	file_p2p_proto_common_proto_rawDescOnce.Do(func() {
		file_p2p_proto_common_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_p2p_proto_common_proto_rawDesc), len(file_p2p_proto_common_proto_rawDesc)))
	})
	return file_p2p_proto_common_proto_rawDescData
}

var file_p2p_proto_common_proto_enumTypes = make([]protoimpl.EnumInfo, 2)
var file_p2p_proto_common_proto_msgTypes = make([]protoimpl.MessageInfo, 12)
var file_p2p_proto_common_proto_goTypes = []any{
	(L1DataAvailabilityMode)(0), // 0: L1DataAvailabilityMode
	(VolitionDomain)(0),         // 1: VolitionDomain
	(*Felt252)(nil),             // 2: Felt252
	(*Hash)(nil),                // 3: Hash
	(*Hash256)(nil),             // 4: Hash256
	(*Hashes)(nil),              // 5: Hashes
	(*Address)(nil),             // 6: Address
	(*PeerID)(nil),              // 7: PeerID
	(*Uint128)(nil),             // 8: Uint128
	(*ConsensusSignature)(nil),  // 9: ConsensusSignature
	(*Patricia)(nil),            // 10: Patricia
	(*BlockID)(nil),             // 11: BlockID
	(*BlockProof)(nil),          // 12: BlockProof
	(*Fin)(nil),                 // 13: Fin
}
var file_p2p_proto_common_proto_depIdxs = []int32{
	3, // 0: Hashes.items:type_name -> Hash
	2, // 1: ConsensusSignature.r:type_name -> Felt252
	2, // 2: ConsensusSignature.s:type_name -> Felt252
	3, // 3: Patricia.root:type_name -> Hash
	3, // 4: BlockID.header:type_name -> Hash
	5, // [5:5] is the sub-list for method output_type
	5, // [5:5] is the sub-list for method input_type
	5, // [5:5] is the sub-list for extension type_name
	5, // [5:5] is the sub-list for extension extendee
	0, // [0:5] is the sub-list for field type_name
}

func init() { file_p2p_proto_common_proto_init() }
func file_p2p_proto_common_proto_init() {
	if File_p2p_proto_common_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_p2p_proto_common_proto_rawDesc), len(file_p2p_proto_common_proto_rawDesc)),
			NumEnums:      2,
			NumMessages:   12,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_p2p_proto_common_proto_goTypes,
		DependencyIndexes: file_p2p_proto_common_proto_depIdxs,
		EnumInfos:         file_p2p_proto_common_proto_enumTypes,
		MessageInfos:      file_p2p_proto_common_proto_msgTypes,
	}.Build()
	File_p2p_proto_common_proto = out.File
	file_p2p_proto_common_proto_goTypes = nil
	file_p2p_proto_common_proto_depIdxs = nil
}
