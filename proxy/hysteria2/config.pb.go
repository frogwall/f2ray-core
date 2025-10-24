package hysteria2

import (
	packetaddr "github.com/v2fly/v2ray-core/v5/common/net/packetaddr"
	protocol "github.com/v2fly/v2ray-core/v5/common/protocol"
	_ "github.com/v2fly/v2ray-core/v5/common/protoext"
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
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Account) Reset() {
	*x = Account{}
	mi := &file_proxy_hysteria2_config_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Account) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Account) ProtoMessage() {}

func (x *Account) ProtoReflect() protoreflect.Message {
	mi := &file_proxy_hysteria2_config_proto_msgTypes[0]
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
	return file_proxy_hysteria2_config_proto_rawDescGZIP(), []int{0}
}

func (x *Account) GetPassword() string {
	if x != nil {
		return x.Password
	}
	return ""
}

type CongestionControl struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Type          string                 `protobuf:"bytes,1,opt,name=type,proto3" json:"type,omitempty"`                          // "bbr" or "brutal"
	UpMbps        uint64                 `protobuf:"varint,2,opt,name=up_mbps,json=upMbps,proto3" json:"up_mbps,omitempty"`       // Upload bandwidth in Mbps
	DownMbps      uint64                 `protobuf:"varint,3,opt,name=down_mbps,json=downMbps,proto3" json:"down_mbps,omitempty"` // Download bandwidth in Mbps
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CongestionControl) Reset() {
	*x = CongestionControl{}
	mi := &file_proxy_hysteria2_config_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CongestionControl) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CongestionControl) ProtoMessage() {}

func (x *CongestionControl) ProtoReflect() protoreflect.Message {
	mi := &file_proxy_hysteria2_config_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CongestionControl.ProtoReflect.Descriptor instead.
func (*CongestionControl) Descriptor() ([]byte, []int) {
	return file_proxy_hysteria2_config_proto_rawDescGZIP(), []int{1}
}

func (x *CongestionControl) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

func (x *CongestionControl) GetUpMbps() uint64 {
	if x != nil {
		return x.UpMbps
	}
	return 0
}

func (x *CongestionControl) GetDownMbps() uint64 {
	if x != nil {
		return x.DownMbps
	}
	return 0
}

type BandwidthConfig struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	MaxTx         uint64                 `protobuf:"varint,1,opt,name=max_tx,json=maxTx,proto3" json:"max_tx,omitempty"` // Max transmit rate in bytes per second
	MaxRx         uint64                 `protobuf:"varint,2,opt,name=max_rx,json=maxRx,proto3" json:"max_rx,omitempty"` // Max receive rate in bytes per second
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *BandwidthConfig) Reset() {
	*x = BandwidthConfig{}
	mi := &file_proxy_hysteria2_config_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *BandwidthConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BandwidthConfig) ProtoMessage() {}

