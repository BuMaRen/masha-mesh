package ctrl

import "testing"

func TestAddressValidate(t *testing.T) {
	tests := []struct {
		name    string
		address string
		got     string
		valid   bool
	}{
		{
			name:    "valid address with port",
			address: "localhost:8443",
			got:     "localhost:8443",
			valid:   true,
		},
		{
			name:    "invalid address - missing port",
			address: "localhost",
			got:     "",
			valid:   false,
		},
		{
			name:    "invalid address - empty string",
			address: "",
			got:     "",
			valid:   false,
		},
		{
			name:    "valid address but can not use in http",
			address: ":443",
			got:     "localhost:443",
			valid:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, valid := addressValidate(tt.address)
			if got != tt.got || valid != tt.valid {
				t.Errorf("addressValidate(%q) = (%q, %v), want (%q, %v)", tt.address, got, valid, tt.got, tt.valid)
			}
		})
	}
}
