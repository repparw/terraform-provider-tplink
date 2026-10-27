package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/repparw/terraform-provider-tplink/internal/client"
)

var (
	_ datasource.DataSource              = (*dhcpReservationsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*dhcpReservationsDataSource)(nil)
)

type dhcpReservationsDataSource struct{ client *client.Client }

func newDHCPReservationsDataSource() datasource.DataSource { return &dhcpReservationsDataSource{} }

func (d *dhcpReservationsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dhcp_reservations"
}

func (d *dhcpReservationsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "All static DHCP address assignments configured on the router.",
		Attributes: map[string]schema.Attribute{
			"reservations": schema.SetNestedAttribute{
				MarkdownDescription: "Configured reservations.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"hostname": modelString("Reservation comment/hostname."),
						"mac":      modelString("MAC address (dash-uppercase)."),
						"ip":       modelString("Assigned IPv4 address."),
						"enabled":  schema.BoolAttribute{MarkdownDescription: "Whether the reservation is active.", Computed: true},
					},
				},
			},
		},
	}
}

type dhcpReservationsModel struct {
	Reservations types.Set `tfsdk:"reservations"`
}

func (d *dhcpReservationsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	cl, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected data type", "provider data is not a *client.Client")
		return
	}
	d.client = cl
}

func (d *dhcpReservationsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "router client is missing")
		return
	}
	reservations, err := d.client.GetDHCPReservations()
	if err != nil {
		resp.Diagnostics.AddError("Unable to read DHCP reservations", err.Error())
		return
	}
	type rowModel struct {
		Hostname types.String `tfsdk:"hostname"`
		MAC      types.String `tfsdk:"mac"`
		IP       types.String `tfsdk:"ip"`
		Enabled  types.Bool   `tfsdk:"enabled"`
	}
	rows := make([]rowModel, 0, len(reservations))
	for _, r := range reservations {
		rows = append(rows, rowModel{
			Hostname: types.StringValue(r.Hostname),
			MAC:      types.StringValue(r.MAC),
			IP:       types.StringValue(r.IP),
			Enabled:  types.BoolValue(r.Enabled),
		})
	}
	set, diags := types.SetValueFrom(ctx, types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"hostname": types.StringType,
			"mac":      types.StringType,
			"ip":       types.StringType,
			"enabled":  types.BoolType,
		},
	}, rows)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &dhcpReservationsModel{Reservations: set})...)
}
