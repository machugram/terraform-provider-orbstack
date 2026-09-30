package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/machugram/terraform-provider-orbstack/internal/orb"
)

var (
	_ datasource.DataSource              = &machinesDataSource{}
	_ datasource.DataSourceWithConfigure = &machinesDataSource{}
)

func newMachinesDataSource() datasource.DataSource { return &machinesDataSource{} }

type machinesDataSource struct {
	data *ProviderData
}

type machinesDataModel struct {
	Running  types.Bool         `tfsdk:"running"`
	Machines []machineListModel `tfsdk:"machines"`
}

type machineListModel struct {
	ID      types.String `tfsdk:"id"`
	Name    types.String `tfsdk:"name"`
	Distro  types.String `tfsdk:"distro"`
	Version types.String `tfsdk:"version"`
	Arch    types.String `tfsdk:"arch"`
	State   types.String `tfsdk:"state"`
}

func (d *machinesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_machines"
}

func (d *machinesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List OrbStack machines.",
		Attributes: map[string]schema.Attribute{
			"running": schema.BoolAttribute{
				Optional:    true,
				Description: "When true, list only running machines.",
			},
			"machines": schema.ListNestedAttribute{
				Computed:    true,
				Description: "Machines reported by orbctl list.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":      schema.StringAttribute{Computed: true, Description: "Machine ULID."},
						"name":    schema.StringAttribute{Computed: true, Description: "Machine name."},
						"distro":  schema.StringAttribute{Computed: true, Description: "Distribution reported by orbctl."},
						"version": schema.StringAttribute{Computed: true, Description: "Distribution version reported by orbctl."},
						"arch":    schema.StringAttribute{Computed: true, Description: "Machine architecture."},
						"state":   schema.StringAttribute{Computed: true, Description: "State reported by orbctl."},
					},
				},
			},
		},
	}
}

func (d *machinesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data, diags := providerData(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	d.data = data
}

func (d *machinesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model machinesDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}
	runningOnly := !model.Running.IsNull() && !model.Running.IsUnknown() && model.Running.ValueBool()
	machines, err := d.data.Orb.ListMachines(ctx, runningOnly)
	if err != nil {
		resp.Diagnostics.AddError("failed to list machines", err.Error())
		return
	}
	model.Machines = make([]machineListModel, 0, len(machines))
	for _, machine := range machines {
		model.Machines = append(model.Machines, machineListFrom(machine))
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

func machineListFrom(machine orb.Machine) machineListModel {
	return machineListModel{
		ID:      types.StringValue(machine.ID),
		Name:    types.StringValue(machine.Name),
		Distro:  types.StringValue(machine.Distro),
		Version: types.StringValue(machine.Version),
		Arch:    types.StringValue(machine.Arch),
		State:   types.StringValue(machine.State),
	}
}
