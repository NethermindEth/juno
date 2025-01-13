// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.2
// 	protoc        (unknown)
// source: header.proto

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

// Note: commitments may change to be for the previous blocks like comet/tendermint
// hash of block header sent to L1
type SignedBlockHeader struct {
	state               protoimpl.MessageState `protogen:"open.v1"`
	BlockHash           *Hash                  `protobuf:"bytes,1,opt,name=block_hash,json=blockHash,proto3" json:"block_hash,omitempty"` //  For the structure of the block hash, see https://docs.starknet.io/documentation/architecture_and_concepts/Network_Architecture/header/#block_hash
	ParentHash          *Hash                  `protobuf:"bytes,2,opt,name=parent_hash,json=parentHash,proto3" json:"parent_hash,omitempty"`
	Number              uint64                 `protobuf:"varint,3,opt,name=number,proto3" json:"number,omitempty"` // This can be deduced from context. We can consider removing this field.
	Time                uint64                 `protobuf:"varint,4,opt,name=time,proto3" json:"time,omitempty"`     // Encoded in Unix time.
	SequencerAddress    *Address               `protobuf:"bytes,5,opt,name=sequencer_address,json=sequencerAddress,proto3" json:"sequencer_address,omitempty"`
	StateRoot           *Hash                  `protobuf:"bytes,6,opt,name=state_root,json=stateRoot,proto3" json:"state_root,omitempty"`                                 // Patricia root of contract and class patricia tries. Each of those tries are of height 251. Same as in L1. Later more trees will be included
	StateDiffCommitment *StateDiffCommitment   `protobuf:"bytes,7,opt,name=state_diff_commitment,json=stateDiffCommitment,proto3" json:"state_diff_commitment,omitempty"` // The state diff commitment returned  by the Starknet Feeder Gateway
	// For more info, see https://community.starknet.io/t/introducing-p2p-authentication-and-mismatch-resolution-in-v0-12-2/97993
	// The leaves contain a hash of the transaction hash and transaction signature.
	Transactions           *Patricia              `protobuf:"bytes,8,opt,name=transactions,proto3" json:"transactions,omitempty"`                               // By order of execution. TBD: required? the client can execute (powerful machine) and match state diff
	Events                 *Patricia              `protobuf:"bytes,9,opt,name=events,proto3" json:"events,omitempty"`                                           // By order of issuance. TBD: in receipts?
	Receipts               *Hash                  `protobuf:"bytes,10,opt,name=receipts,proto3" json:"receipts,omitempty"`                                      // By order of issuance. This is a patricia root. No need for length because it's the same length as transactions.
	ProtocolVersion        string                 `protobuf:"bytes,11,opt,name=protocol_version,json=protocolVersion,proto3" json:"protocol_version,omitempty"` // Starknet version
	L1GasPriceFri          *Uint128               `protobuf:"bytes,12,opt,name=l1_gas_price_fri,json=l1GasPriceFri,proto3" json:"l1_gas_price_fri,omitempty"`
	L1GasPriceWei          *Uint128               `protobuf:"bytes,13,opt,name=l1_gas_price_wei,json=l1GasPriceWei,proto3" json:"l1_gas_price_wei,omitempty"`
	L1DataGasPriceFri      *Uint128               `protobuf:"bytes,14,opt,name=l1_data_gas_price_fri,json=l1DataGasPriceFri,proto3" json:"l1_data_gas_price_fri,omitempty"`
	L1DataGasPriceWei      *Uint128               `protobuf:"bytes,15,opt,name=l1_data_gas_price_wei,json=l1DataGasPriceWei,proto3" json:"l1_data_gas_price_wei,omitempty"`
	L2GasPriceFri          *Uint128               `protobuf:"bytes,16,opt,name=l2_gas_price_fri,json=l2GasPriceFri,proto3" json:"l2_gas_price_fri,omitempty"`
	L2GasPriceWei          *Uint128               `protobuf:"bytes,17,opt,name=l2_gas_price_wei,json=l2GasPriceWei,proto3" json:"l2_gas_price_wei,omitempty"`
	L1DataAvailabilityMode L1DataAvailabilityMode `protobuf:"varint,18,opt,name=l1_data_availability_mode,json=l1DataAvailabilityMode,proto3,enum=L1DataAvailabilityMode" json:"l1_data_availability_mode,omitempty"`
	// for now, we assume a small consensus, so this fits in 1M. Else, these will be repeated and extracted from this message.
	Signatures    []*ConsensusSignature `protobuf:"bytes,19,rep,name=signatures,proto3" json:"signatures,omitempty"` // can be more explicit here about the signature structure as this is not part of account abstraction
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SignedBlockHeader) Reset() {
	*x = SignedBlockHeader{}
	mi := &file_header_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SignedBlockHeader) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SignedBlockHeader) ProtoMessage() {}

