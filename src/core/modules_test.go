package core

import (
	"fmt"
	"reflect"
	"testing"
)

func TestFlags_GetType(t *testing.T) {
	t.Parallel()
	type fields struct {
		File        string
		Directory   string
		GitHubToken string
		Days        *uint
		DryRun      bool
		Entries     []string
		Update      bool
	}

	type args struct {
		module string
	}

	//goland:noinspection HttpUrlsUsage
	tests := []struct {
		name     string
		fields   fields
		args     args
		wantType string
		wantErr  bool
	}{
		{"Local paths", fields{}, args{"./testdata"}, "local", false},
		{"Local paths not found", fields{}, args{"./somewhere"}, "local", true},

		{"Terraform Registry", fields{}, args{"jameswoolfenden/http/ip"}, "registry", false},
		{"Terraform Registry fail", fields{}, args{"jameswoolfenden/http/ip/duff"}, "local", true},
		{"github", fields{}, args{"github.com/jameswoolfenden/terraform-http-ip"}, "github", false},

		{"git", fields{}, args{"git::https://github.com/terraform-aws-modules/terraform-aws-memory-db.git"}, "git", false},
		{"git query string", fields{}, args{"git::https://github.com/terraform-aws-modules/terraform-aws-memory-db.git"}, "git", false},
		{"git query string", fields{}, args{"git::ssh://github.com/terraform-aws-modules/terraform-aws-memory-db"}, "git", false},

		// I dearly wanted to use that name
		{"Bitbucket", fields{}, args{"bitbucket.org/hashicorp/terraform-consul-aws"}, "bitbucket", false},

		{"Shallow", fields{}, args{"git::https://github.com/terraform-aws-modules/terraform-aws-memory-db.git?depth=1"}, "shallow", false}, //

		{"Mercurial repositories", fields{}, args{"hg::http://example.com/vpc.hg"}, "mercurial", false},
		//
		{"archive", fields{}, args{"https://example.com/vpc-module.zip"}, "archive", false},
		{"archive", fields{}, args{"https://example.com/vpc-module?archive=zip"}, "archive", false},

		{"S3 buckets", fields{}, args{"s3::https://s3-eu-west-1.amazonaws.com/examplecorp-terraform-modules/vpc.zip"}, "s3", false},
		{"GCS buckets", fields{}, args{"gcs::https://www.googleapis.com/storage/v1/modules/foomodule.zip"}, "gcs", false},

		{"Modules in Package Sub-directories", fields{}, args{"hashicorp/consul/aws//modules/consul-cluster"}, "registry", false},
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

func TestFlags_UpdateSource(t *testing.T) {
	type fields struct {
		File        string
		Directory   string
		GitHubToken string
		Days        *uint
		DryRun      bool
		Entries     []string
		Update      bool
	}

	var days uint = 0

	type args struct {
		module     string
		moduleType string
		version    string
	}
	//goland:noinspection HttpUrlsUsage
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		want1   string
		wantErr bool
	}{
		{"Local paths", fields{}, args{"./testdata", "local", ""}, "./testdata", "", false},
		{"Local paths not found", fields{}, args{"./somewhere", "local", ""}, "./somewhere", "", false},

		{"github",
			fields{"", "", gitHubToken, &days, false, nil, true},
			args{"github.com/hashicorp/terraform-aws-consul", "github", ""},
			"git::https://github.com/hashicorp/terraform-aws-consul.git?ref=e9ceb573687c3d28516c9e3714caca84db64a766",
			"v0.11.0",
			false},
		{"Terraform Registry fail",
			fields{},
			args{"jameswoolfenden/http/ip/duff", "registry", ""},
			"",
			"",
			true},
		{"git",
			fields{"", "", gitHubToken, &days, false, nil, false},
			args{"git::https://github.com/terraform-aws-modules/terraform-aws-memory-db.git", "git", ""},
			"git::https://github.com/terraform-aws-modules/terraform-aws-memory-db.git?ref=b67d63d7f4b1cd7886909fa5c5c5893ecad55734",
			"v3.0.0", false},
		{"git update",
			fields{"", "", gitHubToken, &days, false, nil, true},
			args{"git::https://github.com/terraform-aws-modules/terraform-aws-memory-db.git", "git", ""},
			"git::https://github.com/terraform-aws-modules/terraform-aws-memory-db.git?ref=b67d63d7f4b1cd7886909fa5c5c5893ecad55734",
			"v3.0.0", false},
		{"git version",
			fields{"", "", gitHubToken, &days, false, nil, false},
			args{"git::https://github.com/terraform-aws-modules/terraform-aws-memory-db.git?ref=v1.0.0", "git", ""},
			"git::https://github.com/terraform-aws-modules/terraform-aws-memory-db.git?ref=c1a0698ae1ae4ced03399809ef3e0253b07c44a9",
			"v1.0.0", false},
		{"git version update",
			fields{"", "", gitHubToken, &days, false, nil, true},
			args{"git::https://github.com/terraform-aws-modules/terraform-aws-memory-db.git?ref=v1.0.0", "git", ""},
			"git::https://github.com/terraform-aws-modules/terraform-aws-memory-db.git?ref=b67d63d7f4b1cd7886909fa5c5c5893ecad55734",
			"v3.0.0", false},
		{"git version missing",
			fields{"", "", gitHubToken, &days, false, nil, false},
			args{"git::https://github.com/terraform-aws-modules/terraform-aws-memory-db.git?ref=v1.2.0", "git", ""},
			"", "", true},
		{"git hash",
			fields{"", "", gitHubToken, &days, false, nil, false},
			args{"git::https://github.com/terraform-aws-modules/terraform-aws-memory-db.git?ref=c6d56c1", "git", ""},
			"git::https://github.com/terraform-aws-modules/terraform-aws-memory-db.git?ref=c6d56c1", "c6d56c1", false},
		{name: "git hash update",
			fields:  fields{"", "", gitHubToken, &days, false, nil, true},
			args:    args{"git::https://github.com/terraform-aws-modules/terraform-aws-memory-db.git?ref=93facd14e9e3a66704d84a0236a8a3b813f047be", "git", ""},
			want:    "git::https://github.com/terraform-aws-modules/terraform-aws-memory-db.git?ref=b67d63d7f4b1cd7886909fa5c5c5893ecad55734",
			want1:   "v3.0.0",
			wantErr: false},

		//{"git query string", fields{}, args{"git::https://github.com/terraform-aws-modules/terraform-aws-memory-db.git"}, "git", false},
		//{"git query string", fields{}, args{"git::ssh://github.com/terraform-aws-modules/terraform-aws-memory-db.git"}, "git", false},
		//
		// I dearly wanted to use that name
		{"Bitbucket", fields{}, args{"bitbucket.org/hashicorp/terraform-consul-aws", "bitbucket", ""},
			"",
			"",
			false},

		{"Shallow", fields{}, args{"git::https://github.com/terraform-aws-modules/terraform-aws-memory-db.git?depth=1", "shallow", ""},
			"git::https://github.com/terraform-aws-modules/terraform-aws-memory-db.git?depth=1",
			"",
			false}, //

		{"Mercurial repositories", fields{}, args{"hg::http://example.com/vpc.hg", "mercurial", ""},
			"hg::http://example.com/vpc.hg",
			"",
			false},

		{"archive", fields{}, args{"https://example.com/vpc-module.zip", "archive", ""},
			"https://example.com/vpc-module.zip",
			"",
			false},
		{"archive", fields{}, args{"https://example.com/vpc-module?archive=zip", "archive", ""},
			"https://example.com/vpc-module?archive=zip",
			"",
			false},

		{"S3 buckets", fields{}, args{"s3::https://s3-eu-west-1.amazonaws.com/examplecorp-terraform-modules/vpc.zip", "s3", ""},
			"s3::https://s3-eu-west-1.amazonaws.com/examplecorp-terraform-modules/vpc.zip",
			"",
			false},
		{"GCS buckets", fields{}, args{"gcs::https://www.googleapis.com/storage/v1/modules/foomodule.zip", "gcs", ""},
			"gcs::https://www.googleapis.com/storage/v1/modules/foomodule.zip",
			"",
			false},
		{"subdir registry",
			fields{"", "", gitHubToken, &days, false, nil, true},
			args{"hashicorp/consul/aws//modules/consul-cluster", "registry", ""},
			"git::https://github.com/hashicorp/terraform-aws-consul.git//modules/consul-cluster?ref=e9ceb573687c3d28516c9e3714caca84db64a766",
			"v0.11.0",
			false},
		{"subdir github",
			fields{"", "", gitHubToken, &days, false, nil, true},
			args{"github.com/hashicorp/terraform-aws-consul//modules/consul-cluster", "github", ""},
			"git::https://github.com/hashicorp/terraform-aws-consul.git//modules/consul-cluster?ref=e9ceb573687c3d28516c9e3714caca84db64a766",
			"v0.11.0",
			false},
		//{"Modules 2", fields{}, args{"git::https://example.com/network.git//modules/vpc", "git", ""},
		//	"git::https://example.com/network.git//modules/vpc",
		//	"",
		//	false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			myFlags := &Flags{
				File:        tt.fields.File,
				Directory:   tt.fields.Directory,
				GitHubToken: tt.fields.GitHubToken,
				Days:        tt.fields.Days,
				DryRun:      tt.fields.DryRun,
				Entries:     tt.fields.Entries,
				Update:      tt.fields.Update,
			}
			got, got1, err := myFlags.UpdateSource(tt.args.module, tt.args.moduleType, tt.args.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateSource() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("UpdateSource() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("UpdateSource() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestFlags_UpdateGithubSource(t *testing.T) {
	t.Parallel()
	type fields struct {
		File            string
		Directory       string
		GitHubToken     string
		Days            *uint
		DryRun          bool
		Entries         []string
		Update          bool
		ContinueOnError bool
	}

	type args struct {
		version   string
		newModule string
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		want1   string
		wantErr bool
	}{
		{"Pass update", fields{Update: true, GitHubToken: gitHubToken}, args{newModule: "github.com/jameswoolfenden/terraform-http-ip.git"},
			"git::https://github.com/jameswoolfenden/terraform-http-ip.git?ref=2f3cef24e667fb840a3d3481f5a1aaa5a1ac7d28",
			"v0.3.14", false},
		{"Not action", fields{Update: true}, args{newModule: "github.com/jameswoolfenden/ip.git"}, "", "", true},
		{"Fail no .git", fields{Update: true}, args{newModule: "jameswoolfenden/ip"}, "", "", true},
		{"Fail too short", fields{Update: true}, args{newModule: "jameswoolfenden/ip"}, "", "", true},
		{"Pass", fields{Update: false, GitHubToken: gitHubToken}, args{newModule: "github.com/jameswoolfenden/terraform-http-ip.git"},
			"git::https://github.com/jameswoolfenden/terraform-http-ip.git?ref=2f3cef24e667fb840a3d3481f5a1aaa5a1ac7d28",
			"v0.3.14", false},
		{"Pass with version",
			fields{Update: false, GitHubToken: gitHubToken}, args{version: "81a0a7c", newModule: "github.com/jameswoolfenden/terraform-http-ip.git"},
			"git::https://github.com/jameswoolfenden/terraform-http-ip.git?ref=81a0a7c",
			"81a0a7c", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			myFlags := &Flags{
				File:            tt.fields.File,
				Directory:       tt.fields.Directory,
				GitHubToken:     tt.fields.GitHubToken,
				Days:            tt.fields.Days,
				DryRun:          tt.fields.DryRun,
				Entries:         tt.fields.Entries,
				Update:          tt.fields.Update,
				ContinueOnError: tt.fields.ContinueOnError,
			}
			got, got1, err := myFlags.UpdateGithubSource(tt.args.version, tt.args.newModule)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateGithubSource() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("UpdateGithubSource() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("UpdateGithubSource() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestFlags_UpdateModule(t *testing.T) {
	type fields struct {
		File            string
		Directory       string
		GitHubToken     string
		Days            *uint
		DryRun          bool
		Entries         []string
		Update          bool
		ContinueOnError bool
	}
	type args struct {
		file string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"add version", fields{Update: true}, args{"testdata/modules/github-git/module.tf"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			myFlags := &Flags{
				File:            tt.fields.File,
				Directory:       tt.fields.Directory,
				GitHubToken:     tt.fields.GitHubToken,
				Days:            tt.fields.Days,
				DryRun:          tt.fields.DryRun,
				Entries:         tt.fields.Entries,
				Update:          tt.fields.Update,
				ContinueOnError: tt.fields.ContinueOnError,
			}
			if err := myFlags.UpdateModule(tt.args.file); (err != nil) != tt.wantErr {
				t.Errorf("UpdateModule() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCustomErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "URL Join Error",
			err:      &urlJoinError{fmt.Errorf("invalid path")},
			expected: "failed to join url: invalid path",
		},
		{
			name:     "Empty Module Error",
			err:      &moduleEmptyError{},
			expected: "module name cannot be empty",
		},
		{
			name:     "Empty URL Error",
			err:      &emptyURL{},
			expected: "URL is empty",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.err.Error() != tt.expected {
				t.Errorf("Error() = %v, want %v", tt.err.Error(), tt.expected)
			}
		})
	}
}

func TestRegistry_GetLatest_EdgeCases(t *testing.T) {
	t.Parallel()

	type fields struct {
		Registry      bool
		LatestVersion string
	}

	tests := []struct {
		name    string
		fields  fields
		module  string
		want    *string
		wantErr bool
	}{
		{
			name:    "Empty Module",
			fields:  fields{false, ""},
			module:  "",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Module With Special Characters",
			fields:  fields{false, ""},
			module:  "test/module/with spaces/and#special@chars",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			myRegistry := &Registry{
				Registry:      tt.fields.Registry,
				LatestVersion: tt.fields.LatestVersion,
			}
			got, err := myRegistry.GetLatest(tt.module)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetLatest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetLatest() got = %v, want %v", got, tt.want)
			}
		})
	}
}
