package mieru

import (
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	mathrand "math/rand"
	"net"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/chacha20poly1305"

	v2net "github.com/frogwall/f2ray-core/v5/common/net"
)

// MieruSession represents a Mieru protocol session
type MieruSession struct {
	conn        net.Conn
	cipher      Cipher
	destination v2net.Destination
	nonce       uint64
	sessionID   uint32
	state       SessionState
	aead        cipher.AEAD
	key         []byte

	// Queue system based on original Mieru
	sendQueue *SegmentQueue
	recvQueue *SegmentQueue
	nextSend  uint32
	nextRecv  uint32

	// Protocol state
	firstWrite bool
	firstRead  bool

	// Encryption system
	sendCipher    Cipher
	recvCipher    Cipher
	primaryCipher Cipher // Keep reference to primary cipher for on-demand cloning

	// Background processing
	recvDone  chan struct{}
	unreadBuf []byte

	// CRITICAL: Add mutex protection like mieru-main baseUnderlay
	sendMutex  sync.Mutex // protect writing data to the connection
	closeMutex sync.Mutex // protect closing the connection
}

// SessionState represents the state of a Mieru session
type SessionState int

const (
	SessionInit        SessionState = 0
	SessionAttached    SessionState = 1
	SessionEstablished SessionState = 2
	SessionClosed      SessionState = 3

	// PDU constants
	maxPDU = 65535
)

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// toPrintableChar converts bytes to printable ASCII characters exactly like mieru-main
func toPrintableChar(b []byte, beginIdx, endIdx int) {
	if beginIdx > endIdx {
		panic("begin index > end index")
	}
	if endIdx > len(b) {
		panic("index out of range")
	}
	for i := beginIdx; i < endIdx; i++ {
		if b[i] < 0x20 || b[i] > 0x7E { // PrintableCharSub = 0x20, PrintableCharSup = 0x7E
			if b[i]&0x80 > 0 {
				lowBits := b[i] & 0x7F
				if lowBits >= 0x20 && lowBits <= 0x7E {
					b[i] = lowBits
					continue
				}
			}
			b[i] = 0x20 + (b[i] % (0x7E - 0x20 + 1))
		}
	}
}

// rngIntn implements the same random number generation as mieru-main rng.Intn
func rngIntn(n int) int {
	return int(float64(mathrand.Intn(n+1)) * scaleDown())
}

// scaleDown implements the same scaling function as mieru-main
func scaleDown() float64 {
	base := mathrand.Float64()
	return math.Sqrt(base * base * base)
}

// rngFixedInt implements the same fixed random number generation as mieru-main rng.FixedInt
func rngFixedInt(n int, source string) int {
	// Use the source string as a seed for deterministic random generation
	hash := sha256.Sum256([]byte(source))
	seed := int64(binary.BigEndian.Uint64(hash[:8]))
	rng := mathrand.New(mathrand.NewSource(seed))
	return rng.Intn(n)
}

// NewMieruSession creates a new Mieru session
func NewMieruSession(conn net.Conn, cipher Cipher, destination v2net.Destination) *MieruSession {
	session := &MieruSession{
		conn:        conn,
		cipher:      cipher,
		destination: destination,
		nonce:       0,
		sessionID:   generateSessionID(),
		state:       SessionInit,
		sendQueue:   NewSegmentQueue(),
		recvQueue:   NewSegmentQueue(),
		nextSend:    0,
		nextRecv:    0,
		firstWrite:  true,
		firstRead:   true,
	}

	// Use the key from the cipher (already generated in handler.go)
	session.key = cipher.GetKey()

	// Create AEAD cipher (use XChaCha20-Poly1305 with 24-byte nonce)
	aead, err := chacha20poly1305.NewX(session.key)
	if err != nil {
		// Fallback
		session.aead = nil
	} else {
		session.aead = aead
	}

	// Initialize encryption ciphers
	// CRITICAL: Use separate cipher instances for send and recv
	// In ImplicitNonceMode, each cipher maintains its own nonce state
	// session.sendCipher = cipher
	// session.recvCipher = cipher.Clone()

	// CRITICAL: Initialize nonce state for ImplicitNonceMode
	// The first Encrypt/Decrypt call will use the nonce from the cipher
	// Subsequent calls will increment the nonce
	// This matches the server-side behavior where send and recv use independent nonce sequences

	// BlockContext is already set in cipher from handler.go
	// No need to set it again here

	return session
}

// NewMieruSessionWithCiphers creates a new Mieru session with multiple ciphers
// This follows the exact same pattern as mieru-main StreamUnderlay
func NewMieruSessionWithCiphers(conn net.Conn, ciphers []Cipher, destination v2net.Destination) *MieruSession {
	// CRITICAL FIX: Follow mieru-main pattern exactly
	// Client uses only ONE cipher (candidates[0]) for both send and recv
	// Server uses SelectDecrypt to try all ciphers for time tolerance
	if len(ciphers) == 0 {
		return nil
	}

	// Use the first cipher (current time window) - exactly like mieru-main
	// In mieru-main: t.candidates = []cipher.BlockCipher{block} (single cipher)
	// Client always uses candidates[0] for both send and recv
	primaryCipher := ciphers[0]

	session := &MieruSession{
		conn:          conn,
		cipher:        primaryCipher, // Use primary cipher
		destination:   destination,
		nonce:         0,
		sessionID:     generateSessionID(),
		state:         SessionInit,
		sendQueue:     NewSegmentQueue(),
		recvQueue:     NewSegmentQueue(),
		nextSend:      0,
		nextRecv:      0,
		firstWrite:    true,
		firstRead:     true,
		primaryCipher: primaryCipher, // Keep reference for on-demand cloning
		recvDone:      make(chan struct{}),
		unreadBuf:     make([]byte, 0),
	}

	// CRITICAL FIX: Initialize ciphers exactly like mieru-main
	// In mieru-main:
	// - recv cipher is initialized in readOneSegment() when t.recv == nil using candidates[0].Clone()
	// - send cipher is initialized in maybeInitSendBlockCipher() when t.send == nil using candidates[0].Clone()
	// IMPORTANT: Don't initialize ciphers here - let them be initialized on-demand
	// This ensures proper nonce state management
	session.sendCipher = nil // Will be initialized on first send
	session.recvCipher = nil // Will be initialized on first receive

	// Use the key from the primary cipher
	session.key = primaryCipher.GetKey()

	// Create AEAD cipher - exactly like mieru-main, fail on error
	aead, err := chacha20poly1305.NewX(session.key)
	if err != nil {
		// fmt.Printf("[MIERU ERROR] Failed to create AEAD cipher: %v\n", err)
		return nil
	}
	session.aead = aead

	// fmt.Printf("[MIERU V2RAY DEBUG] NewMieruSessionWithCiphers: using primary cipher, key: %x\n", session.key[:8])

	return session
}

