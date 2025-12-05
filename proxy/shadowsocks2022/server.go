package shadowsocks2022

import (
	"bytes"
	"context"
	"crypto/cipher"
	cryptoRand "crypto/rand"
	"encoding/binary"
	"io"
	"time"

	"github.com/v2fly/struc"
	"lukechampine.com/blake3"

	core "github.com/frogwall/f2ray-core/v5"
	"github.com/frogwall/f2ray-core/v5/common"
	"github.com/frogwall/f2ray-core/v5/common/buf"
	"github.com/frogwall/f2ray-core/v5/common/crypto"
	"github.com/frogwall/f2ray-core/v5/common/log"
	"github.com/frogwall/f2ray-core/v5/common/net"
	"github.com/frogwall/f2ray-core/v5/common/net/packetaddr"
	"github.com/frogwall/f2ray-core/v5/common/protocol"
	udp_proto "github.com/frogwall/f2ray-core/v5/common/protocol/udp"
	"github.com/frogwall/f2ray-core/v5/common/session"
	"github.com/frogwall/f2ray-core/v5/common/signal"
	"github.com/frogwall/f2ray-core/v5/common/task"
	"github.com/frogwall/f2ray-core/v5/features/policy"
	"github.com/frogwall/f2ray-core/v5/features/routing"
	"github.com/frogwall/f2ray-core/v5/transport/internet"
	"github.com/frogwall/f2ray-core/v5/transport/internet/udp"
)

type Server struct {
	config        *ServerConfig
	users         map[string]*protocol.MemoryUser // PSK hash -> User
	policyManager policy.Manager
}

// NewServer creates a new Shadowsocks-2022 server
func NewServer(ctx context.Context, config *ServerConfig) (*Server, error) {
	v := core.MustFromContext(ctx)
	s := &Server{
		config:        config,
		users:         make(map[string]*protocol.MemoryUser),
		policyManager: v.GetFeature(policy.ManagerType()).(policy.Manager),
	}

	// Build user map with PSK hash as key
	for _, user := range config.Users {
		mUser, err := user.ToMemoryUser()
		if err != nil {
			return nil, newError("failed to parse user account").Base(err)
		}

		// Get user PSK from account
		if account, ok := mUser.Account.(*Account); ok {
			// Calculate PSK hash (first 16 bytes of BLAKE3)
			pskHash := blake3.Sum512(account.UserPsk)
			hashKey := string(pskHash[:16])
			s.users[hashKey] = mUser
		}
	}

	return s, nil
}

func (s *Server) Network() []net.Network {
	list := s.config.Network
	if len(list) == 0 {
		list = append(list, net.Network_TCP)
	}
	return list
}

// findUserByPSKHash finds a user by their PSK hash from EIH
func (s *Server) findUserByPSKHash(pskHash []byte) (*protocol.MemoryUser, []byte, error) {
	hashKey := string(pskHash)
	if user, ok := s.users[hashKey]; ok {
		// Extract user PSK from account
		if account, ok := user.Account.(*Account); ok {
			return user, account.UserPsk, nil
		}
		return nil, nil, newError("invalid account type")
	}
	return nil, nil, newError("user not found for PSK hash")
}

// decodeEIH decodes and validates EIH, returns effective PSK and user
func (s *Server) decodeEIH(eihData [][16]byte, salt []byte, method Method, keyDerivation KeyDerivation) ([]byte, *protocol.MemoryUser, error) {
	if len(eihData) == 0 {
		// No EIH, single-user mode (backward compatibility)
		if len(s.config.Users) > 0 {
			// Use first user
			mUser, err := s.config.Users[0].ToMemoryUser()
			if err != nil {
				return nil, nil, newError("failed to get default user").Base(err)
			}
			if account, ok := mUser.Account.(*Account); ok {
				return account.UserPsk, mUser, nil
			}
		}
		// Fallback to server PSK
		return s.config.Psk, nil, nil
	}

	// Decrypt last EIH with server PSK to get user PSK hash
	lastEIH := eihData[len(eihData)-1]

	// Derive identity key from server PSK
	identityKey := make([]byte, method.GetSessionSubKeyAndSaltLength())
	err := keyDerivation.GetIdentitySubKey(s.config.Psk, salt, identityKey)
	if err != nil {
		return nil, nil, newError("failed to derive identity key").Base(err)
	}

	// Decrypt EIH to get PSK hash
	pskHash := make([]byte, 16)
	err = method.DecryptEIH(identityKey, lastEIH[:], pskHash)
	if err != nil {
		return nil, nil, newError("failed to decrypt EIH").Base(err)
	}

	// Look up user by PSK hash
	user, userPsk, err := s.findUserByPSKHash(pskHash)
	if err != nil {
		return nil, nil, newError("user lookup failed").Base(err)
	}

	return userPsk, user, nil
}

