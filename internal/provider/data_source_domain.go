package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = &domainDataSource{}
	_ datasource.DataSourceWithConfigure = &domainDataSource{}
)

// Allows underscore-prefixed labels (RFC 2782 SRV, RFC 6763 DNS-SD, etc.)
var hostnameValidator = regexp.MustCompile(`^([a-zA-Z0-9_](?:[a-zA-Z0-9_-]{0,61}[a-zA-Z0-9_])?\.)+[a-zA-Z]{2,}$`)

type domainDataSource struct {
	clientProvider *providerData
}

type domainDataSourceModel struct {
	Hostname types.String `tfsdk:"hostname"`
	Domain   types.Object `tfsdk:"domain"`
}

func NewDomainDataSource() datasource.DataSource {
	return &domainDataSource{}
}

func (d *domainDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

func (d *domainDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Resolves a hostname to its Dynu root domain and returns domain details.",
		Attributes: map[string]schema.Attribute{
			"hostname": schema.StringAttribute{
				Required:    true,
				Description: "Fully-qualified hostname, such as www.example.com.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
					stringvalidator.RegexMatches(hostnameValidator, "must be a valid fully-qualified hostname"),
				},
			},
			"domain": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "Dynu root domain details for the supplied hostname.",
				Attributes:  domainAttributes(),
			},
		},
	}
}

func (d *domainDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	providerData, ok := req.ProviderData.(*providerData)
	if !ok {
		resp.Diagnostics.AddError("Unexpected data source configure type", fmt.Sprintf("Expected *providerData, got %T", req.ProviderData))
		return
	}

	d.clientProvider = providerData
}

func (d *domainDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	hostname, err := hostnameFromConfig(ctx, req.Config)
	if err != nil {
		resp.Diagnostics.AddError("Unable to parse data source configuration", err.Error())
		return
	}

	if hostname.IsUnknown() || hostname.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("hostname"), "Invalid hostname", "The hostname must be known and non-null.")
		return
	}

	domainID, _, err := d.clientProvider.client.GetRootDomain(ctx, hostname.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(diagnosticSummary("Unable to resolve Dynu domain from hostname", err), err.Error())
		return
	}

	domain, err := d.clientProvider.client.GetDomainByID(ctx, domainID)
	if err != nil {
		resp.Diagnostics.AddError(diagnosticSummary("Unable to get Dynu domain", err), err.Error())
		return
	}

	domainObject, diags := domainObjectValue(*domain)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state := domainDataSourceModel{
		Hostname: hostname,
		Domain:   domainObject,
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