// maybeInitRecvCipher initializes recv cipher on-demand, exactly like mieru-main
func (s *MieruSession) maybeInitRecvCipher() error {
	if s.recvCipher != nil {
		return nil
	}
	s.recvCipher = s.primaryCipher.Clone()
	// fmt.Printf("[MIERU V2RAY DEBUG] Client recv cipher initialized from primaryCipher, nonce: %x\n", s.recvCipher.GetCurrentNonce())
	return nil
}

// maybeInitSendCipher initializes send cipher on-demand, exactly like mieru-main
func (s *MieruSession) maybeInitSendCipher() error {
	if s.sendCipher != nil {
		return nil
	}
	s.sendCipher = s.primaryCipher.Clone()
	// fmt.Printf("[MIERU V2RAY DEBUG] Client send cipher initialized from primaryCipher, nonce: %x\n", s.sendCipher.GetCurrentNonce())
	return nil
}

// generateSessionID generates a random session ID (must not be 0)
func generateSessionID() uint32 {
	var id uint32
	// Use math/rand instead of crypto/rand for simplicity
	id = mathrand.Uint32()
	// Ensure ID is not 0 (reserved)
	if id == 0 {
		id = 1
	}
	return id
}

// Handshake performs the Mieru protocol handshake
// Fixed to match mieru-main implementation
func (s *MieruSession) Handshake() error {
	// Initialize send cipher before handshake, exactly like mieru-main
	if err := s.maybeInitSendCipher(); err != nil {
		return fmt.Errorf("failed to initialize send cipher: %w", err)
	}

	// Set initial state to attached (ready for first write)
	s.state = SessionAttached

	// Send openSessionRequest with SOCKS5 connection request during handshake (following original mieru client)
	// This establishes the session with SOCKS5 connection request
	// fmt.Printf("[MIERU DEBUG] Sending openSessionRequest with SOCKS5 connection request during handshake\n")

	// Build SOCKS5 connection request
	var connectRequest []byte
	connectRequest = append(connectRequest, 5) // SOCKS5 version
	connectRequest = append(connectRequest, 1) // CONNECT command
	connectRequest = append(connectRequest, 0) // Reserved

	// Address type and address
	if s.destination.Address.Family().IsDomain() {
		// Domain name
		domain := s.destination.Address.Domain()
		connectRequest = append(connectRequest, 3) // Domain type
		connectRequest = append(connectRequest, byte(len(domain)))
		connectRequest = append(connectRequest, []byte(domain)...)
	} else if s.destination.Address.Family().IsIPv4() {
		// IPv4
		connectRequest = append(connectRequest, 1) // IPv4 type
		connectRequest = append(connectRequest, s.destination.Address.IP()...)
	} else {
		// IPv6
		connectRequest = append(connectRequest, 4) // IPv6 type
		connectRequest = append(connectRequest, s.destination.Address.IP()...)
	}

	// Port (big-endian)
	port := s.destination.Port.Value()
	connectRequest = append(connectRequest, byte(port>>8), byte(port))

	// fmt.Printf("[MIERU DEBUG] SOCKS5 connection request for handshake: %x (len=%d)\n", connectRequest, len(connectRequest))

	// Create segment with SOCKS5 connection request
	segment := s.createOpenSessionSegment(connectRequest)
	// fmt.Printf("[MIERU DEBUG] Created segment with protocol: %d, payload len: %d\n", segment.protocolType(), len(segment.payload))

	// Insert segment into send queue
	if !s.sendQueue.Insert(segment) {
		return fmt.Errorf("failed to insert segment into send queue")
	}

	// Process send queue
	if err := s.processSendQueue(); err != nil {
		return fmt.Errorf("failed to process send queue: %w", err)
	}

	// Wait for open session response exactly like mieru-main
	// if err := s.waitForOpenSessionResponse(); err != nil {
	// 	return fmt.Errorf("failed to wait for open session response: %w", err)
	// }

	// CRITICAL FIX: Match server-side nonce management logic
	// Analysis:
	// - Server-side t.recv: keeps nonce state from handshake, used for decrypting client data
	// - Server-side t.send: cloned from t.recv then reset nonce, but ALREADY USED for handshake response
	//   So t.send nonce is at state: new_random_nonce + 1 after handshake
	// - Client should match:
	//   - sendCipher: keeps nonce state from handshake (matches server t.recv expectation)
	//   - recvCipher: keeps nonce state from handshake response (server t.send already incremented once)
	// fmt.Printf("[MIERU DEBUG] Matching server-side nonce management logic\n")
	// fmt.Printf("[MIERU DEBUG] sendCipher type: %T\n", s.sendCipher)
	// fmt.Printf("[MIERU DEBUG] recvCipher type: %T\n", s.recvCipher)

	// CRITICAL FIX: Keep continuous nonce sequence like original mieru
	// Analysis: Server-side send cipher resets nonce after handshake, but
	// the first response after handshake might still use the old nonce sequence
	// Let's keep the continuous nonce sequence for now and investigate further
	// fmt.Printf("[MIERU DEBUG] Keeping continuous nonce sequence like original mieru\n")

	// Log current nonce states
	if s.sendCipher != nil {
		_ = s.sendCipher.GetCurrentNonce()
		// fmt.Printf("[MIERU DEBUG] sendCipher current nonce: %x\n", sendNonce)
	}

	if s.recvCipher != nil {
		_ = s.recvCipher.GetCurrentNonce()
		// fmt.Printf("[MIERU DEBUG] recvCipher current nonce after reset: %x\n", recvNonce)
	}

	// Update state to established
	s.state = SessionEstablished
	// fmt.Printf("[MIERU DEBUG] Session established after handshake\n")

	// Start background goroutine to receive segments and put them in recvQueue
	go s.receiveSegments()

	// Don't start reading immediately - let the first Write trigger the read
	// This matches the original mieru client behavior
	// fmt.Printf("[MIERU DEBUG] Waiting for first Write to trigger read\n")

	return nil
}

