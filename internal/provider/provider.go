// Package provider defines the tplink Terraform/OpenTofu provider.
//
// The provider keeps one authenticated router session per Terraform run.
// The router allows exactly one admin session, so a run evicts any
// interactive web-UI login; sessions are re-established transparently when
// the router expires them.
package provider

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/repparw/terraform-provider-tplink/internal/client"
)

// Compile-time interface checks.
var (
	_ provider.Provider = (*tplinkProvider)(nil)
)

type tplinkProvider struct {
	version string
}

// New returns the provider implementation constructor.
func New(version string) provider.Provider {
	return &tplinkProvider{version: version}
}

func (p *tplinkProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "tplink"
	resp.Version = p.version
}

type providerModel struct {
	Host          types.String `tfsdk:"host"`
	Username      types.String `tfsdk:"username"`
	Password      types.String `tfsdk:"password"`
	TLSSkipVerify types.Bool   `tfsdk:"tls_skip_verify"`
	Timeout       types.Int64  `tfsdk:"timeout_seconds"`
}

func (p *tplinkProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage TP-Link home routers running the new-generation " +
			"(SG-hardened) web UI, such as the Archer AX23 v2 on firmware 1.1.2.",
		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				MarkdownDescription: "Router base address, e.g. `http://192.168.0.1`.",
				Required:            true,
			},
			"username": schema.StringAttribute{
				MarkdownDescription: "Admin username. Defaults to `admin`.",
				Optional:            true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "Admin password. Prefer an environment variable or a secret store; never commit it.",
				Required:            true,
				Sensitive:           true,
			},
			"tls_skip_verify": schema.BoolAttribute{
				MarkdownDescription: "Skip TLS verification (the router ships a self-signed certificate).",
				Optional:            true,
			},
			"timeout_seconds": schema.Int64Attribute{
				MarkdownDescription: "Per-request timeout in seconds. Defaults to 30.",
				Optional:            true,
			},
		},
	}
}

func (p *tplinkProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	cfg := client.Config{
		Host:          data.Host.ValueString(),
		Username:      data.Username.ValueString(),
		Password:      data.Password.ValueString(),
		TLSSkipVerify: data.TLSSkipVerify.ValueBool(),
	}
	if !data.Timeout.IsNull() {
		cfg.Timeout = time.Duration(data.Timeout.ValueInt64()) * time.Second
	}
	cl, err := client.New(cfg)
	if err != nil {
		resp.Diagnostics.AddError("Invalid provider configuration", err.Error())
		return
	}
	resp.ResourceData = cl
	resp.DataSourceData = cl
}

func (p *tplinkProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		newFirmwareDataSource,
		newRouterDataSource,
		newDevicesDataSource,
		newDHCPReservationsDataSource,
		newPortForwardsDataSource,
	}
}

func (p *tplinkProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		newDHCPReservationResource,
		newPortForwardResource,
	}
}
