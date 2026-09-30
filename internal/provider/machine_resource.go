package provider

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/machugram/terraform-provider-orbstack/internal/orb"
)

var (
	_ resource.Resource                   = &machineResource{}
	_ resource.ResourceWithConfigure      = &machineResource{}
	_ resource.ResourceWithImportState    = &machineResource{}
	_ resource.ResourceWithValidateConfig = &machineResource{}
	_ resource.ResourceWithModifyPlan     = &machineResource{}
)

var notAFlag = regexp.MustCompile(`^[^-]`)

func newMachineResource() resource.Resource { return &machineResource{} }

type machineResource struct {
	data *ProviderData
}

type machineModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Image          types.String `tfsdk:"image"`
	Arch           types.String `tfsdk:"arch"`
	Username       types.String `tfsdk:"username"`
	CloudInit      types.String `tfsdk:"cloud_init"`
	CloudInitFile  types.String `tfsdk:"cloud_init_file"`
	Isolated       types.Bool   `tfsdk:"isolated"`
	IsolateNetwork types.Bool   `tfsdk:"isolate_network"`
	Mounts         types.List   `tfsdk:"mounts"`
	CPUs           types.Int64  `tfsdk:"cpus"`
	MemoryMiB      types.Int64  `tfsdk:"memory_mib"`
	DiskGiB        types.Int64  `tfsdk:"disk_gib"`
	PowerState     types.String `tfsdk:"power_state"`
	DefaultMachine types.Bool   `tfsdk:"default_machine"`
	State          types.String `tfsdk:"state"`
	Distro         types.String `tfsdk:"distro"`
	Version        types.String `tfsdk:"version"`
	DiskSizeBytes  types.Int64  `tfsdk:"disk_size_bytes"`
}

func (r *machineResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_machine"
}

func (r *machineResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "An OrbStack Linux machine.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "OrbStack machine ULID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Machine name. Renamed in place. Must not start with a hyphen.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
					stringvalidator.RegexMatches(notAFlag, "must not start with '-'"),
				},
			},
			"image": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("ubuntu"),
				Description: "Distribution, as distro or distro:version. Must not start with a hyphen.",
				Validators:  []validator.String{stringvalidator.RegexMatches(notAFlag, "must not start with '-'")},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"arch": schema.StringAttribute{
				Optional:    true,
				Description: "arm64 or amd64. Omit to use the host architecture.",
				Validators:  []validator.String{stringvalidator.OneOf("arm64", "amd64")},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"username": schema.StringAttribute{
				Optional:    true,
				Description: "Default user. Passed as --user.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"cloud_init": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				WriteOnly:   true,
				Description: "Inline cloud-init user data. Not stored in state.",
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("cloud_init_file")),
				},
			},
			"cloud_init_file": schema.StringAttribute{
				Optional:    true,
				Description: "Path to a cloud-init user data file.",
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("cloud_init")),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"isolated": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Create an isolated machine.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"isolate_network": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Block the machine from other machines and host IPs.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"mounts": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "Host mounts as SOURCE or SOURCE:DEST. Requires isolated.",
				Validators: []validator.List{
					listvalidator.ValueStringsAre(stringvalidator.LengthAtLeast(1)),
				},
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"cpus": schema.Int64Attribute{
				Optional:    true,
				Description: "CPU core limit. Unset is unlimited.",
				Validators:  []validator.Int64{int64validator.AtLeast(1)},
			},
			"memory_mib": schema.Int64Attribute{
				Optional:    true,
				Description: "Memory limit in MiB. Unset is unlimited.",
				Validators:  []validator.Int64{int64validator.AtLeast(1)},
			},
			"disk_gib": schema.Int64Attribute{
				Optional:    true,
				Description: "Disk limit in GiB. Unset is unlimited.",
				Validators:  []validator.Int64{int64validator.AtLeast(1)},
			},
			"power_state": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("running"),
				Description: "running or stopped.",
				Validators:  []validator.String{stringvalidator.OneOf("running", "stopped")},
			},
			"default_machine": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Use this machine as the orbctl default.",
			},
			"state": schema.StringAttribute{
				Computed:    true,
				Description: "State reported by orbctl.",
			},
			"distro": schema.StringAttribute{
				Computed:    true,
				Description: "Distribution reported by orbctl.",
			},
			"version": schema.StringAttribute{
				Computed:    true,
				Description: "Distribution version reported by orbctl.",
			},
			"disk_size_bytes": schema.Int64Attribute{
				Computed:    true,
				Description: "Disk usage in bytes reported by orbctl info.",
			},
		},
	}
}