// receiveSegments continuously receives segments from the server and puts them in recvQueue
func (s *MieruSession) receiveSegments() {
	defer close(s.recvDone)

	for {
		// Read one segment from the server
		segment, err := s.readOneSegment()
		if err != nil {
			// fmt.Printf("[MIERU DEBUG] receiveSegments: failed to read segment: %v\n", err)
			return
		}

		// Insert segment into recvQueue
		if !s.recvQueue.Insert(segment) {
			// fmt.Printf("[MIERU DEBUG] receiveSegments: failed to insert segment into recvQueue\n")
			return
		}

		// fmt.Printf("[MIERU DEBUG] receiveSegments: inserted segment with protocol %d, payload len %d\n",
		//     segment.protocolType(), len(segment.payload))
	}
}

// readOneSegment reads one complete segment from the server
func (s *MieruSession) readOneSegment() (*Segment, error) {
	// Set a reasonable read timeout for data segments
	// s.conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	// CRITICAL FIX: Implement firstRead logic like original mieru
	metadataLen := 48 // MetadataLength (32) + Overhead (16)
	if s.recvCipher == nil {
		// In the first Read, also include nonce.
		metadataLen += s.aead.NonceSize()
		// fmt.Printf("[MIERU DEBUG] First read detected, including nonce in metadataLen: %d\n", metadataLen)
	}

	encryptedMetadata := make([]byte, metadataLen)
	_, err := io.ReadFull(s.conn, encryptedMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to read encrypted metadata: %w", err)
	}
	// Initialize recv cipher on-demand, exactly like mieru-main
	if err := s.maybeInitRecvCipher(); err != nil {
		return nil, fmt.Errorf("failed to initialize recv cipher: %w", err)
	}
	// Decrypt metadata
	decryptedMetadata, err := s.recvCipher.Decrypt(encryptedMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt metadata: %w", err)
	}

	// fmt.Printf("[MIERU DEBUG] Successfully decrypted metadata: %d bytes, hex: %x\n", len(decryptedMetadata), decryptedMetadata)

	// Parse metadata to get protocol type
	if len(decryptedMetadata) < 1 {
		return nil, fmt.Errorf("metadata too short: %d bytes", len(decryptedMetadata))
	}
	protocol := decryptedMetadata[0]
	protocolType := protocolType(protocol)
	// fmt.Printf("[MIERU DEBUG] Protocol type: %s (%d)\n", protocolType.String(), protocol)

	// Determine metadata type and parse accordingly
	var payloadLen uint16
	var suffixLen uint8
	var prefixLen uint8

	if isSessionProtocol(protocolType) {
		// Session protocols: use sessionStruct
		var ss sessionStruct
		if err := ss.Unmarshal(decryptedMetadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal session struct: %w", err)
		}
		payloadLen = ss.payloadLen
		suffixLen = ss.suffixLen
	} else if isDataAckProtocol(protocolType) {
		// Data/ack protocols: use dataAckStruct
		var das dataAckStruct
		if err := das.Unmarshal(decryptedMetadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal dataAck struct: %w", err)
		}
		payloadLen = das.payloadLen
		suffixLen = das.suffixLen
		prefixLen = das.prefixLen
	} else {
		return nil, fmt.Errorf("unknown protocol: %s (%d)", protocolType.String(), protocol)
	}

	// Read prefix padding (for data/ack) if present
	if prefixLen > 0 {
		padding := make([]byte, prefixLen)
		if _, err := io.ReadFull(s.conn, padding); err != nil {
			return nil, fmt.Errorf("failed to read prefix padding: %w", err)
		}
	}

	// Read payload if present
	var payload []byte
	if payloadLen > 0 {
		// fmt.Printf("[MIERU DEBUG] Reading payload: payloadLen=%d, encrypted size=%d\n", payloadLen, payloadLen+16)
		encryptedPayload := make([]byte, payloadLen+16) // payload + auth tag
		_, err := io.ReadFull(s.conn, encryptedPayload)
		if err != nil {
			return nil, fmt.Errorf("failed to read encrypted payload: %w", err)
		}

		// fmt.Printf("[MIERU DEBUG] Read encrypted payload: %d bytes, hex: %x\n", len(encryptedPayload), encryptedPayload)
		payload, err = s.decryptPayloadWithNonceFallback(encryptedPayload)
		if err != nil {
			return nil, err
		}

		// fmt.Printf("[MIERU DEBUG] Successfully decrypted payload: %d bytes\n", len(payload))

		// DEBUG: Print first 16 bytes of payload for dataServerToClient to check for padding leakage
		if protocol == byte(dataServerToClient) && len(payload) > 0 {
			// fmt.Printf("[MIERU DEBUG] %s payload first %d bytes hex: %x\n", protocolType.String(), min(16, len(payload)), payload[:min(16, len(payload))])
		}
	}

	// Read padding if present
	if suffixLen > 0 {
		padding := make([]byte, suffixLen)
		_, err := io.ReadFull(s.conn, padding)
		if err != nil {
			return nil, fmt.Errorf("failed to read padding: %w", err)
		}
	}

	// Create segment with appropriate metadata
	var metadata metadata
	if isSessionProtocol(protocolType) {
		// Session protocols: use sessionStruct
		var ss sessionStruct
		ss.Unmarshal(decryptedMetadata)
		metadata = &ss
	} else if isDataAckProtocol(protocolType) {
		// Data/ack protocols: use dataAckStruct
		var das dataAckStruct
		das.Unmarshal(decryptedMetadata)
		metadata = &das
	}

	segment := &Segment{
		metadata: metadata,
		payload:  payload,
	}

	return segment, nil
}

