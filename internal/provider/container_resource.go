package provider

import (
	"context"
	"errors"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/float64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/machugram/terraform-provider-orbstack/internal/docker"
)

var (
	_ resource.Resource                   = &containerResource{}
	_ resource.ResourceWithConfigure      = &containerResource{}
	_ resource.ResourceWithImportState    = &containerResource{}
	_ resource.ResourceWithValidateConfig = &containerResource{}
	_ resource.ResourceWithModifyPlan     = &containerResource{}
)

func newContainerResource() resource.Resource { return &containerResource{} }

type containerResource struct {
	data   *ProviderData
	engine *docker.Client
}

type containerModel struct {
	ID          types.String  `tfsdk:"id"`
	Name        types.String  `tfsdk:"name"`
	Image       types.String  `tfsdk:"image"`
	Ports       types.Map     `tfsdk:"ports"`
	BindAddress types.String  `tfsdk:"bind_address"`
	Env         types.Map     `tfsdk:"env"`
	Volumes     types.Map     `tfsdk:"volumes"`
	Command     types.List    `tfsdk:"command"`
	Workdir     types.String  `tfsdk:"workdir"`
	CPUs        types.Float64 `tfsdk:"cpus"`
	MemoryMiB   types.Int64   `tfsdk:"memory_mib"`
	Restart     types.String  `tfsdk:"restart"`
	Running     types.Bool    `tfsdk:"running"`
	ImageID     types.String  `tfsdk:"image_id"`
}

func (r *containerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_container"
}

func (r *containerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A container on the OrbStack Docker engine.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Container id.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Container name.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"image": schema.StringAttribute{
				Required:    true,
				Description: "Image reference. Create always pulls it.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"ports": schema.MapAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "Host port to container port. TCP, published on bind_address.",
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
				},
			},
			"bind_address": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("127.0.0.1"),
				Description: "IP address published ports bind to. Defaults to 127.0.0.1. Use 0.0.0.0 to listen on all interfaces.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"env": schema.MapAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Sensitive:   true,
				WriteOnly:   true,
				Description: "Environment variables. Not stored in state. Empty and null are the same.",
			},
			"volumes": schema.MapAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "Host path to container path. Bind mounts, read-write.",
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
				},
			},
			"command": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "Command. Omit to keep the image command.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"workdir": schema.StringAttribute{
				Optional:    true,
				Description: "Working directory.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"cpus": schema.Float64Attribute{
				Optional:    true,
				Description: "CPU limit.",
				Validators:  []validator.Float64{float64validator.AtLeast(1e-9)},
				PlanModifiers: []planmodifier.Float64{
					float64planmodifier.RequiresReplace(),
				},
			},
			"memory_mib": schema.Int64Attribute{
				Optional:    true,
				Description: "Memory limit in MiB.",
				Validators:  []validator.Int64{int64validator.AtLeast(1)},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"restart": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("unless-stopped"),
				Description: "Restart policy: no, on-failure, always, or unless-stopped.",
				Validators:  []validator.String{stringvalidator.OneOf("no", "on-failure", "always", "unless-stopped")},
			},
			"running": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Start the container. Stop uses a 10 second timeout.",
			},
			"image_id": schema.StringAttribute{
				Computed:    true,
				Description: "Image id from inspect. A moved tag does not replace the container.",
			},
		},
	}
}