func (x *SignedBlockHeader) ProtoReflect() protoreflect.Message {
	mi := &file_header_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SignedBlockHeader.ProtoReflect.Descriptor instead.
func (*SignedBlockHeader) Descriptor() ([]byte, []int) {
	return file_header_proto_rawDescGZIP(), []int{0}
}

func (x *SignedBlockHeader) GetBlockHash() *Hash {
	if x != nil {
		return x.BlockHash
	}
	return nil
}

func (x *SignedBlockHeader) GetParentHash() *Hash {
	if x != nil {
		return x.ParentHash
	}
	return nil
}

func (x *SignedBlockHeader) GetNumber() uint64 {
	if x != nil {
		return x.Number
	}
	return 0
}

func (x *SignedBlockHeader) GetTime() uint64 {
	if x != nil {
		return x.Time
	}
	return 0
}

func (x *SignedBlockHeader) GetSequencerAddress() *Address {
	if x != nil {
		return x.SequencerAddress
	}
	return nil
}

func (x *SignedBlockHeader) GetStateRoot() *Hash {
	if x != nil {
		return x.StateRoot
	}
	return nil
}

func (x *SignedBlockHeader) GetStateDiffCommitment() *StateDiffCommitment {
	if x != nil {
		return x.StateDiffCommitment
	}
	return nil
}

func (x *SignedBlockHeader) GetTransactions() *Patricia {
	if x != nil {
		return x.Transactions
	}
	return nil
}

func (x *SignedBlockHeader) GetEvents() *Patricia {
	if x != nil {
		return x.Events
	}
	return nil
}

func (x *SignedBlockHeader) GetReceipts() *Hash {
	if x != nil {
		return x.Receipts
	}
	return nil
}

func (x *SignedBlockHeader) GetProtocolVersion() string {
	if x != nil {
		return x.ProtocolVersion
	}
	return ""
}

func (x *SignedBlockHeader) GetL1GasPriceFri() *Uint128 {
	if x != nil {
		return x.L1GasPriceFri
	}
	return nil
}

func (x *SignedBlockHeader) GetL1GasPriceWei() *Uint128 {
	if x != nil {
		return x.L1GasPriceWei
	}
	return nil
}

func (x *SignedBlockHeader) GetL1DataGasPriceFri() *Uint128 {
	if x != nil {
		return x.L1DataGasPriceFri
	}
	return nil
}

func (x *SignedBlockHeader) GetL1DataGasPriceWei() *Uint128 {
	if x != nil {
		return x.L1DataGasPriceWei
	}
	return nil
}

func (x *SignedBlockHeader) GetL2GasPriceFri() *Uint128 {
	if x != nil {
		return x.L2GasPriceFri
	}
	return nil
}

func (x *SignedBlockHeader) GetL2GasPriceWei() *Uint128 {
	if x != nil {
		return x.L2GasPriceWei
	}
	return nil
}

func (x *SignedBlockHeader) GetL1DataAvailabilityMode() L1DataAvailabilityMode {
	if x != nil {
		return x.L1DataAvailabilityMode
	}
	return L1DataAvailabilityMode_Calldata
}

func (x *SignedBlockHeader) GetSignatures() []*ConsensusSignature {
	if x != nil {
		return x.Signatures
	}
	return nil
}

// sent to all peers (except the ones this was received from, if any).
// for a fraction of peers, also send the GetBlockHeaders response (as if they asked for it for this block)
type NewBlock struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Types that are valid to be assigned to MaybeFull:
	//
	//	*NewBlock_Id
	//	*NewBlock_Header
	MaybeFull     isNewBlock_MaybeFull `protobuf_oneof:"maybe_full"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *NewBlock) Reset() {
	*x = NewBlock{}
	mi := &file_header_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *NewBlock) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NewBlock) ProtoMessage() {}

func (x *NewBlock) ProtoReflect() protoreflect.Message {
	mi := &file_header_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NewBlock.ProtoReflect.Descriptor instead.
func (*NewBlock) Descriptor() ([]byte, []int) {
	return file_header_proto_rawDescGZIP(), []int{1}
}

func (x *NewBlock) GetMaybeFull() isNewBlock_MaybeFull {
	if x != nil {
		return x.MaybeFull
	}
	return nil
}

func (x *NewBlock) GetId() *BlockID {
	if x != nil {
		if x, ok := x.MaybeFull.(*NewBlock_Id); ok {
			return x.Id
		}
	}
	return nil
}

func (x *NewBlock) GetHeader() *BlockHeadersResponse {
	if x != nil {
		if x, ok := x.MaybeFull.(*NewBlock_Header); ok {
			return x.Header
		}
	}
	return nil
}

type isNewBlock_MaybeFull interface {
	isNewBlock_MaybeFull()
}

type NewBlock_Id struct {
	Id *BlockID `protobuf:"bytes,1,opt,name=id,proto3,oneof"`
}

type NewBlock_Header struct {
	Header *BlockHeadersResponse `protobuf:"bytes,2,opt,name=header,proto3,oneof"`
}

func (*NewBlock_Id) isNewBlock_MaybeFull() {}

func (*NewBlock_Header) isNewBlock_MaybeFull() {}

type BlockHeadersRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Iteration     *Iteration             `protobuf:"bytes,1,opt,name=iteration,proto3" json:"iteration,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *BlockHeadersRequest) Reset() {
	*x = BlockHeadersRequest{}
	mi := &file_header_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *BlockHeadersRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BlockHeadersRequest) ProtoMessage() {}

