package provider

import (
	"context"
	"testing"

	"github.com/beatz174-bit/terraform-provider-dynu/internal/testutil/fakedynu"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestIntegrationResourceDomainLifecycleAndImport(t *testing.T) {
	ctx := context.Background()
	fake := fakedynu.NewServer()
	defer fake.Close()
	r := NewDomainResource().(*domainResource)
	configureResource(t, r, fake.BaseURL())
	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	planModel := domainResourceModel{Name: types.StringValue("new.example.com"), TTL: types.Int64Value(120), IPv4Address: types.StringValue("203.0.113.7")}
	plan := tfsdk.Plan{Schema: schemaResp.Schema}
	_ = plan.Set(ctx, &planModel)
	createResp := resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	r.Create(ctx, resource.CreateRequest{Plan: plan}, &createResp)
	if createResp.Diagnostics.HasError() {
		t.Fatalf("create diagnostics: %v", createResp.Diagnostics)
	}

	var state domainResourceModel
	_ = createResp.State.Get(ctx, &state)
	if state.ID.ValueInt64() == 0 {
		t.Fatal("expected created id")
	}
	if state.State.IsNull() || state.State.IsUnknown() {
		t.Fatal("expected state attribute to be populated after create")
	}
	if state.UnicodeName.IsNull() || state.UnicodeName.IsUnknown() {
		t.Fatal("expected unicode_name to be populated after create")
	}
	if state.DNSSEC.IsNull() || state.DNSSEC.IsUnknown() {
		t.Fatal("expected dnssec to be a known bool after create")
	}

	state.TTL = types.Int64Value(300)
	plan = tfsdk.Plan{Schema: schemaResp.Schema}
	_ = plan.Set(ctx, &state)
	updateResp := resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	r.Update(ctx, resource.UpdateRequest{Plan: plan, State: createResp.State}, &updateResp)
	if updateResp.Diagnostics.HasError() {
		t.Fatalf("update diagnostics: %v", updateResp.Diagnostics)
	}

	importResp := resource.ImportStateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	r.ImportState(ctx, resource.ImportStateRequest{ID: "5002"}, &importResp)
	if importResp.Diagnostics.HasError() {
		t.Fatalf("import diagnostics: %v", importResp.Diagnostics)
	}
	var imported int64
	_ = importResp.State.GetAttribute(ctx, path.Root("id"), &imported)
	if imported != 5002 {
		t.Fatalf("unexpected imported id %d", imported)
	}
}