func (s *Server) Process(ctx context.Context, network net.Network, conn internet.Connection, dispatcher routing.Dispatcher) error {
	switch network {
	case net.Network_TCP:
		return s.handleTCPConnection(ctx, conn, dispatcher)
	case net.Network_UDP:
		return s.handleUDPPayload(ctx, conn, dispatcher)
	default:
		return newError("unknown network: ", network)
	}
}

func (s *Server) handleTCPConnection(ctx context.Context, conn internet.Connection, dispatcher routing.Dispatcher) error {
	sessionPolicy := s.policyManager.ForLevel(0)
	conn.SetReadDeadline(time.Now().Add(sessionPolicy.Timeouts.Handshake))

	keyDerivation := newBLAKE3KeyDerivation()
	var method Method
	switch s.config.Method {
	case "2022-blake3-aes-128-gcm":
		method = newAES128GCMMethod()
	case "2022-blake3-aes-256-gcm":
		method = newAES256GCMMethod()
	default:
		return newError("unknown method: ", s.config.Method)
	}

	// Read pre-session key header (salt + EIH)
	var preSessionKeyHeader TCPRequestHeader1PreSessionKey
	preSessionKeyHeader.Salt = newRequestSaltWithLength(method.GetSessionSubKeyAndSaltLength())
	// Determine EIH count: 1 for multi-user mode (server PSK -> user PSK)
	eihCount := 0
	if len(s.config.Users) > 0 {
		eihCount = 1 // One EIH layer for user identification
	}
	preSessionKeyHeader.EIH = newAESEIH(eihCount)
	{
		err := struc.Unpack(conn, &preSessionKeyHeader)
		if err != nil {
			log.Record(&log.AccessMessage{
				From:   conn.RemoteAddr(),
				To:     "",
				Status: log.AccessRejected,
				Reason: err,
			})
			return newError("failed to unpack pre session key header").Base(err)
		}
	}

	c2sSalt := preSessionKeyHeader.Salt.Bytes()

	// Decode EIH to get effective PSK and user
	var effectivePsk []byte
	var currentUser *protocol.MemoryUser
	var err error

	if eihData, ok := preSessionKeyHeader.EIH.(*aesEIH); ok && len(eihData.eih) > 0 {
		effectivePsk, currentUser, err = s.decodeEIH(eihData.eih, c2sSalt, method, keyDerivation)
		if err != nil {
			log.Record(&log.AccessMessage{
				From:   conn.RemoteAddr(),
				To:     "",
				Status: log.AccessRejected,
				Reason: err,
			})
			return newError("failed to decode EIH").Base(err)
		}
	} else {
		// No EIH, use default user or server PSK
		effectivePsk, currentUser, err = s.decodeEIH(nil, c2sSalt, method, keyDerivation)
		if err != nil {
			return newError("failed to get effective PSK").Base(err)
		}
	}

	// Update session policy based on user level
	if currentUser != nil {
		sessionPolicy = s.policyManager.ForLevel(currentUser.Level)
	}

	// Derive session subkey
	sessionKey := make([]byte, method.GetSessionSubKeyAndSaltLength())
	{
		err := keyDerivation.GetSessionSubKey(effectivePsk, c2sSalt, sessionKey)
		if err != nil {
			return newError("failed to get session sub key").Base(err)
		}
	}

	aead, err := method.GetStreamAEAD(sessionKey)
	if err != nil {
		return newError("failed to get stream AEAD").Base(err)
	}

	// Read fixed-length header
	fixedLengthHeaderEncryptedBuffer := buf.New()
	defer fixedLengthHeaderEncryptedBuffer.Release()
	{
		_, err := fixedLengthHeaderEncryptedBuffer.ReadFullFrom(conn, 11+int32(aead.Overhead()))
		if err != nil {
			return newError("failed to read fixed length header encrypted").Base(err)
		}
	}

	c2sNonce := crypto.GenerateInitialAEADNonce()
	fixedLengthHeaderDecryptedBuffer := buf.New()
	defer fixedLengthHeaderDecryptedBuffer.Release()
	{
		decryptionBuffer := fixedLengthHeaderDecryptedBuffer.Extend(11)
		_, err = aead.Open(decryptionBuffer[:0], c2sNonce(), fixedLengthHeaderEncryptedBuffer.Bytes(), nil)
		if err != nil {
			return newError("failed to decrypt fixed length header").Base(err)
		}
	}

	var fixedLengthHeader TCPRequestHeader2FixedLength
	{
		err := struc.Unpack(bytes.NewReader(fixedLengthHeaderDecryptedBuffer.Bytes()), &fixedLengthHeader)
		if err != nil {
			return newError("failed to unpack fixed length header").Base(err)
		}
	}

	if fixedLengthHeader.Type != TCPHeaderTypeClientToServerStream {
		return newError("unexpected TCP header type")
	}

	// Validate timestamp (±30 seconds tolerance for replay protection)
	timeDifference := int64(fixedLengthHeader.Timestamp) - time.Now().Unix()
	if timeDifference < -30 || timeDifference > 30 {
		return newError("timestamp is too far away, timeDifference = ", timeDifference)
	}

	// Read variable-length header
	variableLengthHeaderEncryptedBuffer := buf.New()
	defer variableLengthHeaderEncryptedBuffer.Release()
	{
		_, err := variableLengthHeaderEncryptedBuffer.ReadFullFrom(conn, int32(fixedLengthHeader.HeaderLength)+int32(aead.Overhead()))
		if err != nil {
			return newError("failed to read variable length header encrypted").Base(err)
		}
	}

	variableLengthHeaderDecryptedBuffer := buf.New()
	defer variableLengthHeaderDecryptedBuffer.Release()
	{
		decryptionBuffer := variableLengthHeaderDecryptedBuffer.Extend(int32(fixedLengthHeader.HeaderLength))
		_, err = aead.Open(decryptionBuffer[:0], c2sNonce(), variableLengthHeaderEncryptedBuffer.Bytes(), nil)
		if err != nil {
			return newError("failed to decrypt variable length header").Base(err)
		}
	}

	// Parse destination address and port
	var port net.Port
	var address net.Address
	{
		addressBuffer := buf.New()
		defer addressBuffer.Release()
		address, port, err = addrParser.ReadAddressPort(addressBuffer, bytes.NewReader(variableLengthHeaderDecryptedBuffer.Bytes()))
		if err != nil {
			return newError("failed to read address port").Base(err)
		}
	}

	conn.SetReadDeadline(time.Time{})

	dest := net.Destination{
		Network: net.Network_TCP,
		Address: address,
		Port:    port,
	}

	inbound := session.InboundFromContext(ctx)
	if inbound == nil {
		panic("no inbound metadata")
	}

	// Set user in inbound context for statistics
	if currentUser != nil {
		inbound.User = currentUser
	}

	ctx = log.ContextWithAccessMessage(ctx, &log.AccessMessage{
		From:   conn.RemoteAddr(),
		To:     dest,
		Status: log.AccessAccepted,
		Reason: "",
		Email: func() string {
			if currentUser != nil {
				return currentUser.Email
			}
			return ""
		}(),
	})
	newError("tunnelling request to ", dest).WriteToLog(session.ExportIDToError(ctx))

	ctx, cancel := context.WithCancel(ctx)
	timer := signal.CancelAfterInactivity(ctx, cancel, sessionPolicy.Timeouts.ConnectionIdle)

	ctx = policy.ContextWithBufferPolicy(ctx, sessionPolicy.Buffer)
	link, err := dispatcher.Dispatch(ctx, dest)
	if err != nil {
		return err
	}

	// Create encrypted reader for client-to-server stream
	c2sAEADAuthenticator := &crypto.AEADAuthenticator{
		AEAD:                    aead,
		NonceGenerator:          c2sNonce,
		AdditionalDataGenerator: crypto.GenerateEmptyBytes(),
	}
	c2sReader := crypto.NewAuthenticationReader(c2sAEADAuthenticator, &AEADChunkSizeParser{
		Auth: c2sAEADAuthenticator,
	}, conn, protocol.TransferTypeStream, nil)

	requestDone := func() error {
		defer timer.SetTimeout(sessionPolicy.Timeouts.DownlinkOnly)
		if err := buf.Copy(c2sReader, link.Writer, buf.UpdateActivity(timer)); err != nil {
			return newError("failed to transport all TCP request").Base(err)
		}
		return nil
	}

	responseDone := func() error {
		defer timer.SetTimeout(sessionPolicy.Timeouts.UplinkOnly)

		// Generate response salt
		responseSalt := newRequestSaltWithLength(method.GetSessionSubKeyAndSaltLength())
		{
			err := responseSalt.FillAllFrom(cryptoRand.Reader)
			if err != nil {
				return newError("failed to fill response salt").Base(err)
			}
		}

		// Derive S2C session subkey
		s2cSessionKey := make([]byte, method.GetSessionSubKeyAndSaltLength())
		{
			err := keyDerivation.GetSessionSubKey(effectivePsk, responseSalt.Bytes(), s2cSessionKey)
			if err != nil {
				return newError("failed to get S2C session sub key").Base(err)
			}
		}

		s2cAEAD, err := method.GetStreamAEAD(s2cSessionKey)
		if err != nil {
			return newError("failed to get S2C stream AEAD").Base(err)
		}

		// Write response header
		responsePreSessionKeyHeader := &TCPResponseHeader1PreSessionKey{
			Salt: responseSalt,
		}
		{
			err := struc.Pack(conn, responsePreSessionKeyHeader)
			if err != nil {
				return newError("failed to pack response pre session key header").Base(err)
			}
		}

		// Read first payload chunk
		firstPayload, err := link.Reader.ReadMultiBuffer()
		if err != nil {
			return err
		}

		responseFixedLengthHeader := &TCPResponseHeader2FixedLength{
			Type:                 TCPHeaderTypeServerToClientStream,
			Timestamp:            uint64(time.Now().Unix()),
			RequestSalt:          preSessionKeyHeader.Salt,
			InitialPayloadLength: uint16(firstPayload.Len()),
		}

		responseFixedLengthHeaderBuffer := buf.New()
		defer responseFixedLengthHeaderBuffer.Release()
		{
			err := struc.Pack(responseFixedLengthHeaderBuffer, responseFixedLengthHeader)
			if err != nil {
				return newError("failed to pack response fixed length header").Base(err)
			}
		}

		s2cNonce := crypto.GenerateInitialAEADNonce()
		{
			fixedLengthEncrypted := make([]byte, responseFixedLengthHeaderBuffer.Len()+int32(s2cAEAD.Overhead()))
			s2cAEAD.Seal(fixedLengthEncrypted[:0], s2cNonce(), responseFixedLengthHeaderBuffer.Bytes(), nil)
			_, err := conn.Write(fixedLengthEncrypted)
			if err != nil {
				return newError("failed to write response fixed length header").Base(err)
			}
		}

		// Write initial payload
		firstPayloadBytes := make([]byte, firstPayload.Len())
		firstPayload.Copy(firstPayloadBytes)
		{
			initialPayloadEncrypted := make([]byte, len(firstPayloadBytes)+s2cAEAD.Overhead())
			s2cAEAD.Seal(initialPayloadEncrypted[:0], s2cNonce(), firstPayloadBytes, nil)
			_, err := conn.Write(initialPayloadEncrypted)
			if err != nil {
				return newError("failed to write initial payload").Base(err)
			}
		}
		buf.ReleaseMulti(firstPayload)

		// Create encrypted writer for server-to-client stream
		s2cAEADAuthenticator := &crypto.AEADAuthenticator{
			AEAD:                    s2cAEAD,
			NonceGenerator:          s2cNonce,
			AdditionalDataGenerator: crypto.GenerateEmptyBytes(),
		}
		s2cWriter := crypto.NewAuthenticationWriter(s2cAEADAuthenticator, &crypto.AEADChunkSizeParser{
			Auth: s2cAEADAuthenticator,
		}, conn, protocol.TransferTypeStream, nil)

		if err := buf.Copy(link.Reader, s2cWriter, buf.UpdateActivity(timer)); err != nil {
			return newError("failed to transport all TCP response").Base(err)
		}

		return nil
	}

	requestDoneAndCloseWriter := task.OnSuccess(requestDone, task.Close(link.Writer))
	if err := task.Run(ctx, requestDoneAndCloseWriter, responseDone); err != nil {
		common.Interrupt(link.Reader)
		common.Interrupt(link.Writer)
		return newError("connection ends").Base(err)
	}

	return nil
}

