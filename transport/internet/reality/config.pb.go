package reality

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

type Config struct {
	state       protoimpl.MessageState `protogen:"open.v1"`
	ServerName  string                 `protobuf:"bytes,1,opt,name=server_name,json=serverName,proto3" json:"server_name,omitempty"`
	PublicKey   []byte                 `protobuf:"bytes,2,opt,name=public_key,json=publicKey,proto3" json:"public_key,omitempty"` // X25519 public key bytes (client)
	ShortId     []byte                 `protobuf:"bytes,3,opt,name=short_id,json=shortId,proto3" json:"short_id,omitempty"`       // 8~16 bytes
	Fingerprint string                 `protobuf:"bytes,4,opt,name=fingerprint,proto3" json:"fingerprint,omitempty"`              // uTLS fingerprint name
	Show        bool                   `protobuf:"varint,5,opt,name=show,proto3" json:"show,omitempty"`                           // verbose for debugging
	SpiderX     string                 `protobuf:"bytes,6,opt,name=spider_x,json=spiderX,proto3" json:"spider_x,omitempty"`       // optional client-provided path (compat)
	// Server-only fields
	PrivateKey    []byte   `protobuf:"bytes,7,opt,name=private_key,json=privateKey,proto3" json:"private_key,omitempty"`    // X25519 private key bytes (server)
	ServerNames   []string `protobuf:"bytes,8,rep,name=server_names,json=serverNames,proto3" json:"server_names,omitempty"` // Allowed server names
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Config) Reset() {
	*x = Config{}
	mi := &file_transport_internet_reality_config_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Config) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Config) ProtoMessage() {}

func (x *Config) ProtoReflect() protoreflect.Message {
	mi := &file_transport_internet_reality_config_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Config.ProtoReflect.Descriptor instead.
func (*Config) Descriptor() ([]byte, []int) {
	return file_transport_internet_reality_config_proto_rawDescGZIP(), []int{0}
}

func (x *Config) GetServerName() string {
	if x != nil {
		return x.ServerName
	}
	return ""
}

func (x *Config) GetPublicKey() []byte {
	if x != nil {
		return x.PublicKey
	}
	return nil
}

func (x *Config) GetShortId() []byte {
	if x != nil {
		return x.ShortId
	}
	return nil
}

func (x *Config) GetFingerprint() string {
	if x != nil {
		return x.Fingerprint
	}
	return ""
}

func (x *Config) GetShow() bool {
	if x != nil {
		return x.Show
	}
	return false
}

func (x *Config) GetSpiderX() string {
	if x != nil {
		return x.SpiderX
	}
	return ""
}

func (x *Config) GetPrivateKey() []byte {
	if x != nil {
		return x.PrivateKey
	}
	return nil
}

func (x *Config) GetServerNames() []string {
	if x != nil {
		return x.ServerNames
	}
	return nil
}

var File_transport_internet_reality_config_proto protoreflect.FileDescriptor

const file_transport_internet_reality_config_proto_rawDesc = "" +
	"\n" +
	"'transport/internet/reality/config.proto\x12%v2ray.core.transport.internet.reality\"\xf8\x01\n" +
	"\x06Config\x12\x1f\n" +
	"\vserver_name\x18\x01 \x01(\tR\n" +
	"serverName\x12\x1d\n" +
	"\n" +
	"public_key\x18\x02 \x01(\fR\tpublicKey\x12\x19\n" +
	"\bshort_id\x18\x03 \x01(\fR\ashortId\x12 \n" +
	"\vfingerprint\x18\x04 \x01(\tR\vfingerprint\x12\x12\n" +
	"\x04show\x18\x05 \x01(\bR\x04show\x12\x19\n" +
	"\bspider_x\x18\x06 \x01(\tR\aspiderX\x12\x1f\n" +
	"\vprivate_key\x18\a \x01(\fR\n" +
	"privateKey\x12!\n" +
	"\fserver_names\x18\b \x03(\tR\vserverNamesB\x93\x01\n" +
	")com.v2ray.core.transport.internet.realityP\x01Z<github.com/frogwall/f2ray-core/v5/transport/internet/reality\xaa\x02%V2Ray.Core.Transport.Internet.Realityb\x06proto3"

var (
	file_transport_internet_reality_config_proto_rawDescOnce sync.Once
	file_transport_internet_reality_config_proto_rawDescData []byte
)

func file_transport_internet_reality_config_proto_rawDescGZIP() []byte {
	file_transport_internet_reality_config_proto_rawDescOnce.Do(func() {
		file_transport_internet_reality_config_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_transport_internet_reality_config_proto_rawDesc), len(file_transport_internet_reality_config_proto_rawDesc)))
	})
	return file_transport_internet_reality_config_proto_rawDescData
}

var file_transport_internet_reality_config_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_transport_internet_reality_config_proto_goTypes = []any{
	(*Config)(nil), // 0: v2ray.core.transport.internet.reality.Config
}
var file_transport_internet_reality_config_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_transport_internet_reality_config_proto_init() }
func file_transport_internet_reality_config_proto_init() {
	if File_transport_internet_reality_config_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_transport_internet_reality_config_proto_rawDesc), len(file_transport_internet_reality_config_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_transport_internet_reality_config_proto_goTypes,
		DependencyIndexes: file_transport_internet_reality_config_proto_depIdxs,
		MessageInfos:      file_transport_internet_reality_config_proto_msgTypes,
	}.Build()
	File_transport_internet_reality_config_proto = out.File
	file_transport_internet_reality_config_proto_goTypes = nil
	file_transport_internet_reality_config_proto_depIdxs = nil
}
