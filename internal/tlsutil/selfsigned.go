// Package tlsutil provides helpers to manage the web server certificate.
package tlsutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

const (
	certValidity = 825 * 24 * time.Hour
	// Regenerate the certificate when it expires within this delay.
	renewBefore = 30 * 24 * time.Hour
)

// EnsureSelfSigned returns the paths of a self-signed certificate and key stored
// in dir, generating them when they are missing or about to expire.
func EnsureSelfSigned(dir string) (certFile, keyFile string, err error) {
	certFile = filepath.Join(dir, "selfsigned.crt")
	keyFile = filepath.Join(dir, "selfsigned.key")

	if pair, err := tls.LoadX509KeyPair(certFile, keyFile); err == nil &&
		time.Until(pair.Leaf.NotAfter) > renewBefore {
		return certFile, keyFile, nil
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", "", err
	}
	if err := generate(certFile, keyFile); err != nil {
		return "", "", fmt.Errorf("generate self-signed certificate: %w", err)
	}
	return certFile, keyFile, nil
}

func generate(certFile, keyFile string) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}

	hostname, _ := os.Hostname()
	dnsNames := []string{"localhost"}
	if hostname != "" && hostname != "localhost" {
		dnsNames = append(dnsNames, hostname)
	}

	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "EasyNetServices", Organization: []string{"EasyNetServices"}},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(certValidity),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              dnsNames,
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}

	if err := writePEM(keyFile, "PRIVATE KEY", keyDER, 0o600); err != nil {
		return err
	}
	return writePEM(certFile, "CERTIFICATE", der, 0o644)
}

func writePEM(path, blockType string, der []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if err := pem.Encode(f, &pem.Block{Type: blockType, Bytes: der}); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
