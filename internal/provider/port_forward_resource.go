package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/repparw/terraform-provider-tplink/internal/client"
)

var (
	_ resource.Resource                = (*portForwardResource)(nil)
	_ resource.ResourceWithConfigure   = (*portForwardResource)(nil)
	_ resource.ResourceWithImportState = (*portForwardResource)(nil)
)

type portForwardResource struct{ client *client.Client }

func newPortForwardResource() resource.Resource { return &portForwardResource{} }

func (r *portForwardResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_port_forward"
}

func (r *portForwardResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A port forwarding (virtual server) rule. Identified by its name, which is " +
			"also the import ID. Renaming a rule replaces it.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Rule name (unique on the router). Also the resource ID.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"ip": schema.StringAttribute{
				MarkdownDescription: "Internal (LAN) IPv4 address to forward to.",
				Required:            true,
			},
			"external_port": schema.StringAttribute{
				MarkdownDescription: "WAN-side port (`443`) or range (`8000-9000`).",
				Required:            true,
			},
			"internal_port": schema.StringAttribute{
				MarkdownDescription: "LAN-side port (single port).",
				Required:            true,
			},
			"protocol": schema.StringAttribute{
				MarkdownDescription: "TCP, UDP or ALL (case-insensitive).",
				Required:            true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the rule is active. Defaults to true.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Resource identifier (the rule name).",
				Computed:            true,
			},
		},
	}
}

type portForwardModel struct {
	Name         types.String `tfsdk:"name"`
	IP           types.String `tfsdk:"ip"`
	ExternalPort types.String `tfsdk:"external_port"`
	InternalPort types.String `tfsdk:"internal_port"`
	Protocol     types.String `tfsdk:"protocol"`
	Enabled      types.Bool   `tfsdk:"enabled"`
	ID           types.String `tfsdk:"id"`
}

func (r *portForwardResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	cl, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected data type", "provider data is not a *client.Client")
		return
	}
	r.client = cl
}

func toClientPortForward(m portForwardModel) client.PortForward {
	return client.PortForward{
		Name:         m.Name.ValueString(),
		IP:           m.IP.ValueString(),
		ExternalPort: m.ExternalPort.ValueString(),
		InternalPort: m.InternalPort.ValueString(),
		Protocol:     m.Protocol.ValueString(),
		Enabled:      m.Enabled.ValueBool(),
	}
}

func (r *portForwardResource) stateFrom(res client.PortForward, configuredProtocol types.String) portForwardModel {
	// Preserve the configured case (e.g. "udp") when the router returns "UDP".
	// Terraform requires the post-apply state to match explicit configuration.
	if !strings.EqualFold(configuredProtocol.ValueString(), res.Protocol) {
		configuredProtocol = types.StringValue(res.Protocol)
	}
	return portForwardModel{
		Name:         types.StringValue(res.Name),
		IP:           types.StringValue(res.IP),
		ExternalPort: types.StringValue(res.ExternalPort),
		InternalPort: types.StringValue(res.InternalPort),
		Protocol:     configuredProtocol,
		Enabled:      types.BoolValue(res.Enabled),
		ID:           types.StringValue(res.Name),
	}
}

func (r *portForwardResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan portForwardModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.AddPortForward(toClientPortForward(plan)); err != nil {
		resp.Diagnostics.AddError("Unable to create port forward", err.Error())
		return
	}
	created, err := r.client.GetPortForwards()
	if err != nil {
		resp.Diagnostics.AddError("Unable to verify port forward after write", err.Error())
		return
	}
	for _, res := range created {
		if res.Name == plan.Name.ValueString() {
			if !portForwardMatchesPlan(res, plan) {
				resp.Diagnostics.AddError("Port forward differs from planned values",
					fmt.Sprintf("router returned unexpected fields for rule %q after write", plan.Name.ValueString()))
				return
			}
			resp.Diagnostics.Append(resp.State.Set(ctx, r.stateFrom(res, plan.Protocol))...)
			return
		}
	}
	resp.Diagnostics.AddError("Port forward missing after write",
		fmt.Sprintf("rule %q was not readable after creation", plan.Name.ValueString()))
}

func (r *portForwardResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state portForwardModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	rules, err := r.client.GetPortForwards()
	if err != nil {
		resp.Diagnostics.AddError("Unable to read port forwards", err.Error())
		return
	}
	for _, res := range rules {
		if res.Name == state.Name.ValueString() {
			resp.Diagnostics.Append(resp.State.Set(ctx, r.stateFrom(res))...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}

func (r *portForwardResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan portForwardModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Name changes are force-replaced, so state name == plan name here.
	if err := r.client.UpdatePortForward(plan.Name.ValueString(), toClientPortForward(plan)); err != nil {
		resp.Diagnostics.AddError("Unable to update port forward", err.Error())
		return
	}
	updated, err := r.client.GetPortForwards()
	if err != nil {
		resp.Diagnostics.AddError("Unable to verify port forward after write", err.Error())
		return
	}
	for _, res := range updated {
		if res.Name == plan.Name.ValueString() {
			if !portForwardMatchesPlan(res, plan) {
				resp.Diagnostics.AddError("Port forward differs from planned values",
					fmt.Sprintf("router returned unexpected fields for rule %q after write", plan.Name.ValueString()))
				return
			}
			resp.Diagnostics.Append(resp.State.Set(ctx, r.stateFrom(res))...)
			return
		}
	}
	resp.Diagnostics.AddError("Port forward missing after update",
		fmt.Sprintf("rule %q was not readable after update", plan.Name.ValueString()))
}

func (r *portForwardResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state portForwardModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeletePortForward(state.Name.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to delete port forward", err.Error())
		return
	}
	remaining, err := r.client.GetPortForwards()
	if err != nil {
		resp.Diagnostics.AddError("Unable to verify port forward deletion", err.Error())
		return
	}
	for _, rule := range remaining {
		if rule.Name == state.Name.ValueString() {
			resp.Diagnostics.AddError("Port forward still present after delete",
				fmt.Sprintf("rule %q remains configured on the router", state.Name.ValueString()))
			return
		}
	}
}

func (r *portForwardResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import ID is the rule name.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), req.ID)...)
}

// portForwardMatchesPlan verifies the router actually stored all intended fields.
func portForwardMatchesPlan(res client.PortForward, plan portForwardModel) bool {
	return res.Name == plan.Name.ValueString() &&
		res.IP == plan.IP.ValueString() &&
		res.ExternalPort == plan.ExternalPort.ValueString() &&
		res.InternalPort == plan.InternalPort.ValueString() &&
		strings.EqualFold(res.Protocol, plan.Protocol.ValueString()) &&
		(plan.Enabled.IsUnknown() || res.Enabled == plan.Enabled.ValueBool())
}
