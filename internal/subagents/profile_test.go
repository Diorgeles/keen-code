package subagents

import "testing"

func TestFind(t *testing.T) {
	profile, ok := Find([]Profile{{Name: "explorer"}}, "explorer")
	if !ok {
		t.Fatal("expected profile to be found")
	}
	if profile.Name != "explorer" {
		t.Fatalf("unexpected profile: %+v", profile)
	}

	if _, ok := Find([]Profile{{Name: "explorer"}}, "missing"); ok {
		t.Fatal("expected missing profile not to be found")
	}
}

func TestPermissionToolNamesInheritsAndMapsCapabilities(t *testing.T) {
	inherited := []string{"read_file", "write_file", "delegate_task", "call_mcp_tool"}
	if got := permissionToolNames(Profile{}, inherited); !sameStrings(got, []string{"read_file", "write_file"}) {
		t.Fatalf("expected inherited child tools without special tools, got %v", got)
	}

	profile := Profile{Permissions: []string{"read", "write", "bash", "web"}, PermissionsSet: true}
	want := []string{"read_file", "glob", "grep", "write_file", "edit_file", "bash", "web_fetch"}
	if got := permissionToolNames(profile, inherited); !sameStrings(got, want) {
		t.Fatalf("expected mapped capability tools, got %v", got)
	}
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
