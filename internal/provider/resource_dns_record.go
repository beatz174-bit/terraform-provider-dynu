package provider

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/beatz174-bit/terraform-provider-dynu/internal/dynuclient"
)

var (
	_ resource.Resource                   = &dnsRecordResource{}
	_ resource.ResourceWithConfigure      = &dnsRecordResource{}
	_ resource.ResourceWithImportState    = &dnsRecordResource{}
	_ resource.ResourceWithValidateConfig = &dnsRecordResource{}
)

type dnsRecordResource struct {
	clientProvider *providerData
}

type dnsRecordResourceModel struct {
	ID         types.String `tfsdk:"id"`
	Hostname   types.String `tfsdk:"hostname"`
	RecordType types.String `tfsdk:"record_type"`
	Content    types.String `tfsdk:"content"`
	Dynamic    types.Bool   `tfsdk:"dynamic"`
	TTL        types.Int64  `tfsdk:"ttl"`
	Enabled    types.Bool   `tfsdk:"enabled"`
	Group      types.String `tfsdk:"group"`
	Host       types.String `tfsdk:"host"`
	Priority   types.Int64  `tfsdk:"priority"`
	Weight     types.Int64  `tfsdk:"weight"`
	Port       types.Int64  `tfsdk:"port"`
	Flags      types.Int64  `tfsdk:"flags"`
	Tag        types.String `tfsdk:"tag"`
	Value      types.String `tfsdk:"value"`
	NodeName   types.String `tfsdk:"node_name"`
	DomainID   types.Int64  `tfsdk:"domain_id"`
	DomainName types.String `tfsdk:"domain_name"`
	UpdatedOn  types.String `tfsdk:"updated_on"`
}

func NewDNSRecordResource() resource.Resource {
	return &dnsRecordResource{}
}

func (r *dnsRecordResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_record"
}

func (r *dnsRecordResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Dynu DNS record for the root domain resolved from hostname.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "Resource identifier in domain_id/record_id format."},
			"hostname": schema.StringAttribute{
				Required:    true,
				Description: "Fully-qualified hostname under the target root domain.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
					stringvalidator.RegexMatches(hostnameValidator, "must be a valid fully-qualified hostname"),
				},
			},
			"record_type": schema.StringAttribute{Required: true, Description: "DNS record type (A, AAAA, CNAME, TXT, etc.).", Validators: []validator.String{stringvalidator.LengthAtLeast(1)}},
			"content":     schema.StringAttribute{Optional: true, Computed: true, Description: "Record content/value for static records. Omit for A/AAAA dynamic intent."},
			"dynamic":     schema.BoolAttribute{Optional: true, Computed: true, Description: "Whether A/AAAA should use Dynu dynamic IP semantics. Defaults to true when content is omitted for A/AAAA."},
			"ttl": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "DNS TTL in seconds.",
				Validators:  []validator.Int64{int64validator.AtLeast(0)},
			},
			"enabled":     schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), Description: "Whether this DNS record is enabled/active."},
			"group":       schema.StringAttribute{Optional: true, Computed: true, Description: "Dynu group value for this record."},
			"host":        schema.StringAttribute{Optional: true, Computed: true, Description: "Host field for supported Dynu record types."},
			"priority":    schema.Int64Attribute{Optional: true, Computed: true, Description: "Priority value used by MX/SRV records."},
			"weight":      schema.Int64Attribute{Optional: true, Computed: true, Description: "Weight value used by SRV records."},
			"port":        schema.Int64Attribute{Optional: true, Computed: true, Description: "Port value used by SRV records."},
			"flags":       schema.Int64Attribute{Optional: true, Computed: true, Description: "Flags value used by CAA records."},
			"tag":         schema.StringAttribute{Optional: true, Computed: true, Description: "Tag value used by CAA records."},
			"value":       schema.StringAttribute{Optional: true, Computed: true, Description: "Value field used by CAA records."},
			"node_name":   schema.StringAttribute{Optional: true, Computed: true, Description: "Node/label portion of the record."},
			"domain_id":   schema.Int64Attribute{Computed: true, Description: "Dynu domain ID resolved from hostname."},
			"domain_name": schema.StringAttribute{Computed: true, Description: "Dynu root domain name resolved from hostname."},
			"updated_on":  schema.StringAttribute{Computed: true, Description: "Last update timestamp as returned by Dynu."},
		},
	}
}

