package main

import (
	"strings"
	"testing"

	"github.com/repparw/terraform-provider-tplink/internal/client"
)

func TestTFQuoteEscapesTemplateSequences(t *testing.T) {
	got := tfQuote("my-${device}-%{ if true }")
	if got != `"my-$${device}-%%{ if true }"` {
		t.Fatalf("Terraform template interpolation was not escaped: %s", got)
	}
}

func TestInventoryRenderEscapesRouterNames(t *testing.T) {
	forwards := []client.PortForward{{Name: "rule-${literal}", IP: "192.168.0.2", ExternalPort: "5353", InternalPort: "5353", Protocol: "UDP"}}
	vars := renderTFVars(nil, nil, forwards)
	imports := renderImports(nil, forwards)
	for _, output := range []string{vars, imports} {
		if !strings.Contains(output, "$${literal}") {
			t.Fatalf("unescaped router name in generated HCL: %s", output)
		}
	}
}
