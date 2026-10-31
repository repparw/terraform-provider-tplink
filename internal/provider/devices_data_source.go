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
	_ datasource.DataSource              = (*devicesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*devicesDataSource)(nil)
)

type devicesDataSource struct{ client *client.Client }

func newDevicesDataSource() datasource.DataSource { return &devicesDataSource{} }

func (d *devicesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_devices"
}

func (d *devicesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Inventory of devices currently connected to the router (wired, host, guest, and IoT wireless segments).",
		Attributes: map[string]schema.Attribute{
			"devices": schema.SetNestedAttribute{
				MarkdownDescription: "Connected devices.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"hostname": modelString("Device hostname as reported by the router."),
						"ip":       modelString("Assigned IPv4 address."),
						"mac":      modelString("MAC address (dash-uppercase)."),
						"type":     modelString("Connection segment: wired, wifi, guest or iot."),
					},
				},
			},
		},
	}
}

type devicesModel struct {
	Devices types.Set `tfsdk:"devices"`
}

type deviceModel struct {
	Hostname types.String `tfsdk:"hostname"`
	IP       types.String `tfsdk:"ip"`
	MAC      types.String `tfsdk:"mac"`
	Type     types.String `tfsdk:"type"`
}

func (d *devicesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *devicesDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "router client is missing")
		return
	}
	devices, err := d.client.GetDevices(, ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read devices", err.Error())
		return
	}
	models := make([]deviceModel, 0, len(devices))
	for _, dev := range devices {
		models = append(models, deviceModel{
			Hostname: types.StringValue(dev.Hostname),
			IP:       types.StringValue(dev.IP),
			MAC:      types.StringValue(dev.MAC),
			Type:     types.StringValue(dev.Type),
		})
	}
	set, diags := types.SetValueFrom(ctx, types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"hostname": types.StringType,
			"ip":       types.StringType,
			"mac":      types.StringType,
			"type":     types.StringType,
		},
	}, models)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state := devicesModel{Devices: set}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
