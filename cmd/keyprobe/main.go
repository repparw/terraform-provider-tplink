// Command keyprobe fingerprints the router's pre-authentication RSA public
// keys without logging in. Read-only: it only issues the unauthenticated
// POSTs the login handshake itself uses (login?form=keys, login?form=auth).
//
// RSA public keys are parsed from the router's hex modulus/exponent strings
// and fingerprinted as SHA-256 over their canonical DER SubjectPublicKeyInfo
// encoding. No key material is printed.
//
// Usage:
//
//	go run ./cmd/keyprobe [-host URL] [-samples N] [-login]
//
// Environment: TPLINK_PASSWORD (required only for -login), TPLINK_HOST.
package main

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/repparw/terraform-provider-tplink/internal/client"
)

type sample struct {
	keysFP string
	keysSz int
	keysE  int
	authFP string
	authSz int
	authE  int
	seq    int
	at     time.Time
}

func fingerprint(nHex, eHex string) (fp string, bits, e int, err error) {
	pub, err := client.ParseRSAPublicKeyHex(nHex, eHex)
	if err != nil {
		return "", 0, 0, err
	}
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", 0, 0, err
	}
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:]), pub.N.BitLen(), pub.E, nil
}

func main() {
	host := flag.String("host", "", "router address (default TPLINK_HOST or http://192.168.0.1)")
	samples := flag.Int("samples", 10, "samples per phase")
	delay := flag.Duration("delay", 0, "delay between samples within a phase")
	login := flag.Bool("login", false, "also sample across a login/logout cycle")
	wait := flag.Duration("wait", 4*time.Minute, "delay between phases")
	flag.Parse()
	if *samples <= 0 || *delay < 0 || *wait < 0 {
		fmt.Fprintln(os.Stderr, "samples must be positive and delays must be non-negative")
		os.Exit(2)
	}

	password := os.Getenv("TPLINK_PASSWORD")
	if *login && password == "" {
		fmt.Fprintln(os.Stderr, "TPLINK_PASSWORD required for -login")
		os.Exit(2)
	}
	target := *host
	if target == "" {
		target = os.Getenv("TPLINK_HOST")
	}
	if target == "" {
		target = "http://192.168.0.1"
	}

	var all []sample
	collect := func(phase string) error {
		var phaseSamples []sample
		for i := 0; i < *samples; i++ {
			s, err := probeOnce(context.Background(), target)
			if err != nil {
				return fmt.Errorf("%s sample %d: %w", phase, i+1, err)
			}
			phaseSamples = append(phaseSamples, s)
			all = append(all, s)
			if i < *samples-1 && *delay > 0 {
				time.Sleep(*delay)
			}
		}
		return reportPhase(phase, phaseSamples, phaseSamples[0])
	}

	fmt.Printf("# phase A: %d unauthenticated samples\n", *samples)
	if err := collect("A"); err != nil {
		fail(err)
	}

	fmt.Printf("\n# waiting %s (phase separation)\n", *wait)
	time.Sleep(*wait)

	fmt.Printf("# phase B: %d unauthenticated samples after wait\n", *samples)
	if err := collect("B"); err != nil {
		fail(err)
	}

	if *login {
		fmt.Printf("\n# phase C: sample -> normal login -> logout -> sample\n")
		s, err := probeOnce(context.Background(), target)
		if err != nil {
			fail(err)
		}
		all = append(all, s)
		if err := reportPhase("C-pre-login", []sample{s}, s); err != nil {
			fail(err)
		}

		cl, err := client.New(client.Config{Host: target, Password: password, TLSSkipVerify: true})
		if err != nil {
			fail(err)
		}
		if err := cl.Authorize(); err != nil {
			fail(fmt.Errorf("login: %w", err))
		}
		if err := cl.Logout(); err != nil {
			fail(err)
		}
		fmt.Println("# logged in and out")

		s, err = probeOnce(context.Background(), target)
		if err != nil {
			fail(err)
		}
		all = append(all, s)
		if err := reportPhase("C-post-login", []sample{s}, s); err != nil {
			fail(err)
		}

		fmt.Printf("\n# waiting %s (phase separation)\n", *wait)
		time.Sleep(*wait)
	}

	fmt.Printf("\n# phase D: %d unauthenticated samples (final)\n", *samples)
	if err := collect("D"); err != nil {
		fail(err)
	}

	// Cross-phase summary.
	fmt.Printf("\n# cross-phase summary\n")
	seen := distinctKeyPairs(all)
	if len(seen) == 1 {
		fmt.Printf("RESULT: keys stable across all %d collected samples (single fingerprint pair)\n", len(all))
	} else {
		fmt.Printf("RESULT: %d distinct fingerprint pairs observed\n", len(seen))
		for fp := range seen {
			fmt.Println("  pair:", fp)
		}
	}
}

func probeOnce(ctx context.Context, target string) (sample, error) {
	k, a, seq, err := client.FetchPreAuthKeys(ctx, client.Config{
		Host:          target,
		TLSSkipVerify: true,
	})
	if err != nil {
		return sample{}, err
	}
	keysFP, keysSz, keysE, err := fingerprint(k[0], k[1])
	if err != nil {
		return sample{}, fmt.Errorf("password key: %w", err)
	}
	authFP, authSz, authE, err := fingerprint(a[0], a[1])
	if err != nil {
		return sample{}, fmt.Errorf("auth key: %w", err)
	}
	return sample{
		keysFP: keysFP, keysSz: keysSz, keysE: keysE,
		authFP: authFP, authSz: authSz, authE: authE,
		seq: seq, at: time.Now(),
	}, nil
}

func reportPhase(phase string, samples []sample, first sample) error {
	if len(samples) == 0 {
		return nil
	}
	for _, s := range samples[1:] {
		if s.keysFP != first.keysFP || s.authFP != first.authFP {
			return fmt.Errorf("%s: fingerprints vary within phase (first keys %s vs %s; auth %s vs %s)",
				phase, first.keysFP, s.keysFP, first.authFP, s.authFP)
		}
	}
	fmt.Printf("%s: %d samples | password key: %d-bit, e=%d, sha256(spki)=%s | auth key: %d-bit, e=%d, sha256(spki)=%s | seq=%d\n",
		phase, len(samples),
		first.keysSz, first.keysE, short(first.keysFP),
		first.authSz, first.authE, short(first.authFP),
		first.seq)
	return nil
}

func short(fp string) string {
	if len(fp) > 16 {
		return fp[:16] + "…"
	}
	return fp
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "ERROR:", err)
	os.Exit(1)
}


// distinctKeyPairs includes every phase, not just the most recent phase's
// samples. The keys may be stable within each phase but rotate between them.
func distinctKeyPairs(samples []sample) map[string]bool {
	seen := make(map[string]bool, len(samples))
	for _, s := range samples {
		seen[s.keysFP+"|"+s.authFP] = true
	}
	return seen
}
