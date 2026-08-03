package provider

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/beatz174-bit/terraform-provider-dynu/internal/dynuclient"
)

var (
	_ resource.Resource                = &domainResource{}
	_ resource.ResourceWithConfigure   = &domainResource{}
	_ resource.ResourceWithImportState = &domainResource{}
)

type domainResource struct{ clientProvider *providerData }

type domainResourceModel struct {
	ID                types.Int64  `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	IPv4Address       types.String `tfsdk:"ipv4_address"`
	IPv6Address       types.String `tfsdk:"ipv6_address"`
	TTL               types.Int64  `tfsdk:"ttl"`
	Group             types.String `tfsdk:"group"`
	State             types.String `tfsdk:"state"`
	Token             types.String `tfsdk:"token"`
	UnicodeName       types.String `tfsdk:"unicode_name"`
	IPv4              types.Bool   `tfsdk:"ipv4"`
	IPv6              types.Bool   `tfsdk:"ipv6"`
	IPv4WildcardAlias types.Bool   `tfsdk:"ipv4_wildcard_alias"`
	IPv6WildcardAlias types.Bool   `tfsdk:"ipv6_wildcard_alias"`
	AllowZoneTransfer types.Bool   `tfsdk:"allow_zone_transfer"`
	DNSSEC            types.Bool   `tfsdk:"dnssec"`
	CreatedOn         types.String `tfsdk:"created_on"`
	UpdatedOn         types.String `tfsdk:"updated_on"`
}

func NewDomainResource() resource.Resource { return &domainResource{} }
func (r *domainResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}
func (r *domainResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "Manages a Dynu DNS domain.", Attributes: map[string]schema.Attribute{
		"id":                  schema.Int64Attribute{Computed: true},
		"name":                schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"ipv4_address":        schema.StringAttribute{Optional: true, Computed: true},
		"ipv6_address":        schema.StringAttribute{Optional: true, Computed: true},
		"ttl":                 schema.Int64Attribute{Optional: true, Computed: true},
		"group":               schema.StringAttribute{Optional: true, Computed: true},
		"state":               schema.StringAttribute{Computed: true},
		"token":               schema.StringAttribute{Computed: true, Sensitive: true},
		"unicode_name":        schema.StringAttribute{Computed: true, Description: "Unicode representation of the domain name."},
		"ipv4":                schema.BoolAttribute{Computed: true, Description: "Whether IPv4 support is enabled for this domain."},
		"ipv6":                schema.BoolAttribute{Computed: true, Description: "Whether IPv6 support is enabled for this domain."},
		"ipv4_wildcard_alias": schema.BoolAttribute{Computed: true, Description: "Whether IPv4 wildcard alias is enabled."},
		"ipv6_wildcard_alias": schema.BoolAttribute{Computed: true, Description: "Whether IPv6 wildcard alias is enabled."},
		"allow_zone_transfer": schema.BoolAttribute{Computed: true, Description: "Whether zone transfer is allowed for this domain."},
		"dnssec":              schema.BoolAttribute{Computed: true, Description: "Whether DNSSEC is enabled for this domain."},
		"created_on":          schema.StringAttribute{Computed: true, Description: "Creation timestamp as returned by Dynu."},
		"updated_on":          schema.StringAttribute{Computed: true, Description: "Last update timestamp as returned by Dynu."},
	}}
}
func (r *domainResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	providerData, ok := req.ProviderData.(*providerData)
	if !ok {
		resp.Diagnostics.AddError("Unexpected resource configure type", fmt.Sprintf("Expected *providerData, got %T", req.ProviderData))
		return
	}
	r.clientProvider = providerData
}
func (r *domainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan domainResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	domain, err := r.clientProvider.client.CreateDomain(ctx, dynuclient.CreateDomainRequest{Name: plan.Name.ValueString(), IPv4Address: stringFromOptional(plan.IPv4Address), IPv6Address: stringFromOptional(plan.IPv6Address), TTL: int64FromOptional(plan.TTL), Group: stringFromOptional(plan.Group)})
	if err != nil {
		resp.Diagnostics.AddError(diagnosticSummary("Unable to create Dynu domain", err), err.Error())
		return
	}
	state := mapDomainResource(*domain)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
func (r *domainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state domainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	domain, err := r.clientProvider.client.GetDomainByID(ctx, state.ID.ValueInt64())
	if err != nil {
		var apiErr *dynuclient.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(diagnosticSummary("Unable to read Dynu domain", err), err.Error())
		return
	}
	next := mapDomainResource(*domain)
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}
func (r *domainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan domainResourceModel
	var state domainResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	domain, err := r.clientProvider.client.UpdateDomain(ctx, state.ID.ValueInt64(), dynuclient.UpdateDomainRequest{Name: plan.Name.ValueString(), IPv4Address: stringFromOptional(plan.IPv4Address), IPv6Address: stringFromOptional(plan.IPv6Address), TTL: int64FromOptional(plan.TTL), Group: stringFromOptional(plan.Group)})
	if err != nil {
		resp.Diagnostics.AddError(diagnosticSummary("Unable to update Dynu domain", err), err.Error())
		return
	}
	next := mapDomainResource(*domain)
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}
func (r *domainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state domainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.clientProvider.client.DeleteDomain(ctx, state.ID.ValueInt64())
	if err == nil {
		return
	}
	var apiErr *dynuclient.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
		return
	}
	resp.Diagnostics.AddError(diagnosticSummary("Unable to delete Dynu domain", err), err.Error())
}
func (r *domainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(strings.TrimSpace(req.ID), 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid domain import ID", "Use the Dynu numeric domain ID.")
		return
	}
	state := domainResourceModel{ID: types.Int64Value(id)}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func mapDomainResource(domain dynuclient.Domain) domainResourceModel {
	return domainResourceModel{
		ID:                types.Int64Value(domain.ID),
		Name:              types.StringValue(strings.TrimSpace(domain.Name)),
		IPv4Address:       mapString(domain.IPv4Address),
		IPv6Address:       mapString(domain.IPv6Address),
		TTL:               types.Int64Value(domain.TTL),
		Group:             mapString(domain.Group),
		State:             mapString(domain.State),
		Token:             mapString(domain.Token),
		UnicodeName:       mapString(domain.UnicodeName),
		IPv4:              types.BoolValue(domain.IPv4),
		IPv6:              types.BoolValue(domain.IPv6),
		IPv4WildcardAlias: types.BoolValue(domain.IPv4WildcardAlias),
		IPv6WildcardAlias: types.BoolValue(domain.IPv6WildcardAlias),
		AllowZoneTransfer: types.BoolValue(domain.AllowZoneTransfer),
		DNSSEC:            types.BoolValue(domain.DNSSEC),
		CreatedOn:         mapString(domain.CreatedOn),
		UpdatedOn:         mapString(domain.UpdatedOn),
	}
}
