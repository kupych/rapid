package providers

import "testing"

func TestVariablesSkipsReserved(t *testing.T) {
	p := &Variables{Vars: map[string]interface{}{
		"token":           "abc",
		"$$auth":          "secret",
		"$$header:X-Test": "yes",
	}}

	tests := []struct {
		name string
		ctx  Context
		want string
	}{
		{"variable ref", Context{Kind: KindVariableRef, Prefix: ""}, "token}"},
		{"path", Context{Kind: KindPath, Prefix: ""}, "${token}"},
		{"argument", Context{Kind: KindArgument, Prefix: ""}, "${token}"},
		{"command", Context{Kind: KindCommand, Prefix: "t"}, "token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.GetSuggestions(tt.ctx)
			if len(got) != 1 || got[0].Text != tt.want {
				t.Fatalf("got %v, want single suggestion %q", got, tt.want)
			}
		})
	}
}

func TestVariablesSkipsReservedWithDollarPrefix(t *testing.T) {
	p := &Variables{Vars: map[string]interface{}{"$$auth": "secret"}}

	if got := p.GetSuggestions(Context{Kind: KindCommand, Prefix: "$$"}); len(got) != 0 {
		t.Errorf("reserved var suggested for explicit $$ prefix: %v", got)
	}
}
