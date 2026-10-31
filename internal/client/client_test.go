package client

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func startMock(t *testing.T, password string) (*mockRouter, *Client) {
	t.Helper()
	router := newMockRouter(t, password)
	server := httptest.NewServer(router.handler())
	router.server = server
	t.Cleanup(server.Close)
	cfg := Config{Host: server.URL, Username: "admin", Password: password}
	cl, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return router, cl
}

// TestProtocolConformance exercises the full SG handshake and every table
// verb against a mock implementing the router's side of the protocol.
func TestProtocolConformance(t *testing.T) {
	router, cl := startMock(t, "s3cret-password")
	if err := cl.Authorize(); err != nil {
		t.Fatalf("authorize: %v", err)
	}

	// Login captured: the mock verified sha256 digest, RSA-v1.5 password
	// decryption, and the OAEP session-key signature already.
	if len(router.seenBodies["login"]) == 0 {
		t.Fatal("mock did not record a login body")
	}

	// Firmware read.
	fw, err := cl.GetFirmware()
	if err != nil {
		t.Fatalf("GetFirmware: %v", err)
	}
	if fw.Model != "" || fw.FirmwareVersion != "" {
		// The mock returns an empty object for unseeded reads; both empty
		// strings are the correct parse result.
		t.Fatalf("unexpected firmware parse: %+v", fw)
	}

	// Port-forward CRUD against the fake table.
	if err := cl.AddPortForward(PortForward{
		Name: "web", IP: "192.168.0.4", ExternalPort: "443",
		InternalPort: "8443", Protocol: "tcp", Enabled: true,
	}); err != nil {
		t.Fatalf("AddPortForward: %v", err)
	}
	if err := cl.AddPortForward(PortForward{
		Name: "game", IP: "192.168.0.18", ExternalPort: "27000-27010",
		InternalPort: "27000", Protocol: "UDP", Enabled: false,
	}); err != nil {
		t.Fatalf("AddPortForward range: %v", err)
	}
	rules, err := cl.GetPortForwards()
	if err != nil {
		t.Fatalf("GetPortForwards: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d: %+v", len(rules), rules)
	}
	if rules[0].Name != "web" || rules[0].Protocol != "TCP" || rules[0].ExternalPort != "443" {
		t.Fatalf("unexpected first rule: %+v", rules[0])
	}
	if rules[1].ExternalPort != "27000-27010" || rules[1].Enabled {
		t.Fatalf("unexpected second rule: %+v", rules[1])
	}

	// Update matches by old payload (name change is force-replaced upstream,
	// so only fields change here).
	if err := cl.UpdatePortForward("web", PortForward{
		Name: "web", IP: "192.168.0.9", ExternalPort: "443",
		InternalPort: "8443", Protocol: "TCP", Enabled: false,
	}); err != nil {
		t.Fatalf("UpdatePortForward: %v", err)
	}
	rules, _ = cl.GetPortForwards()
	if rules[0].IP != "192.168.0.9" || rules[0].Enabled {
		t.Fatalf("update not applied: %+v", rules[0])
	}

	// Delete removes by key + index.
	if err := cl.DeletePortForward("web"); err != nil {
		t.Fatalf("DeletePortForward: %v", err)
	}
	rules, _ = cl.GetPortForwards()
	if len(rules) != 1 || rules[0].Name != "game" {
		t.Fatalf("unexpected rules after delete: %+v", rules)
	}

	// DHCP reservation CRUD.
	res := DHCPReservation{MAC: "aa:bb:cc:dd:ee:ff", IP: "192.168.0.50",
		Hostname: "printer", Enabled: true}
	if err := cl.AddDHCPReservation(res); err != nil {
		t.Fatalf("AddDHCPReservation: %v", err)
	}
	got, err := cl.GetDHCPReservations()
	if err != nil {
		t.Fatalf("GetDHCPReservations: %v", err)
	}
	if len(got) != 1 || got[0].MAC != "AA-BB-CC-DD-EE-FF" || got[0].IP != "192.168.0.50" {
		t.Fatalf("unexpected reservations: %+v", got)
	}
	if err := cl.UpdateDHCPReservation("aa:bb:cc:dd:ee:ff",
		DHCPReservation{MAC: "AA-BB-CC-DD-EE-FF", IP: "192.168.0.51",
			Hostname: "printer", Enabled: false}); err != nil {
		t.Fatalf("UpdateDHCPReservation: %v", err)
	}
	got, _ = cl.GetDHCPReservations()
	if got[0].IP != "192.168.0.51" || got[0].Enabled {
		t.Fatalf("reservation update not applied: %+v", got[0])
	}
	if err := cl.DeleteDHCPReservation("AA-BB-CC-DD-EE-FF"); err != nil {
		t.Fatalf("DeleteDHCPReservation: %v", err)
	}
	got, _ = cl.GetDHCPReservations()
	if len(got) != 0 {
		t.Fatalf("reservation not deleted: %+v", got)
	}

	// Session cleanup.
	if err := cl.Logout(); err != nil {
		t.Fatalf("Logout: %v", err)
	}
}

