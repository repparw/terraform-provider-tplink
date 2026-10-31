package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
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