func (r *dnsRecordResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *dnsRecordResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config dnsRecordResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	recordType, skip := knownNormalizedString(config.RecordType)
	if skip {
		return
	}
	content, contentKnown := stringPointerFromOptionalContentForValidation(config.Content)
	dynamicIntent, ok := resolveDynamicIntent(recordType, config.Content, config.Dynamic, &resp.Diagnostics)
	if !ok {
		return
	}
	validateDNSRecordContentForTypeWithKnowledge(recordType, content, contentKnown, dynamicIntent, &resp.Diagnostics)
	validateDNSRecordTTL(config.TTL, &resp.Diagnostics)
}

func (r *dnsRecordResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dnsRecordResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainID, domainName, err := r.clientProvider.client.GetRootDomain(ctx, plan.Hostname.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(diagnosticSummary("Unable to resolve Dynu domain from hostname", err), err.Error())
		return
	}

	recordType := strings.TrimSpace(plan.RecordType.ValueString())
	dynamicIntent, ok := resolveDynamicIntent(strings.ToUpper(recordType), plan.Content, plan.Dynamic, &resp.Diagnostics)
	if !ok {
		return
	}

	createReq := dynuclient.CreateDNSRecordRequest{
		NodeName:   recordNodeName(plan.NodeName, plan.Hostname, domainName),
		RecordType: recordType,
		Content:    stringPointerFromOptionalContent(plan.Content),
		TTL:        int64FromOptional(plan.TTL),
		State:      boolPointerFromOptional(plan.Enabled),
		Group:      stringFromOptional(plan.Group),
		Host:       stringFromOptional(plan.Host),
		Priority:   int64FromOptional(plan.Priority),
		Weight:     int64FromOptional(plan.Weight),
		Port:       int64FromOptional(plan.Port),
		Flags:      int64FromOptional(plan.Flags),
		Tag:        stringFromOptional(plan.Tag),
		Value:      stringFromOptional(plan.Value),
	}
	createReq = normalizeDNSRecordCreateRequestForType(createReq)
	if !validateDNSRecordContentForType(createReq.RecordType, createReq.Content, dynamicIntent, &resp.Diagnostics) {
		return
	}
	if !validateRecordTTLSeconds(createReq.TTL, &resp.Diagnostics) {
		return
	}

	record, err := r.clientProvider.client.CreateDNSRecord(ctx, domainID, createReq)
	if err != nil && dynamicIntent && isUnsupportedEmptyContentError(err) {
		if retryReq, retryOK := r.applyDynamicBootstrapFallback(ctx, domainID, createReq, &resp.Diagnostics); retryOK {
			record, err = r.clientProvider.client.CreateDNSRecord(ctx, domainID, retryReq)
		}
	}
	if err != nil {
		addDNSRecordWriteDiagnostic("create", createReq.RecordType, createReq.Content, err, &resp.Diagnostics)
		return
	}

	state := mapDNSRecordToState(*record, dynamicIntent)
	state.ID = types.StringValue(formatDNSRecordID(record.DomainID, record.ID))
	// Preserve plan hostname so node_name overrides don't produce a plan/state inconsistency.
	state.Hostname = plan.Hostname
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *dnsRecordResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dnsRecordResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainID, recordID, err := parseDNSRecordID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid resource ID", err.Error())
		return
	}

	record, err := r.clientProvider.client.GetDNSRecord(ctx, domainID, recordID)
	if err != nil {
		var apiErr *dynuclient.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(diagnosticSummary("Unable to read Dynu DNS record", err), err.Error())
		return
	}

	dynamicIntent := inferDynamicIntentFromState(state.RecordType, state.Content, state.Dynamic)
	nextState := mapDNSRecordToState(*record, dynamicIntent)
	nextState.ID = state.ID
	// Preserve the stored hostname so node_name overrides remain stable across refreshes.
	nextState.Hostname = state.Hostname
	resp.Diagnostics.Append(resp.State.Set(ctx, &nextState)...)
}