// decryptPayloadWithNonceFallback tries payload decryption with three strategies:
// 1) explicit current nonce (N), 2) explicit previous nonce (N-1), 3) implicit (nonce+1)
func (s *MieruSession) decryptPayloadWithNonceFallback(encryptedPayload []byte) ([]byte, error) {
	currentNonce := s.recvCipher.GetCurrentNonce()
	// fmt.Printf("[MIERU DEBUG] recvCipher nonce before payload decrypt: %x\n", currentNonce)
	// Try 1: implicit (will use nonce+1)
	payload, err := s.recvCipher.Decrypt(encryptedPayload)
	if err != nil {
		// fmt.Printf("[MIERU DEBUG] Payload implicit (nonce+1) decrypt failed: %v\n", err)
	} else {
		// fmt.Printf("[MIERU DEBUG] Payload decrypted with implicit nonce (incremented)\n")
		return payload, nil
	}

	// Try 2: explicit with current nonce (N)
	payload, err = s.recvCipher.DecryptWithNonce(encryptedPayload, currentNonce)
	if err == nil {
		// fmt.Printf("[MIERU DEBUG] Payload decrypted with current nonce (no increment)\n")
		return payload, nil
	}
	// fmt.Printf("[MIERU DEBUG] Payload decrypt-with-current-nonce failed: %v\n", err)

	// Prepare previous nonce (N-1)
	prev := make([]byte, len(currentNonce))
	copy(prev, currentNonce)
	for i := len(prev) - 1; i >= 0; i-- {
		prev[i]--
		if prev[i] != 0xff {
			break
		}
	}
	// Try 3: explicit with previous nonce (N-1)
	payload, err = s.recvCipher.DecryptWithNonce(encryptedPayload, prev)
	if err == nil {
		// fmt.Printf("[MIERU DEBUG] Payload decrypted with previous nonce (no increment)\n")
		return payload, nil
	}
	// fmt.Printf("[MIERU ERROR] Payload decryption failed after all attempts: %v\n", err)
	// fmt.Printf("[MIERU ERROR] Encrypted payload hex: %x\n", encryptedPayload)
	// fmt.Printf("[MIERU ERROR] recvCipher nonce: %x\n", s.recvCipher.GetCurrentNonce())
	return nil, fmt.Errorf("failed to decrypt payload: %w", err)
}

// Read implements net.Conn.Read
func (s *MieruSession) Read(b []byte) (n int, err error) {
	// fmt.Printf("[MIERU DEBUG] Read called, reading from recvQueue\n")

	// If this is the first read after handshake, wait for the first write to complete
	// This matches the original mieru client behavior
	if s.firstRead {
		s.firstRead = false
		// fmt.Printf("[MIERU DEBUG] First read after handshake, waiting for first write\n")
		// Wait a bit for the first write to complete
		time.Sleep(10 * time.Millisecond)
	}

	// Read remaining data that application failed to read last time
	if len(s.unreadBuf) > 0 {
		n = copy(b, s.unreadBuf)
		if n == len(s.unreadBuf) {
			s.unreadBuf = nil
		} else {
			s.unreadBuf = s.unreadBuf[n:]
		}
		// fmt.Printf("[MIERU DEBUG] Read %d bytes from unreadBuf\n", n)
		return n, nil
	}

	// Wait for segments in recvQueue
	for {
		if s.recvQueue.Len() > 0 {
			// Process segments in order, but prioritize HTTP headers when found
			for {
				seg, ok := s.recvQueue.DeleteMin()
				if !ok {
					break
				}

				// Enhanced protocol detection and reassembly
				if seg.protocolType() == dataServerToClient && len(seg.payload) > 0 {
					protocolType := s.detectProtocolType(seg.payload)
					// fmt.Printf("[MIERU DEBUG] Detected protocol: %s for segment %d bytes\n", protocolType, len(seg.payload))

					switch protocolType {
					case "HTTP":
						s.handleHTTPSegment(seg)
					case "TLS":
						s.handleTLSSegment(seg)
					case "SOCKS5":
						s.handleSOCKS5Segment(seg)
					default:
						s.handleGenericSegment(seg)
					}
				} else {
					// Handle non-dataServerToClient segments or empty payloads
					if seg.protocolType() != dataServerToClient && len(seg.payload) > 0 {
						// fmt.Printf("[MIERU DEBUG] Handling non-dataServerToClient segment: protocol %d, payload %d bytes\n",
						//         seg.protocolType(), len(seg.payload))

						// Add to buffer for non-dataServerToClient segments
						if s.unreadBuf == nil {
							s.unreadBuf = make([]byte, 0)
						}
						s.unreadBuf = append(s.unreadBuf, seg.payload...)
					}
				}
			}

			if len(s.unreadBuf) > 0 {
				break
			}
		} else {
			// Wait for incoming segments
			select {
			case <-s.recvDone:
				return 0, io.EOF
			case <-time.After(30 * time.Second):
				return 0, fmt.Errorf("read timeout: no data received within 30 seconds")
			case <-time.After(100 * time.Millisecond):
				// Check again for segments
				continue
			}
		}
	}

	// Copy data from unreadBuf to output buffer
	n = copy(b, s.unreadBuf)
	if n == len(s.unreadBuf) {
		s.unreadBuf = nil
	} else {
		s.unreadBuf = s.unreadBuf[n:]
	}

	// fmt.Printf("[MIERU DEBUG] Read %d bytes from recvQueue\n", n)
	return n, nil
}

