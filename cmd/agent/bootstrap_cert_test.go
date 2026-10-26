package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestBootstrapEndpointsFallsBackToPublic(t *testing.T) {
	a := newBootstrapApp(t, false)
	a.cfg.Panel.Host = "203.0.113.10"
	a.cfg.Panel.Port = 8080
	a.cfg.Panel.PublicHost = "panel.example.com"
	a.cfg.Panel.PublicPort = 443
	a.cfg.Panel.PublicTLS = true

	endpoints, err := a.bootstrapEndpoints()
	if err != nil {
		t.Fatalf("bootstrapEndpoints: %v", err)
	}
	if len(endpoints) != 2 {
		t.Fatalf("expected direct + public endpoints, got %d", len(endpoints))
	}
	// Direct is HTTP, public is HTTPS: try public first to avoid sending the
	// bootstrap token in cleartext.
	if endpoints[0].url != "https://panel.example.com:443/api/v1/agent/bootstrap" {
		t.Errorf("first (public) URL = %q", endpoints[0].url)
	}
	if endpoints[0].tlsCfg == nil || endpoints[0].tlsCfg.ServerName != "panel.example.com" {
		t.Errorf("public endpoint must verify the CDN hostname, got %+v", endpoints[0].tlsCfg)
	}
	if endpoints[1].url != "http://203.0.113.10:8080/api/v1/agent/bootstrap" {
		t.Errorf("second (direct) URL = %q", endpoints[1].url)
	}

	// Direct HTTPS is the strictest path, so it stays first when both are TLS.
	a.cfg.Panel.TLS = true
	endpoints, err = a.bootstrapEndpoints()
	if err != nil {
		t.Fatal(err)
	}
	if endpoints[0].url != "https://203.0.113.10:8080/api/v1/agent/bootstrap" {
		t.Errorf("direct HTTPS should be first, got %q", endpoints[0].url)
	}

	// Same endpoint twice: no pointless retry.
	a.cfg.Panel.PublicHost = a.cfg.Panel.Host
	a.cfg.Panel.PublicPort = a.cfg.Panel.Port
	a.cfg.Panel.PublicTLS = a.cfg.Panel.TLS
	endpoints, err = a.bootstrapEndpoints()
	if err != nil {
		t.Fatal(err)
	}
	if len(endpoints) != 1 {
		t.Fatalf("duplicate endpoint should be skipped, got %d", len(endpoints))
	}
}

func TestVerifyBootstrapCertChainsToPanelCA(t *testing.T) {
	certPEM, keyPEM, caPEM := makeBootstrapCert(t, 7)
	caPath := filepath.Join(t.TempDir(), "data", "certs", "ca-cert.pem")
	if err := os.MkdirAll(filepath.Dir(caPath), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(caPath, caPEM, 0640); err != nil {
		t.Fatal(err)
	}

	if err := verifyBootstrapCert(string(certPEM), string(keyPEM), caPath, 7); err != nil {
		t.Fatalf("valid cert rejected: %v", err)
	}
	if err := verifyBootstrapCert(string(certPEM), string(keyPEM), caPath, 8); err == nil {
		t.Error("certificate for server 7 must not be accepted for server 8")
	}

	_, otherKey, _ := makeBootstrapCert(t, 7)
	if err := verifyBootstrapCert(string(certPEM), string(otherKey), caPath, 7); err == nil {
		t.Error("certificate must match the returned private key")
	}

	_, _, otherCAPEM := makeBootstrapCert(t, 7)
	otherCAPath := filepath.Join(t.TempDir(), "certs", "ca-cert.pem")
	if err := os.MkdirAll(filepath.Dir(otherCAPath), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(otherCAPath, otherCAPEM, 0640); err != nil {
		t.Fatal(err)
	}
	if err := verifyBootstrapCert(string(certPEM), string(keyPEM), otherCAPath, 7); err == nil {
		t.Error("certificate must not be accepted with a different panel CA")
	}
}

func makeBootstrapCert(t *testing.T, serverID int64) (certPEM, keyPEM, caPEM []byte) {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{Organization: []string{"CoreFusion"}, CommonName: "CoreFusion Agent CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}

	agentKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	agentTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Organization:       []string{"CoreFusion"},
			OrganizationalUnit: []string{"server-" + strconv.FormatInt(serverID, 10)},
			CommonName:         "agent-test",
		},
		NotBefore:   time.Now().Add(-time.Minute),
		NotAfter:    time.Now().Add(time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	agentDER, err := x509.CreateCertificate(rand.Reader, agentTmpl, caCert, &agentKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(agentKey)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: agentDER}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}),
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
}
