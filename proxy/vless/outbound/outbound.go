package outbound

//go:generate go run github.com/frogwall/f2ray-core/v5/common/errors/errorgen

import (
	"context"

	core "github.com/frogwall/f2ray-core/v5"
	"github.com/frogwall/f2ray-core/v5/common"
	"github.com/frogwall/f2ray-core/v5/common/buf"
	"github.com/frogwall/f2ray-core/v5/common/net"
	"github.com/frogwall/f2ray-core/v5/common/protocol"
	"github.com/frogwall/f2ray-core/v5/common/retry"
	"github.com/frogwall/f2ray-core/v5/common/serial"
	"github.com/frogwall/f2ray-core/v5/common/session"
	"github.com/frogwall/f2ray-core/v5/common/signal"
	"github.com/frogwall/f2ray-core/v5/common/task"
	"github.com/frogwall/f2ray-core/v5/features/policy"
	"github.com/frogwall/f2ray-core/v5/proxy"
	"github.com/frogwall/f2ray-core/v5/proxy/vless"
	"github.com/frogwall/f2ray-core/v5/proxy/vless/encoding"
	"github.com/frogwall/f2ray-core/v5/transport"
	"github.com/frogwall/f2ray-core/v5/transport/internet"
	vision "github.com/frogwall/f2ray-core/v5/proxy/vision"
)

func init() {
	common.Must(common.RegisterConfig((*Config)(nil), func(ctx context.Context, config interface{}) (interface{}, error) {
		return New(ctx, config.(*Config))
	}))

	common.Must(common.RegisterConfig((*SimplifiedConfig)(nil), func(ctx context.Context, config interface{}) (interface{}, error) {
		simplifiedClient := config.(*SimplifiedConfig)
		fullClient := &Config{Vnext: []*protocol.ServerEndpoint{
			{
				Address: simplifiedClient.Address,
				Port:    simplifiedClient.Port,
				User: []*protocol.User{
					{
						Account: serial.ToTypedMessage(&vless.Account{Id: simplifiedClient.Uuid, Encryption: "none"}),
					},
				},
			},
		}}

		return common.CreateObject(ctx, fullClient)
	}))
}

// Handler is an outbound connection handler for VLess protocol.
type Handler struct {
	serverList    *protocol.ServerList
	serverPicker  protocol.ServerPicker
	policyManager policy.Manager
}

// New creates a new VLess outbound handler.
func New(ctx context.Context, config *Config) (*Handler, error) {
	serverList := protocol.NewServerList()
	for _, rec := range config.Vnext {
		s, err := protocol.NewServerSpecFromPB(rec)
		if err != nil {
			return nil, newError("failed to parse server spec").Base(err).AtError()
		}
		serverList.AddServer(s)
	}

	v := core.MustFromContext(ctx)
	handler := &Handler{
		serverList:    serverList,
		serverPicker:  protocol.NewRoundRobinServerPicker(serverList),
		policyManager: v.GetFeature(policy.ManagerType()).(policy.Manager),
	}

	return handler, nil
}