func (r *containerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data, diags := providerData(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	r.data = data
}

func (r *containerResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var model containerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() || model.Ports.IsUnknown() || model.Env.IsUnknown() || model.Volumes.IsUnknown() || model.Command.IsUnknown() || model.BindAddress.IsUnknown() {
		return
	}
	request, diags := containerRequest(ctx, model)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := docker.SpecFromRequest(request); err != nil {
		attr := path.Root("ports")
		switch {
		case strings.Contains(err.Error(), "bind_address"):
			attr = path.Root("bind_address")
		case strings.Contains(err.Error(), "volume"):
			attr = path.Root("volumes")
		}
		resp.Diagnostics.AddAttributeError(attr, "invalid container", err.Error())
	}
}

func (r *containerResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var plan containerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !plan.Env.IsUnknown() && !plan.Env.IsNull() && len(plan.Env.Elements()) == 0 {
		plan.Env = types.MapNull(types.StringType)
		resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
	}

	var config containerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || config.Env.IsUnknown() {
		return
	}
	env, diags := mapStrings(ctx, config.Env)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	replace, diags := replaceIfSecretChanged(ctx, req.State.Raw.IsNull(), req.Private, resp.Private, "env_hash", hashEnv(env), path.Root("env"))
	resp.Diagnostics.Append(diags...)
	resp.RequiresReplace = append(resp.RequiresReplace, replace...)
}

func (r *containerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, config containerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !config.Env.IsUnknown() {
		plan.Env = config.Env
	}
	request, diags := containerRequest(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	engine, diags := r.client(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	created, err := engine.CreateContainer(ctx, request)
	if err != nil {
		resp.Diagnostics.AddError("failed to create container", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, containerModelFrom(plan, created, false))...)
}

func (r *containerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state containerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	engine, diags := r.client(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	found, err := engine.FindContainer(ctx, stringVal(state.ID), stringVal(state.Name))
	if errors.Is(err, docker.ErrNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("failed to read container", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, containerModelFrom(state, found, stringVal(state.Image) == ""))...)
}

func (r *containerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state containerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	engine, diags := r.client(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	updated, err := engine.UpdateContainer(ctx, stringVal(state.ID), stringVal(plan.Restart), boolVal(plan.Running))
	if errors.Is(err, docker.ErrNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("failed to update container", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, containerModelFrom(plan, updated, false))...)
}

func (r *containerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state containerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	engine, diags := r.client(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := stringVal(state.ID)
	if id == "" {
		id = stringVal(state.Name)
	}
	if err := engine.RemoveContainer(ctx, id); err != nil {
		resp.Diagnostics.AddError("failed to delete container", err.Error())
	}
}

func (r *containerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	engine, diags := r.client(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	found, err := engine.FindContainer(ctx, req.ID, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("failed to import container", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, containerModelFrom(containerModel{}, found, true))...)
}

func (r *containerResource) client(_ context.Context) (*docker.Client, diag.Diagnostics) {
	var out diag.Diagnostics
	if r.data == nil {
		out.AddError("provider not configured", "missing provider data")
		return nil, out
	}
	if r.engine != nil {
		return r.engine, nil
	}
	engine, err := docker.New(r.data.DockerHost)
	if err != nil {
		out.AddError("failed to configure docker client", err.Error())
		return nil, out
	}
	r.engine = engine
	return r.engine, nil
}

func containerRequest(ctx context.Context, model containerModel) (docker.ContainerRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	ports, portDiags := mapStrings(ctx, model.Ports)
	env, envDiags := mapStrings(ctx, model.Env)
	volumes, volDiags := mapStrings(ctx, model.Volumes)
	command, cmdDiags := listStrings(ctx, model.Command)
	diags.Append(portDiags...)
	diags.Append(envDiags...)
	diags.Append(volDiags...)
	diags.Append(cmdDiags...)
	running := true
	if !model.Running.IsNull() && !model.Running.IsUnknown() {
		running = model.Running.ValueBool()
	}
	return docker.ContainerRequest{
		Name:        stringVal(model.Name),
		Image:       stringVal(model.Image),
		BindAddress: stringVal(model.BindAddress),
		Ports:       ports,
		Env:         env,
		Volumes:     volumes,
		Command:     command,
		Workdir:     stringVal(model.Workdir),
		CPUs:        floatPtr(model.CPUs),
		MemoryMiB:   intPtr(model.MemoryMiB),
		Restart:     stringVal(model.Restart),
		Running:     running,
	}, diags
}

func containerModelFrom(prior containerModel, container docker.Container, fillIdentity bool) containerModel {
	model := prior
	model.ID = types.StringValue(container.ID)
	if container.Name != "" {
		model.Name = types.StringValue(container.Name)
	}
	model.ImageID = types.StringValue(container.ImageID)
	model.Running = types.BoolValue(container.Running)
	if container.Restart != "" {
		model.Restart = types.StringValue(container.Restart)
	}
	if ports, diags := stringMap(context.Background(), container.Ports); !diags.HasError() {
		model.Ports = ports
	}
	if volumes, diags := stringMap(context.Background(), container.Volumes); !diags.HasError() {
		model.Volumes = volumes
	}
	if container.CPUs > 0 {
		model.CPUs = types.Float64Value(container.CPUs)
	} else if fillIdentity {
		model.CPUs = types.Float64Null()
	}
	if container.MemoryMiB > 0 {
		model.MemoryMiB = types.Int64Value(container.MemoryMiB)
	} else if fillIdentity {
		model.MemoryMiB = types.Int64Null()
	}
	if fillIdentity {
		if container.ImageRef != "" {
			model.Image = types.StringValue(container.ImageRef)
		}
		model.Command = types.ListNull(types.StringType)
		model.Workdir = types.StringNull()
	}
	model.Env = types.MapNull(types.StringType)
	return model
}
