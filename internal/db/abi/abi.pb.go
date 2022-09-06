// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.0
// 	protoc        v3.21.5
// source: abi.proto

package abi

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

type Abi struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Functions   []*Function `protobuf:"bytes,1,rep,name=functions,proto3" json:"functions,omitempty"`
	Events      []*AbiEvent `protobuf:"bytes,2,rep,name=events,proto3" json:"events,omitempty"`
	Structs     []*Struct   `protobuf:"bytes,3,rep,name=structs,proto3" json:"structs,omitempty"`
	L1Handlers  []*Function `protobuf:"bytes,4,rep,name=l1Handlers,proto3" json:"l1Handlers,omitempty"`
	Constructor *Function   `protobuf:"bytes,5,opt,name=constructor,proto3" json:"constructor,omitempty"`
}

func (x *Abi) Reset() {
	*x = Abi{}
	if protoimpl.UnsafeEnabled {
		mi := &file_abi_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Abi) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Abi) ProtoMessage() {}

func (x *Abi) ProtoReflect() protoreflect.Message {
	mi := &file_abi_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Abi.ProtoReflect.Descriptor instead.
func (*Abi) Descriptor() ([]byte, []int) {
	return file_abi_proto_rawDescGZIP(), []int{0}
}

func (x *Abi) GetFunctions() []*Function {
	if x != nil {
		return x.Functions
	}
	return nil
}

func (x *Abi) GetEvents() []*AbiEvent {
	if x != nil {
		return x.Events
	}
	return nil
}

func (x *Abi) GetStructs() []*Struct {
	if x != nil {
		return x.Structs
	}
	return nil
}

func (x *Abi) GetL1Handlers() []*Function {
	if x != nil {
		return x.L1Handlers
	}
	return nil
}

func (x *Abi) GetConstructor() *Function {
	if x != nil {
		return x.Constructor
	}
	return nil
}

type Function struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name    string             `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Inputs  []*Function_Input  `protobuf:"bytes,2,rep,name=inputs,proto3" json:"inputs,omitempty"`
	Outputs []*Function_Output `protobuf:"bytes,3,rep,name=outputs,proto3" json:"outputs,omitempty"`
}

func (x *Function) Reset() {
	*x = Function{}
	if protoimpl.UnsafeEnabled {
		mi := &file_abi_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Function) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Function) ProtoMessage() {}

func (x *Function) ProtoReflect() protoreflect.Message {
	mi := &file_abi_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Function.ProtoReflect.Descriptor instead.
func (*Function) Descriptor() ([]byte, []int) {
	return file_abi_proto_rawDescGZIP(), []int{1}
}

func (x *Function) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Function) GetInputs() []*Function_Input {
	if x != nil {
		return x.Inputs
	}
	return nil
}

func (x *Function) GetOutputs() []*Function_Output {
	if x != nil {
		return x.Outputs
	}
	return nil
}

type AbiEvent struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name string           `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Data []*AbiEvent_Data `protobuf:"bytes,2,rep,name=data,proto3" json:"data,omitempty"`
	Keys []string         `protobuf:"bytes,3,rep,name=keys,proto3" json:"keys,omitempty"`
}

func (x *AbiEvent) Reset() {
	*x = AbiEvent{}
	if protoimpl.UnsafeEnabled {
		mi := &file_abi_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *AbiEvent) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AbiEvent) ProtoMessage() {}

func (x *AbiEvent) ProtoReflect() protoreflect.Message {
	mi := &file_abi_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AbiEvent.ProtoReflect.Descriptor instead.
func (*AbiEvent) Descriptor() ([]byte, []int) {
	return file_abi_proto_rawDescGZIP(), []int{2}
}

func (x *AbiEvent) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *AbiEvent) GetData() []*AbiEvent_Data {
	if x != nil {
		return x.Data
	}
	return nil
}

func (x *AbiEvent) GetKeys() []string {
	if x != nil {
		return x.Keys
	}
	return nil
}