func (x *BandwidthConfig) ProtoReflect() protoreflect.Message {
	mi := &file_proxy_hysteria2_config_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BandwidthConfig.ProtoReflect.Descriptor instead.
func (*BandwidthConfig) Descriptor() ([]byte, []int) {
	return file_proxy_hysteria2_config_proto_rawDescGZIP(), []int{2}
}

func (x *BandwidthConfig) GetMaxTx() uint64 {
	if x != nil {
		return x.MaxTx
	}
	return 0
}

func (x *BandwidthConfig) GetMaxRx() uint64 {
	if x != nil {
		return x.MaxRx
	}
	return 0
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
	mi := &file_proxy_hysteria2_config_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *QUICConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*QUICConfig) ProtoMessage() {}

func (x *QUICConfig) ProtoReflect() protoreflect.Message {
	mi := &file_proxy_hysteria2_config_proto_msgTypes[3]
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
	return file_proxy_hysteria2_config_proto_rawDescGZIP(), []int{3}
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
	Password              string                     `protobuf:"bytes,2,opt,name=password,proto3" json:"password,omitempty"`
	Bandwidth             *BandwidthConfig           `protobuf:"bytes,4,opt,name=bandwidth,proto3" json:"bandwidth,omitempty"`
	Quic                  *QUICConfig                `protobuf:"bytes,5,opt,name=quic,proto3" json:"quic,omitempty"`
	IgnoreClientBandwidth bool                       `protobuf:"varint,7,opt,name=ignore_client_bandwidth,json=ignoreClientBandwidth,proto3" json:"ignore_client_bandwidth,omitempty"`
	unknownFields         protoimpl.UnknownFields
	sizeCache             protoimpl.SizeCache
}

func (x *ClientConfig) Reset() {
	*x = ClientConfig{}
	mi := &file_proxy_hysteria2_config_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ClientConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClientConfig) ProtoMessage() {}

func (x *ClientConfig) ProtoReflect() protoreflect.Message {
	mi := &file_proxy_hysteria2_config_proto_msgTypes[4]
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
	return file_proxy_hysteria2_config_proto_rawDescGZIP(), []int{4}
}

func (x *ClientConfig) GetServer() []*protocol.ServerEndpoint {
	if x != nil {
		return x.Server
	}
	return nil
}

func (x *ClientConfig) GetPassword() string {
	if x != nil {
		return x.Password
	}
	return ""
}

func (x *ClientConfig) GetBandwidth() *BandwidthConfig {
	if x != nil {
		return x.Bandwidth
	}
	return nil
}

func (x *ClientConfig) GetQuic() *QUICConfig {
	if x != nil {
		return x.Quic
	}
	return nil
}

func (x *ClientConfig) GetIgnoreClientBandwidth() bool {
	if x != nil {
		return x.IgnoreClientBandwidth
	}
	return false
}

type ServerConfig struct {
	state                 protoimpl.MessageState    `protogen:"open.v1"`
	PacketEncoding        packetaddr.PacketAddrType `protobuf:"varint,1,opt,name=packet_encoding,json=packetEncoding,proto3,enum=v2ray.core.net.packetaddr.PacketAddrType" json:"packet_encoding,omitempty"`
	Password              string                    `protobuf:"bytes,2,opt,name=password,proto3" json:"password,omitempty"`
	Congestion            *CongestionControl        `protobuf:"bytes,3,opt,name=congestion,proto3" json:"congestion,omitempty"`
	Bandwidth             *BandwidthConfig          `protobuf:"bytes,4,opt,name=bandwidth,proto3" json:"bandwidth,omitempty"`
	Quic                  *QUICConfig               `protobuf:"bytes,5,opt,name=quic,proto3" json:"quic,omitempty"`
	IgnoreClientBandwidth bool                      `protobuf:"varint,6,opt,name=ignore_client_bandwidth,json=ignoreClientBandwidth,proto3" json:"ignore_client_bandwidth,omitempty"`
	DisableUdp            bool                      `protobuf:"varint,7,opt,name=disable_udp,json=disableUdp,proto3" json:"disable_udp,omitempty"`
	UdpIdleTimeout        int64                     `protobuf:"varint,8,opt,name=udp_idle_timeout,json=udpIdleTimeout,proto3" json:"udp_idle_timeout,omitempty"` // in seconds
	unknownFields         protoimpl.UnknownFields
	sizeCache             protoimpl.SizeCache
}

func (x *ServerConfig) Reset() {
	*x = ServerConfig{}
	mi := &file_proxy_hysteria2_config_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ServerConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ServerConfig) ProtoMessage() {}

func (x *ServerConfig) ProtoReflect() protoreflect.Message {
	mi := &file_proxy_hysteria2_config_proto_msgTypes[5]
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
	return file_proxy_hysteria2_config_proto_rawDescGZIP(), []int{5}
}

func (x *ServerConfig) GetPacketEncoding() packetaddr.PacketAddrType {
	if x != nil {
		return x.PacketEncoding
	}
	return packetaddr.PacketAddrType(0)
}

func (x *ServerConfig) GetPassword() string {
	if x != nil {
		return x.Password
	}
	return ""
}

func (x *ServerConfig) GetCongestion() *CongestionControl {
	if x != nil {
		return x.Congestion
	}
	return nil
}

func (x *ServerConfig) GetBandwidth() *BandwidthConfig {
	if x != nil {
		return x.Bandwidth
	}
	return nil
}

func (x *ServerConfig) GetQuic() *QUICConfig {
	if x != nil {
		return x.Quic
	}
	return nil
}

func (x *ServerConfig) GetIgnoreClientBandwidth() bool {
	if x != nil {
		return x.IgnoreClientBandwidth
	}
	return false
}

func (x *ServerConfig) GetDisableUdp() bool {
	if x != nil {
		return x.DisableUdp
	}
	return false
}

func (x *ServerConfig) GetUdpIdleTimeout() int64 {
	if x != nil {
		return x.UdpIdleTimeout
	}
	return 0
}

var File_proxy_hysteria2_config_proto protoreflect.FileDescriptor

const file_proxy_hysteria2_config_proto_rawDesc = "" +
	"\n" +
	"\x1cproxy/hysteria2/config.proto\x12\x1av2ray.core.proxy.hysteria2\x1a\"common/net/packetaddr/config.proto\x1a!common/protocol/server_spec.proto\x1a common/protoext/extensions.proto\"%\n" +
	"\aAccount\x12\x1a\n" +
	"\bpassword\x18\x01 \x01(\tR\bpassword\"]\n" +
	"\x11CongestionControl\x12\x12\n" +
	"\x04type\x18\x01 \x01(\tR\x04type\x12\x17\n" +
	"\aup_mbps\x18\x02 \x01(\x04R\x06upMbps\x12\x1b\n" +
	"\tdown_mbps\x18\x03 \x01(\x04R\bdownMbps\"?\n" +
	"\x0fBandwidthConfig\x12\x15\n" +
	"\x06max_tx\x18\x01 \x01(\x04R\x05maxTx\x12\x15\n" +
	"\x06max_rx\x18\x02 \x01(\x04R\x05maxRx\"\xab\x03\n" +
	"\n" +
	"QUICConfig\x12A\n" +
	"\x1dinitial_stream_receive_window\x18\x01 \x01(\x04R\x1ainitialStreamReceiveWindow\x129\n" +
	"\x19max_stream_receive_window\x18\x02 \x01(\x04R\x16maxStreamReceiveWindow\x12I\n" +
	"!initial_connection_receive_window\x18\x03 \x01(\x04R\x1einitialConnectionReceiveWindow\x12A\n" +
	"\x1dmax_connection_receive_window\x18\x04 \x01(\x04R\x1amaxConnectionReceiveWindow\x12(\n" +
	"\x10max_idle_timeout\x18\x05 \x01(\x03R\x0emaxIdleTimeout\x12*\n" +
	"\x11keep_alive_period\x18\x06 \x01(\x03R\x0fkeepAlivePeriod\x12;\n" +
	"\x1adisable_path_mtu_discovery\x18\a \x01(\bR\x17disablePathMtuDiscovery\"\xc8\x02\n" +
	"\fClientConfig\x12B\n" +
	"\x06server\x18\x01 \x03(\v2*.v2ray.core.common.protocol.ServerEndpointR\x06server\x12\x1a\n" +
	"\bpassword\x18\x02 \x01(\tR\bpassword\x12I\n" +
	"\tbandwidth\x18\x04 \x01(\v2+.v2ray.core.proxy.hysteria2.BandwidthConfigR\tbandwidth\x12:\n" +
	"\x04quic\x18\x05 \x01(\v2&.v2ray.core.proxy.hysteria2.QUICConfigR\x04quic\x126\n" +
	"\x17ignore_client_bandwidth\x18\a \x01(\bR\x15ignoreClientBandwidth:\x19\x82\xb5\x18\x15\n" +
	"\boutbound\x12\thysteria2\"\xf1\x03\n" +
	"\fServerConfig\x12R\n" +
	"\x0fpacket_encoding\x18\x01 \x01(\x0e2).v2ray.core.net.packetaddr.PacketAddrTypeR\x0epacketEncoding\x12\x1a\n" +
	"\bpassword\x18\x02 \x01(\tR\bpassword\x12M\n" +
	"\n" +
	"congestion\x18\x03 \x01(\v2-.v2ray.core.proxy.hysteria2.CongestionControlR\n" +
	"congestion\x12I\n" +
	"\tbandwidth\x18\x04 \x01(\v2+.v2ray.core.proxy.hysteria2.BandwidthConfigR\tbandwidth\x12:\n" +
	"\x04quic\x18\x05 \x01(\v2&.v2ray.core.proxy.hysteria2.QUICConfigR\x04quic\x126\n" +
	"\x17ignore_client_bandwidth\x18\x06 \x01(\bR\x15ignoreClientBandwidth\x12\x1f\n" +
	"\vdisable_udp\x18\a \x01(\bR\n" +
	"disableUdp\x12(\n" +
	"\x10udp_idle_timeout\x18\b \x01(\x03R\x0eudpIdleTimeout:\x18\x82\xb5\x18\x14\n" +
	"\ainbound\x12\thysteria2BP\n" +
	"\x1ecom.v2ray.core.proxy.hysteria2P\x01Z\x0fproxy/hysteria2\xaa\x02\x1aV2Ray.Core.Proxy.Hysteria2b\x06proto3"

var (
	file_proxy_hysteria2_config_proto_rawDescOnce sync.Once
	file_proxy_hysteria2_config_proto_rawDescData []byte
)

func file_proxy_hysteria2_config_proto_rawDescGZIP() []byte {
	file_proxy_hysteria2_config_proto_rawDescOnce.Do(func() {
		file_proxy_hysteria2_config_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_proxy_hysteria2_config_proto_rawDesc), len(file_proxy_hysteria2_config_proto_rawDesc)))
	})
	return file_proxy_hysteria2_config_proto_rawDescData
}