func (s *Server) handleUDPPayload(ctx context.Context, conn internet.Connection, dispatcher routing.Dispatcher) error {
	udpDispatcherConstructor := udp.NewSplitDispatcher
	switch s.config.PacketEncoding {
	case packetaddr.PacketAddrType_None:
		break
	case packetaddr.PacketAddrType_Packet:
		packetAddrDispatcherFactory := udp.NewPacketAddrDispatcherCreator(ctx)
		udpDispatcherConstructor = packetAddrDispatcherFactory.NewPacketAddrDispatcher
	}

	keyDerivation := newBLAKE3KeyDerivation()
	var method Method
	switch s.config.Method {
	case "2022-blake3-aes-128-gcm":
		method = newAES128GCMMethod()
	case "2022-blake3-aes-256-gcm":
		method = newAES256GCMMethod()
	default:
		return newError("unknown method: ", s.config.Method)
	}

	effectivePsk := s.config.Psk

	// UDP session management
	type udpSession struct {
		sessionID       [8]byte
		serverSessionID [8]byte
		sessionAEAD     cipher.AEAD
		blockCipher     cipher.Block
	}
	sessions := make(map[string]*udpSession)

	udpServer := udpDispatcherConstructor(dispatcher, func(ctx context.Context, packet *udp_proto.Packet) {
		// Find session for this response
		request := protocol.RequestHeaderFromContext(ctx)
		if request == nil {
			return
		}

		sessionKey := string(request.User.Email) // Using email as session key
		sess, ok := sessions[sessionKey]
		if !ok {
			newError("UDP session not found for response").WriteToLog(session.ExportIDToError(ctx))
			return
		}

		// Encode server-to-client UDP packet
		resp := &UDPResponse{
			UDPRequest: UDPRequest{
				SessionID: sess.serverSessionID,
				PacketID:  0, // TODO: implement packet counter
				TimeStamp: uint64(time.Now().Unix()),
				Address:   packet.Source.Address,
				Port:      int(packet.Source.Port),
				Payload:   packet.Payload,
			},
			ClientSessionID: sess.sessionID,
		}

		out := buf.New()
		defer out.Release()

		// Encode separate header
		separateHeaderStruct := separateHeader{
			SessionID: sess.serverSessionID,
			PacketID:  resp.PacketID,
		}
		separateHeaderBuffer := buf.New()
		defer separateHeaderBuffer.Release()
		{
			err := struc.Pack(separateHeaderBuffer, &separateHeaderStruct)
			if err != nil {
				newError("failed to pack separate header").Base(err).WriteToLog(session.ExportIDToError(ctx))
				return
			}
		}

		// Encrypt separate header
		{
			encryptedDest := out.Extend(16)
			sess.blockCipher.Encrypt(encryptedDest, separateHeaderBuffer.Bytes())
		}

		// Encode main header
		headerStruct := respHeader{
			Type:            UDPHeaderTypeServerToClientStream,
			TimeStamp:       resp.TimeStamp,
			ClientSessionID: resp.ClientSessionID,
			PaddingLength:   0,
			Padding:         nil,
		}
		responseBodyBuffer := buf.New()
		defer responseBodyBuffer.Release()
		{
			err := struc.Pack(responseBodyBuffer, &headerStruct)
			if err != nil {
				newError("failed to pack response header").Base(err).WriteToLog(session.ExportIDToError(ctx))
				return
			}
		}
		{
			err := addrParser.WriteAddressPort(responseBodyBuffer, resp.Address, net.Port(resp.Port))
			if err != nil {
				newError("failed to write address port").Base(err).WriteToLog(session.ExportIDToError(ctx))
				return
			}
		}
		{
			_, err := io.Copy(responseBodyBuffer, bytes.NewReader(resp.Payload.Bytes()))
			if err != nil {
				newError("failed to copy payload").Base(err).WriteToLog(session.ExportIDToError(ctx))
				return
			}
		}

		// Encrypt body
		{
			encryptedDest := out.Extend(int32(sess.sessionAEAD.Overhead()) + responseBodyBuffer.Len())
			sess.sessionAEAD.Seal(encryptedDest[:0], separateHeaderBuffer.Bytes()[4:16], responseBodyBuffer.Bytes(), nil)
		}

		conn.Write(out.Bytes())
	})

	inbound := session.InboundFromContext(ctx)
	if inbound == nil {
		panic("no inbound metadata")
	}

	reader := buf.NewPacketReader(conn)
	for {
		mpayload, err := reader.ReadMultiBuffer()
		if err != nil {
			break
		}

		for _, payload := range mpayload {
			if payload.Len() < 16 {
				payload.Release()
				continue
			}

			// Decrypt separate header
			separateHeaderBuffer := buf.New()
			defer separateHeaderBuffer.Release()
			{
				// For UDP, we use AES block cipher for separate header
				// This is a simplified version - proper implementation needs AES block cipher
				encryptedDest := separateHeaderBuffer.Extend(16)
				copy(encryptedDest, payload.BytesRange(0, 16))
			}

			var separateHeaderStruct separateHeader
			{
				err := struc.Unpack(bytes.NewReader(payload.BytesRange(0, 16)), &separateHeaderStruct)
				if err != nil {
					newError("failed to unpack separate header").Base(err).WriteToLog(session.ExportIDToError(ctx))
					payload.Release()
					continue
				}
			}

			// Get or create session
			sessionKey := string(separateHeaderStruct.SessionID[:])
			sess, ok := sessions[sessionKey]
			if !ok {
				// Create new session
				var serverSessionID [8]byte
				_, err := cryptoRand.Read(serverSessionID[:])
				if err != nil {
					newError("failed to generate server session ID").Base(err).WriteToLog(session.ExportIDToError(ctx))
					payload.Release()
					continue
				}

				sessionSubKey := make([]byte, method.GetSessionSubKeyAndSaltLength())
				err = keyDerivation.GetSessionSubKey(effectivePsk, separateHeaderStruct.SessionID[:], sessionSubKey)
				if err != nil {
					newError("failed to derive session subkey").Base(err).WriteToLog(session.ExportIDToError(ctx))
					payload.Release()
					continue
				}

				sessionAEAD, err := method.GetStreamAEAD(sessionSubKey)
				if err != nil {
					newError("failed to create session AEAD").Base(err).WriteToLog(session.ExportIDToError(ctx))
					payload.Release()
					continue
				}

				blockCipher, err := method.GetStreamAEAD(effectivePsk)
				if err != nil {
					newError("failed to create block cipher").Base(err).WriteToLog(session.ExportIDToError(ctx))
					payload.Release()
					continue
				}

				sess = &udpSession{
					sessionID:       separateHeaderStruct.SessionID,
					serverSessionID: serverSessionID,
					sessionAEAD:     sessionAEAD,
					blockCipher:     blockCipher.(cipher.Block), // This needs proper type handling
				}
				sessions[sessionKey] = sess
			}

			// Decrypt body
			bodyEncrypted := payload.BytesFrom(16)
			bodyDecrypted := make([]byte, len(bodyEncrypted)-sess.sessionAEAD.Overhead())
			// Construct nonce from session ID (bytes 4-16) and packet ID
			nonce := make([]byte, 12)
			copy(nonce, separateHeaderStruct.SessionID[4:])
			// PacketID is uint64, convert to bytes for nonce
			packetIDBytes := make([]byte, 8)
			binary.BigEndian.PutUint64(packetIDBytes, separateHeaderStruct.PacketID)
			copy(nonce[4:], packetIDBytes[:8])
			_, err = sess.sessionAEAD.Open(bodyDecrypted[:0], nonce, bodyEncrypted, nil)
			if err != nil {
				newError("failed to decrypt UDP body").Base(err).WriteToLog(session.ExportIDToError(ctx))
				payload.Release()
				continue
			}

			// Parse main header
			var headerStruct header
			bodyReader := bytes.NewReader(bodyDecrypted)
			{
				err := struc.Unpack(bodyReader, &headerStruct)
				if err != nil {
					newError("failed to unpack UDP header").Base(err).WriteToLog(session.ExportIDToError(ctx))
					payload.Release()
					continue
				}
			}

			// Validate timestamp
			timeDifference := int64(headerStruct.TimeStamp) - time.Now().Unix()
			if timeDifference < -30 || timeDifference > 30 {
				newError("UDP timestamp too far away, timeDifference = ", timeDifference).WriteToLog(session.ExportIDToError(ctx))
				payload.Release()
				continue
			}

			// Parse destination address
			addressBuffer := buf.New()
			defer addressBuffer.Release()
			address, port, err := addrParser.ReadAddressPort(addressBuffer, bodyReader)
			if err != nil {
				newError("failed to read UDP address port").Base(err).WriteToLog(session.ExportIDToError(ctx))
				payload.Release()
				continue
			}

			dest := net.Destination{
				Network: net.Network_UDP,
				Address: address,
				Port:    port,
			}

			// Read remaining payload
			remainingPayload := buf.New()
			_, err = io.Copy(remainingPayload, bodyReader)
			if err != nil {
				newError("failed to read UDP payload").Base(err).WriteToLog(session.ExportIDToError(ctx))
				payload.Release()
				continue
			}

			currentPacketCtx := ctx
			if inbound.Source.IsValid() {
				currentPacketCtx = log.ContextWithAccessMessage(ctx, &log.AccessMessage{
					From:   inbound.Source,
					To:     dest,
					Status: log.AccessAccepted,
					Reason: "",
				})
			}
			newError("tunnelling UDP request to ", dest).WriteToLog(session.ExportIDToError(currentPacketCtx))

			// Create request header for session tracking
			request := &protocol.RequestHeader{
				Address: dest.Address,
				Port:    dest.Port,
				User: &protocol.MemoryUser{
					Email: sessionKey, // Use session ID as email for tracking
				},
			}
			currentPacketCtx = protocol.ContextWithRequestHeader(currentPacketCtx, request)

			udpServer.Dispatch(currentPacketCtx, dest, remainingPayload)
			payload.Release()
		}
	}

	return nil
}

func init() {
	common.Must(common.RegisterConfig((*ServerConfig)(nil), func(ctx context.Context, config interface{}) (interface{}, error) {
		serverConfig, ok := config.(*ServerConfig)
		if !ok {
			return nil, newError("not a ServerConfig")
		}
		return NewServer(ctx, serverConfig)
	}))
}