func (r *dnsRecordResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan dnsRecordResourceModel
	var state dnsRecordResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainID, recordID, err := parseDNSRecordID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid resource ID", err.Error())
		return
	}

	domainName := ""
	if !plan.DomainName.IsNull() && !plan.DomainName.IsUnknown() {
		domainName = strings.TrimSpace(plan.DomainName.ValueString())
	}
	if domainName == "" && !state.DomainName.IsNull() && !state.DomainName.IsUnknown() {
		domainName = strings.TrimSpace(state.DomainName.ValueString())
	}
	if domainName == "" {
		_, resolvedDomainName, err := r.clientProvider.client.GetRootDomain(ctx, plan.Hostname.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(diagnosticSummary("Unable to resolve Dynu domain from hostname", err), err.Error())
			return
		}
		domainName = resolvedDomainName
	}

	recordType := strings.TrimSpace(plan.RecordType.ValueString())
	dynamicIntent, ok := resolveDynamicIntent(strings.ToUpper(recordType), plan.Content, plan.Dynamic, &resp.Diagnostics)
	if !ok {
		return
	}

	updateReq := dynuclient.UpdateDNSRecordRequest{
		NodeName:   recordNodeName(preferKnownString(plan.NodeName, state.NodeName), plan.Hostname, domainName),
		RecordType: recordType,
		Content:    stringPointerFromOptionalContent(plan.Content),
		TTL:        int64FromOptional(preferKnownInt64(plan.TTL, state.TTL)),
		State:      boolPointerFromOptional(preferKnownBool(plan.Enabled, state.Enabled)),
		Group:      stringFromOptional(preferKnownString(plan.Group, state.Group)),
		Host:       stringFromOptional(preferKnownString(plan.Host, state.Host)),
		Priority:   int64FromOptional(preferKnownInt64(plan.Priority, state.Priority)),
		Weight:     int64FromOptional(preferKnownInt64(plan.Weight, state.Weight)),
		Port:       int64FromOptional(preferKnownInt64(plan.Port, state.Port)),
		Flags:      int64FromOptional(preferKnownInt64(plan.Flags, state.Flags)),
		Tag:        stringFromOptional(preferKnownString(plan.Tag, state.Tag)),
		Value:      stringFromOptional(preferKnownString(plan.Value, state.Value)),
	}
	updateReq = normalizeDNSRecordUpdateRequestForType(updateReq)
	if !validateDNSRecordContentForType(updateReq.RecordType, updateReq.Content, dynamicIntent, &resp.Diagnostics) {
		return
	}
	if !validateRecordTTLSeconds(updateReq.TTL, &resp.Diagnostics) {
		return
	}

	if _, err := r.clientProvider.client.UpdateDNSRecord(ctx, domainID, recordID, updateReq); err != nil {
		if dynamicIntent && isUnsupportedEmptyContentError(err) {
			if retryReq, retryOK := r.applyDynamicBootstrapFallback(ctx, domainID, dynuclient.CreateDNSRecordRequest{
				NodeName:   updateReq.NodeName,
				RecordType: updateReq.RecordType,
				Content:    updateReq.Content,
				TTL:        updateReq.TTL,
				State:      updateReq.State,
				Group:      updateReq.Group,
				Host:       updateReq.Host,
				Priority:   updateReq.Priority,
				Weight:     updateReq.Weight,
				Port:       updateReq.Port,
				Flags:      updateReq.Flags,
				Tag:        updateReq.Tag,
				Value:      updateReq.Value,
			}, &resp.Diagnostics); retryOK {
				updateReq.Content = retryReq.Content
				updateReq.Group = retryReq.Group
				_, err = r.clientProvider.client.UpdateDNSRecord(ctx, domainID, recordID, updateReq)
			}
		}
		if err != nil {
			addDNSRecordWriteDiagnostic("update", updateReq.RecordType, updateReq.Content, err, &resp.Diagnostics)
			return
		}
	}

	record, err := r.clientProvider.client.GetDNSRecord(ctx, domainID, recordID)
	if err != nil {
		resp.Diagnostics.AddError(diagnosticSummary("Unable to read Dynu DNS record", err), err.Error())
		return
	}

	nextState := mapDNSRecordToState(*record, dynamicIntent)
	nextState.ID = state.ID
	// Preserve plan hostname so node_name overrides remain stable after updates.
	nextState.Hostname = plan.Hostname
	resp.Diagnostics.Append(resp.State.Set(ctx, &nextState)...)
}

