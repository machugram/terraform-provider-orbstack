package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/machugram/terraform-provider-orbstack/internal/docker"
	"github.com/machugram/terraform-provider-orbstack/internal/orb"
)

var _ provider.Provider = &orbstackProvider{}

type orbstackProvider struct {
	version string
}

type providerModel struct {
	OrbPath    types.String `tfsdk:"orb_path"`
	DockerHost types.String `tfsdk:"docker_host"`
}

// ProviderData is passed to resources and data sources.
type ProviderData struct {
	Orb        *orb.Client
	DockerHost string
}

// New returns a provider factory.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &orbstackProvider{version: version}
	}
}

func (p *orbstackProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "orbstack"
	resp.Version = p.version
}

func (p *orbstackProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage OrbStack Linux machines and Docker containers on macOS. Machines are created with orbctl. Containers are created on the OrbStack Docker engine, and create pulls the image.",
		Attributes: map[string]schema.Attribute{
			"orb_path": schema.StringAttribute{
				Optional:    true,
				Description: "Path to orbctl. Defaults to orbctl on PATH.",
			},
			"docker_host": schema.StringAttribute{
				Optional:    true,
				Description: "Docker host URL. Defaults to the OrbStack socket.",
			},
		},
	}
}

func (p *orbstackProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var model providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	orbPath := "orbctl"
	if !model.OrbPath.IsNull() && !model.OrbPath.IsUnknown() && model.OrbPath.ValueString() != "" {
		orbPath = model.OrbPath.ValueString()
	}
	configuredHost := ""
	if !model.DockerHost.IsNull() && !model.DockerHost.IsUnknown() {
		configuredHost = model.DockerHost.ValueString()
	}

	data := &ProviderData{
		Orb:        &orb.Client{Path: orbPath},
		DockerHost: docker.ResolveHost(ctx, configuredHost),
	}
	resp.ResourceData = data
	resp.DataSourceData = data
}

func (p *orbstackProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		newMachineResource,
		newContainerResource,
	}
}

func (p *orbstackProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		newMachineDataSource,
		newMachinesDataSource,
		newContainerDataSource,
	}
}