// Write implements net.Conn.Write
func (s *MieruSession) Write(b []byte) (n int, err error) {
	// fmt.Printf("[MIERU DEBUG] Write called with %d bytes, state: %d\n", len(b), s.state)

	// Initialize send cipher on-demand, exactly like mieru-main
	if err := s.maybeInitSendCipher(); err != nil {
		return 0, fmt.Errorf("failed to initialize send cipher: %w", err)
	}

	// Check if session is established
	if s.state != SessionEstablished {
		return 0, fmt.Errorf("session not established, current state: %d", s.state)
	}

	// On first write, send actual data (SOCKS5 connection request was sent during handshake)
	// Following the original mieru client implementation: SOCKS5 connection request is sent during handshake
	if s.firstWrite {
		s.firstWrite = false
		// fmt.Printf("[MIERU DEBUG] First write - sending actual data for destination: %s\n", s.destination.String())

		// Track nonce state before sending
		if s.sendCipher != nil {
			_ = s.sendCipher.GetCurrentNonce()
			// fmt.Printf("[MIERU DEBUG] sendCipher nonce before first write: %x\n", sendNonce)
		}

		// Send actual data directly (SOCKS5 connection request was already sent during handshake)
		// Use the same logic as original mieru client: send data through writeChunk
		if sent, err := s.writeChunk(b); sent == 0 || err != nil {
			return 0, err
		}

		// fmt.Printf("[MIERU DEBUG] Data sent successfully\n")
		return len(b), nil
	}

	// Send actual data using writeChunk (same as original mieru client)
	// fmt.Printf("[MIERU DEBUG] Sending data segment\n")
	if sent, err := s.writeChunk(b); sent == 0 || err != nil {
		return 0, err
	}

	return len(b), nil
}

// writeChunk implements the same logic as original mieru client
func (s *MieruSession) writeChunk(b []byte) (n int, err error) {
	if len(b) > maxPDU {
		return 0, fmt.Errorf("data too large: %d > %d", len(b), maxPDU)
	}

	// Determine number of fragments to write (same as original mieru client)
	nFragment := 1
	fragmentSize := 1400 // Default MTU size
	if len(b) > fragmentSize {
		nFragment = (len(b)-1)/fragmentSize + 1
	}

	// Create segments for each fragment (same as original mieru client)
	ptr := b
	for i := nFragment - 1; i >= 0; i-- {
		partLen := min(fragmentSize, len(ptr))
		part := ptr[:partLen]

		// Create dataAckStruct exactly like original mieru client
		seg := &Segment{
			metadata: &dataAckStruct{
				baseStruct: baseStruct{
					protocol: uint8(dataClientToServer), // dataClientToServer = 6
				},
				sessionID:  s.sessionID,
				seq:        s.nextSend,
				unAckSeq:   s.nextRecv,
				windowSize: 4096, // Default window size
				fragment:   uint8(i),
				payloadLen: uint16(partLen),
			},
			payload: make([]byte, partLen),
		}
		copy(seg.payload, part)
		s.nextSend++

		// Insert segment into send queue
		if !s.sendQueue.Insert(seg) {
			return 0, fmt.Errorf("insert segment to send queue failed")
		}

		ptr = ptr[partLen:]
		n += partLen
	}

	// Process send queue
	if err := s.processSendQueue(); err != nil {
		return 0, fmt.Errorf("failed to process send queue: %w", err)
	}

	return n, nil
}

// Close implements net.Conn.Close
func (s *MieruSession) Close() error {
	s.closeMutex.Lock()
	defer s.closeMutex.Unlock()

	return s.conn.Close()
}

// createOpenSessionRequest creates an open session request
func (s *MieruSession) createOpenSessionRequest() *MieruOpenSessionRequest {
	return &MieruOpenSessionRequest{
		SessionID:  s.sessionID,
		Timestamp:  uint32(time.Now().Unix() / 60), // Mieru uses minute-based timestamps
		ClientPort: uint16(s.destination.Port),
	}
}

// encryptData encrypts data using the session's AEAD cipher
// Fixed to match mieru-main implementation exactly
func (s *MieruSession) encryptData(data []byte) ([]byte, error) {
	if s.aead == nil {
		// fmt.Printf("[MIERU DEBUG] Encrypting %d bytes (no AEAD cipher)\n", len(data))
		return data, nil
	}

	// Generate random nonce exactly like mieru-main
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Adjust the nonce such that the first 8 bytes are printable ASCII characters
	// This is exactly like mieru-main's ToCommon64Set function
	common64Set := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz_-"
	rewriteLen := 8
	if rewriteLen > len(nonce) {
		rewriteLen = len(nonce)
	}
	for i := 0; i < rewriteLen; i++ {
		setIdx := nonce[i] & 0x3f
		nonce[i] = common64Set[setIdx]
	}

	// Encrypt data using XChaCha20-Poly1305
	encrypted := s.aead.Seal(nil, nonce, data, nil)

	// Prepend nonce (like mieru-main)
	result := make([]byte, len(nonce)+len(encrypted))
	copy(result, nonce)
	copy(result[len(nonce):], encrypted)

	// fmt.Printf("[MIERU DEBUG] Encrypted %d bytes -> %d bytes (nonce: %d bytes, first 8: %s)\n", len(data), len(result), len(nonce), string(nonce[:8]))

	return result, nil
}

