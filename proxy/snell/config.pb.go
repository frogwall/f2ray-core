package snell

import (
	protocol "github.com/frogwall/f2ray-core/v5/common/protocol"
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

type Account struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Password      string                 `protobuf:"bytes,1,opt,name=password,proto3" json:"password,omitempty"`
	Cipher        string                 `protobuf:"bytes,2,opt,name=cipher,proto3" json:"cipher,omitempty"`
	Version       uint32                 `protobuf:"varint,3,opt,name=version,proto3" json:"version,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Account) Reset() {
	*x = Account{}
	mi := &file_proxy_snell_config_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Account) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Account) ProtoMessage() {}

func (x *Account) ProtoReflect() protoreflect.Message {
	mi := &file_proxy_snell_config_proto_msgTypes[0]
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
	return file_proxy_snell_config_proto_rawDescGZIP(), []int{0}
}

func (x *Account) GetPassword() string {
	if x != nil {
		return x.Password
	}
	return ""
}

func (x *Account) GetCipher() string {
	if x != nil {
		return x.Cipher
	}
	return ""
}

func (x *Account) GetVersion() uint32 {
	if x != nil {
		return x.Version
	}
	return 0
}

type ServerConfig struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	User          *protocol.User         `protobuf:"bytes,1,opt,name=user,proto3" json:"user,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ServerConfig) Reset() {
	*x = ServerConfig{}
	mi := &file_proxy_snell_config_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ServerConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ServerConfig) ProtoMessage() {}

func (x *ServerConfig) ProtoReflect() protoreflect.Message {
	mi := &file_proxy_snell_config_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ServerConfig.ProtoReflect.Descriptor instead.
func (*ServerConfig) Descriptor() ([]byte, []int) {
	return file_proxy_snell_config_proto_rawDescGZIP(), []int{1}
}

func (x *ServerConfig) GetUser() *protocol.User {
	if x != nil {
		return x.User
	}
	return nil
}

type ClientConfig struct {
	state         protoimpl.MessageState     `protogen:"open.v1"`
	Server        []*protocol.ServerEndpoint `protobuf:"bytes,1,rep,name=server,proto3" json:"server,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ClientConfig) Reset() {
	*x = ClientConfig{}
	mi := &file_proxy_snell_config_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ClientConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClientConfig) ProtoMessage() {}

func (x *ClientConfig) ProtoReflect() protoreflect.Message {
	mi := &file_proxy_snell_config_proto_msgTypes[2]
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
	return file_proxy_snell_config_proto_rawDescGZIP(), []int{2}
}

func (x *ClientConfig) GetServer() []*protocol.ServerEndpoint {
	if x != nil {
		return x.Server
	}
	return nil
}

var File_proxy_snell_config_proto protoreflect.FileDescriptor

const file_proxy_snell_config_proto_rawDesc = "" +
	"\n" +
	"\x18proxy/snell/config.proto\x12\x16v2ray.core.proxy.snell\x1a\x1acommon/protocol/user.proto\x1a!common/protocol/server_spec.proto\"W\n" +
	"\aAccount\x12\x1a\n" +
	"\bpassword\x18\x01 \x01(\tR\bpassword\x12\x16\n" +
	"\x06cipher\x18\x02 \x01(\tR\x06cipher\x12\x18\n" +
	"\aversion\x18\x03 \x01(\rR\aversion\"D\n" +
	"\fServerConfig\x124\n" +
	"\x04user\x18\x01 \x01(\v2 .v2ray.core.common.protocol.UserR\x04user\"R\n" +
	"\fClientConfig\x12B\n" +
	"\x06server\x18\x01 \x03(\v2*.v2ray.core.common.protocol.ServerEndpointR\x06serverBf\n" +
	"\x1acom.v2ray.core.proxy.snellP\x01Z-github.com/frogwall/f2ray-core/v5/proxy/snell\xaa\x02\x16V2Ray.Core.Proxy.Snellb\x06proto3"

var (
	file_proxy_snell_config_proto_rawDescOnce sync.Once
	file_proxy_snell_config_proto_rawDescData []byte
)

func file_proxy_snell_config_proto_rawDescGZIP() []byte {
	file_proxy_snell_config_proto_rawDescOnce.Do(func() {
		file_proxy_snell_config_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_proxy_snell_config_proto_rawDesc), len(file_proxy_snell_config_proto_rawDesc)))
	})
	return file_proxy_snell_config_proto_rawDescData
}

var file_proxy_snell_config_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_proxy_snell_config_proto_goTypes = []any{
	(*Account)(nil),                 // 0: v2ray.core.proxy.snell.Account
	(*ServerConfig)(nil),            // 1: v2ray.core.proxy.snell.ServerConfig
	(*ClientConfig)(nil),            // 2: v2ray.core.proxy.snell.ClientConfig
	(*protocol.User)(nil),           // 3: v2ray.core.common.protocol.User
	(*protocol.ServerEndpoint)(nil), // 4: v2ray.core.common.protocol.ServerEndpoint
}
var file_proxy_snell_config_proto_depIdxs = []int32{
	3, // 0: v2ray.core.proxy.snell.ServerConfig.user:type_name -> v2ray.core.common.protocol.User
	4, // 1: v2ray.core.proxy.snell.ClientConfig.server:type_name -> v2ray.core.common.protocol.ServerEndpoint
	2, // [2:2] is the sub-list for method output_type
	2, // [2:2] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_proxy_snell_config_proto_init() }
func file_proxy_snell_config_proto_init() {
	if File_proxy_snell_config_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_proxy_snell_config_proto_rawDesc), len(file_proxy_snell_config_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_proxy_snell_config_proto_goTypes,
		DependencyIndexes: file_proxy_snell_config_proto_depIdxs,
		MessageInfos:      file_proxy_snell_config_proto_msgTypes,
	}.Build()
	File_proxy_snell_config_proto = out.File
	file_proxy_snell_config_proto_goTypes = nil
	file_proxy_snell_config_proto_depIdxs = nil
}
