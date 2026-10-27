// Package hub provides WebSocket functionality.
package hub

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// PTY frames travel as WebSocket binary messages on the device stream and on the
// operator terminal stream. Per-chunk JSON (the shape used for control traffic)
// would mean base64 and escaping overhead on every keystroke and every byte of
// output, and would not survive binary payloads such as a `cat` of a file.
//
// Wire format (big-endian):
//
//	[1B channel][1B type][2B length][payload...]
//
// The 2-byte length caps a single frame at MaxPTYPayload bytes. That bound is the
// point: a device cannot make the server allocate an unbounded buffer by claiming
// a huge frame, and a flooded reader cannot grow without limit.
const (
	// PTYFrameHeaderSize is the fixed size of the frame header.
	PTYFrameHeaderSize = 4

	// MaxPTYPayload is the largest payload a single frame may carry. A PTY read
	// buffer is small, so this is generous; it exists to bound allocation.
	MaxPTYPayload = 64 * 1024

	// DefaultPTYClientBuffer is the per-connection queue depth for outbound PTY
	// frames. It is small on purpose: the credit window is the real flow control,
	// and a deep buffer would only add latency to interactive traffic.
	DefaultPTYClientBuffer = 64
)

// PTY frame channels.
const (
	// ChannelPTY carries terminal sessions.
	ChannelPTY byte = 0x01
)

// PTY frame types.
const (
	// PTYTypeData is terminal data: output device->operator, input operator->device.
	PTYTypeData byte = 0x01
	// PTYTypeAck returns flow-control credit (see the registry's credit window).
	PTYTypeAck byte = 0x02
	// PTYTypeOpen requests a new session.
	PTYTypeOpen byte = 0x03
	// PTYTypeClose tears a session down.
	PTYTypeClose byte = 0x04
	// PTYTypeResize carries a window size change (rows, cols).
	PTYTypeResize byte = 0x05
	// PTYTypeError reports a session-level failure.
	PTYTypeError byte = 0x06
)

// ErrPTYFrameTooShort is returned when a buffer cannot hold a frame header.
var ErrPTYFrameTooShort = errors.New("pty frame shorter than header")

// ErrPTYFrameTooLarge is returned when a declared length exceeds MaxPTYPayload.
var ErrPTYFrameTooLarge = errors.New("pty frame exceeds maximum payload")

// EncodePTYFrame builds a binary frame. It panics on an oversized payload
// because every caller works with a read buffer already bounded by
// MaxPTYPayload; silently truncating terminal data would be worse than failing
// loudly in development.
func EncodePTYFrame(channel, frameType byte, payload []byte) []byte {
	if len(payload) > MaxPTYPayload {
		panic(fmt.Sprintf("pty payload %d exceeds max %d", len(payload), MaxPTYPayload))
	}

	out := make([]byte, PTYFrameHeaderSize+len(payload))
	out[0] = channel
	out[1] = frameType
	binary.BigEndian.PutUint16(out[2:4], uint16(len(payload)))
	copy(out[PTYFrameHeaderSize:], payload)

	return out
}

// DecodePTYFrame splits a binary frame into its channel, type and payload. The
// payload aliases the input slice; callers that retain it must copy.
func DecodePTYFrame(raw []byte) (channel, frameType byte, payload []byte, err error) {
	if len(raw) < PTYFrameHeaderSize {
		return 0, 0, nil, ErrPTYFrameTooShort
	}

	channel = raw[0]
	frameType = raw[1]
	length := int(binary.BigEndian.Uint16(raw[2:4]))

	if length > MaxPTYPayload {
		return 0, 0, nil, ErrPTYFrameTooLarge
	}
	if len(raw) < PTYFrameHeaderSize+length {
		return 0, 0, nil, fmt.Errorf("pty frame declares %d payload bytes but carries %d",
			length, len(raw)-PTYFrameHeaderSize)
	}

	return channel, frameType, raw[PTYFrameHeaderSize : PTYFrameHeaderSize+length], nil
}

// EncodePTYResize packs a window size into the payload of a resize frame.
func EncodePTYResize(rows, cols uint16) []byte {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint16(payload[0:2], rows)
	binary.BigEndian.PutUint16(payload[2:4], cols)

	return payload
}

// DecodePTYResize reads a window size from a resize frame payload.
func DecodePTYResize(payload []byte) (rows, cols uint16, err error) {
	if len(payload) < 4 {
		return 0, 0, errors.New("pty resize payload shorter than 4 bytes")
	}

	return binary.BigEndian.Uint16(payload[0:2]), binary.BigEndian.Uint16(payload[2:4]), nil
}

// Session-prefixed payloads.
//
// A device multiplexes several sessions over one WebSocket, so a frame that
// carries session data must name the session. The session ID is a UUID string;
// it is written as a length-prefixed field so the remainder is the raw bytes.
//
//	[1B idLen][idLen bytes session id][data...]

// MaxPTYSessionIDLen bounds the session ID field. A UUID is 36 bytes.
const MaxPTYSessionIDLen = 64

// ErrPTYSessionHeader is returned when a session-prefixed payload is malformed.
var ErrPTYSessionHeader = errors.New("pty frame has an invalid session header")

// EncodeSessionData prefixes data with its session ID.
func EncodeSessionData(sessionID string, data []byte) []byte {
	if len(sessionID) > MaxPTYSessionIDLen {
		panic("pty session id exceeds maximum length")
	}

	out := make([]byte, 1+len(sessionID)+len(data))
	out[0] = byte(len(sessionID))
	copy(out[1:], sessionID)
	copy(out[1+len(sessionID):], data)

	return out
}

// DecodeSessionData splits a session-prefixed payload into its session ID and
// data. The data slice aliases the input; callers that retain it must copy.
func DecodeSessionData(payload []byte) (sessionID string, data []byte, err error) {
	if len(payload) < 1 {
		return "", nil, ErrPTYSessionHeader
	}

	idLen := int(payload[0])
	if idLen == 0 || idLen > MaxPTYSessionIDLen {
		return "", nil, ErrPTYSessionHeader
	}
	if len(payload) < 1+idLen {
		return "", nil, ErrPTYSessionHeader
	}

	return string(payload[1 : 1+idLen]), payload[1+idLen:], nil
}
