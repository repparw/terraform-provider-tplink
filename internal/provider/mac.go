package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"

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

// A MAC may be configured using any supported spelling. Terraform forbids
// changing an explicitly configured value in a plan, so preserve the original
// spelling and only replace a resource when the underlying identity changes.
func macRequiresReplace() planmodifier.String {
	return stringplanmodifier.RequiresReplaceIf(macReplacementDecision,
		"Replace the reservation only when its normalized MAC address changes.",
		"Replace the reservation only when its normalized MAC address changes.",
	)
}

func macReplacementDecision(_ context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
	if req.PlanValue.IsNull() || req.PlanValue.IsUnknown() ||
		req.StateValue.IsNull() || req.StateValue.IsUnknown() {
		return
	}
	resp.RequiresReplace = client.NormalizeMAC(req.PlanValue.ValueString()) !=
		client.NormalizeMAC(req.StateValue.ValueString())
}
