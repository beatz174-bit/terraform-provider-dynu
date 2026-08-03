package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/beatz174-bit/terraform-provider-dynu/internal/dynuclient"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestParseDNSRecordID(t *testing.T) {
	domainID, recordID, err := parseDNSRecordID("1001/55")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if domainID != 1001 || recordID != 55 {
		t.Fatalf("unexpected ids: domain=%d record=%d", domainID, recordID)
	}
}

func TestParseDNSRecordIDInvalid(t *testing.T) {
	if _, _, err := parseDNSRecordID("1001"); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestValidateDNSRecordContentForType(t *testing.T) {
	ipv4 := "8.8.8.8"
	ipv6 := "2606:4700:4700::1111"
	docIPv4A := "192.0.2.123"
	docIPv4B := "198.51.100.123"
	docIPv4C := "203.0.113.123"
	docIPv6 := "2001:db8::123"
	nonIP := "not-an-ip"
	nonEmpty := "hello"
	blank := ""

	tests := []struct {
		name       string
		recordType string
		content    *string
		dynamic    bool
		wantValid  bool
	}{
		{name: "A accepts static ipv4", recordType: "A", content: &ipv4, wantValid: true},
		{name: "A accepts documentation ipv4 192.0.2.0/24", recordType: "A", content: &docIPv4A, wantValid: true},
		{name: "A accepts documentation ipv4 198.51.100.0/24", recordType: "A", content: &docIPv4B, wantValid: true},
		{name: "A accepts documentation ipv4 203.0.113.0/24", recordType: "A", content: &docIPv4C, wantValid: true},
		{name: "A rejects non-ip", recordType: "A", content: &nonIP, wantValid: false},
		{name: "A rejects ipv6", recordType: "A", content: &ipv6, wantValid: false},
		{name: "A accepts dynamic nil", recordType: "A", content: nil, dynamic: true, wantValid: true},
		{name: "A accepts dynamic blank", recordType: "A", content: &blank, dynamic: true, wantValid: true},
		{name: "AAAA accepts static ipv6", recordType: "AAAA", content: &ipv6, wantValid: true},
		{name: "AAAA accepts documentation ipv6 2001:db8::/32", recordType: "AAAA", content: &docIPv6, wantValid: true},
		{name: "AAAA rejects non-ip", recordType: "AAAA", content: &nonIP, wantValid: false},
		{name: "AAAA rejects ipv4", recordType: "AAAA", content: &ipv4, wantValid: false},
		{name: "AAAA accepts dynamic nil", recordType: "AAAA", content: nil, dynamic: true, wantValid: true},
		{name: "TXT requires content", recordType: "TXT", content: nil, wantValid: false},
		{name: "TXT rejects blank content", recordType: "TXT", content: &blank, wantValid: false},
		{name: "TXT accepts non-empty content", recordType: "TXT", content: &nonEmpty, wantValid: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			diags := diag.Diagnostics{}
			got := validateDNSRecordContentForType(tc.recordType, tc.content, tc.dynamic, &diags)
			if got != tc.wantValid {
				t.Fatalf("validateDNSRecordContentForType()=%v, want %v", got, tc.wantValid)
			}
			if tc.wantValid && diags.HasError() {
				t.Fatalf("expected no error diagnostics, got %#v", diags)
			}
			if !tc.wantValid && !diags.HasError() {
				t.Fatal("expected error diagnostics")
			}
		})
	}
}

func TestResolveDynamicIntent(t *testing.T) {
	diags := diag.Diagnostics{}
	dynamic, ok := resolveDynamicIntent("A", types.StringNull(), types.BoolNull(), &diags)
	if !ok || !dynamic || diags.HasError() {
		t.Fatalf("expected omitted A content to resolve to dynamic=true, got dynamic=%v ok=%v diags=%v", dynamic, ok, diags)
	}

	diags = diag.Diagnostics{}
	dynamic, ok = resolveDynamicIntent("A", types.StringValue("8.8.4.4"), types.BoolNull(), &diags)
	if !ok || dynamic || diags.HasError() {
		t.Fatalf("expected static A content to resolve to dynamic=false, got dynamic=%v ok=%v diags=%v", dynamic, ok, diags)
	}
}

