package tailscale

import (
	"log/slog"
	"os"
	"reflect"
	"sort"
	"testing"
)

func TestMergeRoutes(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := NewManager(logger)

	tests := []struct {
		name          string
		currentRoutes []string
		newRoutes     []string
		want          []string
	}{
		{
			name:          "merge with no duplicates",
			currentRoutes: []string{"10.0.0.0/8", "172.16.0.0/12"},
			newRoutes:     []string{"192.168.0.0/16"},
			want:          []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"},
		},
		{
			name:          "merge with duplicates",
			currentRoutes: []string{"10.0.0.0/8", "192.168.0.0/16"},
			newRoutes:     []string{"192.168.0.0/16", "172.16.0.0/12"},
			want:          []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"},
		},
		{
			name:          "empty current routes",
			currentRoutes: []string{},
			newRoutes:     []string{"10.0.0.0/8"},
			want:          []string{"10.0.0.0/8"},
		},
		{
			name:          "empty new routes",
			currentRoutes: []string{"10.0.0.0/8"},
			newRoutes:     []string{},
			want:          []string{"10.0.0.0/8"},
		},
		{
			name:          "both empty",
			currentRoutes: []string{},
			newRoutes:     []string{},
			want:          []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := manager.MergeRoutes(tt.currentRoutes, tt.newRoutes)

			sort.Strings(got)
			sort.Strings(tt.want)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MergeRoutes() = %v, want %v", got, tt.want)
			}
		})
	}
}
