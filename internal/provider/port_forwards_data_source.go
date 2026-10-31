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
	_ datasource.DataSource              = (*portForwardsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*portForwardsDataSource)(nil)
)

type portForwardsDataSource struct{ client *client.Client }

func newPortForwardsDataSource() datasource.DataSource { return &portForwardsDataSource{} }

func (d *portForwardsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_port_forwards"
}

func (d *portForwardsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "All port forwarding (virtual server) rules configured on the router.",
		Attributes: map[string]schema.Attribute{
			"rules": schema.SetNestedAttribute{
				MarkdownDescription: "Configured rules.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name":          modelString("Rule name (unique)."),
						"ip":            modelString("Internal (LAN) address the rule forwards to."),
						"external_port": modelString("WAN-side port or start-end range."),
						"internal_port": modelString("LAN-side port."),
						"protocol":      modelString("TCP, UDP or ALL."),
						"enabled":       schema.BoolAttribute{MarkdownDescription: "Whether the rule is active.", Computed: true},
					},
				},
			},
		},
	}
}

type portForwardsModel struct {
	Rules types.Set `tfsdk:"rules"`
}

func (d *portForwardsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *portForwardsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "router client is missing")
		return
	}
	rules, err := d.client.GetPortForwards(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read port forwards", err.Error())
		return
	}
	type rowModel struct {
		Name         types.String `tfsdk:"name"`
		IP           types.String `tfsdk:"ip"`
		ExternalPort types.String `tfsdk:"external_port"`
		InternalPort types.String `tfsdk:"internal_port"`
		Protocol     types.String `tfsdk:"protocol"`
		Enabled      types.Bool   `tfsdk:"enabled"`
	}
	rows := make([]rowModel, 0, len(rules))
	for _, r := range rules {
		rows = append(rows, rowModel{
			Name:         types.StringValue(r.Name),
			IP:           types.StringValue(r.IP),
			ExternalPort: types.StringValue(r.ExternalPort),
			InternalPort: types.StringValue(r.InternalPort),
			Protocol:     types.StringValue(r.Protocol),
			Enabled:      types.BoolValue(r.Enabled),
		})
	}
	set, diags := types.SetValueFrom(ctx, types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"name":          types.StringType,
			"ip":            types.StringType,
			"external_port": types.StringType,
			"internal_port": types.StringType,
			"protocol":      types.StringType,
			"enabled":       types.BoolType,
		},
	}, rows)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &portForwardsModel{Rules: set})...)
}