// TestDeleteNonFirstDHCPReservation ensures removal uses the selected row's
// index and key, rather than always sending index=0. The mock enforces both.
func TestDeleteNonFirstDHCPReservation(t *testing.T) {
	_, cl := startMock(t, "pw")
	reservations := []DHCPReservation{
		{MAC: "00:11:22:33:44:01", IP: "192.168.0.101", Hostname: "first", Enabled: true},
		{MAC: "00:11:22:33:44:02", IP: "192.168.0.102", Hostname: "middle", Enabled: true},
		{MAC: "00:11:22:33:44:03", IP: "192.168.0.103", Hostname: "last", Enabled: true},
	}
	for _, reservation := range reservations {
		if err := cl.AddDHCPReservation(reservation); err != nil {
			t.Fatalf("AddDHCPReservation(%q): %v", reservation.MAC, err)
		}
	}
	if err := cl.DeleteDHCPReservation("00:11:22:33:44:02"); err != nil {
		t.Fatalf("DeleteDHCPReservation(middle): %v", err)
	}
	got, err := cl.GetDHCPReservations()
	if err != nil {
		t.Fatalf("GetDHCPReservations: %v", err)
	}
	if len(got) != 2 || got[0].MAC != "00-11-22-33-44-01" || got[1].MAC != "00-11-22-33-44-03" {
		t.Fatalf("removed wrong DHCP reservation: %+v", got)
	}
}

// TestStaleDeleteIndexFailsClosed simulates a separate router web UI session
// shifting the row indices between a provider load and its remove request.
// The mock checks BOTH the index and identity, so it rejects the stale
// request rather than deleting the unrelated row now occupying that index.
// Actual firmware behavior under such a race has not been verified.
func TestStaleDeleteIndexFailsClosed(t *testing.T) {
	t.Run("DHCP reservation", func(t *testing.T) {
		router, cl := startMock(t, "pw")
		for i := 1; i <= 2; i++ {
			err := cl.AddDHCPReservation(DHCPReservation{
				MAC: fmt.Sprintf("00:11:22:33:44:%02X", i),
				IP:  fmt.Sprintf("192.168.0.%d", 100+i), Enabled: true,
			})
			if err != nil {
				t.Fatal(err)
			}
		}
		router.allowedFailurePrefix = "remove: key/index mismatch"
		router.beforeWrite = func(path, operation string) {
			if path != "admin/dhcps?form=reservation" || operation != "remove" {
				t.Errorf("unexpected write: %q %q", path, operation)
				return
			}
			rows := router.state[path]
			router.state[path] = append([]map[string]any{{
				"mac": "00-11-22-33-44-00", "ip": "192.168.0.100",
				"hostname": "external", "enable": "on",
			}}, rows...)
		}
		if err := cl.DeleteDHCPReservation("00:11:22:33:44:02"); err == nil {
			t.Fatal("expected a conflict when the router table index changes")
		}
		remaining, err := cl.GetDHCPReservations()
		if err != nil {
			t.Fatal(err)
		}
		if len(remaining) != 3 || remaining[0].MAC != "00-11-22-33-44-00" ||
			remaining[1].MAC != "00-11-22-33-44-01" || remaining[2].MAC != "00-11-22-33-44-02" {
			t.Fatalf("stale delete modified a row: %+v", remaining)
		}
	})
	t.Run("port forward", func(t *testing.T) {
		router, cl := startMock(t, "pw")
		for i := 1; i <= 2; i++ {
			err := cl.AddPortForward(PortForward{
				Name: fmt.Sprintf("rule-%d", i), IP: "192.168.0.10",
				ExternalPort: fmt.Sprintf("900%d", i),
				InternalPort: fmt.Sprintf("900%d", i), Protocol: "TCP", Enabled: true,
			})
			if err != nil {
				t.Fatal(err)
			}
		}
		router.allowedFailurePrefix = "remove: key/index mismatch"
		router.beforeWrite = func(path, operation string) {
			if path != "admin/nat?form=vs" || operation != "remove" {
				t.Errorf("unexpected write: %q %q", path, operation)
				return
			}
			rows := router.state[path]
			router.state[path] = append([]map[string]any{{
				"name": "external", "ipaddr": "192.168.0.10",
				"external_port": "9090", "internal_port": "9090",
				"protocol": "TCP", "enable": "on",
			}}, rows...)
		}
		if err := cl.DeletePortForward("rule-2"); err == nil {
			t.Fatal("expected a conflict when the router table index changes")
		}
		remaining, err := cl.GetPortForwards()
		if err != nil {
			t.Fatal(err)
		}
		if len(remaining) != 3 || remaining[0].Name != "external" ||
			remaining[1].Name != "rule-1" || remaining[2].Name != "rule-2" {
			t.Fatalf("stale delete modified a rule: %+v", remaining)
		}
	})
}

