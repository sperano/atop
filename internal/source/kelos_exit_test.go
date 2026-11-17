package source

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseContainerExits(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		in      string
		want    []ContainerExit
		wantErr string
	}{
		{"empty is no exits", "", nil, ""},
		{"one", "review-round-cap:4", []ContainerExit{{"review-round-cap", 4}}, ""},
		{"several, spaced, blank entries skipped", " review-round-cap:4, ,wait-for-blockers:3,",
			[]ContainerExit{{"review-round-cap", 4}, {"wait-for-blockers", 3}}, ""},
		{"largest code", "a:255", []ContainerExit{{"a", 255}}, ""},
		{"no code", "review-round-cap", nil, `"review-round-cap" is not <container>:<exit code 1-255>`},
		{"empty name", ":4", nil, `":4" is not`},
		{"upper-case name", "Cap:4", nil, `"Cap:4" is not`},
		{"name ending in a dash", "cap-:4", nil, `"cap-:4" is not`},
		{"zero code", "cap:0", nil, `"cap:0" is not`},
		{"code above 255", "cap:256", nil, `"cap:256" is not`},
		{"non-numeric code", "cap:x", nil, `"cap:x" is not`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseContainerExits(tt.in)
			if tt.wantErr != "" {
				if err == nil || !strings.HasPrefix(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want prefix %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, %v; want %v", got, err, tt.want)
			}
		})
	}
}

func TestFormatContainerExits(t *testing.T) {
	t.Parallel()
	exits := []ContainerExit{{"review-round-cap", 4}, {"wait-for-blockers", 3}}
	if got, want := FormatContainerExits(exits), "review-round-cap:4,wait-for-blockers:3"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := FormatContainerExits(nil); got != "" {
		t.Errorf("no exits = %q, want empty", got)
	}
}

func TestContainerExitString(t *testing.T) {
	t.Parallel()
	if got, want := (ContainerExit{"kelos-agent", 1}).String(), "kelos-agent exit 1"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
