package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/repparw/terraform-provider-tplink/internal/client"
)

// macValidator rejects values that cannot be interpreted as a MAC address,
// so malformed input fails at plan time instead of at the router.
type macValidator struct{}

func macFormatValidator() validator.String { return macValidator{} }

func (v macValidator) Description(_ context.Context) string {
	return "value must be a MAC address in colon, dash, or Cisco dotted format"
}

func (v macValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v macValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	normalized := client.NormalizeMAC(req.ConfigValue.ValueString())
	if !client.IsMAC(req.ConfigValue.ValueString()) || !strings.Contains(normalized, "-") {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid MAC Address",
			fmt.Sprintf("value %q is not a valid MAC address", req.ConfigValue.ValueString()),
		)
	}
}

// normalizeMACPlanModifier rewrites the planned MAC address to the router's
// canonical dash-uppercase form, so a user writing "aa:bb:cc:dd:ee:ff" does
// not get a permanent diff (and a replacement plan) against the normalized
// state.
type normalizeMACPlanModifier struct{}

func macNormalizer() planmodifier.String { return normalizeMACPlanModifier{} }

func (m normalizeMACPlanModifier) Description(_ context.Context) string {
	return "Normalizes the MAC address to the router's dash-uppercase form."
}

func (m normalizeMACPlanModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m normalizeMACPlanModifier) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	normalized := client.NormalizeMAC(req.ConfigValue.ValueString())
	if req.PlanValue.ValueString() == normalized {
		return
	}
	resp.PlanValue = types.StringValue(normalized)
}
