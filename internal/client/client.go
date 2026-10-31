// Package client implements the TP-Link "SG-hardened" new-generation router
// API (SG_L1_S2 certification, as used by Archer AX23 v2 / AX53 / BE-series
// on the 2024-2026 Vue web UI, firmware 1.1.x/1.11.x-era builds).
//
// Wire protocol (verified against Archer AX23 v2.0, fw 1.1.2 Build 20250814):
//
//   - Every call is a form POST to {host}/cgi-bin/luci/;stok=<stok>/<path>.
//   - Login: POST /login?form=keys -> RSA-2048 modulus+exponent for the
//     password; /login?form=auth -> sequence number + RSA key for the session
//     key exchange. The password itself is RSA PKCS#1 v1.5 encrypted; the
//     login payload is AES-128-CBC encrypted with a session key (16 random
//     digits) that is delivered to the router inside an RSA-OAEP-encrypted
//     signature (SHA-1 OAEP, 53-byte chunks).
//   - After login every request body is AES-128-CBC (base64) and signed with
//     chunked HMAC-SHA256 over "h=<sha256-of-ciphertext>&s=<seq+len>",
//     where the hash is recomputed per request from the encrypted body.
//   - Table endpoints use operation verbs: read/load (list), write (scalar),
//     insert (new=<json>), update (key/new/old=<json>), remove (key/index).
//   - Sessions carry a stok token (URL) plus a sysauth cookie. One admin
//     session exists at a time; a second login evicts the first.
package client

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// signatureOffset is the chunk size (bytes) for the login RSA-OAEP signature
// and the per-request HMAC signature. It is sized for PKCS#1 v1.5's 11-byte
// overhead; the login path caps the OAEP chunk at keysize-2*20-2 bytes to
// respect OAEP's larger overhead (see signLogin).
const signatureOffset = 53

const aesKeyLen = 16

// Error codes the router returns under four different field spellings
// (errorCode, errorcode, error, error_code); normalized here.
const (
	errTimeout      = "timeout"
	errUserConflict = "user conflict"
)

// Client talks to one router. It is safe for concurrent use: calls are
// serialized because the router keeps a single admin session and the
// signature scheme assumes sequential requests.
type Client struct {
	host     string
	username string
	password string
	http     *http.Client

	mu          sync.Mutex
	mutationMu  sync.Mutex // covers multi-request table operations
	aesKey      string
	aesIV       string
	seq         int
	nn, ee      string // auth key material (session key exchange)
	pwdNN       string // password key material (modulus, hex)
	pwdPin      string // optional configured pin for the password key
	authPin     string // optional configured pin for the auth key
	pwdEE       string
	stok        string
	sysauth     string
	authorized  bool
	loginDigest string // sha256(username + password)
}

// Config configures a Client.
type Config struct {
	// Host is the router base address, e.g. "http://192.168.0.1".
	Host string
	// Username defaults to "admin" when empty.
	Username string
	// Password is the router admin password.
	Password string
	// TLSSkipVerify skips TLS verification for https:// hosts (the router
	// ships a self-signed certificate).
	TLSSkipVerify bool
	// Timeout is the per-request timeout; defaults to 30s.
	Timeout time.Duration
	// PasswordKeyPin, when set, is the expected SHA-256 fingerprint (hex, of
	// the key's DER SubjectPublicKeyInfo encoding) of the router's
	// password-encryption RSA key served by login?form=keys. The router's
	// key is fingerprinted before any password encryption happens; a
	// mismatch fails the login. Case-insensitive; an optional "sha256:"
	// prefix is accepted.
	PasswordKeyPin string
	// AuthKeyPin, when set, is the expected SHA-256 fingerprint of the
	// session key-exchange RSA key served by login?form=auth. Same rules as
	// PasswordKeyPin.
	AuthKeyPin string
}

// New returns a Client for cfg. It performs no I/O; call Authorize.
func New(cfg Config) (*Client, error) {
	host := strings.TrimRight(cfg.Host, "/")
	if host == "" {
		return nil, errors.New("tplink: host is required")
	}
	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		host = "http://" + host
	}
	username := cfg.Username
	if username == "" {
		username = "admin"
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	pwdPin, err := normalizePin(cfg.PasswordKeyPin)
	if err != nil {
		return nil, fmt.Errorf("tplink: password key pin: %w", err)
	}
	authPin, err := normalizePin(cfg.AuthKeyPin)
	if err != nil {
		return nil, fmt.Errorf("tplink: auth key pin: %w", err)
	}
	if cfg.TLSSkipVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // self-signed router cert
	}
	sum := sha256.Sum256([]byte(username + cfg.Password))
	return &Client{
		host:        host,
		username:    username,
		password:    cfg.Password,
		http:        &http.Client{Timeout: timeout, Transport: transport},
		loginDigest: hex.EncodeToString(sum[:]),
		pwdPin:      pwdPin,
		authPin:     authPin,
	}, nil
}

