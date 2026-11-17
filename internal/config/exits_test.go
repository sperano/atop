package config

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

const cappedExitsEnv = currentEnvPrefix + "CAPPED_EXITS"

func wantDefaultExits() ExitList {
	return ExitList{{Container: "review-round-cap", Code: 4}}
}

func TestParseCappedExits(t *testing.T) {
	t.Parallel()
	envExits := map[string]string{cappedExitsEnv: "wait-for-blockers:3"}
	tests := []struct {
		name string
		args []string
		env  map[string]string
		want ExitList
	}{
		{"default", nil, nil, wantDefaultExits()},
		{"env replaces the default", nil, envExits, ExitList{{Container: "wait-for-blockers", Code: 3}}},
		{"flag beats env", []string{"-capped-exits", "a:1,b:2"}, envExits,
			ExitList{{Container: "a", Code: 1}, {Container: "b", Code: 2}}},
		{"empty flag turns capping off", []string{"-capped-exits="}, nil, nil},
		{"serve has the flag", []string{ServeCommand, "-capped-exits", "a:1"}, nil, ExitList{{Container: "a", Code: 1}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Parse(testName, tt.args, envFrom(tt.env))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.CappedExits, tt.want) {
				t.Errorf("CappedExits = %v, want %v", got.CappedExits, tt.want)
			}
		})
	}
}

func TestParseCappedExitsErrors(t *testing.T) {
	t.Parallel()
	_, err := Parse(testName, nil, envFrom(map[string]string{cappedExitsEnv: "cap:0"}))
	if !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), cappedExitsEnv+`: "cap:0" is not`) {
		t.Errorf("bad env: err = %v, want ErrInvalid naming %s", err, cappedExitsEnv)
	}
	_, err = Parse(testName, []string{"-capped-exits", "cap"}, envFrom(nil))
	if err == nil || errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), `-capped-exits: "cap" is not`) {
		t.Errorf("bad flag: err = %v, want the flag package's error", err)
	}
}

func TestExitListString(t *testing.T) {
	t.Parallel()
	l := ExitList{{Container: "a", Code: 1}, {Container: "b", Code: 2}}
	if got, want := l.String(), "a:1,b:2"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
