package cli

import "testing"

func TestHooksPathAction(t *testing.T) {
	const ours = "/home/dev/.sourceguard-shield/hooks"

	cases := []struct {
		name      string
		current   string
		wantUnset bool
	}{
		{"matches ours", ours, true},
		{"already unset", "", false},
		{"foreign hooksPath", "/home/dev/project/.husky", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			unset, msg := hooksPathAction(c.current, ours)
			if unset != c.wantUnset {
				t.Errorf("hooksPathAction(%q, %q) unset = %v, want %v", c.current, ours, unset, c.wantUnset)
			}
			if msg == "" {
				t.Errorf("hooksPathAction(%q, %q) returned empty message", c.current, ours)
			}
		})
	}
}
