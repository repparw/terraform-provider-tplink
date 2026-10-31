package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/repparw/terraform-provider-tplink/internal/client"
)

var (
	_ datasource.DataSource              = (*firmwareDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*firmwareDataSource)(nil)
)

type firmwareDataSource struct{ client *client.Client }

func newFirmwareDataSource() datasource.DataSource { return &firmwareDataSource{} }

func (d *firmwareDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_firmware"
}

func (d *firmwareDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Router model, hardware, and firmware identification.",
		Attributes: map[string]schema.Attribute{
			"model":            modelString("Router model name."),
			"hardware_version": modelString("Hardware revision."),
			"firmware_version": modelString("Firmware version string."),
		},
	}
}

func modelString(description string) schema.StringAttribute {
	return schema.StringAttribute{MarkdownDescription: description, Computed: true}
}

type firmwareModel struct {
	Model           types.String `tfsdk:"model"`
	HardwareVersion types.String `tfsdk:"hardware_version"`
	FirmwareVersion types.String `tfsdk:"firmware_version"`
}

func (d *firmwareDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *firmwareDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "router client is missing")
		return
	}
	fw, err := d.client.GetFirmware(, ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read firmware", err.Error())
		return
	}
	state := firmwareModel{
		Model:           types.StringValue(fw.Model),
		HardwareVersion: types.StringValue(fw.HardwareVersion),
		FirmwareVersion: types.StringValue(fw.FirmwareVersion),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