func (r *dnsRecordResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dnsRecordResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainID, recordID, err := parseDNSRecordID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid resource ID", err.Error())
		return
	}

	err = r.clientProvider.client.DeleteDNSRecord(ctx, domainID, recordID)
	if err == nil {
		return
	}
	var apiErr *dynuclient.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
		return
	}
	resp.Diagnostics.AddError(diagnosticSummary("Unable to delete Dynu DNS record", err), err.Error())
}

func (r *dnsRecordResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	domainID, recordID, err := parseDNSRecordID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}

	state := dnsRecordResourceModel{ID: types.StringValue(formatDNSRecordID(domainID, recordID))}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func mapDNSRecordToState(record dynuclient.DNSRecord, dynamicIntent bool) dnsRecordResourceModel {
	content := normalizeRecordContentForState(record.RecordType, record.Content, dynamicIntent, record.Host, record.Value)
	return dnsRecordResourceModel{
		Hostname:   mapString(record.Hostname),
		RecordType: mapString(record.RecordType),
		Content:    content,
		Dynamic:    types.BoolValue(dynamicIntent),
		TTL:        types.Int64Value(record.TTL),
		Enabled:    types.BoolValue(record.State),
		Group:      mapString(record.Group),
		Host:       mapString(record.Host),
		Priority:   types.Int64Value(record.Priority),
		Weight:     types.Int64Value(record.Weight),
		Port:       types.Int64Value(record.Port),
		Flags:      types.Int64Value(record.Flags),
		Tag:        mapString(record.Tag),
		Value:      mapString(record.Value),
		NodeName:   mapString(record.NodeName),
		DomainID:   types.Int64Value(record.DomainID),
		DomainName: mapString(record.DomainName),
		UpdatedOn:  mapString(record.UpdatedOn),
	}
}

func parseDNSRecordID(id string) (int64, int64, error) {
	parts := strings.Split(strings.TrimSpace(id), "/")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("expected import ID in domain_id/record_id format")
	}
	domainID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || domainID <= 0 {
		return 0, 0, fmt.Errorf("invalid domain_id in ID %q", id)
	}
	recordID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || recordID <= 0 {
		return 0, 0, fmt.Errorf("invalid record_id in ID %q", id)
	}
	return domainID, recordID, nil
}

func formatDNSRecordID(domainID int64, recordID int64) string {
	return fmt.Sprintf("%d/%d", domainID, recordID)
}

func recordNodeName(nodeName types.String, hostname types.String, domainName string) string {
	if !nodeName.IsNull() && !nodeName.IsUnknown() {
		return strings.TrimSpace(nodeName.ValueString())
	}
	host := strings.TrimSpace(hostname.ValueString())
	domain := strings.TrimSpace(domainName)
	if strings.EqualFold(host, domain) {
		return ""
	}
	suffix := "." + domain
	if strings.HasSuffix(strings.ToLower(host), strings.ToLower(suffix)) {
		return strings.TrimSuffix(host, suffix)
	}
	return host
}

func int64FromOptional(value types.Int64) int64 {
	if value.IsNull() || value.IsUnknown() {
		return 0
	}
	return value.ValueInt64()
}

func boolPointerFromOptional(value types.Bool) *bool {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	v := value.ValueBool()
	return &v
}

func stringFromOptional(value types.String) string {
	if value.IsNull() || value.IsUnknown() {
		return ""
	}
	return strings.TrimSpace(value.ValueString())
}

func preferKnownString(planValue types.String, stateValue types.String) types.String {
	if !planValue.IsNull() && !planValue.IsUnknown() {
		return planValue
	}
	if !stateValue.IsNull() && !stateValue.IsUnknown() {
		return stateValue
	}
	return planValue
}

