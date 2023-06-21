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
		days      *int
	}

	dir := "testdata/files/"
	bogus := "testdata/bogus/"
	empty := "testdata/empty"

	var zero = 0

	tests := []struct {
		name    string
		args    args
		want    []os.DirEntry
		wantErr bool
	}{
		{"Pass", args{&dir, &zero}, nil, false},
		{"Bogus", args{&bogus, &zero}, nil, false},
		{"Empty", args{&empty, &zero}, nil, false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Files(tt.args.directory, gitHubToken, tt.args.days)
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

	duffDir := "nothere"
	nomatches, _ := os.ReadDir(duffDir)

	noworkflowsdir := "./testdata/noworkflows"
	noworkflows, _ := os.ReadDir(noworkflowsdir)

	noworkflowswithdir := "./testdata/noworkflowswithdir"
	noworkflowswithdircontents, _ := os.ReadDir(noworkflowswithdir)

	tests := []struct {
		name    string
		args    args
		want    []os.DirEntry
		want1   *string
		wantErr bool
	}{
		{"no matches", args{&duffDir, nomatches, nil}, nil, &duffDir, false},
		{"no workflows", args{&noworkflowsdir, noworkflows, nil}, nil, &noworkflowsdir, false},
		{"no workflows with dir", args{&noworkflowswithdir, noworkflowswithdircontents, nil}, nil, &noworkflowswithdir, false},
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

func TestGetBody(t *testing.T) {
	type args struct {
		gitHubToken string
		url         string
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetBody(tt.args.gitHubToken, tt.args.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetBody() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetBody() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateFile(t *testing.T) {
	type args struct {
		file        *string
		gitHubToken string
		days        *int
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := UpdateFile(tt.args.file, tt.args.gitHubToken, tt.args.days); (err != nil) != tt.wantErr {
				t.Errorf("UpdateFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_getHash(t *testing.T) {
	type args struct {
		action      string
		tag         string
		gitHubToken string
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getHash(tt.args.action, tt.args.tag, tt.args.gitHubToken)
			if (err != nil) != tt.wantErr {
				t.Errorf("getHash() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getHash() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getPayload(t *testing.T) {
	t.Parallel()

	type args struct {
		action      string
		gitHubToken string
		days        *int
	}

	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := getPayload(tt.args.action, tt.args.gitHubToken, tt.args.days)
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