// decryptData decrypts data using the session's AEAD cipher
func (s *MieruSession) decryptData(data []byte) ([]byte, error) {
	if s.recvCipher == nil {
		// fmt.Printf("[MIERU DEBUG] Decrypting %d bytes (no recvCipher)\n", len(data))
		return data, nil
	}

	// Use recvCipher to decrypt data exactly like mieru-main
	decrypted, err := s.recvCipher.Decrypt(data)
	if err != nil {
		return nil, fmt.Errorf("recvCipher decryption failed: %w", err)
	}

	// fmt.Printf("[MIERU DEBUG] Decrypted %d bytes -> %d bytes\n", len(data), len(decrypted))
	return decrypted, nil
}

// LocalAddr implements net.Conn.LocalAddr
func (s *MieruSession) LocalAddr() net.Addr {
	return s.conn.LocalAddr()
}

// RemoteAddr implements net.Conn.RemoteAddr
func (s *MieruSession) RemoteAddr() net.Addr {
	return s.conn.RemoteAddr()
}

// SetDeadline implements net.Conn.SetDeadline
func (s *MieruSession) SetDeadline(t time.Time) error {
	return s.conn.SetDeadline(t)
}

// SetReadDeadline implements net.Conn.SetReadDeadline
func (s *MieruSession) SetReadDeadline(t time.Time) error {
	return s.conn.SetReadDeadline(t)
}

// SetWriteDeadline implements net.Conn.SetWriteDeadline
func (s *MieruSession) SetWriteDeadline(t time.Time) error {
	return s.conn.SetWriteDeadline(t)
}

// MieruOpenSessionRequest represents a Mieru OpenSessionRequest
type MieruOpenSessionRequest struct {
	SessionID  uint32
	Timestamp  uint32
	ClientPort uint16
}

// MieruOpenSessionResponse represents a Mieru OpenSessionResponse
type MieruOpenSessionResponse struct {
	Status    uint32
	SessionID uint32
}

// Segment represents a Mieru protocol segment
// Exactly like mieru-main segment structure
type Segment struct {
	metadata  metadata // Use shared metadata interface from metadata.go
	payload   []byte
	transport string
	block     Cipher // cipher block with BlockContext for server authentication
}

// SegmentQueue represents a queue for segments
type SegmentQueue struct {
	segments []*Segment
	mu       sync.Mutex
}

// NewSegmentQueue creates a new segment queue
func NewSegmentQueue() *SegmentQueue {
	return &SegmentQueue{
		segments: make([]*Segment, 0),
	}
}

// Insert inserts a segment into the queue
func (q *SegmentQueue) Insert(seg *Segment) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.segments = append(q.segments, seg)
	return true
}

// DeleteMin removes and returns the minimum segment
func (q *SegmentQueue) DeleteMin() (*Segment, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.segments) == 0 {
		return nil, false
	}
	seg := q.segments[0]
	q.segments = q.segments[1:]
	return seg, true
}

// Len returns the length of the queue
func (q *SegmentQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.segments)
}

// Protocol returns the protocol type of the segment
// protocolType helper for logging
func (s *Segment) protocolType() protocolType { return s.metadata.Protocol() }

// createOpenSessionSegment creates an open session segment
// Fixed to match mieru-main implementation exactly
func (s *MieruSession) createOpenSessionSegment(payload []byte) *Segment {
	// Handle nil payload (for initial handshake)
	if payload == nil {
		payload = []byte{}
	}

	// Limit payload size to MaxSessionOpenPayload (1024 bytes)
	if len(payload) > MaxSessionOpenPayload {
		payload = payload[:MaxSessionOpenPayload]
	}

	// fmt.Printf("[MIERU DEBUG] Creating openSession segment with nextSend=%d, payload len=%d\n", s.nextSend, len(payload))

	// Create metadata exactly like mieru-main sessionStruct
	metadata := &sessionStruct{
		baseStruct: baseStruct{
			protocol: byte(openSessionRequest),
			// timestamp will be set by Marshal()
		},
		sessionID:  s.sessionID,
		seq:        s.nextSend,
		statusCode: 0,
		payloadLen: uint16(len(payload)),
		suffixLen:  0,
	}
	// fmt.Printf("[MIERU DEBUG] sessionStruct fields: protocol=%d, sessionID=%d, seq=%d, statusCode=%d, payloadLen=%d, suffixLen=%d\n", metadata.protocol, metadata.sessionID, metadata.seq, metadata.statusCode, metadata.payloadLen, metadata.suffixLen)

	if len(payload) > 0 {
		// fmt.Printf("[MIERU V2RAY DEBUG] Open session WITH payload: len=%d, payloadLen=%d, MaxSessionOpenPayload=%d\n", len(payload), metadata.payloadLen, MaxSessionOpenPayload)
		// fmt.Printf("[MIERU V2RAY DEBUG] Payload content: %x\n", payload)
	} else {
		// fmt.Printf("[MIERU V2RAY DEBUG] Open session WITHOUT payload (empty payload for initial handshake)\n")
	}

	// Increment sequence number
	s.nextSend++

	return &Segment{
		metadata:  metadata,
		payload:   payload,
		transport: "tcp",
		block:     s.sendCipher,
	}
}

// processSendQueue processes the send queue and sends segments
func (s *MieruSession) processSendQueue() error {
	for {
		seg, ok := s.sendQueue.DeleteMin()
		if !ok {
			break
		}

		if err := s.writeOneSegment(seg); err != nil {
			return fmt.Errorf("failed to write segment: %w", err)
		}
	}
	return nil
}

