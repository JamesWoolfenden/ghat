package core

import (
	"os"
	"reflect"
	"testing"
)

var gitHubToken = os.Getenv("GITHUB_TOKEN")

func TestFiles(t *testing.T) {
	t.Parallel()

	type args struct {
		directory *string
	}

	dir := "testdata/files/"
	bogus := "testdata/bogus/"
	empty := "testdata/empty"

	tests := []struct {
		name    string
		args    args
		want    []os.DirEntry
		wantErr bool
	}{
		{"Pass", args{&dir}, nil, false},
		{"Bogus", args{&bogus}, nil, false},
		{"Empty", args{&empty}, nil, false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Files(tt.args.directory, gitHubToken)
			if (err != nil) != tt.wantErr {
				t.Errorf("Files() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Files() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetGHA(t *testing.T) {
	t.Parallel()

	type args struct {
		directory *string
		matches   []os.DirEntry
		ghat      []os.DirEntry
	}

	tests := []struct {
		name    string
		args    args
		want    []os.DirEntry
		want1   *string
		wantErr bool
	}{
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := GetGHA(tt.args.directory, tt.args.matches, tt.args.ghat)
			t.Parallel()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetGHA() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetGHA() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("GetGHA() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestUpdateFile(t *testing.T) {
	t.Parallel()

	type args struct {
		file *string
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := UpdateFile(tt.args.file, gitHubToken); (err != nil) != tt.wantErr {
				t.Errorf("UpdateFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_getPayload(t *testing.T) {
	t.Parallel()

	type args struct {
		action string
	}

	tests := []struct {
		name    string
		args    args
		want    map[string]interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := getPayload(tt.args.action, gitHubToken)
			if (err != nil) != tt.wantErr {
				t.Errorf("getPayload() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getPayload() got = %v, want %v", got, tt.want)
			}
		})
	}
}