// TestAuthorizeWrongPassword verifies a wrong password surfaces as a login
// failure and never retries (the router lockout rule).
func TestAuthorizeWrongPassword(t *testing.T) {
	router, cl := startMock(t, "correct-password")
	// Change the password the mock expects after client construction: the
	// client's login digest was computed at New(), so the mock rejects it.
	router.password = "different"
	err := cl.Authorize()
	if err == nil {
		t.Fatal("expected login failure")
	}
	if cl.authorized {
		t.Fatal("client must not become authorized on a rejected login")
	}
}

// TestValidation covers the client-side input validation.
func TestValidation(t *testing.T) {
	_, cl := startMock(t, "pw")
	cases := []struct {
		name string
		err  func() error
	}{
		{"bad protocol", func() error {
			return cl.AddPortForward(PortForward{Name: "x", IP: "10.0.0.1",
				ExternalPort: "80", InternalPort: "80", Protocol: "ICMP", Enabled: true})
		}},
		{"bad external port", func() error {
			return cl.AddPortForward(PortForward{Name: "x", IP: "10.0.0.1",
				ExternalPort: "70000", InternalPort: "80", Protocol: "TCP", Enabled: true})
		}},
		{"descending range", func() error {
			return cl.AddPortForward(PortForward{Name: "x", IP: "10.0.0.1",
				ExternalPort: "9000-8000", InternalPort: "80", Protocol: "TCP", Enabled: true})
		}},
		{"bad internal port", func() error {
			return cl.AddPortForward(PortForward{Name: "x", IP: "10.0.0.1",
				ExternalPort: "80", InternalPort: "abc", Protocol: "TCP", Enabled: true})
		}},
		{"bad internal range", func() error {
			return cl.AddPortForward(PortForward{Name: "x", IP: "10.0.0.1",
				ExternalPort: "80", InternalPort: "80-90", Protocol: "TCP", Enabled: true})
		}},
		{"empty name", func() error {
			return cl.AddPortForward(PortForward{Name: "", IP: "10.0.0.1",
				ExternalPort: "80", InternalPort: "80", Protocol: "TCP", Enabled: true})
		}},
	}
	for _, tc := range cases {
		if err := tc.err(); err == nil {
			t.Errorf("%s: expected an error", tc.name)
		}
	}
}

// TestNotAuthorized checks that a client refuses to hit the API before login
// and that Authorize is implicit on first use.
func TestNotAuthorized(t *testing.T) {
	router, cl := startMock(t, "pw")
	if cl.authorized {
		t.Fatal("fresh client must not be authorized")
	}
	// Implicit authorize on first call, then a successful read.
	if err := cl.AddPortForward(PortForward{Name: "implicit", IP: "10.0.0.2",
		ExternalPort: "80", InternalPort: "80", Protocol: "TCP", Enabled: true}); err != nil {
		t.Fatalf("implicit authorize failed: %v", err)
	}
	if !router.authorized {
		t.Fatal("mock router did not authorize")
	}
}

// TestPkcs7Roundtrip covers the AES padding helpers.
func TestPkcs7Roundtrip(t *testing.T) {
	for _, size := range []int{0, 1, 15, 16, 17, 31, 32, 100} {
		plain := bytes.Repeat([]byte("x"), size)
		enc, err := aesEncrypt(plain, "1234567890123456", "6543210987654321")
		if err != nil {
			t.Fatalf("size %d: encrypt: %v", size, err)
		}
		dec, err := aesDecrypt(enc, "1234567890123456", "6543210987654321")
		if err != nil {
			t.Fatalf("size %d: decrypt: %v", size, err)
		}
		if !bytes.Equal(plain, dec) {
			t.Fatalf("size %d: roundtrip mismatch", size)
		}
	}
	if _, err := aesDecrypt(hex.EncodeToString([]byte("bad")), "1234567890123456", "6543210987654321"); err == nil {
		t.Fatal("expected an error for non-base64 ciphertext")
	}
}

