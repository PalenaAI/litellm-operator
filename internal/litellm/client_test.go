/*
Copyright 2026 bitkaio LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package litellm

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"testing"
	"time"
)

// selfSignedCAPEM returns a throwaway self-signed CA certificate in PEM form.
func selfSignedCAPEM(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Unix(0, 0),
		NotAfter:              time.Unix(1<<31-1, 0),
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func TestNewClient_WithCACert_SetsRootCAs(t *testing.T) {
	ca := selfSignedCAPEM(t)
	c := NewClient("https://gw.default.svc:4000", "sk-master", WithCACert(ca)).(*httpClient)

	tr, ok := c.http.Transport.(*http.Transport)
	if !ok || tr == nil {
		t.Fatalf("expected *http.Transport, got %T", c.http.Transport)
	}
	if tr.TLSClientConfig == nil || tr.TLSClientConfig.RootCAs == nil {
		t.Fatal("expected RootCAs to be configured from the CA bundle")
	}
	if tr.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("must never disable verification")
	}
}

func TestNewClient_WithCACert_EmptyIsNoop(t *testing.T) {
	c := NewClient("http://gw.default.svc:4000", "sk-master", WithCACert(nil)).(*httpClient)
	if c.http.Transport != nil {
		t.Fatalf("empty CA must leave the default transport (nil), got %T", c.http.Transport)
	}
}

func TestNewClient_NoOptions_DefaultTransport(t *testing.T) {
	c := NewClient("http://gw.default.svc:4000", "sk-master").(*httpClient)
	if c.http.Transport != nil {
		t.Fatal("no options must leave the default transport")
	}
}