func preferKnownInt64(planValue types.Int64, stateValue types.Int64) types.Int64 {
	if !planValue.IsNull() && !planValue.IsUnknown() {
		return planValue
	}
	if !stateValue.IsNull() && !stateValue.IsUnknown() {
		return stateValue
	}
	return planValue
}

func preferKnownBool(planValue types.Bool, stateValue types.Bool) types.Bool {
	if !planValue.IsNull() && !planValue.IsUnknown() {
		return planValue
	}
	if !stateValue.IsNull() && !stateValue.IsUnknown() {
		return stateValue
	}
	return planValue
}

func stringPointerFromOptionalContent(value types.String) *string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	trimmed := strings.TrimSpace(value.ValueString())
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func stringPointerFromOptionalContentForValidation(value types.String) (*string, bool) {
	if value.IsUnknown() {
		return nil, false
	}
	return stringPointerFromOptionalContent(value), true
}

func stringPointer(value string) *string {
	return &value
}

func normalizeDNSRecordCreateRequestForType(req dynuclient.CreateDNSRecordRequest) dynuclient.CreateDNSRecordRequest {
	switch strings.ToUpper(strings.TrimSpace(req.RecordType)) {
	case "CNAME", "NS", "PTR":
		if normalizedContent := normalizeOptionalContentString(req.Content); normalizedContent != nil {
			req.Host = *normalizedContent
		}
	case "MX":
		if normalizedContent := normalizeOptionalContentString(req.Content); normalizedContent != nil {
			req.Host = *normalizedContent
		}
	case "SRV":
		if normalizedContent := normalizeOptionalContentString(req.Content); normalizedContent != nil {
			req.Host = *normalizedContent
		}
	case "CAA":
		if normalizedContent := normalizeOptionalContentString(req.Content); normalizedContent != nil && req.Value == "" {
			req.Value = *normalizedContent
		}
	}
	return req
}

func normalizeDNSRecordUpdateRequestForType(req dynuclient.UpdateDNSRecordRequest) dynuclient.UpdateDNSRecordRequest {
	switch strings.ToUpper(strings.TrimSpace(req.RecordType)) {
	case "CNAME", "NS", "PTR":
		if normalizedContent := normalizeOptionalContentString(req.Content); normalizedContent != nil {
			req.Host = *normalizedContent
		}
	case "MX":
		if normalizedContent := normalizeOptionalContentString(req.Content); normalizedContent != nil {
			req.Host = *normalizedContent
		}
	case "SRV":
		if normalizedContent := normalizeOptionalContentString(req.Content); normalizedContent != nil {
			req.Host = *normalizedContent
		}
	case "CAA":
		if normalizedContent := normalizeOptionalContentString(req.Content); normalizedContent != nil && req.Value == "" {
			req.Value = *normalizedContent
		}
	}
	return req
}

func normalizeOptionalContentString(content *string) *string {
	if content == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*content)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func validateDNSRecordContentForType(recordType string, content *string, dynamicIntent bool, diagnostics *diag.Diagnostics) bool {
	return validateDNSRecordContentForTypeWithKnowledge(recordType, content, true, dynamicIntent, diagnostics)
}