func (r *machineResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data, diags := providerData(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	r.data = data
}

func (r *machineResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var model machineModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() || model.Isolated.IsUnknown() || model.Mounts.IsUnknown() {
		return
	}
	mounts, diags := listStrings(ctx, model.Mounts)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := orb.ValidateMachine(machineRequest(model, mounts)); err != nil {
		attr := path.Root("mounts")
		switch {
		case strings.HasPrefix(err.Error(), "name "):
			attr = path.Root("name")
		case strings.HasPrefix(err.Error(), "image "):
			attr = path.Root("image")
		}
		resp.Diagnostics.AddAttributeError(attr, "invalid machine", err.Error())
	}
}

func (r *machineResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, config machineModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !config.CloudInit.IsUnknown() {
		plan.CloudInit = config.CloudInit
	}
	mounts, diags := listStrings(ctx, plan.Mounts)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	machine, err := r.data.Orb.CreateMachine(ctx, machineRequest(plan, mounts))
	if err != nil {
		resp.Diagnostics.AddError("failed to create machine", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, machineModelFrom(plan, machine, false))...)
}

func (r *machineResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var config machineModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || config.CloudInit.IsUnknown() {
		return
	}
	replace, diags := replaceIfSecretChanged(ctx, req.State.Raw.IsNull(), req.Private, resp.Private, "cloud_init_hash", hashText(stringVal(config.CloudInit)), path.Root("cloud_init"))
	resp.Diagnostics.Append(diags...)
	resp.RequiresReplace = append(resp.RequiresReplace, replace...)
}

func (r *machineResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state machineModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	machine, err := r.data.Orb.GetMachine(ctx, stringVal(state.Name), stringVal(state.ID))
	if errors.Is(err, orb.ErrNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("failed to read machine", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, machineModelFrom(state, machine, stringVal(state.Name) == ""))...)
}

func (r *machineResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state machineModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	mounts, diags := listStrings(ctx, plan.Mounts)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	machine, err := r.data.Orb.UpdateMachine(ctx, stringVal(state.Name), machineRequest(plan, mounts))
	if err != nil {
		resp.Diagnostics.AddError("failed to update machine", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, machineModelFrom(plan, machine, false))...)
}

func (r *machineResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state machineModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	key := stringVal(state.Name)
	if key == "" {
		key = stringVal(state.ID)
	}
	if err := r.data.Orb.RemoveMachine(ctx, key); err != nil {
		resp.Diagnostics.AddError("failed to delete machine", err.Error())
	}
}

func (r *machineResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	machine, err := r.data.Orb.GetMachine(ctx, "", req.ID)
	if err != nil {
		resp.Diagnostics.AddError("failed to import machine", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, machineModelFrom(machineModel{}, machine, true))...)
}

func machineRequest(model machineModel, mounts []string) orb.MachineRequest {
	return orb.MachineRequest{
		Name:           stringVal(model.Name),
		Image:          stringVal(model.Image),
		Arch:           stringVal(model.Arch),
		Username:       stringVal(model.Username),
		CloudInit:      stringVal(model.CloudInit),
		CloudInitFile:  stringVal(model.CloudInitFile),
		Isolated:       boolVal(model.Isolated),
		IsolateNetwork: boolVal(model.IsolateNetwork),
		Mounts:         mounts,
		CPUs:           intPtr(model.CPUs),
		MemoryMiB:      intPtr(model.MemoryMiB),
		DiskGiB:        intPtr(model.DiskGiB),
		PowerState:     stringVal(model.PowerState),
		DefaultMachine: boolVal(model.DefaultMachine),
	}
}

func machineModelFrom(prior machineModel, machine orb.Machine, fillIdentity bool) machineModel {
	model := prior
	model.ID = types.StringValue(machine.ID)
	model.Name = types.StringValue(machine.Name)
	model.State = types.StringValue(machine.State)
	model.Distro = types.StringValue(machine.Distro)
	model.Version = types.StringValue(machine.Version)
	model.DiskSizeBytes = types.Int64Value(machine.DiskSizeBytes)
	model.PowerState = types.StringValue(machine.PowerState)
	model.DefaultMachine = types.BoolValue(machine.Default)
	model.CPUs = intAttr(machine.CPUs)
	model.MemoryMiB = intAttr(machine.MemoryMiB)
	model.DiskGiB = intAttr(machine.DiskGiB)
	if fillIdentity {
		if machine.Image != "" {
			model.Image = types.StringValue(machine.Image)
		} else {
			model.Image = types.StringNull()
		}
		if machine.Arch != "" {
			model.Arch = types.StringValue(machine.Arch)
		} else {
			model.Arch = types.StringNull()
		}
		if machine.Username != "" {
			model.Username = types.StringValue(machine.Username)
		} else {
			model.Username = types.StringNull()
		}
		model.Isolated = types.BoolValue(machine.Isolated)
		model.IsolateNetwork = types.BoolValue(machine.IsolateNetwork)
		model.CloudInitFile = types.StringNull()
		model.Mounts = types.ListNull(types.StringType)
	}
	model.CloudInit = types.StringNull()
	return model
}