func (x *BlockHeadersRequest) ProtoReflect() protoreflect.Message {
	mi := &file_header_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BlockHeadersRequest.ProtoReflect.Descriptor instead.
func (*BlockHeadersRequest) Descriptor() ([]byte, []int) {
	return file_header_proto_rawDescGZIP(), []int{2}
}

func (x *BlockHeadersRequest) GetIteration() *Iteration {
	if x != nil {
		return x.Iteration
	}
	return nil
}

// Responses are sent ordered by the order given in the request.
type BlockHeadersResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Types that are valid to be assigned to HeaderMessage:
	//
	//	*BlockHeadersResponse_Header
	//	*BlockHeadersResponse_Fin
	HeaderMessage isBlockHeadersResponse_HeaderMessage `protobuf_oneof:"header_message"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *BlockHeadersResponse) Reset() {
	*x = BlockHeadersResponse{}
	mi := &file_header_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *BlockHeadersResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BlockHeadersResponse) ProtoMessage() {}

func (x *BlockHeadersResponse) ProtoReflect() protoreflect.Message {
	mi := &file_header_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BlockHeadersResponse.ProtoReflect.Descriptor instead.
func (*BlockHeadersResponse) Descriptor() ([]byte, []int) {
	return file_header_proto_rawDescGZIP(), []int{3}
}

func (x *BlockHeadersResponse) GetHeaderMessage() isBlockHeadersResponse_HeaderMessage {
	if x != nil {
		return x.HeaderMessage
	}
	return nil
}

func (x *BlockHeadersResponse) GetHeader() *SignedBlockHeader {
	if x != nil {
		if x, ok := x.HeaderMessage.(*BlockHeadersResponse_Header); ok {
			return x.Header
		}
	}
	return nil
}

func (x *BlockHeadersResponse) GetFin() *Fin {
	if x != nil {
		if x, ok := x.HeaderMessage.(*BlockHeadersResponse_Fin); ok {
			return x.Fin
		}
	}
	return nil
}

type isBlockHeadersResponse_HeaderMessage interface {
	isBlockHeadersResponse_HeaderMessage()
}

type BlockHeadersResponse_Header struct {
	Header *SignedBlockHeader `protobuf:"bytes,1,opt,name=header,proto3,oneof"`
}

type BlockHeadersResponse_Fin struct {
	Fin *Fin `protobuf:"bytes,2,opt,name=fin,proto3,oneof"` // Fin is sent after the peer sent all the data or when it encountered a block that it doesn't have its header.
}

func (*BlockHeadersResponse_Header) isBlockHeadersResponse_HeaderMessage() {}

func (*BlockHeadersResponse_Fin) isBlockHeadersResponse_HeaderMessage() {}

type BlockProof struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Proof         [][]byte               `protobuf:"bytes,1,rep,name=proof,proto3" json:"proof,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *BlockProof) Reset() {
	*x = BlockProof{}
	mi := &file_header_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *BlockProof) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BlockProof) ProtoMessage() {}

