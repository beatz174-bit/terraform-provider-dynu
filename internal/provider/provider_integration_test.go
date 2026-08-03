package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/beatz174-bit/terraform-provider-dynu/internal/testutil/fakedynu"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestIntegrationDataSourceDomains(t *testing.T) {
	fake := fakedynu.NewServer()
	defer fake.Close()

	ds := NewDomainsDataSource().(*domainsDataSource)
	configureDataSource(t, ds, fake.BaseURL())

	var schemaResp datasource.SchemaResponse
	ds.Schema(context.Background(), datasource.SchemaRequest{}, &schemaResp)

	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	ds.Read(context.Background(), datasource.ReadRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	var state domainsDataSourceModel
	diags := resp.State.Get(context.Background(), &state)
	if diags.HasError() {
		t.Fatalf("state get diagnostics: %v", diags)
	}

	if len(state.Domains) != 2 {
		t.Fatalf("expected 2 domains, got %d", len(state.Domains))
	}
	if state.Domains[0].Name.ValueString() != "a.example.com" {
		t.Fatalf("expected sorted domain a.example.com first, got %q", state.Domains[0].Name.ValueString())
	}
}

func TestIntegrationDataSourceDomain(t *testing.T) {
	fake := fakedynu.NewServer()
	defer fake.Close()

	ds := NewDomainDataSource().(*domainDataSource)
	configureDataSource(t, ds, fake.BaseURL())

	var schemaResp datasource.SchemaResponse
	ds.Schema(context.Background(), datasource.SchemaRequest{}, &schemaResp)

	req := datasource.ReadRequest{
		Config: tfsdk.Config{
			Schema: schemaResp.Schema,
			Raw: tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
				"hostname": tftypes.String,
			}}, map[string]tftypes.Value{
				"hostname": tftypes.NewValue(tftypes.String, "www.a.example.com"),
			}),
		},
	}
	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	ds.Read(context.Background(), req, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	var state domainDataSourceModel
	diags := resp.State.Get(context.Background(), &state)
	if diags.HasError() {
		t.Fatalf("state get diagnostics: %v", diags)
	}

	var domainID int64
	diags = resp.State.GetAttribute(context.Background(), path.Root("domain").AtName("id"), &domainID)
	if diags.HasError() {
		t.Fatalf("domain.id decode diagnostics: %v", diags)
	}
	var domainName string
	diags = resp.State.GetAttribute(context.Background(), path.Root("domain").AtName("name"), &domainName)
	if diags.HasError() {
		t.Fatalf("domain.name decode diagnostics: %v", diags)
	}
	if domainID != 1001 || domainName != "a.example.com" {
		t.Fatalf("unexpected domain state: id=%d name=%q", domainID, domainName)
	}
}

func TestIntegrationDataSourceDNSRecords(t *testing.T) {
	fake := fakedynu.NewServer()
	defer fake.Close()

	ds := NewDNSRecordsDataSource().(*dnsRecordsDataSource)
	configureDataSource(t, ds, fake.BaseURL())

	var schemaResp datasource.SchemaResponse
	ds.Schema(context.Background(), datasource.SchemaRequest{}, &schemaResp)

	req := datasource.ReadRequest{
		Config: tfsdk.Config{
			Schema: schemaResp.Schema,
			Raw: tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
				"hostname": tftypes.String,
			}}, map[string]tftypes.Value{
				"hostname": tftypes.NewValue(tftypes.String, "www.a.example.com"),
			}),
		},
	}
	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	ds.Read(context.Background(), req, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	var state dnsRecordsDataSourceModel
	diags := resp.State.Get(context.Background(), &state)
	if diags.HasError() {
		t.Fatalf("state get diagnostics: %v", diags)
	}

	if state.DomainID.ValueInt64() != 1001 || state.DomainName.ValueString() != "a.example.com" {
		t.Fatalf("unexpected domain resolution: domain_id=%d domain_name=%q", state.DomainID.ValueInt64(), state.DomainName.ValueString())
	}
	if len(state.Records) != 2 || state.Records[0].ID.ValueInt64() != 10 {
		t.Fatalf("unexpected records state: %#v", state.Records)
	}
	for _, rec := range state.Records {
		if rec.Priority.IsNull() || rec.Priority.IsUnknown() {
			t.Fatalf("expected priority to be a known value (zero for non-MX), got null/unknown for record %d", rec.ID.ValueInt64())
		}
		if rec.Weight.IsNull() || rec.Weight.IsUnknown() {
			t.Fatalf("expected weight to be a known value, got null/unknown for record %d", rec.ID.ValueInt64())
		}
		if rec.Port.IsNull() || rec.Port.IsUnknown() {
			t.Fatalf("expected port to be a known value, got null/unknown for record %d", rec.ID.ValueInt64())
		}
		if rec.Flags.IsNull() || rec.Flags.IsUnknown() {
			t.Fatalf("expected flags to be a known value, got null/unknown for record %d", rec.ID.ValueInt64())
		}
	}
}

