package tuic

import (
	protocol "github.com/frogwall/f2ray-core/v5/common/protocol"
	_ "github.com/frogwall/f2ray-core/v5/common/protoext"
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
	Uuid          string                 `protobuf:"bytes,1,opt,name=uuid,proto3" json:"uuid,omitempty"`
	Password      string                 `protobuf:"bytes,2,opt,name=password,proto3" json:"password,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Account) Reset() {
	*x = Account{}
	mi := &file_proxy_tuic_config_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Account) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Account) ProtoMessage() {}

func (x *Account) ProtoReflect() protoreflect.Message {
	mi := &file_proxy_tuic_config_proto_msgTypes[0]
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
	return file_proxy_tuic_config_proto_rawDescGZIP(), []int{0}
}

func (x *Account) GetUuid() string {
	if x != nil {
		return x.Uuid
	}
	return ""
}

func (x *Account) GetPassword() string {
	if x != nil {
		return x.Password
	}
	return ""
}

type QUICConfig struct {
	state                          protoimpl.MessageState `protogen:"open.v1"`
	InitialStreamReceiveWindow     uint64                 `protobuf:"varint,1,opt,name=initial_stream_receive_window,json=initialStreamReceiveWindow,proto3" json:"initial_stream_receive_window,omitempty"`
	MaxStreamReceiveWindow         uint64                 `protobuf:"varint,2,opt,name=max_stream_receive_window,json=maxStreamReceiveWindow,proto3" json:"max_stream_receive_window,omitempty"`
	InitialConnectionReceiveWindow uint64                 `protobuf:"varint,3,opt,name=initial_connection_receive_window,json=initialConnectionReceiveWindow,proto3" json:"initial_connection_receive_window,omitempty"`
	MaxConnectionReceiveWindow     uint64                 `protobuf:"varint,4,opt,name=max_connection_receive_window,json=maxConnectionReceiveWindow,proto3" json:"max_connection_receive_window,omitempty"`
	MaxIdleTimeout                 int64                  `protobuf:"varint,5,opt,name=max_idle_timeout,json=maxIdleTimeout,proto3" json:"max_idle_timeout,omitempty"`    // in seconds
	KeepAlivePeriod                int64                  `protobuf:"varint,6,opt,name=keep_alive_period,json=keepAlivePeriod,proto3" json:"keep_alive_period,omitempty"` // in seconds
	DisablePathMtuDiscovery        bool                   `protobuf:"varint,7,opt,name=disable_path_mtu_discovery,json=disablePathMtuDiscovery,proto3" json:"disable_path_mtu_discovery,omitempty"`
	unknownFields                  protoimpl.UnknownFields
	sizeCache                      protoimpl.SizeCache
}

func (x *QUICConfig) Reset() {
	*x = QUICConfig{}
	mi := &file_proxy_tuic_config_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *QUICConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*QUICConfig) ProtoMessage() {}

func (x *QUICConfig) ProtoReflect() protoreflect.Message {
	mi := &file_proxy_tuic_config_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use QUICConfig.ProtoReflect.Descriptor instead.
func (*QUICConfig) Descriptor() ([]byte, []int) {
	return file_proxy_tuic_config_proto_rawDescGZIP(), []int{1}
}

func (x *QUICConfig) GetInitialStreamReceiveWindow() uint64 {
	if x != nil {
		return x.InitialStreamReceiveWindow
	}
	return 0
}

func (x *QUICConfig) GetMaxStreamReceiveWindow() uint64 {
	if x != nil {
		return x.MaxStreamReceiveWindow
	}
	return 0
}

func (x *QUICConfig) GetInitialConnectionReceiveWindow() uint64 {
	if x != nil {
		return x.InitialConnectionReceiveWindow
	}
	return 0
}

func (x *QUICConfig) GetMaxConnectionReceiveWindow() uint64 {
	if x != nil {
		return x.MaxConnectionReceiveWindow
	}
	return 0
}

func (x *QUICConfig) GetMaxIdleTimeout() int64 {
	if x != nil {
		return x.MaxIdleTimeout
	}
	return 0
}

func (x *QUICConfig) GetKeepAlivePeriod() int64 {
	if x != nil {
		return x.KeepAlivePeriod
	}
	return 0
}

func (x *QUICConfig) GetDisablePathMtuDiscovery() bool {
	if x != nil {
		return x.DisablePathMtuDiscovery
	}
	return false
}

type ClientConfig struct {
	state                 protoimpl.MessageState     `protogen:"open.v1"`
	Server                []*protocol.ServerEndpoint `protobuf:"bytes,1,rep,name=server,proto3" json:"server,omitempty"`
	UdpRelayMode          string                     `protobuf:"bytes,2,opt,name=udp_relay_mode,json=udpRelayMode,proto3" json:"udp_relay_mode,omitempty"`              // "native" or "quic"
	CongestionControl     string                     `protobuf:"bytes,3,opt,name=congestion_control,json=congestionControl,proto3" json:"congestion_control,omitempty"` // congestion control algorithm
	Quic                  *QUICConfig                `protobuf:"bytes,4,opt,name=quic,proto3" json:"quic,omitempty"`
	ReduceRtt             bool                       `protobuf:"varint,5,opt,name=reduce_rtt,json=reduceRtt,proto3" json:"reduce_rtt,omitempty"` // enable 0-RTT handshake
	MaxUdpRelayPacketSize int32                      `protobuf:"varint,6,opt,name=max_udp_relay_packet_size,json=maxUdpRelayPacketSize,proto3" json:"max_udp_relay_packet_size,omitempty"`
	unknownFields         protoimpl.UnknownFields
	sizeCache             protoimpl.SizeCache
}

func (x *ClientConfig) Reset() {
	*x = ClientConfig{}
	mi := &file_proxy_tuic_config_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ClientConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClientConfig) ProtoMessage() {}

func (x *ClientConfig) ProtoReflect() protoreflect.Message {
	mi := &file_proxy_tuic_config_proto_msgTypes[2]
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
	return file_proxy_tuic_config_proto_rawDescGZIP(), []int{2}
}

func (x *ClientConfig) GetServer() []*protocol.ServerEndpoint {
	if x != nil {
		return x.Server
	}
	return nil
}

func (x *ClientConfig) GetUdpRelayMode() string {
	if x != nil {
		return x.UdpRelayMode
	}
	return ""
}

func (x *ClientConfig) GetCongestionControl() string {
	if x != nil {
		return x.CongestionControl
	}
	return ""
}

func (x *ClientConfig) GetQuic() *QUICConfig {
	if x != nil {
		return x.Quic
	}
	return nil
}

func (x *ClientConfig) GetReduceRtt() bool {
	if x != nil {
		return x.ReduceRtt
	}
	return false
}

func (x *ClientConfig) GetMaxUdpRelayPacketSize() int32 {
	if x != nil {
		return x.MaxUdpRelayPacketSize
	}
	return 0
}

var File_proxy_tuic_config_proto protoreflect.FileDescriptor

const file_proxy_tuic_config_proto_rawDesc = "" +
	"\n" +
	"\x17proxy/tuic/config.proto\x12\x15v2ray.core.proxy.tuic\x1a!common/protocol/server_spec.proto\x1a common/protoext/extensions.proto\"9\n" +
	"\aAccount\x12\x12\n" +
	"\x04uuid\x18\x01 \x01(\tR\x04uuid\x12\x1a\n" +
	"\bpassword\x18\x02 \x01(\tR\bpassword\"\xab\x03\n" +
	"\n" +
	"QUICConfig\x12A\n" +
	"\x1dinitial_stream_receive_window\x18\x01 \x01(\x04R\x1ainitialStreamReceiveWindow\x129\n" +
	"\x19max_stream_receive_window\x18\x02 \x01(\x04R\x16maxStreamReceiveWindow\x12I\n" +
	"!initial_connection_receive_window\x18\x03 \x01(\x04R\x1einitialConnectionReceiveWindow\x12A\n" +
	"\x1dmax_connection_receive_window\x18\x04 \x01(\x04R\x1amaxConnectionReceiveWindow\x12(\n" +
	"\x10max_idle_timeout\x18\x05 \x01(\x03R\x0emaxIdleTimeout\x12*\n" +
	"\x11keep_alive_period\x18\x06 \x01(\x03R\x0fkeepAlivePeriod\x12;\n" +
	"\x1adisable_path_mtu_discovery\x18\a \x01(\bR\x17disablePathMtuDiscovery\"\xcd\x02\n" +
	"\fClientConfig\x12B\n" +
	"\x06server\x18\x01 \x03(\v2*.v2ray.core.common.protocol.ServerEndpointR\x06server\x12$\n" +
	"\x0eudp_relay_mode\x18\x02 \x01(\tR\fudpRelayMode\x12-\n" +
	"\x12congestion_control\x18\x03 \x01(\tR\x11congestionControl\x125\n" +
	"\x04quic\x18\x04 \x01(\v2!.v2ray.core.proxy.tuic.QUICConfigR\x04quic\x12\x1d\n" +
	"\n" +
	"reduce_rtt\x18\x05 \x01(\bR\treduceRtt\x128\n" +
	"\x19max_udp_relay_packet_size\x18\x06 \x01(\x05R\x15maxUdpRelayPacketSize:\x14\x82\xb5\x18\x10\n" +
	"\boutbound\x12\x04tuicBA\n" +
	"\x19com.v2ray.core.proxy.tuicP\x01Z\n" +
	"proxy/tuic\xaa\x02\x15V2Ray.Core.Proxy.Tuicb\x06proto3"

var (
	file_proxy_tuic_config_proto_rawDescOnce sync.Once
	file_proxy_tuic_config_proto_rawDescData []byte
)

func file_proxy_tuic_config_proto_rawDescGZIP() []byte {
	file_proxy_tuic_config_proto_rawDescOnce.Do(func() {
		file_proxy_tuic_config_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_proxy_tuic_config_proto_rawDesc), len(file_proxy_tuic_config_proto_rawDesc)))
	})
	return file_proxy_tuic_config_proto_rawDescData
}

var file_proxy_tuic_config_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_proxy_tuic_config_proto_goTypes = []any{
	(*Account)(nil),                 // 0: v2ray.core.proxy.tuic.Account
	(*QUICConfig)(nil),              // 1: v2ray.core.proxy.tuic.QUICConfig
	(*ClientConfig)(nil),            // 2: v2ray.core.proxy.tuic.ClientConfig
	(*protocol.ServerEndpoint)(nil), // 3: v2ray.core.common.protocol.ServerEndpoint
}
var file_proxy_tuic_config_proto_depIdxs = []int32{
	3, // 0: v2ray.core.proxy.tuic.ClientConfig.server:type_name -> v2ray.core.common.protocol.ServerEndpoint
	1, // 1: v2ray.core.proxy.tuic.ClientConfig.quic:type_name -> v2ray.core.proxy.tuic.QUICConfig
	2, // [2:2] is the sub-list for method output_type
	2, // [2:2] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_proxy_tuic_config_proto_init() }
func file_proxy_tuic_config_proto_init() {
	if File_proxy_tuic_config_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_proxy_tuic_config_proto_rawDesc), len(file_proxy_tuic_config_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_proxy_tuic_config_proto_goTypes,
		DependencyIndexes: file_proxy_tuic_config_proto_depIdxs,
		MessageInfos:      file_proxy_tuic_config_proto_msgTypes,
	}.Build()
	File_proxy_tuic_config_proto = out.File
	file_proxy_tuic_config_proto_goTypes = nil
	file_proxy_tuic_config_proto_depIdxs = nil
}