// TestNormalizeMAC checks the dash-uppercase normalization the router uses.
func TestNormalizeMAC(t *testing.T) {
	cases := map[string]string{
		"aa:bb:cc:dd:ee:ff": "AA-BB-CC-DD-EE-FF",
		"AA-BB-CC-DD-EE-FF": "AA-BB-CC-DD-EE-FF",
		"aabb.ccdd.eeff":    "AA-BB-CC-DD-EE-FF",
	}
	for in, want := range cases {
		if got := NormalizeMAC(in); got != want {
			t.Errorf("NormalizeMAC(%q) = %q, want %q", in, got, want)
		}
	}
}

func jsonRaw(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// TestMACNormalizationEndToEnd exercises the round-trip the resource layer
// relies on: a reservation written with a colon-lowercase MAC must be
// readable and deletable under any equivalent spelling, and never produce a
// second row.
func TestMACNormalizationEndToEnd(t *testing.T) {
	_, cl := startMock(t, "pw")
	res := DHCPReservation{MAC: "aa:bb:cc:dd:ee:01", IP: "192.168.0.60",
		Hostname: "norm", Enabled: true}
	if err := cl.AddDHCPReservation(res); err != nil {
		t.Fatalf("AddDHCPReservation: %v", err)
	}
	for _, spelling := range []string{
		"aa:bb:cc:dd:ee:01", "AA-BB-CC-DD-EE-01", "aabb.ccdd.ee01",
	} {
		got, err := cl.GetDHCPReservations()
		if err != nil {
			t.Fatal(err)
		}
		matches := 0
		for _, r := range got {
			if NormalizeMAC(r.MAC) == NormalizeMAC(spelling) {
				matches++
			}
		}
		if matches != 1 {
			t.Fatalf("spelling %q resolved to %d rows, want 1: %+v", spelling, matches, got)
		}
	}
	if err := cl.DeleteDHCPReservation("aabb.ccdd.ee01"); err != nil {
		t.Fatalf("DeleteDHCPReservation(dotted): %v", err)
	}
	got, _ := cl.GetDHCPReservations()
	if len(got) != 0 {
		t.Fatalf("row not removed: %+v", got)
	}
}

// TestDuplicateCreateAndMissingDelete verifies that identity is unique and
// deleting a missing target is safe when the router was changed out-of-band.
func TestDuplicateCreateAndMissingDelete(t *testing.T) {
	_, cl := startMock(t, "pw")
	r := DHCPReservation{MAC: "01:02:03:04:05:06", IP: "192.168.0.2", Hostname: "test", Enabled: true}
	if err := cl.AddDHCPReservation(r); err != nil {
		t.Fatal(err)
	}
	if err := cl.AddDHCPReservation(r); err == nil {
		t.Fatal("duplicate DHCP reservation unexpectedly accepted")
	}
	res, err := cl.GetDHCPReservations()
	if err != nil || len(res) != 1 {
		t.Fatalf("unexpected DHCP rows: %+v, %v", res, err)
	}
	if err := cl.DeleteDHCPReservation(r.MAC); err != nil {
		t.Fatal(err)
	}
	if err := cl.DeleteDHCPReservation(r.MAC); err != nil {
		t.Fatalf("idempotent DHCP delete: %v", err)
	}

	f := PortForward{Name: "test", IP: "192.168.0.2", ExternalPort: "1234", InternalPort: "1234", Protocol: "udp", Enabled: true}
	if err := cl.AddPortForward(f); err != nil {
		t.Fatal(err)
	}
	if err := cl.AddPortForward(f); err == nil {
		t.Fatal("duplicate port forward unexpectedly accepted")
	}
	rules, err := cl.GetPortForwards()
	if err != nil || len(rules) != 1 {
		t.Fatalf("unexpected port forwards: %+v, %v", rules, err)
	}
	if err := cl.DeletePortForward(f.Name); err != nil {
		t.Fatal(err)
	}
	if err := cl.DeletePortForward(f.Name); err != nil {
		t.Fatalf("idempotent port delete: %v", err)
	}
}

// The provider may apply distinct resources concurrently. Their reads and
// index-based removals must form one serialized transaction per client.
func TestConcurrentDHCPDeletes(t *testing.T) {
	_, cl := startMock(t, "pw")
	const count = 12
	for i := 0; i < count; i++ {
		mac := fmt.Sprintf("00:11:22:33:44:%02X", i)
		if err := cl.AddDHCPReservation(DHCPReservation{MAC: mac, IP: fmt.Sprintf("192.168.0.%d", i+10), Enabled: true}); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	errors := make(chan error, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errors <- cl.DeleteDHCPReservation(fmt.Sprintf("00:11:22:33:44:%02X", i))
		}(i)
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Errorf("concurrent delete: %v", err)
		}
	}
	remaining, err := cl.GetDHCPReservations()
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected empty table, got %+v", remaining)
	}
}

