package util

import (
	"encoding/base64"
	"errors"
	"net/url"
	"strings"

	"github.com/meshtastic/meshtastic-go/pkg/pb"
)

const (
	// ChannelURLScheme is the URL scheme for Meshtastic channel URLs
	ChannelURLScheme = "meshtastic"
	// ChannelURLHost is the host for channel share URLs
	ChannelURLHost = "meshtastic.org"
	// ChannelURLPath is the path prefix for channel URLs
	ChannelURLPath = "/e/"
)

var (
	ErrInvalidURL     = errors.New("invalid meshtastic URL")
	ErrInvalidScheme  = errors.New("invalid URL scheme, expected 'meshtastic' or 'https'")
	ErrInvalidPath    = errors.New("invalid URL path, expected '/e/'")
	ErrEmptyChannelData = errors.New("no channel data in URL")
	ErrDecodeError    = errors.New("failed to decode channel data")
)

// ParseChannelURL parses a meshtastic:// or https://meshtastic.org/e/ URL
// and returns the decoded ChannelSet.
//
// Supported formats:
// - meshtastic://e/<base64-data>
// - https://meshtastic.org/e/#<base64-data>
func ParseChannelURL(urlStr string) (*pb.ChannelSet, error) {
	// Normalize common variations
	urlStr = strings.TrimSpace(urlStr)

	// Handle https URLs
	if strings.HasPrefix(urlStr, "https://") || strings.HasPrefix(urlStr, "http://") {
		return parseHTTPURL(urlStr)
	}

	// Handle meshtastic:// URLs
	if strings.HasPrefix(urlStr, "meshtastic://") {
		return parseMeshtasticURL(urlStr)
	}

	return nil, ErrInvalidScheme
}

func parseHTTPURL(urlStr string) (*pb.ChannelSet, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, ErrInvalidURL
	}

	// Check host
	if u.Host != ChannelURLHost && u.Host != "www."+ChannelURLHost {
		return nil, ErrInvalidURL
	}

	// Check path
	if !strings.HasPrefix(u.Path, "/e/") && !strings.HasPrefix(u.Path, "/e") {
		return nil, ErrInvalidPath
	}

	// Get the base64 data from fragment or path
	var data string
	if u.Fragment != "" {
		data = u.Fragment
	} else {
		// Try to get it from the path
		data = strings.TrimPrefix(u.Path, "/e/")
		data = strings.TrimPrefix(data, "/e")
	}

	if data == "" {
		return nil, ErrEmptyChannelData
	}

	return decodeChannelSet(data)
}

func parseMeshtasticURL(urlStr string) (*pb.ChannelSet, error) {
	// Strip the scheme
	rest := strings.TrimPrefix(urlStr, "meshtastic://")

	// Should start with e/
	if !strings.HasPrefix(rest, "e/") {
		return nil, ErrInvalidPath
	}

	data := strings.TrimPrefix(rest, "e/")
	if data == "" {
		return nil, ErrEmptyChannelData
	}

	return decodeChannelSet(data)
}

func decodeChannelSet(data string) (*pb.ChannelSet, error) {
	// URL-safe base64 decode
	// Replace URL-safe characters with standard base64
	data = strings.ReplaceAll(data, "-", "+")
	data = strings.ReplaceAll(data, "_", "/")

	// Add padding if needed
	switch len(data) % 4 {
	case 2:
		data += "=="
	case 3:
		data += "="
	}

	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, ErrDecodeError
	}

	channelSet, err := pb.UnmarshalChannelSet(decoded)
	if err != nil {
		return nil, ErrDecodeError
	}

	return channelSet, nil
}

// GenerateChannelURL generates a meshtastic:// URL from a ChannelSet.
func GenerateChannelURL(channelSet *pb.ChannelSet) (string, error) {
	if channelSet == nil {
		return "", ErrEmptyChannelData
	}

	// Marshal the channel set
	data, err := pb.MarshalChannelSet(channelSet)
	if err != nil {
		return "", err
	}

	// Encode as URL-safe base64 without padding
	encoded := base64.RawURLEncoding.EncodeToString(data)

	return "meshtastic://e/" + encoded, nil
}

// GenerateChannelHTTPURL generates an https://meshtastic.org/e/ URL from a ChannelSet.
func GenerateChannelHTTPURL(channelSet *pb.ChannelSet) (string, error) {
	if channelSet == nil {
		return "", ErrEmptyChannelData
	}

	// Marshal the channel set
	data, err := pb.MarshalChannelSet(channelSet)
	if err != nil {
		return "", err
	}

	// Encode as URL-safe base64 without padding
	encoded := base64.RawURLEncoding.EncodeToString(data)

	return "https://meshtastic.org/e/#" + encoded, nil
}

// ChannelSetFromChannels creates a ChannelSet from individual channels and LoRa config.
// Only includes active (non-disabled) channels.
func ChannelSetFromChannels(channels []*pb.Channel, loraConfig *pb.Config_LoRaConfig) *pb.ChannelSet {
	cs := &pb.ChannelSet{
		Settings:   make([]*pb.ChannelSettings, 0),
		LoraConfig: loraConfig,
	}

	for _, ch := range channels {
		if ch.Role != uint32(pb.Channel_DISABLED) && ch.Settings != nil {
			cs.Settings = append(cs.Settings, ch.Settings)
		}
	}

	return cs
}

// ChannelsFromChannelSet creates individual Channel messages from a ChannelSet.
// The first channel becomes PRIMARY, the rest become SECONDARY.
func ChannelsFromChannelSet(channelSet *pb.ChannelSet) []*pb.Channel {
	if channelSet == nil {
		return nil
	}

	channels := make([]*pb.Channel, 0, len(channelSet.Settings))
	for i, settings := range channelSet.Settings {
		role := pb.Channel_SECONDARY
		if i == 0 {
			role = pb.Channel_PRIMARY
		}

		channels = append(channels, &pb.Channel{
			Index:    uint32(i),
			Settings: settings,
			Role:     uint32(role),
		})
	}

	return channels
}
