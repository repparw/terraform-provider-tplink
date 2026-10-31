package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/repparw/terraform-provider-tplink/internal/client"
)

func stringRequest(configValue string) (validator.StringRequest, *validator.StringResponse) {
	attrPath := path.Root("mac")
	return validator.StringRequest{
			Config:      tfsdk.Config{Schema: nil},
			ConfigValue: types.StringValue(configValue),
			Path:        attrPath,
		},
		&validator.StringResponse{}
}

func TestMACValidator(t *testing.T) {
	valid := []string{"aa:bb:cc:dd:ee:ff", "AA-BB-CC-DD-EE-FF", "aabb.ccdd.eeff", "AA:BB:CC:DD:EE:FF"}
	for _, in := range valid {
		req, resp := stringRequest(in)
		macValidator{}.ValidateString(context.Background(), req, resp)
		if resp.Diagnostics.HasError() {
			t.Errorf("expected %q to be valid, got: %v", in, resp.Diagnostics)
		}
	}
	invalid := []string{"not-a-mac", "hello", "11:22:33"}
	for _, in := range invalid {
		req, resp := stringRequest(in)
		macValidator{}.ValidateString(context.Background(), req, resp)
		if !resp.Diagnostics.HasError() && in != "" {
			t.Errorf("expected %q to be rejected", in)
		}
	}
}

func TestMACReplacementIsIdentityBased(t *testing.T) {
	testCases := []struct {
		name    string
		oldMAC  string
		newMAC  string
		replace bool
	}{
		{"same MAC, different spelling", "AA-BB-CC-DD-EE-FF", "aabb.ccdd.eeff", false},
		{"different MAC", "AA-BB-CC-DD-EE-FF", "aa:bb:cc:dd:ee:00", true},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := planmodifier.StringRequest{
				StateValue: types.StringValue(tc.oldMAC),
				PlanValue:  types.StringValue(tc.newMAC),
			}
			resp := &stringplanmodifier.RequiresReplaceIfFuncResponse{}
			macReplacementDecision(context.Background(), req, resp)
			if resp.RequiresReplace != tc.replace {
				t.Fatalf("replace=%t, want %t", resp.RequiresReplace, tc.replace)
			}
		})
	}
}

// Resource state must retain explicitly configured spelling after a write or
// refresh, even though router identity and import IDs are canonicalized.
func TestDHCPResourceStatePreservesMAC(t *testing.T) {
	configured := types.StringValue("aa:bb:cc:dd:ee:ff")
	got := (&dhcpReservationResource{}).stateFrom(client.DHCPReservation{
		MAC: "AA-BB-CC-DD-EE-FF", IP: "192.168.0.10", Enabled: true,
	}, configured)
	if got.MAC.ValueString() != configured.ValueString() || got.ID.ValueString() != "AA-BB-CC-DD-EE-FF" {
		t.Fatalf("unexpected state: %+v", got)
	}
}
func TestPortForwardResourceStatePreservesProtocol(t *testing.T) {
	got := (&portForwardResource{}).stateFrom(client.PortForward{
		Name: "web", IP: "192.168.0.10", Protocol: "UDP",
	}, types.StringValue("udp"))
	if got.Protocol.ValueString() != "udp" {
		t.Fatalf("unexpected protocol: %s", got.Protocol.ValueString())
	}
}
