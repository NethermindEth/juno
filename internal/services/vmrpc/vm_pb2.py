# -*- coding: utf-8 -*-
# Generated by the protocol buffer compiler.  DO NOT EDIT!
# source: vm.proto
"""Generated protocol buffer code."""
from google.protobuf import descriptor as _descriptor
from google.protobuf import descriptor_pool as _descriptor_pool
from google.protobuf import message as _message
from google.protobuf import reflection as _reflection
from google.protobuf import symbol_database as _symbol_database
# @@protoc_insertion_point(imports)

_sym_db = _symbol_database.Default()




DESCRIPTOR = _descriptor_pool.Default().AddSerializedFile(b'\n\x08vm.proto\"\x1e\n\x0fGetValueRequest\x12\x0b\n\x03key\x18\x01 \x01(\x0c\"!\n\x10GetValueResponse\x12\r\n\x05value\x18\x01 \x01(\x0c\"s\n\rVMCallRequest\x12\x10\n\x08\x63\x61lldata\x18\x01 \x03(\t\x12\x16\n\x0e\x63\x61ller_address\x18\x02 \x01(\t\x12\x18\n\x10\x63ontract_address\x18\x03 \x01(\t\x12\x0c\n\x04root\x18\x04 \x01(\t\x12\x10\n\x08selector\x18\x05 \x01(\t\"!\n\x0eVMCallResponse\x12\x0f\n\x07retdata\x18\x01 \x03(\t2C\n\x0eStorageAdapter\x12\x31\n\x08GetValue\x12\x10.GetValueRequest\x1a\x11.GetValueResponse\"\x00\x32/\n\x02VM\x12)\n\x04\x43\x61ll\x12\x0e.VMCallRequest\x1a\x0f.VMCallResponse\"\x00\x42\x37Z5github.com/NethermindEth/juno/internal/services/vmrpcb\x06proto3')



_GETVALUEREQUEST = DESCRIPTOR.message_types_by_name['GetValueRequest']
_GETVALUERESPONSE = DESCRIPTOR.message_types_by_name['GetValueResponse']
_VMCALLREQUEST = DESCRIPTOR.message_types_by_name['VMCallRequest']
_VMCALLRESPONSE = DESCRIPTOR.message_types_by_name['VMCallResponse']
GetValueRequest = _reflection.GeneratedProtocolMessageType('GetValueRequest', (_message.Message,), {
  'DESCRIPTOR' : _GETVALUEREQUEST,
  '__module__' : 'vm_pb2'
  # @@protoc_insertion_point(class_scope:GetValueRequest)
  })
_sym_db.RegisterMessage(GetValueRequest)

GetValueResponse = _reflection.GeneratedProtocolMessageType('GetValueResponse', (_message.Message,), {
  'DESCRIPTOR' : _GETVALUERESPONSE,
  '__module__' : 'vm_pb2'
  # @@protoc_insertion_point(class_scope:GetValueResponse)
  })
_sym_db.RegisterMessage(GetValueResponse)

VMCallRequest = _reflection.GeneratedProtocolMessageType('VMCallRequest', (_message.Message,), {
  'DESCRIPTOR' : _VMCALLREQUEST,
  '__module__' : 'vm_pb2'
  # @@protoc_insertion_point(class_scope:VMCallRequest)
  })
_sym_db.RegisterMessage(VMCallRequest)

VMCallResponse = _reflection.GeneratedProtocolMessageType('VMCallResponse', (_message.Message,), {
  'DESCRIPTOR' : _VMCALLRESPONSE,
  '__module__' : 'vm_pb2'
  # @@protoc_insertion_point(class_scope:VMCallResponse)
  })
_sym_db.RegisterMessage(VMCallResponse)

_STORAGEADAPTER = DESCRIPTOR.services_by_name['StorageAdapter']
_VM = DESCRIPTOR.services_by_name['VM']
if _descriptor._USE_C_DESCRIPTORS == False:

  DESCRIPTOR._options = None
  DESCRIPTOR._serialized_options = b'Z5github.com/NethermindEth/juno/internal/services/vmrpc'
  _GETVALUEREQUEST._serialized_start=12
  _GETVALUEREQUEST._serialized_end=42
  _GETVALUERESPONSE._serialized_start=44
  _GETVALUERESPONSE._serialized_end=77
  _VMCALLREQUEST._serialized_start=79
  _VMCALLREQUEST._serialized_end=194
  _VMCALLRESPONSE._serialized_start=196
  _VMCALLRESPONSE._serialized_end=229
  _STORAGEADAPTER._serialized_start=231
  _STORAGEADAPTER._serialized_end=298
  _VM._serialized_start=300
  _VM._serialized_end=347
# @@protoc_insertion_point(module_scope)