func TestValidateDNSRecordContentForTypeWithKnowledge(t *testing.T) {
	nonEmpty := "example.com"
	blank := ""

	tests := []struct {
		name         string
		recordType   string
		content      *string
		contentKnown bool
		dynamic      bool
		wantValid    bool
		wantMessage  string
	}{
		{name: "CNAME unknown content is allowed during validate", recordType: "CNAME", content: nil, contentKnown: false, wantValid: true},
		{name: "CNAME null content errors", recordType: "CNAME", content: nil, contentKnown: true, wantValid: false, wantMessage: "Missing required content"},
		{name: "CNAME empty content errors", recordType: "CNAME", content: &blank, contentKnown: true, wantValid: false, wantMessage: "Missing required content"},
		{name: "CNAME known content passes", recordType: "CNAME", content: &nonEmpty, contentKnown: true, wantValid: true},
		{name: "CNAME dynamic true errors", recordType: "CNAME", content: nil, contentKnown: false, dynamic: true, wantValid: false, wantMessage: "Dynamic mode is only supported"},
		{name: "A dynamic omitted content passes", recordType: "A", content: nil, contentKnown: true, dynamic: true, wantValid: true},
		{name: "AAAA dynamic omitted content passes", recordType: "AAAA", content: nil, contentKnown: true, dynamic: true, wantValid: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			diags := diag.Diagnostics{}
			got := validateDNSRecordContentForTypeWithKnowledge(tc.recordType, tc.content, tc.contentKnown, tc.dynamic, &diags)
			if got != tc.wantValid {
				t.Fatalf("validateDNSRecordContentForTypeWithKnowledge()=%v, want %v", got, tc.wantValid)
			}
			if tc.wantValid && diags.HasError() {
				t.Fatalf("expected no diagnostics, got %#v", diags)
			}
			if !tc.wantValid {
				if !diags.HasError() {
					t.Fatal("expected diagnostics but got none")
				}
				if tc.wantMessage != "" && diags[0].Summary() != "" && !strings.Contains(diags[0].Summary(), tc.wantMessage) {
					t.Fatalf("expected first diagnostic summary to contain %q, got %q", tc.wantMessage, diags[0].Summary())
				}
			}
		})
	}
}

func TestStringPointerFromOptionalContentForValidation(t *testing.T) {
	if content, known := stringPointerFromOptionalContentForValidation(types.StringUnknown()); known || content != nil {
		t.Fatalf("expected unknown content to be unknown with nil pointer, got known=%v content=%v", known, content)
	}
	if content, known := stringPointerFromOptionalContentForValidation(types.StringNull()); !known || content != nil {
		t.Fatalf("expected null content to be known with nil pointer, got known=%v content=%v", known, content)
	}
}

func TestNormalizeRecordContentForState(t *testing.T) {
	if got := normalizeRecordContentForState("AAAA", "2001:0db8:0000:0000:0000:0000:0000:0123", false, "", ""); got.ValueString() != "2001:db8::123" {
		t.Fatalf("expected canonical IPv6, got %q", got.ValueString())
	}
	if got := normalizeRecordContentForState("CNAME", "Example.COM.", false, "", ""); got.ValueString() != "Example.COM" {
		t.Fatalf("expected trailing dot removed, got %q", got.ValueString())
	}
	if got := normalizeRecordContentForState("CNAME", "old.example.com", false, "new.example.co.", ""); got.ValueString() != "new.example.co" {
		t.Fatalf("expected CNAME content to come from host, got %q", got.ValueString())
	}
	if got := normalizeRecordContentForState("A", "(167.179.167.166)", true, "", ""); !got.IsNull() {
		t.Fatalf("expected dynamic content to remain null, got %q", got.ValueString())
	}
	if got := normalizeRecordContentForState("MX", "10 mail.example.com.", false, "mail.example.com", ""); got.ValueString() != "mail.example.com" {
		t.Fatalf("expected MX content from host field, got %q", got.ValueString())
	}
	if got := normalizeRecordContentForState("SRV", "10 5 5060 sip.example.com.", false, "sip.example.com", ""); got.ValueString() != "sip.example.com" {
		t.Fatalf("expected SRV content from host field, got %q", got.ValueString())
	}
	if got := normalizeRecordContentForState("CAA", "0 issue letsencrypt.org", false, "", "letsencrypt.org"); got.ValueString() != "letsencrypt.org" {
		t.Fatalf("expected CAA content from value field, got %q", got.ValueString())
	}
}