// Process implements proxy.Outbound.Process().
func (h *Handler) Process(ctx context.Context, link *transport.Link, dialer internet.Dialer) error {
	var rec *protocol.ServerSpec
	var conn internet.Connection

	if err := retry.ExponentialBackoff(5, 200).On(func() error {
		rec = h.serverPicker.PickServer()
		var err error
		conn, err = dialer.Dial(ctx, rec.Destination())
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return newError("failed to find an available destination").Base(err).AtWarning()
	}
	defer conn.Close()

	outbound := session.OutboundFromContext(ctx)
	if outbound == nil || !outbound.Target.IsValid() {
		return newError("target not specified").AtError()
	}

	target := outbound.Target
	newError("tunneling request to ", target, " via ", rec.Destination().NetAddr()).AtInfo().WriteToLog(session.ExportIDToError(ctx))

	command := protocol.RequestCommandTCP
	if target.Network == net.Network_UDP {
		command = protocol.RequestCommandUDP
	}
	if target.Address.Family().IsDomain() && target.Address.Domain() == "v1.mux.cool" {
		command = protocol.RequestCommandMux
	}

	request := &protocol.RequestHeader{
		Version: encoding.Version,
		User:    rec.PickUser(),
		Command: command,
		Address: target.Address,
		Port:    target.Port,
	}

	account := request.User.Account.(*vless.MemoryAccount)

	requestAddons := &encoding.Addons{
		Flow: account.Flow,
	}

	sessionPolicy := h.policyManager.ForLevel(request.User.Level)
	ctx, cancel := context.WithCancel(ctx)
	timer := signal.CancelAfterInactivity(ctx, cancel, sessionPolicy.Timeouts.ConnectionIdle)

	clientReader := link.Reader // .(*pipe.Reader)
	clientWriter := link.Writer // .(*pipe.Writer)
	var visionState *vision.TrafficState
	var responseAddons *encoding.Addons

	postRequest := func() error {
		defer timer.SetTimeout(sessionPolicy.Timeouts.DownlinkOnly)

		bufferWriter := buf.NewBufferedWriter(buf.NewWriter(conn))
		if err := encoding.EncodeRequestHeader(bufferWriter, request, requestAddons); err != nil {
			return newError("failed to encode request header").Base(err).AtWarning()
		}
		newError("request flow=", requestAddons.Flow).AtInfo().WriteToLog(session.ExportIDToError(ctx))
		// Flush header so that server can respond with response header before body
		if err := bufferWriter.SetBuffered(false); err != nil {
			return newError("failed to flush request header").Base(err).AtWarning()
		}

		var serverWriter buf.Writer
		if request.Command == protocol.RequestCommandUDP {
			serverWriter = encoding.EncodeBodyAddons(bufferWriter, request, requestAddons)
		} else {
			// Always use plain writer for TCP; Vision writer only when server echo is confirmed (not done in postRequest)
			serverWriter = encoding.EncodeBodyAddons(bufferWriter, request, requestAddons)
		}
		if err := buf.CopyOnceTimeout(clientReader, serverWriter, proxy.FirstPayloadTimeout); err != nil && err != buf.ErrNotTimeoutReader && err != buf.ErrReadTimeout {
			return err // ...
		}
		// Already unbuffered above; proceed

		if err := buf.Copy(clientReader, serverWriter, buf.UpdateActivity(timer)); err != nil {
			return newError("failed to transfer request payload").Base(err).AtInfo()
		}

		return nil
	}

	getResponse := func() error {
		defer timer.SetTimeout(sessionPolicy.Timeouts.UplinkOnly)

		// If we didn't decode response header early, do it now before streaming body
		if responseAddons == nil {
			ra, err := encoding.DecodeResponseHeader(conn, request)
			if err != nil {
				return newError("failed to decode response header").Base(err).AtInfo()
			}
			responseAddons = ra
			if responseAddons != nil {
				newError("response flow=", responseAddons.GetFlow()).AtInfo().WriteToLog(session.ExportIDToError(ctx))
			} else {
				newError("response flow=<nil>").AtInfo().WriteToLog(session.ExportIDToError(ctx))
			}
		}

		var serverReader buf.Reader
		if request.Command == protocol.RequestCommandUDP {
			serverReader = encoding.DecodeBodyAddons(conn, request, responseAddons)
		} else if responseAddons != nil && responseAddons.GetFlow() == "xtls-rprx-vision" {
			ob := session.OutboundFromContext(ctx)
			if visionState == nil {
				visionState = vision.NewTrafficState(account.ID.Bytes())
			}
			inner := encoding.DecodeBodyAddons(conn, request, responseAddons)
			serverReader = vision.NewReader(inner, ctx, conn, ob, visionState, false)
		} else {
			serverReader = encoding.DecodeBodyAddons(conn, request, responseAddons)
		}

		if err := buf.Copy(serverReader, clientWriter, buf.UpdateActivity(timer)); err != nil {
			return newError("failed to transfer response payload").Base(err).AtInfo()
		}
		return nil
	}

	if err := task.Run(ctx, postRequest, getResponse); err != nil {
		return newError("connection ends").Base(err).AtInfo()
	}

	return nil
}