// Terraform cancellation must stop an in-flight router HTTP request promptly.
func TestPostFormCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		<-req.Context().Done()
	}))
	defer server.Close()
	cl, err := New(Config{Host: server.URL, Password: "pw"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = cl.postForm(server.URL, "operation=read", nil, "", ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled HTTP request, got %v", err)
	}
}

// fingerprintOf mocks nothing: it is the pin computation itself (SHA-256
// over DER SubjectPublicKeyInfo), used to derive pins from the mock's keys.
func fingerprintOf(t *testing.T, key *rsa.PrivateKey) string {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:])
}

// TestKeyPins verifies optional user-configured RSA public-key pins: matching
// pins log in, substituted keys fail closed before the login POST, malformed
// pins are rejected at configuration time, and legitimate rotation is handled
// by updating the pin.
func TestKeyPins(t *testing.T) {
	router, _ := startMock(t, "pw")
	pwdFP := fingerprintOf(t, router.pwdKey)
	authFP := fingerprintOf(t, router.authKey)

	// Matching pins: login succeeds end to end.
	cl, err := New(Config{
		Host: router.server.URL, Password: "pw",
		PasswordKeyPin: pwdFP, AuthKeyPin: "sha256:" + authFP, // prefix + case tolerated
	})
	if err != nil {
		t.Fatalf("New with pins: %v", err)
	}
	if err := cl.Authorize(); err != nil {
		t.Fatalf("authorize with matching pins: %v", err)
	}

	// Substituted password key: fail closed before the login POST.
	loginBodiesBefore := len(router.seenBodies["login"])
	badPwd, err := New(Config{Host: router.server.URL, Password: "pw",
		PasswordKeyPin: strings.Repeat("ab", 32)})
	if err != nil {
		t.Fatal(err)
	}
	err = badPwd.Authorize()
	if err == nil || !strings.Contains(err.Error(), "password key pin mismatch") {
		t.Fatalf("expected pin mismatch error, got: %v", err)
	}
	if len(router.seenBodies["login"]) != loginBodiesBefore {
		t.Fatal("pin mismatch must fail before the login POST is sent")
	}

	// Substituted auth key: fail closed after keys, before login.
	badAuth, err := New(Config{Host: router.server.URL, Password: "pw",
		AuthKeyPin: strings.Repeat("cd", 32)})
	if err != nil {
		t.Fatal(err)
	}
	err = badAuth.Authorize()
	if err == nil || !strings.Contains(err.Error(), "auth key pin mismatch") {
		t.Fatalf("expected auth pin mismatch error, got: %v", err)
	}
	if len(router.seenBodies["login"]) != loginBodiesBefore {
		t.Fatal("auth pin mismatch must fail before the login POST is sent")
	}

	// Malformed pin: rejected at configuration time, before any I/O.
	if _, err := New(Config{Host: router.server.URL, Password: "pw",
		PasswordKeyPin: "tooshort"}); err == nil {
		t.Fatal("expected malformed pin to be rejected by New")
	}
}

// TestKeyPinRotation verifies legitimate key rotation: a router whose keys
// changed fails the old pin, and succeeds once the pin is updated.
func TestKeyPinRotation(t *testing.T) {
	router, _ := startMock(t, "pw")
	oldPwdFP := fingerprintOf(t, router.pwdKey)

	cl, err := New(Config{Host: router.server.URL, Password: "pw",
		PasswordKeyPin: oldPwdFP})
	if err != nil {
		t.Fatal(err)
	}
	if err := cl.Authorize(); err != nil {
		t.Fatalf("authorize with pre-rotation pin: %v", err)
	}

	// The router rotates its password key.
	newKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	router.pwdKey = newKey
	err = cl.requestPwdKeys(context.Background())
	if err == nil || !strings.Contains(err.Error(), "pin mismatch") {
		t.Fatalf("expected pin mismatch after rotation, got: %v", err)
	}

	// Updating the pin to the rotated key restores login.
	newFP := fingerprintOf(t, router.pwdKey)
	cl2, err := New(Config{Host: router.server.URL, Password: "pw",
		PasswordKeyPin: newFP})
	if err != nil {
		t.Fatal(err)
	}
	if err := cl2.Authorize(); err != nil {
		t.Fatalf("authorize with rotated pin: %v", err)
	}
}
