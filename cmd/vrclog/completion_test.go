package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestCompleteEventTypes(t *testing.T) {
	tests := []struct {
		name       string
		toComplete string
		flagVals   []string
		want       []string
	}{
		{
			name:       "empty input returns all types",
			toComplete: "",
			flagVals:   nil,
			want:       []string{"player_join", "player_left", "world_join"},
		},
		{
			name:       "prefix pla filters to player types",
			toComplete: "pla",
			flagVals:   nil,
			want:       []string{"player_join", "player_left"},
		},
		{
			name:       "prefix player_j filters to player_join",
			toComplete: "player_j",
			flagVals:   nil,
			want:       []string{"player_join"},
		},
		{
			name:       "prefix wo filters to world_join",
			toComplete: "wo",
			flagVals:   nil,
			want:       []string{"world_join"},
		},
		{
			name:       "comma prefix preserves already typed values",
			toComplete: "player_join,wo",
			flagVals:   nil,
			want:       []string{"player_join,world_join"},
		},
		{
			name:       "excludes already typed values",
			toComplete: "player_join,pl",
			flagVals:   nil,
			want:       []string{"player_join,player_left"},
		},
		{
			name:       "empty after comma returns remaining types",
			toComplete: "player_join,",
			flagVals:   nil,
			want:       []string{"player_join,player_left", "player_join,world_join"},
		},
		{
			name:       "excludes values from flag",
			toComplete: "pl",
			flagVals:   []string{"player_left"},
			want:       []string{"player_join"},
		},
		{
			name:       "case insensitive matching",
			toComplete: "PLA",
			flagVals:   nil,
			want:       []string{"player_join", "player_left"},
		},
		{
			name:       "trims whitespace",
			toComplete: "  pla  ",
			flagVals:   nil,
			want:       []string{"player_join", "player_left"},
		},
		{
			name:       "no match returns empty",
			toComplete: "xyz",
			flagVals:   nil,
			want:       nil,
		},
		{
			name:       "all types used returns empty",
			toComplete: "player_join,player_left,world_join,",
			flagVals:   nil,
			want:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh command with the flag for each test
			cmd := &cobra.Command{}
			cmd.Flags().StringSlice("include-types", nil, "")

			// Set flag values if provided
			if tt.flagVals != nil {
				if err := cmd.Flags().Set("include-types", strings.Join(tt.flagVals, ",")); err != nil {
					t.Fatalf("failed to set flag: %v", err)
				}
			}

			complete := completeEventTypes("include-types")
			got, dir := complete(cmd, nil, tt.toComplete)

			// Check directive
			expectedDir := cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
			if dir != expectedDir {
				t.Errorf("directive = %v, want %v", dir, expectedDir)
			}

			// Check candidates
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("candidates = %v, want %v", got, tt.want)
			}
		})
	}
}
