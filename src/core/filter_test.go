package core

import "testing"

func TestGetReleases(t *testing.T) {
	t.Parallel()

	type args struct {
		action      string
		gitHubToken string
	}

	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{"Pass", args{"jameswoolfenden/pike", ""}, "", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := GetReleases(tt.args.action, tt.args.gitHubToken, 14)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetReleases() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetReleases() got = %v, want %v", got, tt.want)
			}
		})
	}
}
