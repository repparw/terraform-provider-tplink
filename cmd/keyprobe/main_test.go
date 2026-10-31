package main

import (
	"strings"
	"testing"
)

func TestDistinctKeyPairsAcrossPhases(t *testing.T) {
	a := sample{keysFP: "pwd-A", authFP: "auth-A"}
	b := sample{keysFP: "pwd-B", authFP: "auth-A"}
	c := sample{keysFP: "pwd-A", authFP: "auth-B"}

	if got := len(distinctKeyPairs([]sample{a, a, a})); got != 1 {
		t.Fatalf("stable keys: got %d pairs, want 1", got)
	}
	if got := len(distinctKeyPairs([]sample{a, b, c})); got != 3 {
		t.Fatalf("between-phase key rotation: got %d pairs, want 3", got)
	}
	if got := len(distinctKeyPairs(nil)); got != 0 {
		t.Fatalf("empty set: got %d pairs, want 0", got)
	}
}

func TestReportPhaseRejectsWithinPhaseRotation(t *testing.T) {
	first := sample{keysFP: "pwd-A", authFP: "auth-A"}
	rotated := sample{keysFP: "pwd-A", authFP: "auth-B"}
	if err := reportPhase("B", []sample{first, rotated}, first); err == nil ||
		!strings.Contains(err.Error(), "vary within phase") {
		t.Fatalf("expected within-phase rotation error, got %v", err)
	}
}
