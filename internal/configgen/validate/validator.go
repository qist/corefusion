package validate

import (
	"fmt"

	"corefusion/internal/configgen/common"
	"corefusion/internal/domain"
)

// DefaultCapabilityValidator validates protocol/transport/TLS combinations.
type DefaultCapabilityValidator struct{}

// NewDefaultCapabilityValidator creates a new validator.
func NewDefaultCapabilityValidator() *DefaultCapabilityValidator {
	return &DefaultCapabilityValidator{}
}

// Validate validates a service config spec.
func (v *DefaultCapabilityValidator) Validate(spec common.ServiceConfigSpec) error {
	if !v.SupportsProtocol(spec.CoreType, spec.Protocol) {
		return fmt.Errorf("core %s does not support protocol %s", spec.CoreType, spec.Protocol)
	}
	if !v.SupportsTransport(spec.CoreType, spec.Protocol, spec.Transport) {
		return fmt.Errorf("core %s protocol %s does not support transport %s", spec.CoreType, spec.Protocol, spec.Transport)
	}
	if !v.SupportsTLS(spec.CoreType, spec.Protocol, spec.Transport, spec.TLSMode) {
		return fmt.Errorf("core %s protocol %s transport %s does not support TLS mode %s", spec.CoreType, spec.Protocol, spec.Transport, spec.TLSMode)
	}
	return nil
}

// SupportsProtocol checks if a core supports a protocol.
func (v *DefaultCapabilityValidator) SupportsProtocol(core domain.CoreType, protocol domain.ProtocolType) bool {
	supported := map[domain.CoreType][]domain.ProtocolType{
		domain.CoreTypeXray: {
			domain.ProtocolVLESS,
			domain.ProtocolVMess,
			domain.ProtocolTrojan,
			domain.ProtocolShadowsocks,
			domain.ProtocolDokodemo,
			domain.ProtocolTUN,
		},
		domain.CoreTypeSingBox: {
			domain.ProtocolVLESS,
			domain.ProtocolVMess,
			domain.ProtocolTrojan,
			domain.ProtocolShadowsocks,
			domain.ProtocolShadowsocks2022,
			domain.ProtocolHysteria2,
			domain.ProtocolTUIC,
			domain.ProtocolMixed,
			domain.ProtocolHTTP,
			domain.ProtocolSocks,
			domain.ProtocolNaive,
			domain.ProtocolAnyTLS,
			domain.ProtocolShadowTLS,
			domain.ProtocolHysteria,
			domain.ProtocolWireGuard,
			domain.ProtocolSnell,
			domain.ProtocolTUN,
			domain.ProtocolDokodemo,
		},
		domain.CoreTypeHysteria2: {
			domain.ProtocolHysteria2,
		},
		domain.CoreTypeTUIC: {
			domain.ProtocolTUIC,
		},
	}

	protocols, ok := supported[core]
	if !ok {
		return false
	}

	for _, p := range protocols {
		if p == protocol {
			return true
		}
	}
	return false
}

// SupportsTransport checks if a core+protocol supports a transport.
func (v *DefaultCapabilityValidator) SupportsTransport(core domain.CoreType, protocol domain.ProtocolType, transport domain.TransportType) bool {
	// Hysteria2, TUIC and Hysteria always use QUIC
	if protocol == domain.ProtocolHysteria2 || protocol == domain.ProtocolTUIC || protocol == domain.ProtocolHysteria {
		return transport == domain.TransportQUIC || transport == ""
	}

	// Transport support by core
	supported := map[domain.CoreType][]domain.TransportType{
		domain.CoreTypeXray: {
			domain.TransportTCP,
			domain.TransportWS,
			domain.TransportGRPC,
			domain.TransportHTTP2,
			domain.TransportReality,
			domain.TransportKCP,
			domain.TransportMKCP,
			domain.TransportXHTTP,
			domain.TransportH2,
		},
		domain.CoreTypeSingBox: {
			domain.TransportTCP,
			domain.TransportWS,
			domain.TransportGRPC,
			domain.TransportHTTP2,
			domain.TransportHTTPUpgrade,
			domain.TransportQUIC,
			domain.TransportReality,
			domain.TransportKCP,
		},
	}

	transports, ok := supported[core]
	if !ok {
		return transport == "" || transport == domain.TransportTCP
	}

	// Empty transport defaults to tcp
	if transport == "" {
		return true
	}

	for _, t := range transports {
		if t == transport {
			return true
		}
	}
	return false
}

// SupportsTLS checks if a core+protocol+transport supports a TLS mode.
func (v *DefaultCapabilityValidator) SupportsTLS(core domain.CoreType, protocol domain.ProtocolType, transport domain.TransportType, tls domain.TLSMode) bool {
	// Reality only works with certain transports
	if tls == domain.TLSModeReality {
		if transport != domain.TransportTCP && transport != domain.TransportGRPC && transport != domain.TransportReality {
			return false
		}
		// Reality only supported by Xray and sing-box
		if core != domain.CoreTypeXray && core != domain.CoreTypeSingBox {
			return false
		}
	}

	// Hysteria2, TUIC and Hysteria always use TLS
	if protocol == domain.ProtocolHysteria2 || protocol == domain.ProtocolTUIC || protocol == domain.ProtocolHysteria {
		return tls == domain.TLSModeTLS || tls == domain.TLSModeNone
	}

	return true
}