func (x *BlockProof) ProtoReflect() protoreflect.Message {
	mi := &file_header_proto_msgTypes[4]
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
	return file_header_proto_rawDescGZIP(), []int{4}
}

func (x *BlockProof) GetProof() [][]byte {
	if x != nil {
		return x.Proof
	}
	return nil
}

var File_header_proto protoreflect.FileDescriptor

var file_header_proto_rawDesc = []byte{
	0x0a, 0x0c, 0x68, 0x65, 0x61, 0x64, 0x65, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x0c,
	0x63, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xa1, 0x07, 0x0a,
	0x11, 0x53, 0x69, 0x67, 0x6e, 0x65, 0x64, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x48, 0x65, 0x61, 0x64,
	0x65, 0x72, 0x12, 0x24, 0x0a, 0x0a, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x5f, 0x68, 0x61, 0x73, 0x68,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x05, 0x2e, 0x48, 0x61, 0x73, 0x68, 0x52, 0x09, 0x62,
	0x6c, 0x6f, 0x63, 0x6b, 0x48, 0x61, 0x73, 0x68, 0x12, 0x26, 0x0a, 0x0b, 0x70, 0x61, 0x72, 0x65,
	0x6e, 0x74, 0x5f, 0x68, 0x61, 0x73, 0x68, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x05, 0x2e,
	0x48, 0x61, 0x73, 0x68, 0x52, 0x0a, 0x70, 0x61, 0x72, 0x65, 0x6e, 0x74, 0x48, 0x61, 0x73, 0x68,
	0x12, 0x16, 0x0a, 0x06, 0x6e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x18, 0x03, 0x20, 0x01, 0x28, 0x04,
	0x52, 0x06, 0x6e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x69, 0x6d, 0x65,
	0x18, 0x04, 0x20, 0x01, 0x28, 0x04, 0x52, 0x04, 0x74, 0x69, 0x6d, 0x65, 0x12, 0x35, 0x0a, 0x11,
	0x73, 0x65, 0x71, 0x75, 0x65, 0x6e, 0x63, 0x65, 0x72, 0x5f, 0x61, 0x64, 0x64, 0x72, 0x65, 0x73,
	0x73, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x08, 0x2e, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73,
	0x73, 0x52, 0x10, 0x73, 0x65, 0x71, 0x75, 0x65, 0x6e, 0x63, 0x65, 0x72, 0x41, 0x64, 0x64, 0x72,
	0x65, 0x73, 0x73, 0x12, 0x24, 0x0a, 0x0a, 0x73, 0x74, 0x61, 0x74, 0x65, 0x5f, 0x72, 0x6f, 0x6f,
	0x74, 0x18, 0x06, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x05, 0x2e, 0x48, 0x61, 0x73, 0x68, 0x52, 0x09,
	0x73, 0x74, 0x61, 0x74, 0x65, 0x52, 0x6f, 0x6f, 0x74, 0x12, 0x48, 0x0a, 0x15, 0x73, 0x74, 0x61,
	0x74, 0x65, 0x5f, 0x64, 0x69, 0x66, 0x66, 0x5f, 0x63, 0x6f, 0x6d, 0x6d, 0x69, 0x74, 0x6d, 0x65,
	0x6e, 0x74, 0x18, 0x07, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x14, 0x2e, 0x53, 0x74, 0x61, 0x74, 0x65,
	0x44, 0x69, 0x66, 0x66, 0x43, 0x6f, 0x6d, 0x6d, 0x69, 0x74, 0x6d, 0x65, 0x6e, 0x74, 0x52, 0x13,
	0x73, 0x74, 0x61, 0x74, 0x65, 0x44, 0x69, 0x66, 0x66, 0x43, 0x6f, 0x6d, 0x6d, 0x69, 0x74, 0x6d,
	0x65, 0x6e, 0x74, 0x12, 0x2d, 0x0a, 0x0c, 0x74, 0x72, 0x61, 0x6e, 0x73, 0x61, 0x63, 0x74, 0x69,
	0x6f, 0x6e, 0x73, 0x18, 0x08, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x09, 0x2e, 0x50, 0x61, 0x74, 0x72,
	0x69, 0x63, 0x69, 0x61, 0x52, 0x0c, 0x74, 0x72, 0x61, 0x6e, 0x73, 0x61, 0x63, 0x74, 0x69, 0x6f,
	0x6e, 0x73, 0x12, 0x21, 0x0a, 0x06, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x73, 0x18, 0x09, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x09, 0x2e, 0x50, 0x61, 0x74, 0x72, 0x69, 0x63, 0x69, 0x61, 0x52, 0x06, 0x65,
	0x76, 0x65, 0x6e, 0x74, 0x73, 0x12, 0x21, 0x0a, 0x08, 0x72, 0x65, 0x63, 0x65, 0x69, 0x70, 0x74,
	0x73, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x05, 0x2e, 0x48, 0x61, 0x73, 0x68, 0x52, 0x08,
	0x72, 0x65, 0x63, 0x65, 0x69, 0x70, 0x74, 0x73, 0x12, 0x29, 0x0a, 0x10, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x63, 0x6f, 0x6c, 0x5f, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x0b, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x0f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x56, 0x65, 0x72, 0x73,
	0x69, 0x6f, 0x6e, 0x12, 0x31, 0x0a, 0x10, 0x6c, 0x31, 0x5f, 0x67, 0x61, 0x73, 0x5f, 0x70, 0x72,
	0x69, 0x63, 0x65, 0x5f, 0x66, 0x72, 0x69, 0x18, 0x0c, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x08, 0x2e,
	0x55, 0x69, 0x6e, 0x74, 0x31, 0x32, 0x38, 0x52, 0x0d, 0x6c, 0x31, 0x47, 0x61, 0x73, 0x50, 0x72,
	0x69, 0x63, 0x65, 0x46, 0x72, 0x69, 0x12, 0x31, 0x0a, 0x10, 0x6c, 0x31, 0x5f, 0x67, 0x61, 0x73,
	0x5f, 0x70, 0x72, 0x69, 0x63, 0x65, 0x5f, 0x77, 0x65, 0x69, 0x18, 0x0d, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x08, 0x2e, 0x55, 0x69, 0x6e, 0x74, 0x31, 0x32, 0x38, 0x52, 0x0d, 0x6c, 0x31, 0x47, 0x61,
	0x73, 0x50, 0x72, 0x69, 0x63, 0x65, 0x57, 0x65, 0x69, 0x12, 0x3a, 0x0a, 0x15, 0x6c, 0x31, 0x5f,
	0x64, 0x61, 0x74, 0x61, 0x5f, 0x67, 0x61, 0x73, 0x5f, 0x70, 0x72, 0x69, 0x63, 0x65, 0x5f, 0x66,
	0x72, 0x69, 0x18, 0x0e, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x08, 0x2e, 0x55, 0x69, 0x6e, 0x74, 0x31,
	0x32, 0x38, 0x52, 0x11, 0x6c, 0x31, 0x44, 0x61, 0x74, 0x61, 0x47, 0x61, 0x73, 0x50, 0x72, 0x69,
	0x63, 0x65, 0x46, 0x72, 0x69, 0x12, 0x3a, 0x0a, 0x15, 0x6c, 0x31, 0x5f, 0x64, 0x61, 0x74, 0x61,
	0x5f, 0x67, 0x61, 0x73, 0x5f, 0x70, 0x72, 0x69, 0x63, 0x65, 0x5f, 0x77, 0x65, 0x69, 0x18, 0x0f,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x08, 0x2e, 0x55, 0x69, 0x6e, 0x74, 0x31, 0x32, 0x38, 0x52, 0x11,
	0x6c, 0x31, 0x44, 0x61, 0x74, 0x61, 0x47, 0x61, 0x73, 0x50, 0x72, 0x69, 0x63, 0x65, 0x57, 0x65,
	0x69, 0x12, 0x31, 0x0a, 0x10, 0x6c, 0x32, 0x5f, 0x67, 0x61, 0x73, 0x5f, 0x70, 0x72, 0x69, 0x63,
	0x65, 0x5f, 0x66, 0x72, 0x69, 0x18, 0x10, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x08, 0x2e, 0x55, 0x69,
	0x6e, 0x74, 0x31, 0x32, 0x38, 0x52, 0x0d, 0x6c, 0x32, 0x47, 0x61, 0x73, 0x50, 0x72, 0x69, 0x63,
	0x65, 0x46, 0x72, 0x69, 0x12, 0x31, 0x0a, 0x10, 0x6c, 0x32, 0x5f, 0x67, 0x61, 0x73, 0x5f, 0x70,
	0x72, 0x69, 0x63, 0x65, 0x5f, 0x77, 0x65, 0x69, 0x18, 0x11, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x08,
	0x2e, 0x55, 0x69, 0x6e, 0x74, 0x31, 0x32, 0x38, 0x52, 0x0d, 0x6c, 0x32, 0x47, 0x61, 0x73, 0x50,
	0x72, 0x69, 0x63, 0x65, 0x57, 0x65, 0x69, 0x12, 0x52, 0x0a, 0x19, 0x6c, 0x31, 0x5f, 0x64, 0x61,
	0x74, 0x61, 0x5f, 0x61, 0x76, 0x61, 0x69, 0x6c, 0x61, 0x62, 0x69, 0x6c, 0x69, 0x74, 0x79, 0x5f,
	0x6d, 0x6f, 0x64, 0x65, 0x18, 0x12, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x17, 0x2e, 0x4c, 0x31, 0x44,
	0x61, 0x74, 0x61, 0x41, 0x76, 0x61, 0x69, 0x6c, 0x61, 0x62, 0x69, 0x6c, 0x69, 0x74, 0x79, 0x4d,
	0x6f, 0x64, 0x65, 0x52, 0x16, 0x6c, 0x31, 0x44, 0x61, 0x74, 0x61, 0x41, 0x76, 0x61, 0x69, 0x6c,
	0x61, 0x62, 0x69, 0x6c, 0x69, 0x74, 0x79, 0x4d, 0x6f, 0x64, 0x65, 0x12, 0x33, 0x0a, 0x0a, 0x73,
	0x69, 0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x73, 0x18, 0x13, 0x20, 0x03, 0x28, 0x0b, 0x32,
	0x13, 0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x65, 0x6e, 0x73, 0x75, 0x73, 0x53, 0x69, 0x67, 0x6e, 0x61,
	0x74, 0x75, 0x72, 0x65, 0x52, 0x0a, 0x73, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x73,
	0x22, 0x65, 0x0a, 0x08, 0x4e, 0x65, 0x77, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x12, 0x1a, 0x0a, 0x02,
	0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x08, 0x2e, 0x42, 0x6c, 0x6f, 0x63, 0x6b,
	0x49, 0x44, 0x48, 0x00, 0x52, 0x02, 0x69, 0x64, 0x12, 0x2f, 0x0a, 0x06, 0x68, 0x65, 0x61, 0x64,
	0x65, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x15, 0x2e, 0x42, 0x6c, 0x6f, 0x63, 0x6b,
	0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x48,
	0x00, 0x52, 0x06, 0x68, 0x65, 0x61, 0x64, 0x65, 0x72, 0x42, 0x0c, 0x0a, 0x0a, 0x6d, 0x61, 0x79,
	0x62, 0x65, 0x5f, 0x66, 0x75, 0x6c, 0x6c, 0x22, 0x3f, 0x0a, 0x13, 0x42, 0x6c, 0x6f, 0x63, 0x6b,
	0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x28,
	0x0a, 0x09, 0x69, 0x74, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x0a, 0x2e, 0x49, 0x74, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x09, 0x69,
	0x74, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x22, 0x70, 0x0a, 0x14, 0x42, 0x6c, 0x6f, 0x63,
	0x6b, 0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x12, 0x2c, 0x0a, 0x06, 0x68, 0x65, 0x61, 0x64, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x12, 0x2e, 0x53, 0x69, 0x67, 0x6e, 0x65, 0x64, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x48, 0x65,
	0x61, 0x64, 0x65, 0x72, 0x48, 0x00, 0x52, 0x06, 0x68, 0x65, 0x61, 0x64, 0x65, 0x72, 0x12, 0x18,
	0x0a, 0x03, 0x66, 0x69, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x04, 0x2e, 0x46, 0x69,
	0x6e, 0x48, 0x00, 0x52, 0x03, 0x66, 0x69, 0x6e, 0x42, 0x10, 0x0a, 0x0e, 0x68, 0x65, 0x61, 0x64,
	0x65, 0x72, 0x5f, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x22, 0x22, 0x0a, 0x0a, 0x42, 0x6c,
	0x6f, 0x63, 0x6b, 0x50, 0x72, 0x6f, 0x6f, 0x66, 0x12, 0x14, 0x0a, 0x05, 0x70, 0x72, 0x6f, 0x6f,
	0x66, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0c, 0x52, 0x05, 0x70, 0x72, 0x6f, 0x6f, 0x66, 0x42, 0x27,
	0x5a, 0x25, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x4e, 0x65, 0x74,
	0x68, 0x65, 0x72, 0x6d, 0x69, 0x6e, 0x64, 0x45, 0x74, 0x68, 0x2f, 0x6a, 0x75, 0x6e, 0x6f, 0x2f,
	0x70, 0x32, 0x70, 0x2f, 0x67, 0x65, 0x6e, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_header_proto_rawDescOnce sync.Once
	file_header_proto_rawDescData = file_header_proto_rawDesc
)