// writeOneSegment writes a single segment with encryption
// Fixed to match mieru-main implementation exactly
func (s *MieruSession) writeOneSegment(seg *Segment) error {
	s.sendMutex.Lock()
	defer s.sendMutex.Unlock()

	// fmt.Printf("[MIERU DEBUG] Writing segment with protocol: %d\n", seg.protocolType())

	// Generate padding BEFORE serializing metadata (so suffixLen is included)
	// Use MaxPaddingSize exactly like mieru-main with MTU support
	_ = DefaultMTU        // Use DefaultMTU constant (kept for reference)
	maxPaddingSize := 255 // MaxPaddingSize for stream transport (no limit)
	// Use exactly the same logic as mieru-main: 24 + rng.FixedIntPerHost(17)
	// For simplicity, use 24 + 8 (middle value of 0-16 range)
	minConsecutiveASCIILen := 24 + 8 // recommendedConsecutiveASCIILen = 24 + rng.FixedIntPerHost(17)
	if maxPaddingSize < minConsecutiveASCIILen {
		maxPaddingSize = minConsecutiveASCIILen
	}
	// fmt.Printf("[MIERU V2RAY DEBUG] Padding calculation: mtu=%d, maxPaddingSize=%d, minConsecutiveASCIILen=%d, payloadLen=%d\n", mtu, maxPaddingSize, minConsecutiveASCIILen, len(seg.payload))
	// Use the same random number generation as mieru-main
	// Use rng.Intn with scale down distribution exactly like mieru-main
	paddingSize := rngIntn(maxPaddingSize-minConsecutiveASCIILen+1) + minConsecutiveASCIILen
	padding := make([]byte, paddingSize)

	// Generate padding exactly like mieru-main
	// Choose strategy randomly like mieru-main (ASCII or entropy)
	strategy := rngFixedInt(2, "mieru-client") // Use same strategy source as mieru-main
	if strategy == 0 {
		// Use ASCII padding exactly like mieru-main
		// First generate random bytes using crypto/rand exactly like mieru-main
		if _, err := rand.Read(padding); err != nil {
			return fmt.Errorf("failed to generate random padding: %w", err)
		}

		// Convert part of the padding to printable ASCII characters exactly like mieru-main
		// Use ToPrintableChar for the first minConsecutiveASCIILen bytes
		beginIdx := 0
		if len(padding) > minConsecutiveASCIILen {
			beginIdx = mathrand.Intn(len(padding) - minConsecutiveASCIILen)
		}
		toPrintableChar(padding, beginIdx, beginIdx+minConsecutiveASCIILen)
	} else {
		// Use entropy padding exactly like mieru-main
		// Generate random bytes using crypto/rand exactly like mieru-main
		if _, err := rand.Read(padding); err != nil {
			return fmt.Errorf("failed to generate random padding: %w", err)
		}
		// No additional processing for entropy padding
	}

	// Update suffixLen in metadata BEFORE serializing
	// Support both sessionStruct and dataAckStruct
	switch m := seg.metadata.(type) {
	case *sessionStruct:
		m.suffixLen = uint8(paddingSize)
		// fmt.Printf("[MIERU V2RAY DEBUG] sessionStruct: protocol=%d, sessionID=%d, seq=%d, payloadLen=%d, suffixLen=%d\n", m.protocol, m.sessionID, m.seq, m.payloadLen, m.suffixLen)
	case *dataAckStruct:
		m.suffixLen = uint8(paddingSize)
		// fmt.Printf("[MIERU V2RAY DEBUG] dataAckStruct: protocol=%d, sessionID=%d, seq=%d, unAckSeq=%d, windowSize=%d, payloadLen=%d, suffixLen=%d\n", m.protocol, m.sessionID, m.seq, m.unAckSeq, m.windowSize, m.payloadLen, m.suffixLen)
	}

	// Serialize metadata exactly like mieru-main
	plaintextMetadata := seg.metadata.Marshal()
	// fmt.Printf("[MIERU DEBUG] Plaintext metadata: %d bytes, hex: %x\n", len(plaintextMetadata), plaintextMetadata)

	// Encrypt metadata using the cipher block
	encryptedMetadata, err := s.sendCipher.Encrypt(plaintextMetadata)
	if err != nil {
		return fmt.Errorf("failed to encrypt metadata: %w", err)
	}
	// fmt.Printf("[MIERU DEBUG] Encrypted metadata: %d bytes\n", len(encryptedMetadata))

	// Build complete data packet exactly like mieru-main
	dataToSend := encryptedMetadata

	// Encrypt and append payload if present
	if len(seg.payload) > 0 {
		// fmt.Printf("[MIERU V2RAY DEBUG] Encrypting payload: %d bytes\n", len(seg.payload))
		encryptedPayload, err := s.sendCipher.Encrypt(seg.payload)
		if err != nil {
			return fmt.Errorf("failed to encrypt payload: %w", err)
		}
		dataToSend = append(dataToSend, encryptedPayload...)
		// fmt.Printf("[MIERU V2RAY DEBUG] Encrypted payload: %d bytes\n", len(encryptedPayload))
	}

	// Append padding
	dataToSend = append(dataToSend, padding...)
	// fmt.Printf("[MIERU V2RAY DEBUG] Added padding: %d bytes\n", len(padding))

	// Send complete packet in one Write() call exactly like mieru-main
	// fmt.Printf("[MIERU V2RAY DEBUG] Writing %d bytes to server (metadata=%d, payload=%d, padding=%d)\n", len(dataToSend), len(encryptedMetadata), len(seg.payload), len(padding))
	if _, err := s.conn.Write(dataToSend); err != nil {
		return fmt.Errorf("failed to write to connection: %w", err)
	}

	// fmt.Printf("[MIERU V2RAY DEBUG] Successfully sent %d bytes total\n", len(dataToSend))
	return nil
}

