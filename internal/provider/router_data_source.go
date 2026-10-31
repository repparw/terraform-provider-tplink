package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/repparw/terraform-provider-tplink/internal/client"
)

var (
	_ datasource.DataSource              = (*routerDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*routerDataSource)(nil)
)

type routerDataSource struct{ client *client.Client }

func newRouterDataSource() datasource.DataSource { return &routerDataSource{} }

func (d *routerDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_router"
}

func (d *routerDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Router runtime status: WAN/LAN addresses, uptime, resource usage, and client counts.",
		Attributes: map[string]schema.Attribute{
			"conn_type":            modelString("WAN connection type (dhcp, pppoe, static, ...)."),
			"wan_ip":               modelString("WAN IPv4 address."),
			"wan_gateway":          modelString("WAN gateway address."),
			"wan_mac":              modelString("WAN interface MAC address."),
			"wan_uptime":           schema.Int64Attribute{MarkdownDescription: "WAN connection uptime in seconds.", Computed: true},
			"lan_ip":               modelString("LAN IPv4 address."),
			"lan_mac":              modelString("LAN interface MAC address."),
			"lan_dhcp_enabled":     schema.BoolAttribute{MarkdownDescription: "Whether the LAN DHCP server is enabled.", Computed: true},
			"cpu_usage":            schema.Float64Attribute{MarkdownDescription: "CPU usage ratio (0-1).", Computed: true},
			"mem_usage":            schema.Float64Attribute{MarkdownDescription: "Memory usage ratio (0-1).", Computed: true},
			"devices_total":        schema.Int64Attribute{MarkdownDescription: "Total connected devices.", Computed: true},
			"wired_total":          schema.Int64Attribute{MarkdownDescription: "Wired clients.", Computed: true},
			"wireless_total":       schema.Int64Attribute{MarkdownDescription: "Wireless clients on host networks.", Computed: true},
			"guest_wireless_total": schema.Int64Attribute{MarkdownDescription: "Wireless clients on guest networks.", Computed: true},
			"iot_wireless_total":   schema.Int64Attribute{MarkdownDescription: "Wireless clients on IoT networks.", Computed: true},
		},
	}
}

type routerModel struct {
	ConnType       types.String  `tfsdk:"conn_type"`
	WANIP          types.String  `tfsdk:"wan_ip"`
	WANGateway     types.String  `tfsdk:"wan_gateway"`
	WANMAC         types.String  `tfsdk:"wan_mac"`
	WANUptime      types.Int64   `tfsdk:"wan_uptime"`
	LANIP          types.String  `tfsdk:"lan_ip"`
	LANMAC         types.String  `tfsdk:"lan_mac"`
	LANDHCPEnabled types.Bool    `tfsdk:"lan_dhcp_enabled"`
	CPUUsage       types.Float64 `tfsdk:"cpu_usage"`
	MemUsage       types.Float64 `tfsdk:"mem_usage"`
	DevicesTotal   types.Int64   `tfsdk:"devices_total"`
	WiredTotal     types.Int64   `tfsdk:"wired_total"`
	WirelessTotal  types.Int64   `tfsdk:"wireless_total"`
	GuestTotal     types.Int64   `tfsdk:"guest_wireless_total"`
	IoTTotal       types.Int64   `tfsdk:"iot_wireless_total"`
}

func (d *routerDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *routerDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "router client is missing")
		return
	}
	st, err := d.client.GetStatus(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read router status", err.Error())
		return
	}
	state := routerModel{
		ConnType:       types.StringValue(st.ConnType),
		WANIP:          types.StringValue(st.WANIP),
		WANGateway:     types.StringValue(st.WANGateway),
		WANMAC:         types.StringValue(st.WANMAC),
		WANUptime:      types.Int64Value(int64(st.WANUptime)),
		LANIP:          types.StringValue(st.LANIP),
		LANMAC:         types.StringValue(st.LANMAC),
		LANDHCPEnabled: types.BoolValue(st.LANDHCPEnabled),
		CPUUsage:       types.Float64Value(st.CPUUsage),
		MemUsage:       types.Float64Value(st.MemUsage),
		DevicesTotal:   types.Int64Value(int64(st.DevicesTotal)),
		WiredTotal:     types.Int64Value(int64(st.WiredTotal)),
		WirelessTotal:  types.Int64Value(int64(st.WirelessTotal)),
		GuestTotal:     types.Int64Value(int64(st.GuestTotal)),
		IoTTotal:       types.Int64Value(int64(st.IoTTotal)),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
