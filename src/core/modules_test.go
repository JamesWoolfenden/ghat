package core

import "testing"

func TestFlags_GetType(t *testing.T) {
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
		name    string
		fields  fields
		args    args
		want    string
		want1   string
		wantErr bool
	}{
		{"Local paths", fields{}, args{"./testdata"}, "./testdata", "local", false},
		{"Local paths not found", fields{}, args{"./somewhere"}, "./somewhere", "", true},
		//{"Terraform Registry",""},
		//
		//{"GitHub",""},
		//
		//{"Bitbucket"""},
		//
		//{"Generic Git, Mercurial repositories",""},
		//
		//{"HTTP URLs",""},
		//
		//{"S3 buckets",""},
		//
		//{"GCS buckets",""},
		//
		//{"Modules in Package Sub-directories",""},
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
			got, got1, err := myFlags.GetType(tt.args.module)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetType() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("GetType() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