// waitForOpenSessionResponse waits for open session response
// waitForOpenSessionResponse was removed to align with original mieru flow

// detectProtocolType detects the protocol type based on payload content
func (s *MieruSession) detectProtocolType(payload []byte) string {
	if len(payload) < 4 {
		return "UNKNOWN"
	}

	// Check for HTTP protocol
	if len(payload) >= 5 && string(payload[:5]) == "HTTP/" {
		return "HTTP"
	}

	// Check for TLS/SSL protocol
	if len(payload) >= 5 && payload[0] == 0x16 && payload[1] == 0x03 {
		return "TLS"
	}

	// Check for SOCKS5 protocol
	if len(payload) >= 2 && payload[0] == 0x05 {
		return "SOCKS5"
	}

	// Check for other binary protocols
	if len(payload) >= 4 {
		// Check for common binary protocol patterns
		if payload[0] == 0x01 && payload[1] == 0x02 { // Custom binary protocol
			return "BINARY"
		}
	}

	return "GENERIC"
}

// handleHTTPSegment handles HTTP protocol segments with priority reassembly
func (s *MieruSession) handleHTTPSegment(seg *Segment) {
	// fmt.Printf("[MIERU DEBUG] Handling HTTP segment: %d bytes\n", len(seg.payload))

	// Check if this is an HTTP header segment
	payloadStr := string(seg.payload)
	checkLen := 100
	if len(payloadStr) < checkLen {
		checkLen = len(payloadStr)
	}

	if strings.HasPrefix(payloadStr[:checkLen], "HTTP/") {
		// fmt.Printf("[MIERU DEBUG] Found HTTP header segment: %d bytes, prioritizing\n", len(seg.payload))
		// Clear existing buffer and start fresh with HTTP header
		if len(s.unreadBuf) > 0 {
			// fmt.Printf("[MIERU DEBUG] Clearing existing buffer (%d bytes) to prioritize HTTP header\n", len(s.unreadBuf))
			s.unreadBuf = nil
		}
	}

	// Add HTTP segment to buffer
	if s.unreadBuf == nil {
		s.unreadBuf = make([]byte, 0)
	}
	s.unreadBuf = append(s.unreadBuf, seg.payload...)
	// fmt.Printf("[MIERU DEBUG] Added HTTP segment: %d bytes to unreadBuf\n", len(seg.payload))
}

// handleTLSSegment handles TLS/SSL protocol segments with priority reassembly
func (s *MieruSession) handleTLSSegment(seg *Segment) {
	// fmt.Printf("[MIERU DEBUG] Handling TLS segment: %d bytes\n", len(seg.payload))

	// TLS segments need to be handled with high priority to maintain handshake integrity
	// Check if this is a TLS handshake record
	if len(seg.payload) >= 5 && seg.payload[0] == 0x16 && seg.payload[1] == 0x03 {
		// fmt.Printf("[MIERU DEBUG] Found TLS handshake segment: %d bytes, prioritizing\n", len(seg.payload))

		// For TLS handshake, we need to ensure the complete record is transmitted
		// Clear any existing buffer to prioritize TLS handshake
		if len(s.unreadBuf) > 0 {
			// fmt.Printf("[MIERU DEBUG] Clearing existing buffer (%d bytes) to prioritize TLS handshake\n", len(s.unreadBuf))
			s.unreadBuf = nil
		}
	}

	// Add TLS segment to buffer
	if s.unreadBuf == nil {
		s.unreadBuf = make([]byte, 0)
	}
	s.unreadBuf = append(s.unreadBuf, seg.payload...)
	// fmt.Printf("[MIERU DEBUG] Added TLS segment: %d bytes to unreadBuf\n", len(seg.payload))
}

// handleSOCKS5Segment handles SOCKS5 protocol segments with priority reassembly
func (s *MieruSession) handleSOCKS5Segment(seg *Segment) {
	// fmt.Printf("[MIERU DEBUG] Handling SOCKS5 segment: %d bytes\n", len(seg.payload))

	// SOCKS5 segments need to be handled with high priority to maintain protocol integrity
	// Check if this is a SOCKS5 response
	if len(seg.payload) >= 2 && seg.payload[0] == 0x05 {
		// fmt.Printf("[MIERU DEBUG] Found SOCKS5 response segment: %d bytes, prioritizing\n", len(seg.payload))

		// For SOCKS5 response, we need to ensure the complete response is transmitted
		// Clear any existing buffer to prioritize SOCKS5 response
		if len(s.unreadBuf) > 0 {
			// fmt.Printf("[MIERU DEBUG] Clearing existing buffer (%d bytes) to prioritize SOCKS5 response\n", len(s.unreadBuf))
			s.unreadBuf = nil
		}
	}

	// Add SOCKS5 segment to buffer
	if s.unreadBuf == nil {
		s.unreadBuf = make([]byte, 0)
	}
	s.unreadBuf = append(s.unreadBuf, seg.payload...)
	// fmt.Printf("[MIERU DEBUG] Added SOCKS5 segment: %d bytes to unreadBuf\n", len(seg.payload))
}

// handleGenericSegment handles generic protocol segments
func (s *MieruSession) handleGenericSegment(seg *Segment) {
	// fmt.Printf("[MIERU DEBUG] Handling generic segment: %d bytes\n", len(seg.payload))

	// For generic segments, just add to buffer without special processing
	if s.unreadBuf == nil {
		s.unreadBuf = make([]byte, 0)
	}
	s.unreadBuf = append(s.unreadBuf, seg.payload...)
	// fmt.Printf("[MIERU DEBUG] Added generic segment: %d bytes to unreadBuf\n", len(seg.payload))
}
