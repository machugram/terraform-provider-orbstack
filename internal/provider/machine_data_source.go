package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/machugram/terraform-provider-orbstack/internal/orb"
)

var (
	_ datasource.DataSource              = &machineDataSource{}
	_ datasource.DataSourceWithConfigure = &machineDataSource{}
)

func newMachineDataSource() datasource.DataSource { return &machineDataSource{} }

type machineDataSource struct {
	data *ProviderData
}

type machineDataModel struct {
	Name          types.String `tfsdk:"name"`
	ID            types.String `tfsdk:"id"`
	State         types.String `tfsdk:"state"`
	Distro        types.String `tfsdk:"distro"`
	Version       types.String `tfsdk:"version"`
	Arch          types.String `tfsdk:"arch"`
	DiskSizeBytes types.Int64  `tfsdk:"disk_size_bytes"`
}

func (d *machineDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_machine"
}

func (d *machineDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Read one OrbStack machine by name or id.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Machine name. Set this or id.",
			},
			"id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Machine ULID. Set this or name.",
			},
			"state":           schema.StringAttribute{Computed: true, Description: "State reported by orbctl."},
			"distro":          schema.StringAttribute{Computed: true, Description: "Distribution reported by orbctl."},
			"version":         schema.StringAttribute{Computed: true, Description: "Distribution version reported by orbctl."},
			"arch":            schema.StringAttribute{Computed: true, Description: "Machine architecture."},
			"disk_size_bytes": schema.Int64Attribute{Computed: true, Description: "Disk usage in bytes reported by orbctl info."},
		},
	}
}

func (d *machineDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data, diags := providerData(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	d.data = data
}

func (d *machineDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model machineDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := stringVal(model.Name)
	id := stringVal(model.ID)
	if (name == "") == (id == "") {
		resp.Diagnostics.AddError("invalid lookup", "Set exactly one of name or id.")
		return
	}
	machine, err := d.data.Orb.GetMachine(ctx, name, id)
	if err != nil {
		resp.Diagnostics.AddError("failed to read machine", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, machineDataFrom(machine))...)
}

func machineDataFrom(machine orb.Machine) machineDataModel {
	return machineDataModel{
		Name:          types.StringValue(machine.Name),
		ID:            types.StringValue(machine.ID),
		State:         types.StringValue(machine.State),
		Distro:        types.StringValue(machine.Distro),
		Version:       types.StringValue(machine.Version),
		Arch:          types.StringValue(machine.Arch),
		DiskSizeBytes: types.Int64Value(machine.DiskSizeBytes),
	}
}
