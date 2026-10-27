package client

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// mockRouter implements the SG-hardened protocol server-side (the router's
// side of the handshake) so the Go client is tested against a real
// implementation of the wire format rather than against itself.
//
// Session flow: the login request carries the client's AES key/IV inside an
// RSA-OAEP-encrypted signature; the mock decrypts it, stores it, and answers
// AES-encrypted. Post-login requests are AES-encrypted and HMAC-signed with
// the stored session key; the mock verifies the HMAC, decrypts the body, and
// dispatches the operation against a fake table.
type mockRouter struct {
	t *testing.T

	pwdKey   *rsa.PrivateKey
	authKey  *rsa.PrivateKey
	seq      int
	stok     string
	sysauth  string
	password string

	authorized bool
	sessKey    string
	sessIV     string

	state      map[string][]map[string]any
	seenBodies map[string][]string
}

func newMockRouter(t *testing.T, password string) *mockRouter {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	authKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	stok := make([]byte, 16)
	if _, err := rand.Read(stok); err != nil {
		t.Fatal(err)
	}
	return &mockRouter{
		t:          t,
		pwdKey:     key,
		authKey:    authKey,
		seq:        1234567,
		stok:       hex.EncodeToString(stok),
		sysauth:    "mock-sysauth-cookie",
		password:   password,
		state:      map[string][]map[string]any{},
		seenBodies: map[string][]string{},
	}
}

func (m *mockRouter) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/cgi-bin/luci/;stok=/login", m.handleLogin)
	mux.HandleFunc("/cgi-bin/luci/", m.handleAuthenticated)
	return mux
}

func (m *mockRouter) publicHex(key *rsa.PrivateKey) (string, string) {
	return fmt.Sprintf("%x", key.PublicKey.N), fmt.Sprintf("%x", big.NewInt(int64(key.PublicKey.E)))
}

func (m *mockRouter) reply(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": data})
}

func (m *mockRouter) replyEncrypted(w http.ResponseWriter, payload []byte) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"data": aesEncryptMock(m.t, payload, m.sessKey, m.sessIV)})
}

func (m *mockRouter) fail(w http.ResponseWriter, msg string) {
	m.t.Errorf("mock router: %s", msg)
	m.reject(w)
}