func validateDNSRecordContentForTypeWithKnowledge(recordType string, content *string, contentKnown bool, dynamicIntent bool, diagnostics *diag.Diagnostics) bool {
	normalizedType := strings.ToUpper(strings.TrimSpace(recordType))
	trimmedContent := ""
	if content != nil {
		trimmedContent = strings.TrimSpace(*content)
	}

	if normalizedType == "A" || normalizedType == "AAAA" {
		if dynamicIntent && trimmedContent == "" {
			return true
		}
		if trimmedContent == "" {
			diagnostics.AddError(
				"Missing static content for DNS record type",
				fmt.Sprintf("The %q record type requires a non-empty content value unless dynamic mode is used.", normalizedType),
			)
			return false
		}

		addr, err := netip.ParseAddr(trimmedContent)
		if err != nil {
			diagnostics.AddError("Invalid DNS record content", fmt.Sprintf("Record type %q requires a valid IP address, got %q.", normalizedType, trimmedContent))
			return false
		}
		if normalizedType == "A" && !addr.Is4() {
			diagnostics.AddError("Invalid DNS record content", fmt.Sprintf("Record type %q requires an IPv4 address, got %q.", normalizedType, trimmedContent))
			return false
		}
		if normalizedType == "AAAA" && !addr.Is6() {
			diagnostics.AddError("Invalid DNS record content", fmt.Sprintf("Record type %q requires an IPv6 address, got %q.", normalizedType, trimmedContent))
			return false
		}
		return true
	}
	if normalizedType == "CNAME" {
		if net.ParseIP(trimmedContent) != nil {
			diagnostics.AddError("Invalid DNS record content", fmt.Sprintf("Record type %q requires a hostname target, got IP %q.", normalizedType, trimmedContent))
			return false
		}
		if strings.HasPrefix(strings.ToLower(trimmedContent), "http://") || strings.HasPrefix(strings.ToLower(trimmedContent), "https://") {
			diagnostics.AddError("Invalid DNS record content", fmt.Sprintf("Record type %q target must not include a URL scheme: %q.", normalizedType, trimmedContent))
			return false
		}
	}

	if dynamicIntent {
		diagnostics.AddError(
			"Dynamic mode is only supported for A and AAAA",
			fmt.Sprintf("The %q record type does not support omitted content.", normalizedType),
		)
		return false
	}

	if !contentKnown {
		return true
	}

	if trimmedContent == "" {
		diagnostics.AddError(
			"Missing required content for DNS record type",
			fmt.Sprintf("The %q record type requires a non-empty content value. Set the content attribute or choose a type that supports omitted content (A/AAAA).", normalizedType),
		)
		return false
	}

	return true
}

func validateDNSRecordTTL(ttl types.Int64, diagnostics *diag.Diagnostics) {
	if ttl.IsNull() || ttl.IsUnknown() {
		return
	}
	validateRecordTTLSeconds(ttl.ValueInt64(), diagnostics)
}

func validateRecordTTLSeconds(ttl int64, diagnostics *diag.Diagnostics) bool {
	if ttl > 0 && ttl < int64((90*time.Second).Seconds()) {
		diagnostics.AddError("Invalid TTL", fmt.Sprintf("invalid TTL %ds: Dynu requires TTL >= 90s", ttl))
		return false
	}
	return true
}

func resolveDynamicIntent(recordType string, content types.String, dynamic types.Bool, diagnostics *diag.Diagnostics) (bool, bool) {
	normalizedType := strings.ToUpper(strings.TrimSpace(recordType))
	contentPtr := stringPointerFromOptionalContent(content)
	contentPresent := contentPtr != nil && strings.TrimSpace(*contentPtr) != ""

	explicitDynamic := false
	if !dynamic.IsNull() && !dynamic.IsUnknown() {
		explicitDynamic = dynamic.ValueBool()
	}

	if normalizedType != "A" && normalizedType != "AAAA" {
		if explicitDynamic {
			diagnostics.AddError("Invalid dynamic setting", fmt.Sprintf("record_type %q cannot use dynamic = true.", normalizedType))
			return false, false
		}
		return false, true
	}

	if contentPresent && explicitDynamic {
		diagnostics.AddError("Conflicting DNS record settings", "Set either content for static records or dynamic = true/omitted content for Dynu dynamic behavior.")
		return false, false
	}
	if contentPresent {
		return false, true
	}
	if explicitDynamic {
		return true, true
	}
	return true, true
}

func inferDynamicIntentFromState(recordType types.String, content types.String, dynamic types.Bool) bool {
	if !dynamic.IsNull() && !dynamic.IsUnknown() {
		return dynamic.ValueBool()
	}

	normalizedType := strings.ToUpper(strings.TrimSpace(recordType.ValueString()))
	if normalizedType != "A" && normalizedType != "AAAA" {
		return false
	}

	return content.IsNull() || (content.IsUnknown())
}

