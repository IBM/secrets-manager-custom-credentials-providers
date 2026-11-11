package job

import (
	"strings"
	"testing"
)

func TestResolveIAMURL_Empty(t *testing.T) {
	got, err := resolveIAMURL("")
	if err != nil || got != "" {
		t.Errorf("resolveIAMURL(\"\") = %q, %v; want \"\" (SDK default), nil", got, err)
	}
}

func TestResolveIAMURL_Accepted(t *testing.T) {
	for _, in := range []string{
		"https://iam.cloud.ibm.com",
		"https://iam.cloud.ibm.com/",
		"HTTPS://IAM.CLOUD.IBM.COM",
		"https://IAM.cloud.ibm.com/",
	} {
		got, err := resolveIAMURL(in)
		if err != nil {
			t.Errorf("resolveIAMURL(%q) unexpected error: %v", in, err)
		}
		if got != "https://iam.cloud.ibm.com" {
			t.Errorf("resolveIAMURL(%q) = %q, want the allowlisted value", in, got)
		}
	}
}

func TestResolveIAMURL_Rejected(t *testing.T) {
	for _, in := range []string{
		"http://127.0.0.1:61434",
		"https://attacker.example.com",
		"http://iam.cloud.ibm.com",
		"https://iam.cloud.ibm.com//",
		"https://iam.cloud.ibm.com/identity",
		"https://iam.cloud.ibm.com/identity/token",
		"https://iam.cloud.ibm.com:443",
		"https://iam.cloud.ibm.com:8443/",
		"https://iam.cloud.ibm.com?x=1",
		"https://iam.cloud.ibm.com/?",
		"https://iam.cloud.ibm.com#frag",
		"https://iam.cloud.ibm.com/#",
		"https://user@iam.cloud.ibm.com",
		"https://iam.cloud.ibm.com@attacker.example.com",
		"https://iam.cloud.ibm.com:pass@attacker.example.com",
		"https://attacker.example.com\\@iam.cloud.ibm.com",
		"https://attacker.example.com#@iam.cloud.ibm.com",
		"https://iam.cloud.ibm.com.attacker.example.com",
		"https://iam.cloud.ibm.com.",
		"https://iam.test.cloud.ibm.com",
		" https://iam.cloud.ibm.com",
		"https://iam.cloud.ibm.com ",
		"https://iam.cloud.ibm.com\n",
		"//iam.cloud.ibm.com",
		"iam.cloud.ibm.com",
		"https:iam.cloud.ibm.com",
		"https:///",
		"ftp://iam.cloud.ibm.com",
		"not a url",
	} {
		got, err := resolveIAMURL(in)
		if err == nil {
			t.Errorf("resolveIAMURL(%q) = %q, expected an error", in, got)
			continue
		}
		if !strings.Contains(err.Error(), "SMIN_URL") || !strings.Contains(err.Error(), "IAM URL") {
			t.Errorf("resolveIAMURL(%q) error is not self-explanatory: %v", in, err)
		}
	}
}
