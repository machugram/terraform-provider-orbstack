package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/machugram/terraform-provider-orbstack/internal/docker"
)

var (
	_ datasource.DataSource              = &containerDataSource{}
	_ datasource.DataSourceWithConfigure = &containerDataSource{}
)

func newContainerDataSource() datasource.DataSource { return &containerDataSource{} }

type containerDataSource struct {
	data *ProviderData
}

type containerDataModel struct {
	Name    types.String `tfsdk:"name"`
	ID      types.String `tfsdk:"id"`
	Image   types.String `tfsdk:"image"`
	ImageID types.String `tfsdk:"image_id"`
	Running types.Bool   `tfsdk:"running"`
}

func (d *containerDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_container"
}

func (d *containerDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Read one OrbStack container by name.",
		Attributes: map[string]schema.Attribute{
			"name":     schema.StringAttribute{Required: true},
			"id":       schema.StringAttribute{Computed: true},
			"image":    schema.StringAttribute{Computed: true},
			"image_id": schema.StringAttribute{Computed: true},
			"running":  schema.BoolAttribute{Computed: true},
		},
	}
}

func (d *containerDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data, diags := providerData(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	d.data = data
}

func (d *containerDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model containerDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if d.data == nil {
		resp.Diagnostics.AddError("provider not configured", "missing provider data")
		return
	}
	engine, err := docker.New(d.data.DockerHost)
	if err != nil {
		resp.Diagnostics.AddError("failed to configure docker client", err.Error())
		return
	}
	found, err := engine.FindContainer(ctx, "", model.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to read container", err.Error())
		return
	}
	model.ID = types.StringValue(found.ID)
	model.Image = types.StringValue(found.ImageRef)
	model.ImageID = types.StringValue(found.ImageID)
	model.Running = types.BoolValue(found.Running)
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}
