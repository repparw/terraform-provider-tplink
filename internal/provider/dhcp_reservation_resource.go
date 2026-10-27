package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/repparw/terraform-provider-tplink/internal/client"
)

var (
	_ resource.Resource                = (*dhcpReservationResource)(nil)
	_ resource.ResourceWithConfigure   = (*dhcpReservationResource)(nil)
	_ resource.ResourceWithImportState = (*dhcpReservationResource)(nil)
)

type dhcpReservationResource struct{ client *client.Client }

func newDHCPReservationResource() resource.Resource { return &dhcpReservationResource{} }

func (r *dhcpReservationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dhcp_reservation"
}

func (r *dhcpReservationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A static DHCP address assignment. Identified by the device MAC address, " +
			"which is also the import ID. The router matches update payloads by row contents, so changing " +
			"the MAC address replaces the row.",
		Attributes: map[string]schema.Attribute{
			"mac": schema.StringAttribute{
				MarkdownDescription: "Device MAC address (colon, dash or Cisco dotted format; normalized " +
					"to the router's dash-uppercase form). Also the resource ID.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace(), macNormalizer()},
				Validators:    []validator.String{macFormatValidator()},
			},
			"ip": schema.StringAttribute{
				MarkdownDescription: "IPv4 address assigned to the device.",
				Required:            true,
			},
			"hostname": schema.StringAttribute{
				MarkdownDescription: "Reservation comment shown in the router UI.",
				Optional:            true,
				Computed:            true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the reservation is active. Defaults to true.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Resource identifier (the normalized MAC address).",
				Computed:            true,
			},
		},
	}
}

type dhcpReservationModel struct {
	MAC      types.String `tfsdk:"mac"`
	IP       types.String `tfsdk:"ip"`
	Hostname types.String `tfsdk:"hostname"`
	Enabled  types.Bool   `tfsdk:"enabled"`
	ID       types.String `tfsdk:"id"`
}

func (r *dhcpReservationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func toClientReservation(m dhcpReservationModel) client.DHCPReservation {
	return client.DHCPReservation{
		MAC:      m.MAC.ValueString(),
		IP:       m.IP.ValueString(),
		Hostname: m.Hostname.ValueString(),
		Enabled:  m.Enabled.ValueBool(),
	}
}

func (r *dhcpReservationResource) stateFrom(res client.DHCPReservation) dhcpReservationModel {
	return dhcpReservationModel{
		MAC:      types.StringValue(res.MAC),
		IP:       types.StringValue(res.IP),
		Hostname: types.StringValue(res.Hostname),
		Enabled:  types.BoolValue(res.Enabled),
		ID:       types.StringValue(res.MAC),
	}
}

func (r *dhcpReservationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dhcpReservationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.AddDHCPReservation(toClientReservation(plan)); err != nil {
		resp.Diagnostics.AddError("Unable to create DHCP reservation", err.Error())
		return
	}
	created, err := r.client.GetDHCPReservations()
	if err != nil {
		resp.Diagnostics.AddError("Unable to verify DHCP reservation after write", err.Error())
		return
	}
	for _, res := range created {
		if res.MAC == client.NormalizeMAC(plan.MAC.ValueString()) {
			resp.Diagnostics.Append(resp.State.Set(ctx, r.stateFrom(res))...)
			return
		}
	}
	resp.Diagnostics.AddError("DHCP reservation missing after write",
		fmt.Sprintf("reservation for %s was not readable after creation", plan.MAC.ValueString()))
}

func (r *dhcpReservationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dhcpReservationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	reservations, err := r.client.GetDHCPReservations()
	if err != nil {
		resp.Diagnostics.AddError("Unable to read DHCP reservations", err.Error())
		return
	}
	for _, res := range reservations {
		if res.MAC == client.NormalizeMAC(state.MAC.ValueString()) {
			resp.Diagnostics.Append(resp.State.Set(ctx, r.stateFrom(res))...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}

func (r *dhcpReservationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan dhcpReservationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// The MAC is force-replaced, so the state MAC equals the plan MAC here;
	// hostname/ip/enabled are updated in place.
	if err := r.client.UpdateDHCPReservation(plan.MAC.ValueString(), toClientReservation(plan)); err != nil {
		resp.Diagnostics.AddError("Unable to update DHCP reservation", err.Error())
		return
	}
	updated, err := r.client.GetDHCPReservations()
	if err != nil {
		resp.Diagnostics.AddError("Unable to verify DHCP reservation after write", err.Error())
		return
	}
	for _, res := range updated {
		if res.MAC == client.NormalizeMAC(plan.MAC.ValueString()) {
			resp.Diagnostics.Append(resp.State.Set(ctx, r.stateFrom(res))...)
			return
		}
	}
	resp.Diagnostics.AddError("DHCP reservation missing after update",
		fmt.Sprintf("reservation %q was not readable after update", plan.MAC.ValueString()))
}

func (r *dhcpReservationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dhcpReservationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteDHCPReservation(state.MAC.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to delete DHCP reservation", err.Error())
		return
	}
	remaining, err := r.client.GetDHCPReservations()
	if err != nil {
		resp.Diagnostics.AddError("Unable to verify DHCP reservation deletion", err.Error())
		return
	}
	for _, reservation := range remaining {
		if reservation.MAC == client.NormalizeMAC(state.MAC.ValueString()) {
			resp.Diagnostics.AddError("DHCP reservation still present after delete",
				fmt.Sprintf("reservation %q remains configured on the router", state.MAC.ValueString()))
			return
		}
	}
}

func (r *dhcpReservationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import ID is the MAC address.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("mac"), client.NormalizeMAC(req.ID))...)
}
