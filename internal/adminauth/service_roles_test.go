package adminauth

import "testing"

func TestSessionProjectPermissions(t *testing.T) {
	owner := &Session{Role: RoleOwner, ProjectIDs: []string{"project_a", "project_b"}}
	if !owner.HasProjectAccess("project_b") || !owner.CanManageProject("project_b") {
		t.Fatal("owner must manage every loaded active project")
	}

	projectAdmin := &Session{
		Role:       RoleMember,
		ProjectIDs: []string{"project_a"},
		Projects:   []Project{{ID: "project_a", Role: ProjectAdminRole}},
	}
	if !projectAdmin.CanManageProject("project_a") {
		t.Fatal("project admin must manage its assigned project")
	}
	if projectAdmin.HasProjectAccess("project_b") || projectAdmin.CanManageProject("project_b") {
		t.Fatal("project admin must not access another project")
	}

	viewer := &Session{
		Role:       RoleMember,
		ProjectIDs: []string{"project_a"},
		Projects:   []Project{{ID: "project_a", Role: ViewerRole}},
	}
	if !viewer.HasProjectAccess("project_a") {
		t.Fatal("viewer must retain analytics access to its assigned project")
	}
	if viewer.CanManageProject("project_a") {
		t.Fatal("viewer must not manage project settings or members")
	}
}