type Struct struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Fields []*Struct_Field `protobuf:"bytes,1,rep,name=fields,proto3" json:"fields,omitempty"`
	Name   string          `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	Size   uint64          `protobuf:"varint,3,opt,name=size,proto3" json:"size,omitempty"`
}

func (x *Struct) Reset() {
	*x = Struct{}
	if protoimpl.UnsafeEnabled {
		mi := &file_abi_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Struct) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Struct) ProtoMessage() {}

func (x *Struct) ProtoReflect() protoreflect.Message {
	mi := &file_abi_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Struct.ProtoReflect.Descriptor instead.
func (*Struct) Descriptor() ([]byte, []int) {
	return file_abi_proto_rawDescGZIP(), []int{3}
}

func (x *Struct) GetFields() []*Struct_Field {
	if x != nil {
		return x.Fields
	}
	return nil
}

func (x *Struct) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Struct) GetSize() uint64 {
	if x != nil {
		return x.Size
	}
	return 0
}

type Function_Input struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Type string `protobuf:"bytes,2,opt,name=type,proto3" json:"type,omitempty"`
}

func (x *Function_Input) Reset() {
	*x = Function_Input{}
	if protoimpl.UnsafeEnabled {
		mi := &file_abi_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Function_Input) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Function_Input) ProtoMessage() {}

func (x *Function_Input) ProtoReflect() protoreflect.Message {
	mi := &file_abi_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Function_Input.ProtoReflect.Descriptor instead.
func (*Function_Input) Descriptor() ([]byte, []int) {
	return file_abi_proto_rawDescGZIP(), []int{1, 0}
}

func (x *Function_Input) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Function_Input) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

type Function_Output struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Type string `protobuf:"bytes,2,opt,name=type,proto3" json:"type,omitempty"`
}

func (x *Function_Output) Reset() {
	*x = Function_Output{}
	if protoimpl.UnsafeEnabled {
		mi := &file_abi_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Function_Output) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Function_Output) ProtoMessage() {}

func (x *Function_Output) ProtoReflect() protoreflect.Message {
	mi := &file_abi_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Function_Output.ProtoReflect.Descriptor instead.
func (*Function_Output) Descriptor() ([]byte, []int) {
	return file_abi_proto_rawDescGZIP(), []int{1, 1}
}

func (x *Function_Output) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Function_Output) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

type AbiEvent_Data struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Type string `protobuf:"bytes,2,opt,name=type,proto3" json:"type,omitempty"`
}

func (x *AbiEvent_Data) Reset() {
	*x = AbiEvent_Data{}
	if protoimpl.UnsafeEnabled {
		mi := &file_abi_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *AbiEvent_Data) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AbiEvent_Data) ProtoMessage() {}

func (x *AbiEvent_Data) ProtoReflect() protoreflect.Message {
	mi := &file_abi_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AbiEvent_Data.ProtoReflect.Descriptor instead.
func (*AbiEvent_Data) Descriptor() ([]byte, []int) {
	return file_abi_proto_rawDescGZIP(), []int{2, 0}
}

func (x *AbiEvent_Data) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *AbiEvent_Data) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

type Struct_Field struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name   string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Type   string `protobuf:"bytes,2,opt,name=type,proto3" json:"type,omitempty"`
	Offset uint32 `protobuf:"varint,3,opt,name=offset,proto3" json:"offset,omitempty"`
}

func (x *Struct_Field) Reset() {
	*x = Struct_Field{}
	if protoimpl.UnsafeEnabled {
		mi := &file_abi_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Struct_Field) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Struct_Field) ProtoMessage() {}

func (x *Struct_Field) ProtoReflect() protoreflect.Message {
	mi := &file_abi_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Struct_Field.ProtoReflect.Descriptor instead.
func (*Struct_Field) Descriptor() ([]byte, []int) {
	return file_abi_proto_rawDescGZIP(), []int{3, 0}
}

func (x *Struct_Field) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Struct_Field) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

func (x *Struct_Field) GetOffset() uint32 {
	if x != nil {
		return x.Offset
	}
	return 0
}

var File_abi_proto protoreflect.FileDescriptor

var file_abi_proto_rawDesc = []byte{
	0x0a, 0x09, 0x61, 0x62, 0x69, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xcc, 0x01, 0x0a, 0x03,
	0x41, 0x62, 0x69, 0x12, 0x27, 0x0a, 0x09, 0x66, 0x75, 0x6e, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x73,
	0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x09, 0x2e, 0x46, 0x75, 0x6e, 0x63, 0x74, 0x69, 0x6f,
	0x6e, 0x52, 0x09, 0x66, 0x75, 0x6e, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x21, 0x0a, 0x06,
	0x65, 0x76, 0x65, 0x6e, 0x74, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x09, 0x2e, 0x41,
	0x62, 0x69, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x52, 0x06, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x73, 0x12,
	0x21, 0x0a, 0x07, 0x73, 0x74, 0x72, 0x75, 0x63, 0x74, 0x73, 0x18, 0x03, 0x20, 0x03, 0x28, 0x0b,
	0x32, 0x07, 0x2e, 0x53, 0x74, 0x72, 0x75, 0x63, 0x74, 0x52, 0x07, 0x73, 0x74, 0x72, 0x75, 0x63,
	0x74, 0x73, 0x12, 0x29, 0x0a, 0x0a, 0x6c, 0x31, 0x48, 0x61, 0x6e, 0x64, 0x6c, 0x65, 0x72, 0x73,
	0x18, 0x04, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x09, 0x2e, 0x46, 0x75, 0x6e, 0x63, 0x74, 0x69, 0x6f,
	0x6e, 0x52, 0x0a, 0x6c, 0x31, 0x48, 0x61, 0x6e, 0x64, 0x6c, 0x65, 0x72, 0x73, 0x12, 0x2b, 0x0a,
	0x0b, 0x63, 0x6f, 0x6e, 0x73, 0x74, 0x72, 0x75, 0x63, 0x74, 0x6f, 0x72, 0x18, 0x05, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x09, 0x2e, 0x46, 0x75, 0x6e, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x0b, 0x63,
	0x6f, 0x6e, 0x73, 0x74, 0x72, 0x75, 0x63, 0x74, 0x6f, 0x72, 0x22, 0xd6, 0x01, 0x0a, 0x08, 0x46,
	0x75, 0x6e, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x27, 0x0a, 0x06, 0x69,
	0x6e, 0x70, 0x75, 0x74, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x0f, 0x2e, 0x46, 0x75,
	0x6e, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x49, 0x6e, 0x70, 0x75, 0x74, 0x52, 0x06, 0x69, 0x6e,
	0x70, 0x75, 0x74, 0x73, 0x12, 0x2a, 0x0a, 0x07, 0x6f, 0x75, 0x74, 0x70, 0x75, 0x74, 0x73, 0x18,
	0x03, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x10, 0x2e, 0x46, 0x75, 0x6e, 0x63, 0x74, 0x69, 0x6f, 0x6e,
	0x2e, 0x4f, 0x75, 0x74, 0x70, 0x75, 0x74, 0x52, 0x07, 0x6f, 0x75, 0x74, 0x70, 0x75, 0x74, 0x73,
	0x1a, 0x2f, 0x0a, 0x05, 0x49, 0x6e, 0x70, 0x75, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d,
	0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x12, 0x0a,
	0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x74, 0x79, 0x70,
	0x65, 0x1a, 0x30, 0x0a, 0x06, 0x4f, 0x75, 0x74, 0x70, 0x75, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x6e,
	0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12,
	0x12, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x74,
	0x79, 0x70, 0x65, 0x22, 0x86, 0x01, 0x0a, 0x08, 0x41, 0x62, 0x69, 0x45, 0x76, 0x65, 0x6e, 0x74,
	0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04,
	0x6e, 0x61, 0x6d, 0x65, 0x12, 0x22, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x02, 0x20, 0x03,
	0x28, 0x0b, 0x32, 0x0e, 0x2e, 0x41, 0x62, 0x69, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x2e, 0x44, 0x61,
	0x74, 0x61, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x12, 0x12, 0x0a, 0x04, 0x6b, 0x65, 0x79, 0x73,
	0x18, 0x03, 0x20, 0x03, 0x28, 0x09, 0x52, 0x04, 0x6b, 0x65, 0x79, 0x73, 0x1a, 0x2e, 0x0a, 0x04,
	0x44, 0x61, 0x74, 0x61, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x22, 0xa0, 0x01, 0x0a,
	0x06, 0x53, 0x74, 0x72, 0x75, 0x63, 0x74, 0x12, 0x25, 0x0a, 0x06, 0x66, 0x69, 0x65, 0x6c, 0x64,
	0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x0d, 0x2e, 0x53, 0x74, 0x72, 0x75, 0x63, 0x74,
	0x2e, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x52, 0x06, 0x66, 0x69, 0x65, 0x6c, 0x64, 0x73, 0x12, 0x12,
	0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61,
	0x6d, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x73, 0x69, 0x7a, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x04,
	0x52, 0x04, 0x73, 0x69, 0x7a, 0x65, 0x1a, 0x47, 0x0a, 0x05, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x12,
	0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e,
	0x61, 0x6d, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x12, 0x16, 0x0a, 0x06, 0x6f, 0x66, 0x66, 0x73, 0x65,
	0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x06, 0x6f, 0x66, 0x66, 0x73, 0x65, 0x74, 0x42,
	0x2c, 0x5a, 0x2a, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x4e, 0x65,
	0x74, 0x68, 0x65, 0x72, 0x6d, 0x69, 0x6e, 0x64, 0x45, 0x74, 0x68, 0x2f, 0x6a, 0x75, 0x6e, 0x6f,
	0x2f, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f, 0x61, 0x62, 0x69, 0x62, 0x06, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_abi_proto_rawDescOnce sync.Once
	file_abi_proto_rawDescData = file_abi_proto_rawDesc
)

func file_abi_proto_rawDescGZIP() []byte {
	file_abi_proto_rawDescOnce.Do(func() {
		file_abi_proto_rawDescData = protoimpl.X.CompressGZIP(file_abi_proto_rawDescData)
	})
	return file_abi_proto_rawDescData
}

var file_abi_proto_msgTypes = make([]protoimpl.MessageInfo, 8)
var file_abi_proto_goTypes = []interface{}{
	(*Abi)(nil),             // 0: Abi
	(*Function)(nil),        // 1: Function
	(*AbiEvent)(nil),        // 2: AbiEvent
	(*Struct)(nil),          // 3: Struct
	(*Function_Input)(nil),  // 4: Function.Input
	(*Function_Output)(nil), // 5: Function.Output
	(*AbiEvent_Data)(nil),   // 6: AbiEvent.Data
	(*Struct_Field)(nil),    // 7: Struct.Field
}
var file_abi_proto_depIdxs = []int32{
	1, // 0: Abi.functions:type_name -> Function
	2, // 1: Abi.events:type_name -> AbiEvent
	3, // 2: Abi.structs:type_name -> Struct
	1, // 3: Abi.l1Handlers:type_name -> Function
	1, // 4: Abi.constructor:type_name -> Function
	4, // 5: Function.inputs:type_name -> Function.Input
	5, // 6: Function.outputs:type_name -> Function.Output
	6, // 7: AbiEvent.data:type_name -> AbiEvent.Data
	7, // 8: Struct.fields:type_name -> Struct.Field
	9, // [9:9] is the sub-list for method output_type
	9, // [9:9] is the sub-list for method input_type
	9, // [9:9] is the sub-list for extension type_name
	9, // [9:9] is the sub-list for extension extendee
	0, // [0:9] is the sub-list for field type_name
}

func init() { file_abi_proto_init() }
func file_abi_proto_init() {
	if File_abi_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_abi_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Abi); i {
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
		file_abi_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Function); i {
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
		file_abi_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*AbiEvent); i {
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
		file_abi_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Struct); i {
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
		file_abi_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Function_Input); i {
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
		file_abi_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Function_Output); i {
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
		file_abi_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*AbiEvent_Data); i {
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
		file_abi_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Struct_Field); i {
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
			RawDescriptor: file_abi_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   8,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_abi_proto_goTypes,
		DependencyIndexes: file_abi_proto_depIdxs,
		MessageInfos:      file_abi_proto_msgTypes,
	}.Build()
	File_abi_proto = out.File
	file_abi_proto_rawDesc = nil
	file_abi_proto_goTypes = nil
	file_abi_proto_depIdxs = nil
}