func file_header_proto_rawDescGZIP() []byte {
	file_header_proto_rawDescOnce.Do(func() {
		file_header_proto_rawDescData = protoimpl.X.CompressGZIP(file_header_proto_rawDescData)
	})
	return file_header_proto_rawDescData
}

var file_header_proto_msgTypes = make([]protoimpl.MessageInfo, 5)
var file_header_proto_goTypes = []any{
	(*SignedBlockHeader)(nil),    // 0: SignedBlockHeader
	(*NewBlock)(nil),             // 1: NewBlock
	(*BlockHeadersRequest)(nil),  // 2: BlockHeadersRequest
	(*BlockHeadersResponse)(nil), // 3: BlockHeadersResponse
	(*BlockProof)(nil),           // 4: BlockProof
	(*Hash)(nil),                 // 5: Hash
	(*Address)(nil),              // 6: Address
	(*StateDiffCommitment)(nil),  // 7: StateDiffCommitment
	(*Patricia)(nil),             // 8: Patricia
	(*Uint128)(nil),              // 9: Uint128
	(L1DataAvailabilityMode)(0),  // 10: L1DataAvailabilityMode
	(*ConsensusSignature)(nil),   // 11: ConsensusSignature
	(*BlockID)(nil),              // 12: BlockID
	(*Iteration)(nil),            // 13: Iteration
	(*Fin)(nil),                  // 14: Fin
}
var file_header_proto_depIdxs = []int32{
	5,  // 0: SignedBlockHeader.block_hash:type_name -> Hash
	5,  // 1: SignedBlockHeader.parent_hash:type_name -> Hash
	6,  // 2: SignedBlockHeader.sequencer_address:type_name -> Address
	5,  // 3: SignedBlockHeader.state_root:type_name -> Hash
	7,  // 4: SignedBlockHeader.state_diff_commitment:type_name -> StateDiffCommitment
	8,  // 5: SignedBlockHeader.transactions:type_name -> Patricia
	8,  // 6: SignedBlockHeader.events:type_name -> Patricia
	5,  // 7: SignedBlockHeader.receipts:type_name -> Hash
	9,  // 8: SignedBlockHeader.l1_gas_price_fri:type_name -> Uint128
	9,  // 9: SignedBlockHeader.l1_gas_price_wei:type_name -> Uint128
	9,  // 10: SignedBlockHeader.l1_data_gas_price_fri:type_name -> Uint128
	9,  // 11: SignedBlockHeader.l1_data_gas_price_wei:type_name -> Uint128
	9,  // 12: SignedBlockHeader.l2_gas_price_fri:type_name -> Uint128
	9,  // 13: SignedBlockHeader.l2_gas_price_wei:type_name -> Uint128
	10, // 14: SignedBlockHeader.l1_data_availability_mode:type_name -> L1DataAvailabilityMode
	11, // 15: SignedBlockHeader.signatures:type_name -> ConsensusSignature
	12, // 16: NewBlock.id:type_name -> BlockID
	3,  // 17: NewBlock.header:type_name -> BlockHeadersResponse
	13, // 18: BlockHeadersRequest.iteration:type_name -> Iteration
	0,  // 19: BlockHeadersResponse.header:type_name -> SignedBlockHeader
	14, // 20: BlockHeadersResponse.fin:type_name -> Fin
	21, // [21:21] is the sub-list for method output_type
	21, // [21:21] is the sub-list for method input_type
	21, // [21:21] is the sub-list for extension type_name
	21, // [21:21] is the sub-list for extension extendee
	0,  // [0:21] is the sub-list for field type_name
}

func init() { file_header_proto_init() }
func file_header_proto_init() {
	if File_header_proto != nil {
		return
	}
	file_common_proto_init()
	file_header_proto_msgTypes[1].OneofWrappers = []any{
		(*NewBlock_Id)(nil),
		(*NewBlock_Header)(nil),
	}
	file_header_proto_msgTypes[3].OneofWrappers = []any{
		(*BlockHeadersResponse_Header)(nil),
		(*BlockHeadersResponse_Fin)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_header_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   5,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_header_proto_goTypes,
		DependencyIndexes: file_header_proto_depIdxs,
		MessageInfos:      file_header_proto_msgTypes,
	}.Build()
	File_header_proto = out.File
	file_header_proto_rawDesc = nil
	file_header_proto_goTypes = nil
	file_header_proto_depIdxs = nil
}
