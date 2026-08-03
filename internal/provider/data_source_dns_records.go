package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = &dnsRecordsDataSource{}
	_ datasource.DataSourceWithConfigure = &dnsRecordsDataSource{}
)

type dnsRecordsDataSource struct {
	clientProvider *providerData
}

type dnsRecordsDataSourceModel struct {
	Hostname   types.String         `tfsdk:"hostname"`
	DomainID   types.Int64          `tfsdk:"domain_id"`
	DomainName types.String         `tfsdk:"domain_name"`
	Records    []dnsRecordStateItem `tfsdk:"records"`
}

type dnsRecordStateItem struct {
	ID         types.Int64  `tfsdk:"id"`
	DomainID   types.Int64  `tfsdk:"domain_id"`
	DomainName types.String `tfsdk:"domain_name"`
	NodeName   types.String `tfsdk:"node_name"`
	Hostname   types.String `tfsdk:"hostname"`
	RecordType types.String `tfsdk:"record_type"`
	TTL        types.Int64  `tfsdk:"ttl"`
	State      types.Bool   `tfsdk:"state"`
	Content    types.String `tfsdk:"content"`
	UpdatedOn  types.String `tfsdk:"updated_on"`
	Group      types.String `tfsdk:"group"`
	Host       types.String `tfsdk:"host"`
	Priority   types.Int64  `tfsdk:"priority"`
	Weight     types.Int64  `tfsdk:"weight"`
	Port       types.Int64  `tfsdk:"port"`
	Flags      types.Int64  `tfsdk:"flags"`
	Tag        types.String `tfsdk:"tag"`
	Value      types.String `tfsdk:"value"`
}

func NewDNSRecordsDataSource() datasource.DataSource {
	return &dnsRecordsDataSource{}
}

func (d *dnsRecordsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_records"
}

func (d *dnsRecordsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists DNS records for the Dynu root domain resolved from a hostname.",
		Attributes: map[string]schema.Attribute{
			"hostname": schema.StringAttribute{
				Required:    true,
				Description: "Fully-qualified hostname under the target root domain.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
					stringvalidator.RegexMatches(hostnameValidator, "must be a valid fully-qualified hostname"),
				},
			},
			"domain_id":   schema.Int64Attribute{Computed: true, Description: "Dynu domain ID resolved from hostname."},
			"domain_name": schema.StringAttribute{Computed: true, Description: "Dynu root domain name resolved from hostname."},
			"records": schema.ListNestedAttribute{
				Computed:    true,
				Description: "Sorted DNS records for the resolved domain. Timestamps are returned as Dynu provides them.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":          schema.Int64Attribute{Computed: true, Description: "Dynu DNS record ID."},
					"domain_id":   schema.Int64Attribute{Computed: true, Description: "Dynu domain ID for this record."},
					"domain_name": schema.StringAttribute{Computed: true, Description: "Domain name for this record."},
					"node_name":   schema.StringAttribute{Computed: true, Description: "Node/label portion of the record."},
					"hostname":    schema.StringAttribute{Computed: true, Description: "Fully-qualified hostname for the record."},
					"record_type": schema.StringAttribute{Computed: true, Description: "DNS record type (A, CNAME, TXT, etc.)."},
					"ttl":         schema.Int64Attribute{Computed: true, Description: "DNS TTL in seconds."},
					"state":       schema.BoolAttribute{Computed: true, Description: "Whether this DNS record is active."},
					"content":     schema.StringAttribute{Computed: true, Description: "Record content/value."},
					"updated_on":  schema.StringAttribute{Computed: true, Description: "Last update timestamp as returned by Dynu."},
					"group":       schema.StringAttribute{Computed: true, Description: "Dynu group value for this record."},
					"host":        schema.StringAttribute{Computed: true, Description: "Host field as returned by Dynu."},
					"priority":    schema.Int64Attribute{Computed: true, Description: "Priority for MX and SRV records."},
					"weight":      schema.Int64Attribute{Computed: true, Description: "Weight for SRV records."},
					"port":        schema.Int64Attribute{Computed: true, Description: "Port for SRV records."},
					"flags":       schema.Int64Attribute{Computed: true, Description: "Flags for CAA records."},
					"tag":         schema.StringAttribute{Computed: true, Description: "Tag for CAA records."},
					"value":       schema.StringAttribute{Computed: true, Description: "Value for CAA records."},
				}},
			},
		},
	}
}

func (d *dnsRecordsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *dnsRecordsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	hostname, err := hostnameFromConfig(ctx, req.Config)
	if err != nil {
		resp.Diagnostics.AddError("Unable to parse data source configuration", err.Error())
		return
	}

	if hostname.IsUnknown() || hostname.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("hostname"), "Invalid hostname", "The hostname must be known and non-null.")
		return
	}

	domainID, domainName, err := d.clientProvider.client.GetRootDomain(ctx, hostname.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(diagnosticSummary("Unable to resolve Dynu domain from hostname", err), err.Error())
		return
	}

	records, err := d.clientProvider.client.ListDNSRecords(ctx, domainID)
	if err != nil {
		resp.Diagnostics.AddError(diagnosticSummary("Unable to list Dynu DNS records", err), err.Error())
		return
	}

	sortDNSRecords(records)

	state := dnsRecordsDataSourceModel{
		Hostname:   hostname,
		DomainID:   types.Int64Value(domainID),
		DomainName: types.StringValue(domainName),
		Records:    make([]dnsRecordStateItem, 0, len(records)),
	}
	for _, record := range records {
		state.Records = append(state.Records, dnsRecordStateItem{
			ID:         types.Int64Value(record.ID),
			DomainID:   types.Int64Value(record.DomainID),
			DomainName: mapString(record.DomainName),
			NodeName:   mapString(record.NodeName),
			Hostname:   mapString(record.Hostname),
			RecordType: mapString(record.RecordType),
			TTL:        types.Int64Value(record.TTL),
			State:      types.BoolValue(record.State),
			Content:    mapString(record.Content),
			UpdatedOn:  mapString(record.UpdatedOn),
			Group:      mapString(record.Group),
			Host:       mapString(record.Host),
			Priority:   types.Int64Value(record.Priority),
			Weight:     types.Int64Value(record.Weight),
			Port:       types.Int64Value(record.Port),
			Flags:      types.Int64Value(record.Flags),
			Tag:        mapString(record.Tag),
			Value:      mapString(record.Value),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
