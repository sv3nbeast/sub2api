package kiro

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
)

func TestValidateProviderThinkingSignatureAcceptsNativeKiroEnvelope(t *testing.T) {
	signature := providerThinkingSignatureFixture(t, true)

	metadata, err := validateProviderThinkingSignature(signature)
	require.NoError(t, err)
	require.Equal(t, "claude-quince", metadata.WireModel)
	require.Equal(t, uint64(16), metadata.ChannelVersion)
	require.Zero(t, metadata.ChannelKind)
	require.Equal(t, "015911059195", metadata.ContextID)
	require.Equal(t, 128, metadata.SignedPayloadBytes)
}

func TestValidateProviderThinkingSignatureAcceptsCurrentChannelVersion17(t *testing.T) {
	signature := providerThinkingSignatureFixtureWithVersion(t, true, 17)

	metadata, err := validateProviderThinkingSignature(signature)
	require.NoError(t, err)
	require.Equal(t, uint64(17), metadata.ChannelVersion)
}

func TestValidateProviderThinkingSignatureAcceptsChannelVersion18(t *testing.T) {
	// Opus 4.8 and Opus 5 on the Q endpoint, 2026-09-26.
	metadata, err := validateProviderThinkingSignature(providerThinkingSignatureFixtureWithVersion(t, true, 18))
	require.NoError(t, err)
	require.Equal(t, uint64(18), metadata.ChannelVersion)
	require.Equal(t, "claude-quince", metadata.WireModel)
}

func TestValidateProviderThinkingSignatureAcceptsCompactChannelVersion18(t *testing.T) {
	// Opus 5.5 on the Q endpoint, 2026-09-26: no channel signature, provider
	// channel or context ID in the channel header.
	metadata, err := validateProviderThinkingSignature(providerThinkingSignatureFixtureOmitting(t, true, 18, 5, 6, 11))
	require.NoError(t, err)
	require.Equal(t, uint64(18), metadata.ChannelVersion)
	require.Equal(t, uint64(1), metadata.ChannelKind)
	require.Empty(t, metadata.WireModel)
	require.Empty(t, metadata.ContextID)
	require.Equal(t, 128, metadata.SignedPayloadBytes)
}

func TestValidateProviderThinkingSignatureRejectsPartialCompactChannel(t *testing.T) {
	// Missing only some of the three fields is a malformed header, not the
	// compact layout.
	_, err := validateProviderThinkingSignature(providerThinkingSignatureFixtureOmitting(t, true, 18, 5, 11))
	require.ErrorContains(t, err, "channel signature field count is 0")
	_, err = validateProviderThinkingSignature(providerThinkingSignatureFixtureOmitting(t, true, 18, 11))
	require.ErrorContains(t, err, "context ID field count is 0")
}

func TestValidateProviderThinkingSignatureRejectsCompactChannelBeforeVersion18(t *testing.T) {
	_, err := validateProviderThinkingSignature(providerThinkingSignatureFixtureOmitting(t, true, 17, 5, 6, 11))
	require.ErrorContains(t, err, "compact channel header needs version 18, got 17")
}

func TestValidateProviderThinkingSignatureRejectsCompactChannelWithoutProviderMarker(t *testing.T) {
	_, err := validateProviderThinkingSignature(providerThinkingSignatureFixtureOmitting(t, false, 18, 5, 6, 11))
	require.ErrorContains(t, err, "provider-native marker field count is 0")
}

func TestValidateProviderThinkingSignatureRejectsUnknownChannelVersion(t *testing.T) {
	_, err := validateProviderThinkingSignature(providerThinkingSignatureFixtureWithVersion(t, true, 19))
	require.ErrorContains(t, err, "channel version is 19, want 16, 17 or 18")
}

func TestValidateProviderThinkingSignatureRejectsFormerLocalFallbackShape(t *testing.T) {
	_, err := validateProviderThinkingSignature(providerThinkingSignatureFixture(t, false))
	require.ErrorContains(t, err, "provider-native marker field count is 0")
}

func TestValidateProviderThinkingSignatureRejectsMalformedBase64(t *testing.T) {
	_, err := validateProviderThinkingSignature("not a signature")
	require.ErrorContains(t, err, "decode provider thinking signature")
}

func providerThinkingSignatureFixtureWithOuterMarker(t *testing.T) string {
	t.Helper()
	wire, err := base64.StdEncoding.DecodeString(providerThinkingSignatureFixture(t, true))
	require.NoError(t, err)
	prefix := protowire.AppendTag(nil, 1, protowire.VarintType)
	prefix = protowire.AppendVarint(prefix, 2)
	wire = append(prefix, wire...)
	return base64.StdEncoding.EncodeToString(wire)
}

func providerThinkingSignatureFixture(t *testing.T, providerNative bool) string {
	t.Helper()
	return providerThinkingSignatureFixtureWithVersion(t, providerNative, 16)
}

func providerThinkingSignatureFixtureWithVersion(t *testing.T, providerNative bool, channelVersion uint64) string {
	t.Helper()
	return providerThinkingSignatureFixtureOmitting(t, providerNative, channelVersion)
}

// providerThinkingSignatureFixtureOmitting builds the envelope without the
// given channel header fields (5 channel signature, 6 provider channel, 11
// context ID). Omitting all three yields Opus 5.5's compact header, which also
// reports channel kind 1.
func providerThinkingSignatureFixtureOmitting(t *testing.T, providerNative bool, channelVersion uint64, omit ...protowire.Number) string {
	t.Helper()
	omitted := func(field protowire.Number) bool {
		for _, f := range omit {
			if f == field {
				return true
			}
		}
		return false
	}
	appendVarint := func(dst []byte, field protowire.Number, value uint64) []byte {
		dst = protowire.AppendTag(dst, field, protowire.VarintType)
		return protowire.AppendVarint(dst, value)
	}
	appendBytes := func(dst []byte, field protowire.Number, value []byte) []byte {
		dst = protowire.AppendTag(dst, field, protowire.BytesType)
		return protowire.AppendBytes(dst, value)
	}
	repeated := func(value byte, count int) []byte {
		result := make([]byte, count)
		for i := range result {
			result[i] = value
		}
		return result
	}

	channel := appendVarint(nil, 1, channelVersion)
	if providerNative {
		channel = appendVarint(channel, 2, 1)
	}
	channel = appendVarint(channel, 3, 2)
	if !omitted(5) {
		channel = appendBytes(channel, 5, repeated(0x51, 64))
	}
	if !omitted(6) {
		channel = appendBytes(channel, 6, []byte("claude-quince"))
	}
	compact := omitted(5) && omitted(6) && omitted(11)
	if compact {
		channel = appendVarint(channel, 7, 1)
	} else {
		channel = appendVarint(channel, 7, 0)
	}
	channel = appendBytes(channel, 8, []byte("thinking"))
	if !omitted(11) {
		channel = appendBytes(channel, 11, []byte("015911059195"))
	}

	inner := appendBytes(nil, 1, channel)
	inner = appendBytes(inner, 2, repeated(0x52, 12))
	inner = appendBytes(inner, 3, repeated(0x53, 12))
	inner = appendBytes(inner, 4, repeated(0x54, 48))
	inner = appendBytes(inner, 5, repeated(0x55, 128))

	body := appendBytes(nil, 2, inner)
	body = appendVarint(body, 3, 1)
	return base64.StdEncoding.EncodeToString(body)
}
