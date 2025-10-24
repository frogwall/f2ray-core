package anytls

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

// Account represents an AnyTLS user account
type Account struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Password      string                 `protobuf:"bytes,1,opt,name=password,proto3" json:"password,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Account) Reset() {
	*x = Account{}
	mi := &file_proxy_anytls_config_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Account) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Account) ProtoMessage() {}

func (x *Account) ProtoReflect() protoreflect.Message {
	mi := &file_proxy_anytls_config_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Account.ProtoReflect.Descriptor instead.
func (*Account) Descriptor() ([]byte, []int) {
	return file_proxy_anytls_config_proto_rawDescGZIP(), []int{0}
}

func (x *Account) GetPassword() string {
	if x != nil {
		return x.Password
	}
	return ""
}

// ServerEndpoint represents an AnyTLS server configuration
type ServerEndpoint struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Address       string                 `protobuf:"bytes,1,opt,name=address,proto3" json:"address,omitempty"`
	Port          uint32                 `protobuf:"varint,2,opt,name=port,proto3" json:"port,omitempty"`
	Password      string                 `protobuf:"bytes,3,opt,name=password,proto3" json:"password,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ServerEndpoint) Reset() {
	*x = ServerEndpoint{}
	mi := &file_proxy_anytls_config_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ServerEndpoint) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ServerEndpoint) ProtoMessage() {}

func (x *ServerEndpoint) ProtoReflect() protoreflect.Message {
	mi := &file_proxy_anytls_config_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ServerEndpoint.ProtoReflect.Descriptor instead.
func (*ServerEndpoint) Descriptor() ([]byte, []int) {
	return file_proxy_anytls_config_proto_rawDescGZIP(), []int{1}
}

func (x *ServerEndpoint) GetAddress() string {
	if x != nil {
		return x.Address
	}
	return ""
}

func (x *ServerEndpoint) GetPort() uint32 {
	if x != nil {
		return x.Port
	}
	return 0
}

func (x *ServerEndpoint) GetPassword() string {
	if x != nil {
		return x.Password
	}
	return ""
}

// ClientConfig is the protobuf config for AnyTLS outbound client
type ClientConfig struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Server is the upstream AnyTLS server endpoint
	Servers []*ServerEndpoint `protobuf:"bytes,1,rep,name=servers,proto3" json:"servers,omitempty"`
	// Idle session check interval in seconds (default: 30)
	IdleSessionCheckInterval uint32 `protobuf:"varint,2,opt,name=idle_session_check_interval,json=idleSessionCheckInterval,proto3" json:"idle_session_check_interval,omitempty"`
	// Idle session timeout in seconds (default: 30)
	IdleSessionTimeout uint32 `protobuf:"varint,3,opt,name=idle_session_timeout,json=idleSessionTimeout,proto3" json:"idle_session_timeout,omitempty"`
	// Minimum number of idle sessions to keep (default: 0)
	MinIdleSession uint32 `protobuf:"varint,4,opt,name=min_idle_session,json=minIdleSession,proto3" json:"min_idle_session,omitempty"`
	unknownFields  protoimpl.UnknownFields
	sizeCache      protoimpl.SizeCache
}

func (x *ClientConfig) Reset() {
	*x = ClientConfig{}
	mi := &file_proxy_anytls_config_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ClientConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClientConfig) ProtoMessage() {}

func (x *ClientConfig) ProtoReflect() protoreflect.Message {
	mi := &file_proxy_anytls_config_proto_msgTypes[2]
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
	return file_proxy_anytls_config_proto_rawDescGZIP(), []int{2}
}

func (x *ClientConfig) GetServers() []*ServerEndpoint {
	if x != nil {
		return x.Servers
	}
	return nil
}

func (x *ClientConfig) GetIdleSessionCheckInterval() uint32 {
	if x != nil {
		return x.IdleSessionCheckInterval
	}
	return 0
}

func (x *ClientConfig) GetIdleSessionTimeout() uint32 {
	if x != nil {
		return x.IdleSessionTimeout
	}
	return 0
}

func (x *ClientConfig) GetMinIdleSession() uint32 {
	if x != nil {
		return x.MinIdleSession
	}
	return 0
}

var File_proxy_anytls_config_proto protoreflect.FileDescriptor

const file_proxy_anytls_config_proto_rawDesc = "" +
	"\n" +
	"\x19proxy/anytls/config.proto\x12\x17v2ray.core.proxy.anytls\"%\n" +
	"\aAccount\x12\x1a\n" +
	"\bpassword\x18\x01 \x01(\tR\bpassword\"Z\n" +
	"\x0eServerEndpoint\x12\x18\n" +
	"\aaddress\x18\x01 \x01(\tR\aaddress\x12\x12\n" +
	"\x04port\x18\x02 \x01(\rR\x04port\x12\x1a\n" +
	"\bpassword\x18\x03 \x01(\tR\bpassword\"\xec\x01\n" +
	"\fClientConfig\x12A\n" +
	"\aservers\x18\x01 \x03(\v2'.v2ray.core.proxy.anytls.ServerEndpointR\aservers\x12=\n" +
	"\x1bidle_session_check_interval\x18\x02 \x01(\rR\x18idleSessionCheckInterval\x120\n" +
	"\x14idle_session_timeout\x18\x03 \x01(\rR\x12idleSessionTimeout\x12(\n" +
	"\x10min_idle_session\x18\x04 \x01(\rR\x0eminIdleSessionBi\n" +
	"\x1bcom.v2ray.core.proxy.anytlsP\x01Z.github.com/frogwall/f2ray-core/v5/proxy/anytls\xaa\x02\x17V2Ray.Core.Proxy.AnyTLSb\x06proto3"

var (
	file_proxy_anytls_config_proto_rawDescOnce sync.Once
	file_proxy_anytls_config_proto_rawDescData []byte
)

func file_proxy_anytls_config_proto_rawDescGZIP() []byte {
	file_proxy_anytls_config_proto_rawDescOnce.Do(func() {
		file_proxy_anytls_config_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_proxy_anytls_config_proto_rawDesc), len(file_proxy_anytls_config_proto_rawDesc)))
	})
	return file_proxy_anytls_config_proto_rawDescData
}

var file_proxy_anytls_config_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_proxy_anytls_config_proto_goTypes = []any{
	(*Account)(nil),        // 0: v2ray.core.proxy.anytls.Account
	(*ServerEndpoint)(nil), // 1: v2ray.core.proxy.anytls.ServerEndpoint
	(*ClientConfig)(nil),   // 2: v2ray.core.proxy.anytls.ClientConfig
}
var file_proxy_anytls_config_proto_depIdxs = []int32{
	1, // 0: v2ray.core.proxy.anytls.ClientConfig.servers:type_name -> v2ray.core.proxy.anytls.ServerEndpoint
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_proxy_anytls_config_proto_init() }
func file_proxy_anytls_config_proto_init() {
	if File_proxy_anytls_config_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_proxy_anytls_config_proto_rawDesc), len(file_proxy_anytls_config_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_proxy_anytls_config_proto_goTypes,
		DependencyIndexes: file_proxy_anytls_config_proto_depIdxs,
		MessageInfos:      file_proxy_anytls_config_proto_msgTypes,
	}.Build()
	File_proxy_anytls_config_proto = out.File
	file_proxy_anytls_config_proto_goTypes = nil
	file_proxy_anytls_config_proto_depIdxs = nil
}
