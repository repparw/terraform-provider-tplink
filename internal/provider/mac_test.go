package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
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

func planModifierRequest(configValue, stateValue string) (planmodifier.StringRequest, *planmodifier.StringResponse) {
	return planmodifier.StringRequest{
			ConfigValue: types.StringValue(configValue),
			StateValue:  types.StringValue(stateValue),
			PlanValue:   types.StringValue(configValue),
		},
		&planmodifier.StringResponse{
			PlanValue: types.StringValue(configValue),
		}
}

func TestNormalizeMACPlanModifier(t *testing.T) {
	req, resp := planModifierRequest("aa:bb:cc:dd:ee:ff", "")
	normalizeMACPlanModifier{}.PlanModifyString(context.Background(), req, resp)
	if got := resp.PlanValue.ValueString(); got != "AA-BB-CC-DD-EE-FF" {
		t.Fatalf("plan value not normalized: %q", got)
	}

	// Already-normalized values pass through unchanged.
	req, resp = planModifierRequest("AA-BB-CC-DD-EE-FF", "AA-BB-CC-DD-EE-FF")
	normalizeMACPlanModifier{}.PlanModifyString(context.Background(), req, resp)
	if got := resp.PlanValue.ValueString(); got != "AA-BB-CC-DD-EE-FF" {
		t.Fatalf("normalized value changed: %q", got)
	}

	// Same device written in a different format must not produce a
	// replacement plan: normalized plan equals normalized state.
	req, resp = planModifierRequest("aabb.ccdd.eeff", "AA-BB-CC-DD-EE-FF")
	normalizeMACPlanModifier{}.PlanModifyString(context.Background(), req, resp)
	if got := resp.PlanValue.ValueString(); got != "AA-BB-CC-DD-EE-FF" {
		t.Fatalf("dotted format not normalized to state form: %q", got)
	}
}

// TestMACNormalizationEndToEnd lives in the client package (it exercises
// the client's mock); see internal/client/client_test.go.
