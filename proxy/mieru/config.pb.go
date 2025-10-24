package mieru

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

type Server struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Address       string                 `protobuf:"bytes,1,opt,name=address,proto3" json:"address,omitempty"`
	Port          int32                  `protobuf:"varint,2,opt,name=port,proto3" json:"port,omitempty"`
	Username      string                 `protobuf:"bytes,3,opt,name=username,proto3" json:"username,omitempty"`
	Password      string                 `protobuf:"bytes,4,opt,name=password,proto3" json:"password,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Server) Reset() {
	*x = Server{}
	mi := &file_proxy_mieru_config_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Server) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Server) ProtoMessage() {}

func (x *Server) ProtoReflect() protoreflect.Message {
	mi := &file_proxy_mieru_config_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Server.ProtoReflect.Descriptor instead.
func (*Server) Descriptor() ([]byte, []int) {
	return file_proxy_mieru_config_proto_rawDescGZIP(), []int{0}
}

func (x *Server) GetAddress() string {
	if x != nil {
		return x.Address
	}
	return ""
}

func (x *Server) GetPort() int32 {
	if x != nil {
		return x.Port
	}
	return 0
}

func (x *Server) GetUsername() string {
	if x != nil {
		return x.Username
	}
	return ""
}

func (x *Server) GetPassword() string {
	if x != nil {
		return x.Password
	}
	return ""
}

type ClientConfig struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Servers       []*Server              `protobuf:"bytes,1,rep,name=servers,proto3" json:"servers,omitempty"`
	Mtu           int32                  `protobuf:"varint,2,opt,name=mtu,proto3" json:"mtu,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ClientConfig) Reset() {
	*x = ClientConfig{}
	mi := &file_proxy_mieru_config_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ClientConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClientConfig) ProtoMessage() {}

func (x *ClientConfig) ProtoReflect() protoreflect.Message {
	mi := &file_proxy_mieru_config_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ClientConfig.ProtoReflect.Descriptor instead.
func (*ClientConfig) Descriptor() ([]byte, []int) {
	return file_proxy_mieru_config_proto_rawDescGZIP(), []int{1}
}

func (x *ClientConfig) GetServers() []*Server {
	if x != nil {
		return x.Servers
	}
	return nil
}

func (x *ClientConfig) GetMtu() int32 {
	if x != nil {
		return x.Mtu
	}
	return 0
}

var File_proxy_mieru_config_proto protoreflect.FileDescriptor

const file_proxy_mieru_config_proto_rawDesc = "" +
	"\n" +
	"\x18proxy/mieru/config.proto\x12\x16v2ray.core.proxy.mieru\"n\n" +
	"\x06Server\x12\x18\n" +
	"\aaddress\x18\x01 \x01(\tR\aaddress\x12\x12\n" +
	"\x04port\x18\x02 \x01(\x05R\x04port\x12\x1a\n" +
	"\busername\x18\x03 \x01(\tR\busername\x12\x1a\n" +
	"\bpassword\x18\x04 \x01(\tR\bpassword\"Z\n" +
	"\fClientConfig\x128\n" +
	"\aservers\x18\x01 \x03(\v2\x1e.v2ray.core.proxy.mieru.ServerR\aservers\x12\x10\n" +
	"\x03mtu\x18\x02 \x01(\x05R\x03mtuBc\n" +
	"\x1acom.v2ray.core.proxy.mieruP\x01Z*github.com/v2fly/v2ray-core/v5/proxy/mieru\xaa\x02\x16V2Ray.Core.Proxy.Mierub\x06proto3"

var (
	file_proxy_mieru_config_proto_rawDescOnce sync.Once
	file_proxy_mieru_config_proto_rawDescData []byte
)

func file_proxy_mieru_config_proto_rawDescGZIP() []byte {
	file_proxy_mieru_config_proto_rawDescOnce.Do(func() {
		file_proxy_mieru_config_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_proxy_mieru_config_proto_rawDesc), len(file_proxy_mieru_config_proto_rawDesc)))
	})
	return file_proxy_mieru_config_proto_rawDescData
}

var file_proxy_mieru_config_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_proxy_mieru_config_proto_goTypes = []any{
	(*Server)(nil),       // 0: v2ray.core.proxy.mieru.Server
	(*ClientConfig)(nil), // 1: v2ray.core.proxy.mieru.ClientConfig
}
var file_proxy_mieru_config_proto_depIdxs = []int32{
	0, // 0: v2ray.core.proxy.mieru.ClientConfig.servers:type_name -> v2ray.core.proxy.mieru.Server
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_proxy_mieru_config_proto_init() }
func file_proxy_mieru_config_proto_init() {
	if File_proxy_mieru_config_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_proxy_mieru_config_proto_rawDesc), len(file_proxy_mieru_config_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_proxy_mieru_config_proto_goTypes,
		DependencyIndexes: file_proxy_mieru_config_proto_depIdxs,
		MessageInfos:      file_proxy_mieru_config_proto_msgTypes,
	}.Build()
	File_proxy_mieru_config_proto = out.File
	file_proxy_mieru_config_proto_goTypes = nil
	file_proxy_mieru_config_proto_depIdxs = nil
}
