package naive

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

type Account struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Username      string                 `protobuf:"bytes,1,opt,name=username,proto3" json:"username,omitempty"`
	Password      string                 `protobuf:"bytes,2,opt,name=password,proto3" json:"password,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Account) Reset() {
	*x = Account{}
	mi := &file_proxy_naive_config_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Account) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Account) ProtoMessage() {}

func (x *Account) ProtoReflect() protoreflect.Message {
	mi := &file_proxy_naive_config_proto_msgTypes[0]
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
	return file_proxy_naive_config_proto_rawDescGZIP(), []int{0}
}

func (x *Account) GetUsername() string {
	if x != nil {
		return x.Username
	}
	return ""
}

func (x *Account) GetPassword() string {
	if x != nil {
		return x.Password
	}
	return ""
}

// NaiveServerEndpoint extends ServerEndpoint with naive-specific fields
type NaiveServerEndpoint struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Address       string                 `protobuf:"bytes,1,opt,name=address,proto3" json:"address,omitempty"`
	Port          uint32                 `protobuf:"varint,2,opt,name=port,proto3" json:"port,omitempty"`
	Username      string                 `protobuf:"bytes,3,opt,name=username,proto3" json:"username,omitempty"`
	Password      string                 `protobuf:"bytes,4,opt,name=password,proto3" json:"password,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *NaiveServerEndpoint) Reset() {
	*x = NaiveServerEndpoint{}
	mi := &file_proxy_naive_config_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *NaiveServerEndpoint) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NaiveServerEndpoint) ProtoMessage() {}

func (x *NaiveServerEndpoint) ProtoReflect() protoreflect.Message {
	mi := &file_proxy_naive_config_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NaiveServerEndpoint.ProtoReflect.Descriptor instead.
func (*NaiveServerEndpoint) Descriptor() ([]byte, []int) {
	return file_proxy_naive_config_proto_rawDescGZIP(), []int{1}
}

func (x *NaiveServerEndpoint) GetAddress() string {
	if x != nil {
		return x.Address
	}
	return ""
}

func (x *NaiveServerEndpoint) GetPort() uint32 {
	if x != nil {
		return x.Port
	}
	return 0
}

func (x *NaiveServerEndpoint) GetUsername() string {
	if x != nil {
		return x.Username
	}
	return ""
}

func (x *NaiveServerEndpoint) GetPassword() string {
	if x != nil {
		return x.Password
	}
	return ""
}

// ClientConfig is the protobuf config for naive client.
type ClientConfig struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Server is a list of upstream naive server endpoints.
	Servers       []*NaiveServerEndpoint `protobuf:"bytes,1,rep,name=servers,proto3" json:"servers,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ClientConfig) Reset() {
	*x = ClientConfig{}
	mi := &file_proxy_naive_config_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ClientConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClientConfig) ProtoMessage() {}

func (x *ClientConfig) ProtoReflect() protoreflect.Message {
	mi := &file_proxy_naive_config_proto_msgTypes[2]
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
	return file_proxy_naive_config_proto_rawDescGZIP(), []int{2}
}

func (x *ClientConfig) GetServers() []*NaiveServerEndpoint {
	if x != nil {
		return x.Servers
	}
	return nil
}

var File_proxy_naive_config_proto protoreflect.FileDescriptor

const file_proxy_naive_config_proto_rawDesc = "" +
	"\n" +
	"\x18proxy/naive/config.proto\x12\x16v2ray.core.proxy.naive\"A\n" +
	"\aAccount\x12\x1a\n" +
	"\busername\x18\x01 \x01(\tR\busername\x12\x1a\n" +
	"\bpassword\x18\x02 \x01(\tR\bpassword\"{\n" +
	"\x13NaiveServerEndpoint\x12\x18\n" +
	"\aaddress\x18\x01 \x01(\tR\aaddress\x12\x12\n" +
	"\x04port\x18\x02 \x01(\rR\x04port\x12\x1a\n" +
	"\busername\x18\x03 \x01(\tR\busername\x12\x1a\n" +
	"\bpassword\x18\x04 \x01(\tR\bpassword\"U\n" +
	"\fClientConfig\x12E\n" +
	"\aservers\x18\x01 \x03(\v2+.v2ray.core.proxy.naive.NaiveServerEndpointR\aserversBf\n" +
	"\x1acom.v2ray.core.proxy.naiveP\x01Z-github.com/frogwall/f2ray-core/v5/proxy/naive\xaa\x02\x16V2Ray.Core.Proxy.Naiveb\x06proto3"

var (
	file_proxy_naive_config_proto_rawDescOnce sync.Once
	file_proxy_naive_config_proto_rawDescData []byte
)

func file_proxy_naive_config_proto_rawDescGZIP() []byte {
	file_proxy_naive_config_proto_rawDescOnce.Do(func() {
		file_proxy_naive_config_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_proxy_naive_config_proto_rawDesc), len(file_proxy_naive_config_proto_rawDesc)))
	})
	return file_proxy_naive_config_proto_rawDescData
}

var file_proxy_naive_config_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_proxy_naive_config_proto_goTypes = []any{
	(*Account)(nil),             // 0: v2ray.core.proxy.naive.Account
	(*NaiveServerEndpoint)(nil), // 1: v2ray.core.proxy.naive.NaiveServerEndpoint
	(*ClientConfig)(nil),        // 2: v2ray.core.proxy.naive.ClientConfig
}
var file_proxy_naive_config_proto_depIdxs = []int32{
	1, // 0: v2ray.core.proxy.naive.ClientConfig.servers:type_name -> v2ray.core.proxy.naive.NaiveServerEndpoint
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_proxy_naive_config_proto_init() }
func file_proxy_naive_config_proto_init() {
	if File_proxy_naive_config_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_proxy_naive_config_proto_rawDesc), len(file_proxy_naive_config_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_proxy_naive_config_proto_goTypes,
		DependencyIndexes: file_proxy_naive_config_proto_depIdxs,
		MessageInfos:      file_proxy_naive_config_proto_msgTypes,
	}.Build()
	File_proxy_naive_config_proto = out.File
	file_proxy_naive_config_proto_goTypes = nil
	file_proxy_naive_config_proto_depIdxs = nil
}
