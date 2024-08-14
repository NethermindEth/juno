// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v3.21.12
// source: kv.proto

package gen

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// KVClient is the client API for KV service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type KVClient interface {
	Version(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*VersionReply, error)
	Tx(ctx context.Context, opts ...grpc.CallOption) (KV_TxClient, error)
}

type kVClient struct {
	cc grpc.ClientConnInterface
}

func NewKVClient(cc grpc.ClientConnInterface) KVClient {
	return &kVClient{cc}
}

func (c *kVClient) Version(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*VersionReply, error) {
	out := new(VersionReply)
	err := c.cc.Invoke(ctx, "/database.KV/Version", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *kVClient) Tx(ctx context.Context, opts ...grpc.CallOption) (KV_TxClient, error) {
	stream, err := c.cc.NewStream(ctx, &KV_ServiceDesc.Streams[0], "/database.KV/Tx", opts...)
	if err != nil {
		return nil, err
	}
	x := &kVTxClient{stream}
	return x, nil
}

type KV_TxClient interface {
	Send(*Cursor) error
	Recv() (*Pair, error)
	grpc.ClientStream
}

type kVTxClient struct {
	grpc.ClientStream
}

func (x *kVTxClient) Send(m *Cursor) error {
	return x.ClientStream.SendMsg(m)
}

func (x *kVTxClient) Recv() (*Pair, error) {
	m := new(Pair)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// KVServer is the server API for KV service.
// All implementations must embed UnimplementedKVServer
// for forward compatibility
type KVServer interface {
	Version(context.Context, *emptypb.Empty) (*VersionReply, error)
	Tx(KV_TxServer) error
	mustEmbedUnimplementedKVServer()
}

// UnimplementedKVServer must be embedded to have forward compatible implementations.
type UnimplementedKVServer struct {
}

func (UnimplementedKVServer) Version(context.Context, *emptypb.Empty) (*VersionReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Version not implemented")
}
func (UnimplementedKVServer) Tx(KV_TxServer) error {
	return status.Errorf(codes.Unimplemented, "method Tx not implemented")
}
func (UnimplementedKVServer) mustEmbedUnimplementedKVServer() {}

// UnsafeKVServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to KVServer will
// result in compilation errors.
type UnsafeKVServer interface {
	mustEmbedUnimplementedKVServer()
}

func RegisterKVServer(s grpc.ServiceRegistrar, srv KVServer) {
	s.RegisterService(&KV_ServiceDesc, srv)
}

func _KV_Version_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(KVServer).Version(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/database.KV/Version",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(KVServer).Version(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _KV_Tx_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(KVServer).Tx(&kVTxServer{stream})
}

type KV_TxServer interface {
	Send(*Pair) error
	Recv() (*Cursor, error)
	grpc.ServerStream
}

type kVTxServer struct {
	grpc.ServerStream
}

func (x *kVTxServer) Send(m *Pair) error {
	return x.ServerStream.SendMsg(m)
}

func (x *kVTxServer) Recv() (*Cursor, error) {
	m := new(Cursor)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// KV_ServiceDesc is the grpc.ServiceDesc for KV service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var KV_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "database.KV",
	HandlerType: (*KVServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Version",
			Handler:    _KV_Version_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Tx",
			Handler:       _KV_Tx_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "kv.proto",
}