var file_proxy_hysteria2_config_proto_msgTypes = make([]protoimpl.MessageInfo, 6)
var file_proxy_hysteria2_config_proto_goTypes = []any{
	(*Account)(nil),                 // 0: v2ray.core.proxy.hysteria2.Account
	(*CongestionControl)(nil),       // 1: v2ray.core.proxy.hysteria2.CongestionControl
	(*BandwidthConfig)(nil),         // 2: v2ray.core.proxy.hysteria2.BandwidthConfig
	(*QUICConfig)(nil),              // 3: v2ray.core.proxy.hysteria2.QUICConfig
	(*ClientConfig)(nil),            // 4: v2ray.core.proxy.hysteria2.ClientConfig
	(*ServerConfig)(nil),            // 5: v2ray.core.proxy.hysteria2.ServerConfig
	(*protocol.ServerEndpoint)(nil), // 6: v2ray.core.common.protocol.ServerEndpoint
	(packetaddr.PacketAddrType)(0),  // 7: v2ray.core.net.packetaddr.PacketAddrType
}
var file_proxy_hysteria2_config_proto_depIdxs = []int32{
	6, // 0: v2ray.core.proxy.hysteria2.ClientConfig.server:type_name -> v2ray.core.common.protocol.ServerEndpoint
	2, // 1: v2ray.core.proxy.hysteria2.ClientConfig.bandwidth:type_name -> v2ray.core.proxy.hysteria2.BandwidthConfig
	3, // 2: v2ray.core.proxy.hysteria2.ClientConfig.quic:type_name -> v2ray.core.proxy.hysteria2.QUICConfig
	7, // 3: v2ray.core.proxy.hysteria2.ServerConfig.packet_encoding:type_name -> v2ray.core.net.packetaddr.PacketAddrType
	1, // 4: v2ray.core.proxy.hysteria2.ServerConfig.congestion:type_name -> v2ray.core.proxy.hysteria2.CongestionControl
	2, // 5: v2ray.core.proxy.hysteria2.ServerConfig.bandwidth:type_name -> v2ray.core.proxy.hysteria2.BandwidthConfig
	3, // 6: v2ray.core.proxy.hysteria2.ServerConfig.quic:type_name -> v2ray.core.proxy.hysteria2.QUICConfig
	7, // [7:7] is the sub-list for method output_type
	7, // [7:7] is the sub-list for method input_type
	7, // [7:7] is the sub-list for extension type_name
	7, // [7:7] is the sub-list for extension extendee
	0, // [0:7] is the sub-list for field type_name
}

func init() { file_proxy_hysteria2_config_proto_init() }
func file_proxy_hysteria2_config_proto_init() {
	if File_proxy_hysteria2_config_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_proxy_hysteria2_config_proto_rawDesc), len(file_proxy_hysteria2_config_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   6,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_proxy_hysteria2_config_proto_goTypes,
		DependencyIndexes: file_proxy_hysteria2_config_proto_depIdxs,
		MessageInfos:      file_proxy_hysteria2_config_proto_msgTypes,
	}.Build()
	File_proxy_hysteria2_config_proto = out.File
	file_proxy_hysteria2_config_proto_goTypes = nil
	file_proxy_hysteria2_config_proto_depIdxs = nil
}
