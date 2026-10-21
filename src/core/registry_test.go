package core

import (
	"reflect"
	"regexp"
	"testing"
)

func TestIsOK(t *testing.T) {
	t.Parallel()

	type args struct {
		url string
	}

	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{"Pass", args{"https://registry.terraform.io/v1/modules/jameswoolfenden/ip/http/versions"}, true, false},
		{"Fail", args{"https://registry.terraform.io/v1/modules/jameswoolfenden/ip/https/versions"}, false, true},
		{"NotUrl", args{"jameswoolfenden/ip/https"}, false, true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := IsOK(tt.args.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsOK() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("IsOK() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRegistry_IsRegistryModule(t *testing.T) {
	t.Parallel()

	type fields struct {
		Registry bool
	}

	type args struct {
		module string
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    bool
		wantErr bool
	}{
		{"Pass", fields{false}, args{"jameswoolfenden/ip/http"}, true, false},
		{"Fail", fields{false}, args{"jameswoolfenden/ip/https"}, false, true},
		{"NotUrl", fields{false}, args{"https://jameswoolfenden/ip/https"}, false, true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			myRegistry := &Registry{
				Registry: tt.fields.Registry,
			}
			got, err := myRegistry.IsRegistryModule(tt.args.module)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsRegistryModule() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if got != tt.want {
				t.Errorf("IsRegistryModule() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRegistry_GetLatest(t *testing.T) {
	t.Parallel()

	type fields struct {
		Registry      bool
		LatestVersion string
	}

	type args struct {
		module string
	}

	tests := []struct {
		name       string
		fields     fields
		args       args
		wantSemver bool
		wantErr    bool
	}{
		{"Pass", fields{false, ""}, args{"jameswoolfenden/ip/http"}, true, false},
		{"Fail", fields{false, ""}, args{"jameswoolfenden/ip/guff"}, false, true},
		{"No Repo", fields{false, ""}, args{"jameswoolfenden/ip/guff"}, false, true},
	}
	// semverRe matches a bare "x.y.z" version string, not pinned to any specific
	// release — the "Pass" case queries jameswoolfenden/ip/http on the live
	// Terraform registry, whose latest version advances over time.
	semverRe := regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			myRegistry := &Registry{
				Registry:      tt.fields.Registry,
				LatestVersion: tt.fields.LatestVersion,
			}
			got, err := myRegistry.GetLatest(tt.args.module)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetLatestRelease() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantSemver {
				if got != nil {
					t.Errorf("GetLatestRelease() got = %v, want nil", got)
				}
				return
			}
			if got == nil || !semverRe.MatchString(*got) {
				t.Errorf("GetLatestRelease() got = %v, want a semver version", got)
			}
		})
	}
}