func TestIntegrationDataSourceDiagnosticsFromAPIError(t *testing.T) {
	fake := fakedynu.NewServer()
	defer fake.Close()
	fake.SetAPIError("/dns/getroot/www.a.example.com", fakedynu.APIError{HTTPStatus: 400, StatusCode: 400, Type: "Validation Exception", Message: "bad hostname"})

	ds := NewDomainDataSource().(*domainDataSource)
	configureDataSource(t, ds, fake.BaseURL())

	var schemaResp datasource.SchemaResponse
	ds.Schema(context.Background(), datasource.SchemaRequest{}, &schemaResp)

	req := datasource.ReadRequest{Config: tfsdk.Config{
		Schema: schemaResp.Schema,
		Raw: tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{"hostname": tftypes.String}}, map[string]tftypes.Value{
			"hostname": tftypes.NewValue(tftypes.String, "www.a.example.com"),
		}),
	}}
	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	ds.Read(context.Background(), req, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected diagnostics error")
	}
	if !strings.Contains(resp.Diagnostics[0].Summary(), "Unable to resolve Dynu domain from hostname") {
		t.Fatalf("unexpected diagnostics summary: %s", resp.Diagnostics[0].Summary())
	}
}

func TestIntegrationDataSourceDiagnosticsFromAuthError(t *testing.T) {
	fake := fakedynu.NewServer()
	defer fake.Close()
	fake.SetAPIError("/dns/getroot/www.a.example.com", fakedynu.APIError{HTTPStatus: 401, StatusCode: 401, Type: "Unauthorized", Message: "invalid api key"})

	ds := NewDomainDataSource().(*domainDataSource)
	configureDataSource(t, ds, fake.BaseURL())

	var schemaResp datasource.SchemaResponse
	ds.Schema(context.Background(), datasource.SchemaRequest{}, &schemaResp)

	req := datasource.ReadRequest{Config: tfsdk.Config{
		Schema: schemaResp.Schema,
		Raw: tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{"hostname": tftypes.String}}, map[string]tftypes.Value{
			"hostname": tftypes.NewValue(tftypes.String, "www.a.example.com"),
		}),
	}}
	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	ds.Read(context.Background(), req, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected diagnostics error")
	}
	if !strings.Contains(resp.Diagnostics[0].Summary(), "authentication failed") {
		t.Fatalf("unexpected diagnostics summary: %s", resp.Diagnostics[0].Summary())
	}
}

func TestIntegrationDataSourceDiagnosticsFromNotFoundError(t *testing.T) {
	fake := fakedynu.NewServer()
	defer fake.Close()

	ds := NewDomainDataSource().(*domainDataSource)
	configureDataSource(t, ds, fake.BaseURL())

	var schemaResp datasource.SchemaResponse
	ds.Schema(context.Background(), datasource.SchemaRequest{}, &schemaResp)

	req := datasource.ReadRequest{Config: tfsdk.Config{
		Schema: schemaResp.Schema,
		Raw: tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{"hostname": tftypes.String}}, map[string]tftypes.Value{
			"hostname": tftypes.NewValue(tftypes.String, "missing.a.example.com"),
		}),
	}}
	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	ds.Read(context.Background(), req, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected diagnostics error")
	}
	if !strings.Contains(resp.Diagnostics[0].Summary(), "not found") {
		t.Fatalf("unexpected diagnostics summary: %s", resp.Diagnostics[0].Summary())
	}
}

func configureDataSource(t *testing.T, ds datasource.DataSourceWithConfigure, baseURL string) {
	t.Helper()
	resp := datasource.ConfigureResponse{}
	ds.Configure(context.Background(), datasource.ConfigureRequest{
		ProviderData: &providerData{client: newDynuClient("dummy-local-key", types.StringValue(baseURL))},
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("configure diagnostics: %v", resp.Diagnostics)
	}
}