func normalizeRecordContentForState(recordType string, content string, dynamicIntent bool, host string, value string) types.String {
	if dynamicIntent {
		return types.StringNull()
	}

	normalizedType := strings.ToUpper(strings.TrimSpace(recordType))
	trimmed := strings.TrimSpace(strings.Trim(strings.TrimSpace(content), "()"))
	if trimmed == "" {
		return types.StringNull()
	}

	switch normalizedType {
	case "AAAA":
		if addr, err := netip.ParseAddr(trimmed); err == nil && addr.Is6() {
			return types.StringValue(addr.String())
		}
	case "A":
		if addr, err := netip.ParseAddr(trimmed); err == nil && addr.Is4() {
			return types.StringValue(addr.String())
		}
	case "CNAME", "MX", "SRV", "NS", "PTR":
		// API returns the target in host field; content contains zone-style prefix (e.g. "10 mail.example.com.")
		if trimmedHost := strings.TrimSpace(host); trimmedHost != "" {
			return types.StringValue(strings.TrimSuffix(trimmedHost, "."))
		}
		return types.StringValue(strings.TrimSuffix(trimmed, "."))
	case "CAA":
		// API returns the certificate authority in value field; content contains "flags tag value"
		if trimmedValue := strings.TrimSpace(value); trimmedValue != "" {
			return types.StringValue(strings.TrimSuffix(trimmedValue, "."))
		}
	}

	return types.StringValue(trimmed)
}

func isUnsupportedEmptyContentError(err error) bool {
	var apiErr *dynuclient.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	if apiErr.StatusCode != 400 && apiErr.StatusCode != 505 {
		return false
	}
	normalizedType := strings.ToLower(strings.TrimSpace(apiErr.Type))
	if normalizedType != "validation exception" {
		return false
	}
	normalizedMessage := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(apiErr.Message, ".")))
	return strings.Contains(normalizedMessage, "content is required") ||
		strings.Contains(normalizedMessage, "ipv4address is required") ||
		strings.Contains(normalizedMessage, "ipv6address is required") ||
		strings.Contains(normalizedMessage, "invalid ip address") ||
		normalizedMessage == "invalid"
}

func (r *dnsRecordResource) applyDynamicBootstrapFallback(ctx context.Context, domainID int64, req dynuclient.CreateDNSRecordRequest, diagnostics *diag.Diagnostics) (dynuclient.CreateDNSRecordRequest, bool) {
	if req.Content != nil {
		return req, true
	}
	domain, err := r.clientProvider.client.GetDomainByID(ctx, domainID)
	if err != nil {
		diagnostics.AddError(diagnosticSummary("Unable to fetch Dynu domain for dynamic fallback", err), err.Error())
		return req, false
	}
	switch strings.ToUpper(strings.TrimSpace(req.RecordType)) {
	case "A":
		if strings.TrimSpace(domain.IPv4Address) == "" {
			diagnostics.AddError("Unable to emulate dynamic A record", "Dynu rejected omitted IPv4 content and the root domain has no IPv4 address to bootstrap from.")
			return req, false
		}
		req.Content = stringPointer(strings.TrimSpace(domain.IPv4Address))
	case "AAAA":
		if strings.TrimSpace(domain.IPv6Address) == "" {
			diagnostics.AddError("Unable to emulate dynamic AAAA record", "Dynu rejected omitted IPv6 content and the root domain has no IPv6 address to bootstrap from.")
			return req, false
		}
		req.Content = stringPointer(strings.TrimSpace(domain.IPv6Address))
	default:
		return req, true
	}
	if strings.TrimSpace(req.Group) == "" {
		req.Group = strings.TrimSpace(domain.Group)
	}
	return req, true
}

func addDNSRecordWriteDiagnostic(operation string, recordType string, content *string, err error, diagnostics *diag.Diagnostics) {
	detail := err.Error()
	var apiErr *dynuclient.APIError
	if errors.As(err, &apiErr) {
		presence := "omitted"
		if content != nil {
			presence = fmt.Sprintf("set to %q", *content)
		}
		detail = fmt.Sprintf("%s. Dynu rejected this %s request for record type %q where content was %s.", err.Error(), operation, strings.ToUpper(strings.TrimSpace(recordType)), presence)
	}

	diagnostics.AddError(diagnosticSummary(fmt.Sprintf("Unable to %s Dynu DNS record", operation), err), detail)
}

func knownNormalizedString(value types.String) (string, bool) {
	if value.IsNull() || value.IsUnknown() {
		return "", true
	}
	return strings.ToUpper(strings.TrimSpace(value.ValueString())), false
}
