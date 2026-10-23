package updateauth

import "testing"

func TestNativePackageMetadata(t *testing.T) {
	for _, test := range []struct {
		kind, metadata, version, arch string
		valid                         bool
	}{
		{"rpm", "postika\n0.2.0\n1\nx86_64\n", "0.2.0", "amd64", true},
		{"rpm", "postika\n0.2.0\n1\naarch64\n", "", "arm64", true},
		{"rpm", "other\n0.2.0\n1\nx86_64\n", "0.2.0", "amd64", false},
		{"rpm", "postika\n0.1.0\n1\nx86_64\n", "0.2.0", "amd64", false},
		{"rpm", "postika\n0.2.0\n1\naarch64\n", "0.2.0", "amd64", false},
		{"rpm", "postika\n\n1\nx86_64\n", "", "amd64", false},
		{"arch", "Name : postika\nVersion : 0.2.0-1\nArchitecture : x86_64\n", "0.2.0", "amd64", true},
		{"arch", "Name : postika\nVersion : 0.2.0-1\nArchitecture : aarch64\n", "", "arm64", true},
		{"arch", "Name : postika\nVersion : 0.1.0-1\nArchitecture : x86_64\n", "0.2.0", "amd64", false},
		{"arch", "Name : postika\nArchitecture : x86_64\n", "", "amd64", false},
		{"unknown", "", "0.2.0", "amd64", false},
	} {
		_, err := PackageMetadata(test.kind, test.metadata, test.version, test.arch)
		if (err == nil) != test.valid {
			t.Fatalf("%+v: %v", test, err)
		}
	}
}
