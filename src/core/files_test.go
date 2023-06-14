package core

import (
	"os"
	"reflect"
	"testing"
)

func TestFiles(t *testing.T) {
	type args struct {
		directory *string
	}
	tests := []struct {
		name    string
		args    args
		want    []os.DirEntry
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Files(tt.args.directory)
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
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := GetGHA(tt.args.directory, tt.args.matches, tt.args.ghat)
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
		t.Run(tt.name, func(t *testing.T) {
			if err := UpdateFile(tt.args.file); (err != nil) != tt.wantErr {
				t.Errorf("UpdateFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_getPayload(t *testing.T) {
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
		t.Run(tt.name, func(t *testing.T) {
			got, err := getPayload(tt.args.action)
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
