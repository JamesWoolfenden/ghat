package core

import "testing"

func TestFlags_UpdateHooks(t *testing.T) {
	t.Parallel()
	type fields struct {
		File            string
		Directory       string
		GitHubToken     string
		Days            int
		DryRun          bool
		Entries         []string
		Update          bool
		ContinueOnError bool
	}

	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{name: "Empty", fields: fields{GitHubToken: gitHubToken}, wantErr: true},
		{name: "guff", fields: fields{Directory: "guff", GitHubToken: gitHubToken}, wantErr: true},
		{name: "Pass relative", fields: fields{Directory: "../../", GitHubToken: gitHubToken}, wantErr: false},
		//{name: "Pass absolute", fields: fields{Directory: "E:/Code/pike", GitHubToken: gitHubToken}, wantErr: false},
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
			if err := myFlags.UpdateHooks(); (err != nil) != tt.wantErr {
				t.Errorf("UpdateHooks() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
