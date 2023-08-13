package core

import "testing"

func TestFlags_GetType(t *testing.T) {
	t.Parallel()
	type fields struct {
		File        string
		Directory   string
		GitHubToken string
		Days        int
		DryRun      bool
		Entries     []string
		Update      bool
	}

	type args struct {
		module string
	}

	tests := []struct {
		name     string
		fields   fields
		args     args
		wantType string
		wantErr  bool
	}{
		{"Local paths", fields{}, args{"./testdata"}, "local", false},
		{"Local paths not found", fields{}, args{"./somewhere"}, "local", true},

		{"Terraform Registry", fields{}, args{"jameswoolfenden/http/ip"}, "github", false},
		{"Terraform Registry fail", fields{}, args{"jameswoolfenden/http/ip/duff"}, "", true},

		// I dearly wanted to use that name
		{"Bitbucket", fields{}, args{"bitbucket.org/hashicorp/terraform-consul-aws"}, "bitbucket", false},

		{"Shallow", fields{}, args{"git::https://github.com/terraform-aws-modules/terraform-aws-memory-db.git?depth=1"}, "shallow", false}, //

		{"Mercurial repositories", fields{}, args{"hg::http://example.com/vpc.hg"}, "mercurial", false},
		//
		{"archive", fields{}, args{"https://example.com/vpc-module.zip"}, "archive", false},
		{"archive", fields{}, args{"https://example.com/vpc-module?archive=zip"}, "archive", false},

		{"S3 buckets", fields{}, args{"s3::https://s3-eu-west-1.amazonaws.com/examplecorp-terraform-modules/vpc.zip"}, "s3", false},
		{"GCS buckets", fields{}, args{"gcs::https://www.googleapis.com/storage/v1/modules/foomodule.zip"}, "gcs", false},

		{"Modules in Package Sub-directories", fields{}, args{"hashicorp/consul/aws//modules/consul-cluster"}, "github", false},
		{"Modules 2", fields{}, args{"git::https://example.com/network.git//modules/vpc"}, "git", false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			myFlags := &Flags{
				File:        tt.fields.File,
				Directory:   tt.fields.Directory,
				GitHubToken: tt.fields.GitHubToken,
				Days:        tt.fields.Days,
				DryRun:      tt.fields.DryRun,
				Entries:     tt.fields.Entries,
				Update:      tt.fields.Update,
			}
			got, err := myFlags.GetType(tt.args.module)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetType() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if got != tt.wantType {
				t.Errorf("GetType() got = %v, want %v", got, tt.wantType)
			}
		})
	}
}