func randomDigits(n int) (string, error) {
	b := make([]byte, n)
	for i := range b {
		// Preserve the router protocol's decimal-digit key format, but draw
		// each digit from a cryptographically secure, unbiased source.
		digit, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", fmt.Errorf("tplink: generate session key: %w", err)
		}
		b[i] = '0' + byte(digit.Int64())
	}
	return string(b), nil
}

func (c *Client) formURL(stok, path string) string {
	return fmt.Sprintf("%s/cgi-bin/luci/;stok=%s/%s", c.host, stok, path)
}

func (c *Client) postForm(rawURL, body string, headers map[string]string, sysauth string, ctx context.Context) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Referer", c.host+"/webpages/index.html")
	if sysauth != "" {
		req.AddCookie(&http.Cookie{Name: "sysauth", Value: sysauth})
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.http.Do(req)
}

// Authorize performs the SG login handshake and stores the session.
func (c *Client) Authorize() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.authorizeLocked(context.Background())
}

func (c *Client) authorizeLocked(ctx context.Context) error {
	if err := c.requestPwdKeys(ctx); err != nil {
		return err
	}
	if err := c.requestAuthKeys(ctx); err != nil {
		return err
	}

	var err error
	c.aesKey, err = randomDigits(aesKeyLen)
	if err != nil {
		return err
	}
	c.aesIV, err = randomDigits(aesKeyLen)
	if err != nil {
		return err
	}

	encryptedPwd, err := rsaV15Encrypt([]byte(c.password), c.pwdNN, c.pwdEE)
	if err != nil {
		return fmt.Errorf("tplink: encrypt password: %w", err)
	}
	loginData := "operation=login&password=" + encryptedPwd + "&confirm=true"
	encryptedData, err := aesEncrypt([]byte(loginData), c.aesKey, c.aesIV)
	if err != nil {
		return fmt.Errorf("tplink: encrypt login payload: %w", err)
	}
	sign, err := c.signLogin(len(encryptedData))
	if err != nil {
		return err
	}

	// The router's form parser requires the sign parameter first; a form
	// encoder that sorts keys (data before sign) is rejected with HTTP 403.
	form := "sign=" + sign + "&data=" + url.QueryEscape(encryptedData)
	resp, err := c.postForm(c.formURL("", "login?form=login"), form, nil, "", ctx)
	if err != nil {
		return fmt.Errorf("tplink: login request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var envelope struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || envelope.Data == "" {
		return fmt.Errorf("tplink: cannot authorize: unexpected login response (status %d): %s",
			resp.StatusCode, truncate(body))
	}
	decrypted, err := aesDecrypt(envelope.Data, c.aesKey, c.aesIV)
	if err != nil {
		return fmt.Errorf("tplink: cannot authorize: %w", err)
	}
	var login struct {
		Success bool `json:"success"`
		Data    struct {
			Stok string `json:"stok"`
		} `json:"data"`
		DataError *ErrorBody `json:"-"`
	}
	if err := json.Unmarshal(decrypted, &login); err != nil {
		return fmt.Errorf("tplink: cannot authorize: bad login payload: %w", err)
	}
	if !login.Success {
		return fmt.Errorf("tplink: login failed: %s", errorFrom(decrypted))
	}
	if login.Data.Stok == "" {
		return errors.New("tplink: login succeeded without a session token")
	}
	c.stok = login.Data.Stok
	for _, ck := range resp.Cookies() {
		if ck.Name == "sysauth" {
			c.sysauth = ck.Value
		}
	}
	if c.sysauth == "" {
		return errors.New("tplink: login succeeded without a sysauth cookie")
	}
	c.authorized = true
	return nil
}

func (c *Client) requestPwdKeys(ctx context.Context) error {
	resp, err := c.postForm(c.formURL("", "login?form=keys"),
		"operation=read", nil, "", ctx)
	if err != nil {
		return fmt.Errorf("tplink: fetch password keys: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var parsed struct {
		Data struct {
			Password []string `json:"password"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil || len(parsed.Data.Password) != 2 {
		return fmt.Errorf("tplink: failed to get password keys: %s", truncate(body))
	}
	c.pwdNN, c.pwdEE = parsed.Data.Password[0], parsed.Data.Password[1]
	if err := c.verifyPin(c.pwdPin, c.pwdNN, c.pwdEE, "password"); err != nil {
		return err
	}
	return nil
}

func (c *Client) requestAuthKeys(ctx context.Context) error {
	resp, err := c.postForm(c.formURL("", "login?form=auth"),
		"operation=read", nil, "", ctx)
	if err != nil {
		return fmt.Errorf("tplink: fetch auth keys: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var parsed struct {
		Data struct {
			Seq int      `json:"seq"`
			Key []string `json:"key"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil || len(parsed.Data.Key) != 2 || parsed.Data.Seq == 0 {
		return fmt.Errorf("tplink: failed to get auth keys: %s", truncate(body))
	}
	c.seq = parsed.Data.Seq
	c.nn, c.ee = parsed.Data.Key[0], parsed.Data.Key[1]
	if err := c.verifyPin(c.authPin, c.nn, c.ee, "auth"); err != nil {
		return err
	}
	return nil
}

// verifyPin enforces an optional user-configured RSA public-key pin. The
// router's key is fingerprinted (SHA-256 over DER SubjectPublicKeyInfo) and
// compared before any password encryption or session-key exchange happens;
// a mismatch fails closed and never re-enrolls.
func (c *Client) verifyPin(pin, nHex, eHex, role string) error {
	if pin == "" {
		return nil
	}
	pub, err := rsaPublic(nHex, eHex)
	if err != nil {
		return fmt.Errorf("tplink: %s key: %w", role, err)
	}
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return fmt.Errorf("tplink: %s key: %w", role, err)
	}
	sum := sha256.Sum256(der)
	fp := hex.EncodeToString(sum[:])
	if fp != pin {
		return fmt.Errorf(
			"tplink: %s key pin mismatch: router key fingerprint sha256:%s does not match the configured pin sha256:%s; "+
				"if the router was replaced or its keys were rotated intentionally, update the pin",
			role, fp, pin)
	}
	return nil
}

// normalizePin validates and canonicalizes a user-configured key pin:
// lowercase hex, with an optional "sha256:" prefix stripped.
func normalizePin(pin string) (string, error) {
	p := strings.ToLower(strings.TrimSpace(pin))
	if p == "" {
		return "", nil // no pin configured
	}
	p = strings.TrimPrefix(p, "sha256:")
	if len(p) != 64 {
		return "", errors.New("pin must be 64 hex characters (SHA-256), optionally prefixed with 'sha256:'")
	}
	if _, err := hex.DecodeString(p); err != nil {
		return "", errors.New("pin must be hexadecimal")
	}
	return p, nil
}

// do performs one authenticated, encrypted request and returns the response
// payload (the "data" member, or the whole decrypted body when absent).
// On a session timeout it re-authorizes once and retries.
func requestContext(ctxs []context.Context) context.Context {
	if len(ctxs) > 0 && ctxs[0] != nil {
		return ctxs[0]
	}
	return context.Background()
}

// Terraform can cancel an in-flight HTTP operation; legacy command-line
// callers may omit a context and use context.Background().
func (c *Client) do(path string, form url.Values, ctxs ...context.Context) (json.RawMessage, error) {
	ctx := requestContext(ctxs)
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.authorized {
		if err := c.authorizeLocked(ctx); err != nil {
			return nil, err
		}
	}
	payload, err := c.doLocked(ctx, path, form)
	if err == nil {
		return payload, nil
	}
	var authErr *AuthExpiredError
	if errors.As(err, &authErr) {
		if err := c.authorizeLocked(ctx); err != nil {
			return nil, err
		}
		return c.doLocked(ctx, path, form)
	}
	return nil, err
}

// AuthExpiredError signals the router ended the session (single-admin limit
// or idle expiry); the caller may retry once after re-authorizing.
type AuthExpiredError struct{ Code string }

func (e *AuthExpiredError) Error() string {
	return "tplink: session expired (" + e.Code + ")"
}

func (c *Client) doLocked(ctx context.Context, path string, form url.Values) (json.RawMessage, error) {
	plain := []byte(form.Encode())
	encrypted, err := aesEncrypt(plain, c.aesKey, c.aesIV)
	if err != nil {
		return nil, err
	}
	sign, err := c.signRequest(encrypted)
	if err != nil {
		return nil, err
	}
	body := "sign=" + sign + "&data=" + url.QueryEscape(encrypted)
	resp, err := c.postForm(c.formURL(c.stok, path), body, nil, c.sysauth, ctx)
	if err != nil {
		return nil, fmt.Errorf("tplink: request %s: %w", path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.Data == "" {
		return nil, fmt.Errorf("tplink: request %s: malformed response: %s", path, truncate(raw))
	}
	decrypted, err := aesDecrypt(envelope.Data, c.aesKey, c.aesIV)
	if err != nil {
		return nil, fmt.Errorf("tplink: request %s: %w", path, err)
	}

	var parsed struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
		Error   *ErrorBody      `json:"-"`
	}
	if err := json.Unmarshal(decrypted, &parsed); err != nil {
		return nil, fmt.Errorf("tplink: request %s: bad payload: %w (%s)", path, err, truncate(decrypted))
	}
	if parsed.Success {
		if len(parsed.Data) == 0 {
			return json.RawMessage(decrypted), nil
		}
		return parsed.Data, nil
	}
	code := errorFrom(decrypted)
	switch code {
	case errTimeout, errUserConflict:
		return nil, &AuthExpiredError{Code: code}
	}
	return nil, &RouterError{Path: path, Code: code, Body: string(decrypted)}
}

// ErrorBody captures the router's error field in any of its spellings.
type ErrorBody struct {
	Code    string
	RawBody string
}

func (e *ErrorBody) UnmarshalJSON(b []byte) error {
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	for _, key := range []string{"errorCode", "errorcode", "error", "error_code"} {
		if v, ok := m[key]; ok {
			switch t := v.(type) {
			case string:
				e.Code = t
			case float64:
				e.Code = strconv.Itoa(int(t))
			}
			return nil
		}
	}
	return nil
}

// RouterError is a request the router answered with success=false.
type RouterError struct {
	Path string
	Code string
	Body string
}

func (e *RouterError) Error() string {
	if e.Code == "" {
		return fmt.Sprintf("tplink: %s: request failed: %s", e.Path, truncate([]byte(e.Body)))
	}
	return fmt.Sprintf("tplink: %s: %s", e.Path, e.Code)
}

func errorFrom(decrypted []byte) string {
	var eb ErrorBody
	if err := json.Unmarshal(decrypted, &eb); err == nil {
		return eb.Code
	}
	return ""
}

// signLogin builds the RSA-OAEP-encrypted login signature that carries the
// AES session key to the router: chunks of "k=<key>&i=<iv>&h=<digest>&s=<seq+len>"
// encrypted in OAEP blocks capped to the key's safe payload size.
func (c *Client) signLogin(dataLen int) (string, error) {
	signStr := fmt.Sprintf("k=%s&i=%s&h=%s&s=%d", c.aesKey, c.aesIV, c.loginDigest, c.seq+dataLen)
	rsaByteLen := len(c.nn) / 2
	step := signatureOffset
	if max := rsaByteLen - 2*sha1.Size - 2; step > max {
		step = max
	}
	if step <= 0 {
		return "", errors.New("tplink: auth RSA key too small for OAEP signature")
	}
	var out strings.Builder
	for i := 0; i < len(signStr); i += step {
		end := min(i+step, len(signStr))
		chunk, err := rsaOAEPEncrypt([]byte(signStr[i:end]), c.nn, c.ee)
		if err != nil {
			return "", fmt.Errorf("tplink: login signature: %w", err)
		}
		out.WriteString(chunk)
	}
	return out.String(), nil
}

// signRequest builds the chunked HMAC-SHA256 signature for an authenticated
// request. The hash member is recomputed per request from the encrypted body
// (the router's "REPLACE_HASH" behavior).
func (c *Client) signRequest(encryptedData string) (string, error) {
	bodyHash := sha256.Sum256([]byte(encryptedData))
	signStr := fmt.Sprintf("h=%s&s=%d", hex.EncodeToString(bodyHash[:]), c.seq+len(encryptedData))
	key := []byte(fmt.Sprintf("k=%s&i=%s", c.aesKey, c.aesIV))
	var out strings.Builder
	for i := 0; i < len(signStr); i += signatureOffset {
		end := min(i+signatureOffset, len(signStr))
		mac := hmac.New(sha256.New, key)
		mac.Write([]byte(signStr[i:end]))
		out.WriteString(hex.EncodeToString(mac.Sum(nil)))
	}
	return out.String(), nil
}

func truncate(b []byte) string {
	s := string(b)
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	return s
}

// Logout frees the router's single admin session.
func (c *Client) Logout() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.authorized {
		return nil
	}
	c.authorized = false
	// Best-effort: ignore errors, the session dies with the process anyway.
	_, _ = c.doLocked(context.Background(), "admin/system?form=logout", url.Values{"operation": {"write"}})
	c.stok, c.sysauth = "", ""
	return nil
}

// rsaPublic parses modulus/exponent hex strings into a public key.
func rsaPublic(nHex, eHex string) (*rsa.PublicKey, error) {
	return ParseRSAPublicKeyHex(nHex, eHex)
}

// ParseRSAPublicKeyHex parses a router RSA public key given as hexadecimal
// modulus and exponent strings (the wire format of login?form=keys and
// login?form=auth).
func ParseRSAPublicKeyHex(nHex, eHex string) (*rsa.PublicKey, error) {
	n, ok := new(big.Int).SetString(nHex, 16)
	if !ok {
		return nil, errors.New("tplink: bad RSA modulus")
	}
	e, ok := new(big.Int).SetString(eHex, 16)
	if !ok {
		return nil, errors.New("tplink: bad RSA exponent")
	}
	return &rsa.PublicKey{N: n, E: int(e.Int64())}, nil
}

// FetchPreAuthKeys retrieves the router's pre-authentication RSA key pairs —
// the password-encryption key (login?form=keys) and the session
// key-exchange key (login?form=auth) — plus the handshake sequence number,
// without authenticating or creating a session. Returns the password key
// [modulus, exponent], the auth key [modulus, exponent] (hex), and seq.
func FetchPreAuthKeys(ctx context.Context, cfg Config) ([2]string, [2]string, int, error) {
	cl, err := New(cfg)
	if err != nil {
		return [2]string{}, [2]string{}, 0, err
	}
	cl.mu.Lock()
	defer cl.mu.Unlock()
	if err := cl.requestPwdKeys(ctx); err != nil {
		return [2]string{}, [2]string{}, 0, err
	}
	if err := cl.requestAuthKeys(ctx); err != nil {
		return [2]string{}, [2]string{}, 0, err
	}
	return [2]string{cl.pwdNN, cl.pwdEE}, [2]string{cl.nn, cl.ee}, cl.seq, nil
}

func rsaV15Encrypt(data []byte, nHex, eHex string) (string, error) {
	pub, err := rsaPublic(nHex, eHex)
	if err != nil {
		return "", err
	}
	out, err := rsa.EncryptPKCS1v15(rand.Reader, pub, data)
	if err != nil {
		return "", err
	}
	hexed := hex.EncodeToString(out)
	return zeroPad(hexed, len(nHex)), nil
}

func rsaOAEPEncrypt(data []byte, nHex, eHex string) (string, error) {
	pub, err := rsaPublic(nHex, eHex)
	if err != nil {
		return "", err
	}
	out, err := rsa.EncryptOAEP(sha1.New(), rand.Reader, pub, data, nil)
	if err != nil {
		return "", err
	}
	hexed := hex.EncodeToString(out)
	return zeroPad(hexed, len(nHex)), nil
}

func zeroPad(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return strings.Repeat("0", width-len(s)) + s
}

func aesEncrypt(plaintext []byte, key, iv string) (string, error) {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}
	padded := pkcs7Pad(plaintext, block.BlockSize())
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, []byte(iv)).CryptBlocks(out, padded)
	return base64.StdEncoding.EncodeToString(out), nil
}

func aesDecrypt(b64 string, key, iv string) ([]byte, error) {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, err
	}
	ciphertext, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("bad ciphertext encoding: %w", err)
	}
	if len(ciphertext)%block.BlockSize() != 0 || len(ciphertext) == 0 {
		return nil, errors.New("ciphertext is not block-aligned")
	}
	out := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, []byte(iv)).CryptBlocks(out, ciphertext)
	return pkcs7Unpad(out, block.BlockSize())
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padLen := blockSize - len(data)%blockSize
	return append(data, bytes.Repeat([]byte{byte(padLen)}, padLen)...)
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, errors.New("invalid padded data length")
	}
	padLen := int(data[len(data)-1])
	if padLen == 0 || padLen > blockSize || padLen > len(data) {
		return nil, errors.New("invalid padding")
	}
	for _, b := range data[len(data)-padLen:] {
		if int(b) != padLen {
			return nil, errors.New("invalid padding bytes")
		}
	}
	return data[:len(data)-padLen], nil
}
