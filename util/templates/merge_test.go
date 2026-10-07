package templates

import (
	"bytes"
	"maps"
	"testing"

	"github.com/evcc-io/evcc/util"
	"github.com/stretchr/testify/require"
)

func TestMergeMaps(t *testing.T) {
	target := map[string]any{
		"foo": "bar",
		"nested": map[string]any{
			"bar": "baz",
		},
	}
	other := map[string]any{
		"Foo": 1,
		"Nested": map[string]any{
			"Bar": 2,
		},
		"baz": 3,
	}

	require.NoError(t, mergeMaps(other, target))
	require.Equal(t, map[string]any{
		"foo": 1,
		"nested": map[string]any{
			"bar": 2,
		},
		"baz": 3,
	}, target)
}

func TestMergeMapsDuplicateKeys(t *testing.T) {
	tests := []struct {
		name     string
		target   map[string]any
		other    map[string]any
		want     map[string]any
		selected string
		ignored  string
	}{
		{
			name:     "secret values",
			target:   map[string]any{"password": ""},
			other:    map[string]any{"Password": "ignored-secret", "password": "selected-secret"},
			want:     map[string]any{"password": "selected-secret"},
			selected: "password",
			ignored:  "Password",
		},
		{
			name:     "canonical camel case",
			target:   map[string]any{"maxCurrent": 0},
			other:    map[string]any{"maxcurrent": 32, "maxCurrent": 16},
			want:     map[string]any{"maxCurrent": 16},
			selected: "maxCurrent",
			ignored:  "maxcurrent",
		},
		{
			name:     "canonical lowercase",
			target:   map[string]any{"maxcurrent": 0},
			other:    map[string]any{"maxcurrent": 32, "maxCurrent": 16},
			want:     map[string]any{"maxcurrent": 32},
			selected: "maxcurrent",
			ignored:  "maxCurrent",
		},
		{
			name:     "equal values",
			target:   map[string]any{"minCurrent": 0},
			other:    map[string]any{"mincurrent": 6, "minCurrent": 6},
			want:     map[string]any{"minCurrent": 6},
			selected: "minCurrent",
			ignored:  "mincurrent",
		},
		{
			name:     "canonical zero",
			target:   map[string]any{"maxCurrent": 6},
			other:    map[string]any{"maxcurrent": 32, "maxCurrent": 0},
			want:     map[string]any{"maxCurrent": 0},
			selected: "maxCurrent",
			ignored:  "maxcurrent",
		},
		{
			name:     "canonical nil",
			target:   map[string]any{"maxCurrent": 6},
			other:    map[string]any{"maxcurrent": 32, "maxCurrent": nil},
			want:     map[string]any{"maxCurrent": nil},
			selected: "maxCurrent",
			ignored:  "maxcurrent",
		},
		{
			name:     "no canonical source key",
			target:   map[string]any{"maxCurrent": 0},
			other:    map[string]any{"MAXCURRENT": 16, "maxcurrent": 32},
			want:     map[string]any{"maxCurrent": 16},
			selected: "MAXCURRENT",
			ignored:  "maxcurrent",
		},
		{
			name:     "no target key",
			target:   map[string]any{},
			other:    map[string]any{"MAXCURRENT": 16, "maxcurrent": 32},
			want:     map[string]any{"MAXCURRENT": 16},
			selected: "MAXCURRENT",
			ignored:  "maxcurrent",
		},
		{
			name:     "nested canonical",
			target:   map[string]any{"nested": map[string]any{"maxCurrent": 0, "keep": true}},
			other:    map[string]any{"Nested": map[string]any{"maxcurrent": 32, "maxCurrent": 16}},
			want:     map[string]any{"nested": map[string]any{"maxCurrent": 16, "keep": true}},
			selected: "nested.maxCurrent",
			ignored:  "nested.maxcurrent",
		},
		{
			name:     "new nested map",
			target:   map[string]any{},
			other:    map[string]any{"nested": map[string]int{"maxcurrent": 32, "maxCurrent": 16}},
			want:     map[string]any{"nested": map[string]any{"maxCurrent": 16}},
			selected: "nested.maxCurrent",
			ignored:  "nested.maxcurrent",
		},
		{
			name:     "duplicate maps do not combine",
			target:   map[string]any{"nested": map[string]any{"keep": true}},
			other:    map[string]any{"Nested": map[string]any{"ignored": 32}, "nested": map[string]any{"selected": 16}},
			want:     map[string]any{"nested": map[string]any{"keep": true, "selected": 16}},
			selected: "nested",
			ignored:  "Nested",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := util.NewLogger("templates")
			var warnings bytes.Buffer
			output := log.WARN.Writer()
			log.WARN.SetOutput(&warnings)
			t.Cleanup(func() { log.WARN.SetOutput(output) })

			original := maps.Clone(tt.other)
			for range 100 {
				target := maps.Clone(tt.target)
				warnings.Reset()
				require.NoError(t, mergeMaps(tt.other, target))
				require.Equal(t, tt.want, target)
				require.Contains(t, warnings.String(), `using "`+tt.selected+`"`)
				require.Contains(t, warnings.String(), `remove "`+tt.ignored+`"`)
				require.NotContains(t, warnings.String(), "ignored-secret")
				require.NotContains(t, warnings.String(), "selected-secret")
				require.Equal(t, original, tt.other)
			}
		})
	}
}

func TestMergeMapsSingleAlias(t *testing.T) {
	log := util.NewLogger("templates")
	var warnings bytes.Buffer
	output := log.WARN.Writer()
	log.WARN.SetOutput(&warnings)
	t.Cleanup(func() { log.WARN.SetOutput(output) })

	target := map[string]any{"maxCurrent": 6}
	require.NoError(t, mergeMaps(map[string]any{"maxcurrent": 16}, target))
	require.Equal(t, map[string]any{"maxCurrent": 16}, target)
	require.Empty(t, warnings.String())
}