func TestNormalizeDNSRecordUpdateRequestForType(t *testing.T) {
	oldHost := "example.com"
	newContent := "example.co"
	req := normalizeDNSRecordUpdateRequestForType(dynuclient.UpdateDNSRecordRequest{
		RecordType: "CNAME",
		Content:    &newContent,
		Host:       oldHost,
	})
	if req.Host != "example.co" {
		t.Fatalf("expected CNAME host to be normalized from content, got %q", req.Host)
	}
}

func TestValidateRecordTTLSeconds(t *testing.T) {
	cases := []struct {
		ttl  int64
		want bool
	}{
		{ttl: 0, want: true},
		{ttl: 89, want: false},
		{ttl: 90, want: true},
		{ttl: 1800, want: true},
	}
	for _, tc := range cases {
		diags := diag.Diagnostics{}
		got := validateRecordTTLSeconds(tc.ttl, &diags)
		if got != tc.want {
			t.Fatalf("validateRecordTTLSeconds(%d)=%v, want %v", tc.ttl, got, tc.want)
		}
	}
}

func TestDNSRecordSchemaHasEnabledAndNoLocation(t *testing.T) {
	r := NewDNSRecordResource()
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, resp)
	if _, ok := resp.Schema.Attributes["enabled"]; !ok {
		t.Fatal("expected enabled attribute in schema")
	}
	if _, ok := resp.Schema.Attributes["location"]; ok {
		t.Fatal("expected location attribute to be removed from schema")
	}
}

func TestInferDynamicIntentFromState(t *testing.T) {
	tests := []struct {
		name       string
		recordType types.String
		content    types.String
		dynamic    types.Bool
		want       bool
	}{
		{
			name:       "explicit dynamic false wins",
			recordType: types.StringValue("A"),
			content:    types.StringNull(),
			dynamic:    types.BoolValue(false),
			want:       false,
		},
		{
			name:       "legacy A null content treated dynamic",
			recordType: types.StringValue("A"),
			content:    types.StringNull(),
			dynamic:    types.BoolNull(),
			want:       true,
		},
		{
			name:       "legacy AAAA unknown content treated dynamic",
			recordType: types.StringValue("AAAA"),
			content:    types.StringUnknown(),
			dynamic:    types.BoolNull(),
			want:       true,
		},
		{
			name:       "legacy non-A not dynamic",
			recordType: types.StringValue("TXT"),
			content:    types.StringNull(),
			dynamic:    types.BoolNull(),
			want:       false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := inferDynamicIntentFromState(tc.recordType, tc.content, tc.dynamic); got != tc.want {
				t.Fatalf("inferDynamicIntentFromState()=%v, want %v", got, tc.want)
			}
		})
	}
}

func TestIsUnsupportedEmptyContentError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "status 400 content required",
			err: &dynuclient.APIError{
				StatusCode: 400,
				Type:       "Validation Exception",
				Message:    "content is required",
			},
			want: true,
		},
		{
			name: "status 505 invalid ip address",
			err: &dynuclient.APIError{
				StatusCode: 505,
				Type:       "Validation Exception",
				Message:    "Invalid IP address.",
			},
			want: true,
		},
		{
			name: "status 505 generic invalid with period",
			err: &dynuclient.APIError{
				StatusCode: 505,
				Type:       "Validation Exception",
				Message:    "Invalid.",
			},
			want: true,
		},
		{
			name: "status 505 generic invalid without period",
			err: &dynuclient.APIError{
				StatusCode: 505,
				Type:       "Validation Exception",
				Message:    "Invalid",
			},
			want: true,
		},
		{
			name: "status 505 unrelated message",
			err: &dynuclient.APIError{
				StatusCode: 505,
				Type:       "Validation Exception",
				Message:    "Some other validation failure",
			},
			want: false,
		},
		{
			name: "non validation exception",
			err: &dynuclient.APIError{
				StatusCode: 505,
				Type:       "Unauthorized",
				Message:    "Invalid IP address.",
			},
			want: false,
		},
		{
			name: "status 500 generic invalid",
			err: &dynuclient.APIError{
				StatusCode: 500,
				Type:       "Validation Exception",
				Message:    "Invalid.",
			},
			want: false,
		},
		{
			name: "status 505 non validation generic invalid",
			err: &dynuclient.APIError{
				StatusCode: 505,
				Type:       "Some Other Exception",
				Message:    "Invalid.",
			},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isUnsupportedEmptyContentError(tc.err); got != tc.want {
				t.Fatalf("isUnsupportedEmptyContentError()=%v, want %v", got, tc.want)
			}
		})
	}
}
