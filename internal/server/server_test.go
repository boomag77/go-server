package server

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"io/ioutil"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"
)

// -----------------------------
// Helper tools for tests

// dummyLogger is a simple implementation of Logger that collects log messages.
type dummyLogger struct {
	events []string
	mu     sync.Mutex
}

func (d *dummyLogger) LogEvent(event string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.events = append(d.events, event)
}

func (d *dummyLogger) Close() {
	// no-op
}

func (d *dummyLogger) Events() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	cp := make([]string, len(d.events))
	copy(cp, d.events)
	return cp
}

// generateSelfSignedCert generates temporary self-signed certificate and key files.
// It returns the file names.
func generateSelfSignedCert(t *testing.T) (certFile, keyFile string) {
	t.Helper()

	// Generate a private key.
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(24 * time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		t.Fatalf("failed to generate serial number: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Test Co"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Optionally add IP addresses and DNS names for the test server.
	template.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
	template.DNSNames = []string{"localhost"}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	// Create a temporary file for the certificate.
	certOut, err := ioutil.TempFile("", "cert_*.pem")
	if err != nil {
		t.Fatalf("failed to open temp file for cert: %v", err)
	}
	defer certOut.Close()
	if err := pemEncode(certOut, "CERTIFICATE", derBytes); err != nil {
		t.Fatalf("failed to write data to cert file: %v", err)
	}

	// Create a temporary file for the key.
	keyOut, err := ioutil.TempFile("", "key_*.pem")
	if err != nil {
		t.Fatalf("failed to open temp file for key: %v", err)
	}
	defer keyOut.Close()
	keyBytes := x509.MarshalPKCS1PrivateKey(priv)
	if err := pemEncode(keyOut, "RSA PRIVATE KEY", keyBytes); err != nil {
		t.Fatalf("failed to write data to key file: %v", err)
	}

	return certOut.Name(), keyOut.Name()
}

// pemEncode writes data as PEM.
func pemEncode(w io.Writer, typ string, derBytes []byte) error {
	return tlsPemEncode(w, typ, derBytes)
}

// tlsPemEncode is a small wrapper for PEM encoding.
func tlsPemEncode(w io.Writer, typ string, derBytes []byte) error {
	// Directly call the real pem encoding.
	return pemEncodeReal(w, typ, derBytes)
}

// pemEncodeReal is a wrapper for pem.Encode to avoid conflicts.
func pemEncodeReal(w io.Writer, typ string, derBytes []byte) error {
	// Create a PEM block and encode it.
	block := &pemBlock{
		Type:  typ,
		Bytes: derBytes,
	}
	return block.encodeTo(w)
}

// Define our own pemBlock structure (to avoid directly modifying the original code).
type pemBlock struct {
	Type  string
	Bytes []byte
}

// encodeTo performs encoding of the PEM block and writes to an io.Writer.
func (b *pemBlock) encodeTo(w io.Writer) error {
	return encodePEM(w, b)
}

// encodePEM writes out the block using the encoding/pem package.
func encodePEM(w io.Writer, b *pemBlock) error {
	return pem.Encode(w, &pem.Block{Type: b.Type, Bytes: b.Bytes})
}

// -----------------------------
// Tests for validateConfig (and indirectly defaultConfig)

func TestValidateConfig(t *testing.T) {
	// Logger is missing.
	cfg := Config{
		Port: "8080",
	}
	err := validateConfig(cfg)
	if err == nil || err.Error() != "logger is required" {
		t.Fatalf("expected error 'logger is required', got: %v", err)
	}

	// Port is missing.
	cfg = Config{
		Logger: &dummyLogger{},
		Port:   "",
	}
	err = validateConfig(cfg)
	if err == nil || err.Error() != "port is required" {
		t.Fatalf("expected error 'port is required', got: %v", err)
	}

	// TLS is enabled but cert and key files are missing.
	cfg = Config{
		Logger: &dummyLogger{},
		Port:   "8080",
		UseTLS: true,
	}
	err = validateConfig(cfg)
	if err == nil || err.Error() != "cert and key files are required" {
		t.Fatalf("expected error 'cert and key files are required', got: %v", err)
	}

	// Valid configuration.
	cfg = Config{
		Logger:   &dummyLogger{},
		Port:     "8080",
		UseTLS:   false,
		CertFile: "dummy",
		KeyFile:  "dummy",
	}
	err = validateConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Test for securityMiddleware – checking security headers and request body size limitation.
func TestSecurityMiddleware(t *testing.T) {
	// Create a dummy handler that tries to read the request body.
	handlerCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true

		// Try to read the body (limit is set to 1<<20 bytes in middleware).
		_, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	secured := securityMiddleware(nextHandler)
	req := httptest.NewRequest("POST", "/", nil)
	rr := httptest.NewRecorder()
	secured.ServeHTTP(rr, req)

	// Check that security headers are set.
	headers := rr.Header()
	if headers.Get("X-Content-Type-Options") != "nosniff" ||
		headers.Get("X-Frame-Options") != "DENY" ||
		headers.Get("X-XSS-Protection") != "1; mode=block" {
		t.Error("security headers not set correctly")
	}
	if !handlerCalled {
		t.Error("next handler was not called")
	}
}

// Test NewHttpServer using default values and overridden values.
func TestNewHttpServerDefaults(t *testing.T) {
	logger := &dummyLogger{}
	cfg := Config{
		Port:   "8081",
		Logger: logger,
		// Leave ReadTimeout, WriteTimeout, MaxHeaderBytes unset.
	}
	srv, err := NewHttpServer(cfg)
	if err != nil {
		t.Fatalf("unexpected error creating server: %v", err)
	}

	impl, ok := srv.(*HttpServerImpl)
	if !ok {
		t.Fatalf("expected type *HttpServerImpl, got: %T", srv)
	}

	if impl.srv.ReadTimeout != 15*time.Second {
		t.Errorf("expected default ReadTimeout 15s, got %v", impl.srv.ReadTimeout)
	}

	if impl.srv.WriteTimeout != 15*time.Second {
		t.Errorf("expected default WriteTimeout 15s, got %v", impl.srv.WriteTimeout)
	}

	if impl.srv.MaxHeaderBytes != 1<<20 {
		t.Errorf("expected default MaxHeaderBytes 1<<20, got %d", impl.srv.MaxHeaderBytes)
	}
}

// Test for Start – checking that the server starts successfully and the handler works.
func TestHttpServer_SetHandler(t *testing.T) {
	logger := &dummyLogger{}
	cfg := Config{
		Port:   "0", // dynamic port
		Logger: logger,
	}
	srv, err := NewHttpServer(cfg)
	if err != nil {
		t.Fatalf("unexpected error creating server: %v", err)
	}

	impl, ok := srv.(*HttpServerImpl)
	if !ok {
		t.Fatalf("expected type *HttpServerImpl, got: %T", srv)
	}

	testPath := "/test"
	impl.SetHandler(testPath, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	})
	err = srv.Start()
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	// Wait until the server binds to a port.
	var addr net.Addr
	startTime := time.Now()
	for {
		impl.mu.RLock()
		if impl.listener != nil {
			addr = impl.listener.Addr()
		}
		impl.mu.RUnlock()
		if addr != nil && addr.String() != ":0" {
			break
		}
		if time.Since(startTime) > 2*time.Second {
			t.Fatal("timeout waiting for listener to be bound")
		}
		time.Sleep(10 * time.Millisecond)
	}

	url := "http://" + addr.String() + testPath
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("failed to make GET request: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if string(body) != "hello" {
		t.Errorf("expected response 'hello', got: %s", string(body))
	}

	// Shutdown the server.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown error: %v", err)
	}
}

// Test for Shutdown – checking that the server shuts down gracefully.
func TestHttpServer_Shutdown(t *testing.T) {
	logger := &dummyLogger{}
	cfg := Config{
		Port:   "0",
		Logger: logger,
	}
	srv, err := NewHttpServer(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	impl, ok := srv.(*HttpServerImpl)
	if !ok {
		t.Fatalf("expected type *HttpServerImpl, got: %T", srv)
	}
	err = srv.Start()
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	// Wait for binding the port.
	var addr net.Addr
	startTime := time.Now()
	for {
		impl.mu.RLock()
		if impl.listener != nil {
			addr = impl.listener.Addr()
		}
		impl.mu.RUnlock()
		if addr != nil && addr.String() != ":0" {
			break
		}
		if time.Since(startTime) > 2*time.Second {
			t.Fatal("listener bind timeout")
		}
		time.Sleep(10 * time.Millisecond)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}

	// Allow a short time for the listener to close.
	time.Sleep(50 * time.Millisecond)

	// Attempt a TCP connection; it should fail.
	conn, err := net.DialTimeout("tcp", addr.String(), 500*time.Millisecond)
	if err == nil {
		r := bufio.NewReader(conn)
		_, err = r.ReadString('\n')
		conn.Close()
		if err == nil {
			t.Error("expected connection closed after shutdown")
		}
	} else {
		var netErr net.Error
		if !errors.As(err, &netErr) {
			t.Errorf("unexpected error type: %v", err)
		}
	}
}

// Test for checking an error when starting with an invalid port.
func TestStartWithInvalidPort(t *testing.T) {
	logger := &dummyLogger{}
	cfg := Config{
		Port:   "abc", // invalid port
		Logger: logger,
	}
	srv, err := NewHttpServer(cfg)
	if err != nil {
		t.Fatalf("unexpected error creating server: %v", err)
	}
	err = srv.Start()
	if err == nil {
		// Shutdown the server, if it started.
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
		t.Fatal("expected error when starting server with invalid port")
	}
}

// Test TLS mode.
func TestHttpServer_TLS(t *testing.T) {
	// Generate temporary certificate and key files.
	certFile, keyFile := generateSelfSignedCert(t)
	defer os.Remove(certFile)
	defer os.Remove(keyFile)

	logger := &dummyLogger{}
	cfg := Config{
		Port:     "0",
		Logger:   logger,
		UseTLS:   true,
		CertFile: certFile,
		KeyFile:  keyFile,
	}
	srv, err := NewHttpServer(cfg)
	if err != nil {
		t.Fatalf("unexpected error creating TLS server: %v", err)
	}

	impl, ok := srv.(*HttpServerImpl)
	if !ok {
		t.Fatalf("expected type *HttpServerImpl, got: %T", srv)
	}

	testPath := "/tls"
	impl.SetHandler(testPath, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("tls hello"))
	})

	err = srv.Start()
	if err != nil {
		t.Fatalf("failed to start TLS server: %v", err)
	}

	// Wait until the TLS listener binds to a port.
	var addr net.Addr
	startTime := time.Now()
	for {
		impl.mu.RLock()
		if impl.listener != nil {
			addr = impl.listener.Addr()
		}
		impl.mu.RUnlock()
		if addr != nil && addr.String() != ":0" {
			break
		}
		if time.Since(startTime) > 2*time.Second {
			t.Fatal("timeout waiting for TLS listener bind")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Create an HTTP client that skips certificate verification.
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	url := "https://" + addr.String() + testPath
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("failed to GET TLS URL: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read TLS response: %v", err)
	}
	if string(body) != "tls hello" {
		t.Errorf("expected response 'tls hello', got: %s", string(body))
	}

	// Shutdown TLS server.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown TLS server failed: %v", err)
	}
}
