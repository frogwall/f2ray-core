package outbound

//go:generate go run github.com/frogwall/f2ray-core/v5/common/errors/errorgen

import (
	"context"
	"time"

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
	"github.com/frogwall/f2ray-core/v5/proxy/vision"
	"github.com/frogwall/f2ray-core/v5/proxy/vless"
	"github.com/frogwall/f2ray-core/v5/proxy/vless/encoding"
	"github.com/frogwall/f2ray-core/v5/transport"
	"github.com/frogwall/f2ray-core/v5/transport/internet"
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
	outbounds := session.OutboundsFromContext(ctx)
	if len(outbounds) == 0 {
		// If outbounds is nil or empty, try to get single outbound
		ob := session.OutboundFromContext(ctx)
		if ob == nil {
			return newError("target not specified").AtError()
		}
		outbounds = []*session.Outbound{ob}
	}
	ob := outbounds[len(outbounds)-1]
	if !ob.Target.IsValid() && ob.Target.Address.String() != "v1.rvs.cool" {
		return newError("target not specified").AtError()
	}
	ob.Name = "vless"

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

	target := ob.Target
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

	// Force disable Vision flow in request - server doesn't support standard Vision implementation
	requestAddons := &encoding.Addons{
		Flow: account.Flow,
	}

	var newCtx context.Context
	var newCancel context.CancelFunc

	sessionPolicy := h.policyManager.ForLevel(request.User.Level)
	ctx, cancel := context.WithCancel(ctx)
	timer := signal.CancelAfterInactivity(ctx, func() {
		cancel()
		if newCancel != nil {
			newCancel()
		}
	}, sessionPolicy.Timeouts.ConnectionIdle)

	clientReader := link.Reader
	clientWriter := link.Writer

	// Initialize traffic state with raw UUID (Vision doesn't use ProcessUUID)
	trafficState := vision.NewTrafficState(account.ID.Bytes())
	newError("[UUID DEBUG] Account UUID bytes: ", account.ID.Bytes()).AtInfo().WriteToLog()
	if request.Command == protocol.RequestCommandUDP && (requestAddons.Flow == "xtls-rprx-vision" || request.Port != 53 && request.Port != 443) {
		request.Command = protocol.RequestCommandMux
		request.Address = net.DomainAddress("v1.mux.cool")
		request.Port = net.Port(666)
	}

	postRequest := func() error {
		defer timer.SetTimeout(sessionPolicy.Timeouts.DownlinkOnly)

		newError("[FLOW DEBUG] Request flow: ", requestAddons.Flow).AtInfo().WriteToLog()
		newError("[FLOW DEBUG] Request command: ", request.Command).AtInfo().WriteToLog()

		bufferWriter := buf.NewBufferedWriter(buf.NewWriter(conn))
		if err := encoding.EncodeRequestHeader(bufferWriter, request, requestAddons); err != nil {
			return newError("failed to encode request header").Base(err).AtWarning()
		}

		// default: serverWriter := bufferWriter
		serverWriter := encoding.EncodeBodyAddons(bufferWriter, request, requestAddons, trafficState, true, ctx, conn, ob)
		newError("[FLOW DEBUG] Created serverWriter, Flow=", requestAddons.Flow).AtInfo().WriteToLog()

		timeoutReader, ok := clientReader.(buf.TimeoutReader)
		if ok {
			multiBuffer, err1 := timeoutReader.ReadMultiBufferTimeout(time.Millisecond * 500)
			if err1 == nil {
				if err := serverWriter.WriteMultiBuffer(multiBuffer); err != nil {
					return err
				}
			} else if err1 != buf.ErrReadTimeout {
				return err1
			} else if requestAddons.Flow == "xtls-rprx-vision" {
				mb := make(buf.MultiBuffer, 1)
				if err := serverWriter.WriteMultiBuffer(mb); err != nil {
					return err
				}
			}
		}

		// Flush
		if err := bufferWriter.SetBuffered(false); err != nil {
			return newError("failed to flush request header").Base(err).AtWarning()
		}

		if err := buf.Copy(clientReader, serverWriter, buf.UpdateActivity(timer)); err != nil {
			return newError("failed to transfer request payload").Base(err).AtInfo()
		}

		return nil
	}

	getResponse := func() error {
		defer timer.SetTimeout(sessionPolicy.Timeouts.UplinkOnly)

		responseAddons, err := encoding.DecodeResponseHeader(conn, request)
		if err != nil {
			return newError("failed to decode response header").Base(err).AtInfo()
		}

		newError("[FLOW DEBUG] Response header decoded, ResponseFlow=", responseAddons.Flow, ", RequestFlow=", requestAddons.Flow).AtInfo().WriteToLog()

		// Debug: peek at first data after response header
		// Note: This will consume data from conn, which will break the subsequent read
		// So we'll skip this debug code for now

		// default: serverReader := buf.NewReader(conn)
		serverReader := encoding.DecodeBodyAddons(conn, request, responseAddons)

		// Use Vision Reader whenever request flow is Vision (matching Xray behavior)
		useVision := requestAddons.Flow == "xtls-rprx-vision"
		if useVision && responseAddons.Flow != "xtls-rprx-vision" {
			newError("[FLOW DEBUG] Request is Vision but response flow is empty, still using Vision Reader (Xray behavior)").AtInfo().WriteToLog()
		}

		if useVision {
			newError("[FLOW DEBUG] Creating Vision Reader for response").AtInfo().WriteToLog()
			// Note: Xray's signature is different but we're using our own implementation
			// Xray: NewVisionReader(reader, trafficState, isUplink, ctx, conn, input, rawInput, ob)
			// Ours: NewReader(r, ctx, conn, input, rawInput, ob, state, isUplink)
			serverReader = vision.NewReader(serverReader, ctx, conn, nil, nil, ob, trafficState, false)
			newError("[FLOW DEBUG] Using XtlsRead for response").AtInfo().WriteToLog()
			err = encoding.XtlsRead(serverReader, clientWriter, timer, conn, trafficState, false, ctx)
		} else {
			newError("[FLOW DEBUG] Using plain reader for response").AtInfo().WriteToLog()
			newError("[FLOW DEBUG] Using buf.Copy for response").AtInfo().WriteToLog()
			err = buf.Copy(serverReader, clientWriter, buf.UpdateActivity(timer))
		}

		if err != nil {
			return newError("failed to transfer response payload").Base(err).AtInfo()
		}

		return nil
	}

	if newCtx != nil {
		ctx = newCtx
	}

	if err := task.Run(ctx, postRequest, task.OnSuccess(getResponse, task.Close(clientWriter))); err != nil {
		return newError("connection ends").Base(err).AtInfo()
	}

	return nil
}
