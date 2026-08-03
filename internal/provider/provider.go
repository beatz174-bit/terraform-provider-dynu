package provider

import (
	"context"
	"os"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/beatz174-bit/terraform-provider-dynu/internal/dynuclient"
)

var _ provider.Provider = &dynuProvider{}

type dynuProvider struct {
	version string
}

type dynuProviderModel struct {
	APIKey  types.String `tfsdk:"api_key"`
	BaseURL types.String `tfsdk:"base_url"`
}

type providerData struct {
	client *dynuclient.Client
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &dynuProvider{version: version}
	}
}

func (p *dynuProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "dynu"
	resp.Version = p.version
}

func (p *dynuProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Terraform provider for Dynu DNS domains, records, and data sources.",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Dynu API key. Can also be set via the DYNU_API_KEY environment variable.",
			},
			"base_url": schema.StringAttribute{
				Optional:    true,
				Description: "Override Dynu API base URL. Primarily intended for automated tests.",
			},
		},
	}
}

func (p *dynuProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data dynuProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiKey := resolveAPIKey(data.APIKey)
	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Missing Dynu API key",
			"Configure api_key in the provider block (for example, from var.dynu_api_key set in terraform.tfvars).",
		)
		return
	}

	providerData := &providerData{client: newDynuClient(apiKey, data.BaseURL)}
	resp.DataSourceData = providerData
	resp.ResourceData = providerData
}

func resolveAPIKey(configValue types.String) string {
	if !configValue.IsNull() && !configValue.IsUnknown() {
		if v := strings.TrimSpace(configValue.ValueString()); v != "" {
			return v
		}
	}
	return strings.TrimSpace(os.Getenv("DYNU_API_KEY"))
}

func (p *dynuProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewDomainsDataSource,
		NewDomainDataSource,
		NewDNSRecordsDataSource,
	}
}

func (p *dynuProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewDomainResource,
		NewDNSRecordResource,
	}
}

func newDynuClient(apiKey string, baseURL types.String) *dynuclient.Client {
	if !baseURL.IsNull() && !baseURL.IsUnknown() && strings.TrimSpace(baseURL.ValueString()) != "" {
		return dynuclient.New(apiKey, dynuclient.WithBaseURL(strings.TrimSpace(baseURL.ValueString())))
	}

	return dynuclient.New(apiKey)
}