// reject answers success=false without failing the test (normal auth
// rejections).
func (m *mockRouter) reject(w http.ResponseWriter) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"success":false}`))
}

func (m *mockRouter) record(path, body string) {
	m.seenBodies[path] = append(m.seenBodies[path], body)
}

func (m *mockRouter) handleLogin(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Query().Get("form") {
	case "keys":
		nn, ee := m.publicHex(m.pwdKey)
		m.reply(w, map[string]any{"password": []string{nn, ee}})
	case "auth":
		nn, ee := m.publicHex(m.authKey)
		m.reply(w, map[string]any{"seq": m.seq, "key": []string{nn, ee}})
	case "login":
		m.handleLoginSubmit(w, r)
	default:
		m.fail(w, "no such callback: "+r.URL.RawQuery)
	}
}

// decryptLoginSignature decrypts the chunked OAEP signature to recover the
// client's AES session key, IV, login digest and sequence.
func (m *mockRouter) decryptLoginSignature(sign string) (k, i, h string, s int, err error) {
	// Each RSA-2048 block arrives as 512 hex characters.
	blockLen := m.authKey.PublicKey.N.BitLen() / 4
	if len(sign) == 0 || len(sign)%blockLen != 0 {
		return "", "", "", 0, fmt.Errorf("signature not block-aligned: %d vs %d", len(sign), blockLen)
	}
	var plain strings.Builder
	for off := 0; off < len(sign); off += blockLen {
		block, err := hex.DecodeString(sign[off : off+blockLen])
		if err != nil {
			return "", "", "", 0, err
		}
		dec, err := rsa.DecryptOAEP(sha1.New(), rand.Reader, m.authKey, block, nil)
		if err != nil {
			return "", "", "", 0, err
		}
		plain.Write(dec)
	}
	parts := strings.Split(plain.String(), "&")
	if len(parts) != 4 || !strings.HasPrefix(parts[0], "k=") ||
		!strings.HasPrefix(parts[1], "i=") || !strings.HasPrefix(parts[2], "h=") ||
		!strings.HasPrefix(parts[3], "s=") {
		return "", "", "", 0, fmt.Errorf("bad signature payload: %s", plain.String())
	}
	s, err = strconv.Atoi(parts[3][2:])
	if err != nil {
		return "", "", "", 0, err
	}
	return parts[0][2:], parts[1][2:], parts[2][2:], s, nil
}

func (m *mockRouter) handleLoginSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		m.t.Fatal(err)
	}
	k, iv, h, s, err := m.decryptLoginSignature(r.PostForm.Get("sign"))
	if err != nil {
		m.fail(w, "login signature: "+err.Error())
		return
	}
	// s must be seq + len(base64 ciphertext).
	if s != m.seq+len(r.PostForm.Get("data")) {
		m.fail(w, fmt.Sprintf("login s=%d, want seq+len(data)=%d", s, m.seq+len(r.PostForm.Get("data"))))
		return
	}
	// A digest or password mismatch is a normal rejected login, not a
	// protocol bug: reject without failing the test.
	if h != fmt.Sprintf("%x", sha256.Sum256([]byte("admin"+m.password))) {
		m.reject(w)
		return
	}
	body := aesDecryptMock(m.t, r.PostForm.Get("data"), k, iv)
	m.record("login", string(body))

	re := regexp.MustCompile(`^operation=login&password=([0-9a-fA-F]+)&confirm=true$`)
	match := re.FindStringSubmatch(string(body))
	if match == nil {
		m.fail(w, "bad login body: "+string(body))
		return
	}
	// The v1.5 ciphertext is zero-padded to the modulus hex length (512 chars
	// for RSA-2048); each RSA block is half as many raw bytes.
	hexPwd := match[1]
	blockLen := m.pwdKey.PublicKey.N.BitLen() / 4
	if len(hexPwd) > blockLen {
		m.fail(w, "password ciphertext longer than modulus")
		return
	}
	if len(hexPwd)%2 == 1 {
		hexPwd = "0" + hexPwd
	}
	block, err := hex.DecodeString(hexPwd)
	if err != nil {
		m.fail(w, "bad password hex: "+err.Error())
		return
	}
	plain, err := rsa.DecryptPKCS1v15(rand.Reader, m.pwdKey, block)
	if err != nil {
		m.reject(w)
		return
	}
	if string(plain) != m.password {
		m.reject(w)
		return
	}
	m.authorized = true
	m.sessKey, m.sessIV = k, iv

	respBody, _ := json.Marshal(map[string]any{
		"success": true,
		"data":    map[string]any{"stok": m.stok},
	})
	http.SetCookie(w, &http.Cookie{Name: "sysauth", Value: m.sysauth})
	m.replyEncrypted(w, respBody)
}

// verifyRequestSignature recomputes the chunked HMAC-SHA256 signature the
// client must send for post-login requests.
func (m *mockRouter) verifyRequestSignature(sign, encryptedData string) error {
	bodyHash := sha256.Sum256([]byte(encryptedData))
	signStr := fmt.Sprintf("h=%s&s=%d", hex.EncodeToString(bodyHash[:]), m.seq+len(encryptedData))
	key := []byte(fmt.Sprintf("k=%s&i=%s", m.sessKey, m.sessIV))
	var expected strings.Builder
	for i := 0; i < len(signStr); i += 53 {
		end := min(i+53, len(signStr))
		mac := hmac.New(sha256.New, key)
		mac.Write([]byte(signStr[i:end]))
		expected.WriteString(hex.EncodeToString(mac.Sum(nil)))
	}
	if sign != expected.String() {
		return fmt.Errorf("HMAC signature mismatch")
	}
	return nil
}

func (m *mockRouter) handleAuthenticated(w http.ResponseWriter, r *http.Request) {
	if !m.authorized {
		m.fail(w, "not authorized")
		return
	}
	if !strings.Contains(r.URL.Path, ";stok="+m.stok) {
		m.fail(w, "missing stok in URL: "+r.URL.Path)
		return
	}
	if ck, err := r.Cookie("sysauth"); err != nil || ck.Value != m.sysauth {
		m.fail(w, "missing sysauth cookie")
		return
	}
	if err := r.ParseForm(); err != nil {
		m.t.Fatal(err)
	}
	if err := m.verifyRequestSignature(r.PostForm.Get("sign"), r.PostForm.Get("data")); err != nil {
		m.fail(w, err.Error())
		return
	}
	body := aesDecryptMock(m.t, r.PostForm.Get("data"), m.sessKey, m.sessIV)
	form, err := url.ParseQuery(string(body))
	if err != nil {
		m.fail(w, "bad request body: "+err.Error())
		return
	}

	base := r.URL.Path[strings.Index(r.URL.Path, ";stok="+m.stok)+len(";stok="+m.stok):]
	formName := r.URL.Query().Get("form")
	pathKey := strings.TrimPrefix(base, "/") + "?form=" + formName
	m.record(pathKey, string(body))

	operation := form.Get("operation")
	switch operation {
	case "read":
		m.replyEncrypted(w, []byte(`{"success":true,"data":{"data":`+mockRows(m.state[pathKey])+`}}`))
	case "load":
		rows := m.state[pathKey]
		if rows == nil {
			rows = []map[string]any{}
		}
		m.replyEncrypted(w, []byte(`{"success":true,"data":`+mockRows(rows)+`}`))
	case "insert":
		var row map[string]any
		if err := json.Unmarshal([]byte(form.Get("new")), &row); err != nil {
			m.fail(w, "bad insert new: "+err.Error())
			return
		}
		m.state[pathKey] = append(m.state[pathKey], row)
		m.replyEncrypted(w, []byte(`{"success":true,"data":{}}`))
	case "update":
		var newRow, oldRow map[string]any
		if err := json.Unmarshal([]byte(form.Get("new")), &newRow); err != nil {
			m.fail(w, "bad update new: "+err.Error())
			return
		}
		if err := json.Unmarshal([]byte(form.Get("old")), &oldRow); err != nil {
			m.fail(w, "bad update old: "+err.Error())
			return
		}
		rows := m.state[pathKey]
		for idx, existing := range rows {
			if sameRow(existing, oldRow) {
				rows[idx] = newRow
				m.replyEncrypted(w, []byte(`{"success":true,"data":{}}`))
				return
			}
		}
		m.fail(w, "update: no row matches old payload")
	case "remove":
		var removed bool
		key := form.Get("key")
		rows := m.state[pathKey]
		for idx, existing := range rows {
			// Rows are identified by their natural key: name for port
			// forwards, mac for reservations.
			if existing["name"] == key || existing["mac"] == key {
				m.state[pathKey] = append(rows[:idx], rows[idx+1:]...)
				removed = true
				break
			}
		}
		if !removed {
			m.fail(w, "remove: no row matches key")
			return
		}
		m.replyEncrypted(w, []byte(`{"success":true,"data":{}}`))
	case "write":
		// Scalar writes (DHCP server toggle, logout, ...): accept and succeed.
		m.replyEncrypted(w, []byte(`{"success":true,"data":{}}`))
	default:
		m.fail(w, "unknown operation: "+operation)
	}
}

// sameRow compares by natural key: MAC for reservations, name for port
// forwards (mirroring how the router identifies rows).
func sameRow(a, b map[string]any) bool {
	if ma, ok := a["mac"]; ok {
		if mb, ok := b["mac"]; ok {
			return ma == mb
		}
	}
	return a["name"] == b["name"]
}

func (m *mockRouter) readState(pathKey string) map[string]any {
	rows := m.state[pathKey]
	if rows == nil {
		rows = []map[string]any{}
	}
	return map[string]any{"data": rows}
}

// --- shared AES helpers for the mock ---

func aesDecryptMock(t *testing.T, data string, key, iv string) []byte {
	t.Helper()
	block, _ := aes.NewCipher([]byte(key))
	ct, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(ct) == 0 || len(ct)%block.BlockSize() != 0 {
		t.Fatal("mock: ciphertext not block aligned")
	}
	out := make([]byte, len(ct))
	cipher.NewCBCDecrypter(block, []byte(iv)).CryptBlocks(out, ct)
	pad := int(out[len(out)-1])
	if pad == 0 || pad > 16 {
		t.Fatal("mock: bad padding")
	}
	return out[:len(out)-pad]
}

func aesEncryptMock(t *testing.T, plaintext []byte, key, iv string) string {
	t.Helper()
	block, _ := aes.NewCipher([]byte(key))
	padLen := 16 - len(plaintext)%16
	padded := append(append([]byte{}, plaintext...), make([]byte, padLen)...)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(padLen)
	}
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, []byte(iv)).CryptBlocks(out, padded)
	return base64.StdEncoding.EncodeToString(out)
}

// mockRows renders a fake table as JSON (a list; empty tables are {}).
func mockRows(rows []map[string]any) string {
	if len(rows) == 0 {
		return "{}"
	}
	b, _ := json.Marshal(rows)
	return string(b)
}
