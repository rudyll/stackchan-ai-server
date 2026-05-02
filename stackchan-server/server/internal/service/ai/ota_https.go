/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

package ai

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
)

// StartOTAHTTPS starts an HTTPS server on :443 for the OTA endpoint.
// This intercepts device OTA requests when DNS for api.tenclass.net is
// overridden to point to this host. A self-signed cert is generated at
// startup — ESP32 firmware typically skips certificate validation for
// custom server URLs.
func StartOTAHTTPS() {
	go func() {
		ctx := gctx.New()

		cert, err := selfSignedCert()
		if err != nil {
			g.Log().Warningf(ctx, "OTA HTTPS: cert generation failed: %v", err)
			return
		}

		mux := http.NewServeMux()
		mux.HandleFunc("/xiaozhi/ota/", HandleOTA)

		srv := &http.Server{
			Addr:    ":443",
			Handler: mux,
			TLSConfig: &tls.Config{
				Certificates: []tls.Certificate{cert},
			},
		}

		g.Log().Infof(ctx, "OTA HTTPS intercept listening on :443 (api.tenclass.net DNS override)")
		if err := srv.ListenAndServeTLS("", ""); err != nil {
			g.Log().Warningf(ctx, "OTA HTTPS server: %v", err)
		}
	}()
}

// selfSignedCert generates an in-memory self-signed TLS certificate valid for
// api.tenclass.net and api.XiaoZhi.me.
func selfSignedCert() (tls.Certificate, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "api.tenclass.net"},
		DNSNames:     []string{"api.tenclass.net", "api.XiaoZhi.me"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return tls.X509KeyPair(certPEM, keyPEM)
}
