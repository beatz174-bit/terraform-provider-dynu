package provider

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/beatz174-bit/terraform-provider-dynu/internal/dynuclient"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestResolveAPIKey(t *testing.T) {
	tests := []struct {
		name   string
		config types.String
		envVal string
		want   string
	}{
		{name: "configured", config: types.StringValue("config-key"), want: "config-key"},
		{name: "null when missing", config: types.StringNull(), want: ""},
		{name: "trim spaces", config: types.StringValue("  config-key "), want: "config-key"},
		{name: "unknown when pending", config: types.StringUnknown(), want: ""},
		{name: "env var fallback when null", config: types.StringNull(), envVal: "env-key", want: "env-key"},
		{name: "config takes priority over env", config: types.StringValue("config-key"), envVal: "env-key", want: "config-key"},
		{name: "env var fallback when unknown", config: types.StringUnknown(), envVal: "env-key", want: "env-key"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envVal != "" {
				t.Setenv("DYNU_API_KEY", tc.envVal)
			}
			if got := resolveAPIKey(tc.config); got != tc.want {
				t.Fatalf("resolveAPIKey() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestProviderSchema(t *testing.T) {
	p := New("test")()
	var resp provider.SchemaResponse
	p.Schema(context.Background(), provider.SchemaRequest{}, &resp)

	apiKeyAttr, ok := resp.Schema.Attributes["api_key"]
	if !ok {
		t.Fatal("expected api_key in provider schema")
	}
	if !apiKeyAttr.IsOptional() {
		t.Fatal("expected api_key to be optional")
	}

	baseURLAttr, ok := resp.Schema.Attributes["base_url"]
	if !ok {
		t.Fatal("expected base_url in provider schema")
	}
	if !baseURLAttr.IsOptional() {
		t.Fatal("expected base_url to be optional")
	}
}

func TestDiagnosticSummary(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		summary string
		want    string
	}{
		{name: "non api error", err: os.ErrNotExist, summary: "Unable to list Dynu domains", want: "Unable to list Dynu domains"},
		{name: "auth error", err: &dynuclient.APIError{StatusCode: 401, Type: "Unauthorized", Message: "invalid"}, summary: "Unable to list Dynu domains", want: "Unable to list Dynu domains (authentication failed)"},
		{name: "not found", err: &dynuclient.APIError{StatusCode: 404, Type: "Not Found", Message: "missing"}, summary: "Unable to resolve Dynu domain from hostname", want: "Unable to resolve Dynu domain from hostname (not found)"},
		{name: "404 validation exception", err: &dynuclient.APIError{StatusCode: 404, Type: "Request Exception", Message: "Invalid."}, summary: "Unable to update Dynu DNS record", want: "Unable to update Dynu DNS record"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := diagnosticSummary(tc.summary, tc.err); got != tc.want {
				t.Fatalf("diagnosticSummary() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestReadOnlyExampleUsesValidProviderArguments(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "..", "examples", "read_only", "providers.tf"))
	if err != nil {
		t.Fatalf("read example providers.tf: %v", err)
	}

	config := string(contents)
	if strings.Contains(config, "username") {
		t.Fatal("example providers.tf must not use unsupported provider argument 'username'")
	}
	if !strings.Contains(config, "api_key") {
		t.Fatal("example providers.tf must include api_key argument")
	}
}
