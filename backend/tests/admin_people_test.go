package tests

// Accounts, groups and project roles, as global administrators manage them,
// and the profile people change themselves.

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"solstice/internal/jirafixture"
)

type adminUser struct {
	ID          int64
	Username    string
	Name        string
	Email       string
	Active      bool
	Admin       bool
	HasPassword bool
	Groups      []idName
	Tokens      *int
}

// newAccount creates an active account with a password, and returns its id
// and user name.
func newAccount(t *testing.T, admin *client, password string) (int64, string) {
	t.Helper()
	username := unique("compte.")
	var created struct{ ID int64 }
	admin.call("POST", "/api/admin/users", map[string]any{"username": username, "name": "Compte " + username,
		"email": username + "@exemple.fr", "password": password}, 201, &created)
	return created.ID, username
}

func TestAdminUsers(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", jirafixture.Password)
	username := unique("nouveau.")
	for _, bad := range []struct {
		in   map[string]any
		want int
	}{
		{map[string]any{"username": "Pas Valide", "password": "une phrase assez longue"}, 400},
		{map[string]any{"username": username, "password": "court"}, 400},
		{map[string]any{"username": username, "email": "pas-une-adresse", "password": "une phrase assez longue"}, 400},
		{map[string]any{"username": username, "password": "une phrase assez longue", "groups": []int64{999999}}, 400},
		{map[string]any{"username": "paul.martin", "password": "une phrase assez longue"}, 409},
	} {
		if status, body := admin.do("POST", "/api/admin/users", bad.in); status != bad.want {
			t.Errorf("create %v: %d %s, want %d", bad.in, status, body, bad.want)
		}
	}
	paul.call("POST", "/api/admin/users", map[string]any{"username": username, "password": "une phrase assez longue"}, 403, nil)

	var created struct{ ID int64 }
	admin.call("POST", "/api/admin/users", map[string]any{"username": strings.ToUpper(username), "name": "Nouvelle Recrue",
		"email": username + "@exemple.fr", "password": "une phrase assez longue",
		"groups": []int64{groupID(t, "developpeurs")}}, 201, &created)
	path := fmt.Sprintf("/api/admin/users/%d", created.ID)
	// Other tests count the members of rh and developpeurs.
	t.Cleanup(func() { admin.do("PUT", path+"/groups", map[string]any{"groups": []int64{}}) })
	var list []adminUser
	admin.call("GET", "/api/admin/users?q="+username, nil, 200, &list)
	if len(list) != 1 || list[0].Username != username || !list[0].Active || list[0].Admin || !list[0].HasPassword ||
		len(list[0].Groups) != 1 || list[0].Groups[0].Name != "developpeurs" {
		t.Fatalf("listed: %+v", list)
	}
	paul.call("GET", "/api/admin/users", nil, 403, nil)

	// The new account signs in and works where its group works.
	key := newProject(t, admin, nil) // open to developpeurs
	recruit := login(t, username, "une phrase assez longue")
	if status := recruit.get("/api/projects/"+key, nil); status != 200 {
		t.Errorf("a new developer GET %s: %d", key, status)
	}
	if status := recruit.get("/api/admin/users", nil); status != 403 {
		t.Errorf("a new account lists users: %d", status)
	}

	// Groups replaced.
	var u adminUser
	admin.call("PUT", path+"/groups", map[string]any{"groups": []int64{groupID(t, "rh")}}, 200, &u)
	if len(u.Groups) != 1 || u.Groups[0].Name != "rh" {
		t.Errorf("groups after PUT: %+v", u.Groups)
	}
	admin.call("PUT", path+"/groups", map[string]any{"groups": []int64{999999}}, 400, nil)
	if status := recruit.get("/api/projects/"+key, nil); status != 404 {
		t.Errorf("GET %s after leaving developpeurs: %d", key, status)
	}

	// A reset password: shown once, the sessions end.
	var reset struct{ Password string }
	admin.call("POST", path+"/reset-password", nil, 200, &reset)
	if len(reset.Password) < 16 {
		t.Errorf("generated password %q", reset.Password)
	}
	if status := recruit.get("/api/me", nil); status != 401 {
		t.Errorf("session after a reset: %d", status)
	}
	recruit = login(t, username, reset.Password)

	// Changes; then deactivated, signed out, and refused at sign-in.
	admin.call("PATCH", path, map[string]any{"email": "pas-une-adresse"}, 400, nil)
	admin.call("PATCH", path, map[string]any{"password": "court"}, 400, nil)
	admin.call("PATCH", path, map[string]any{"name": "Recrue Renommée", "admin": true}, 204, nil)
	admin.call("GET", path, nil, 200, &u)
	if u.Name != "Recrue Renommée" || !u.Admin || u.Tokens == nil || *u.Tokens != 0 {
		t.Errorf("after PATCH: %+v", u)
	}
	admin.call("PATCH", path, map[string]any{"active": false, "admin": false}, 204, nil)
	if status := recruit.get("/api/me", nil); status != 401 {
		t.Errorf("session of a deactivated account: %d", status)
	}
	if status, _ := anonymousAt(t, ownServer(t)).do("POST", "/api/login",
		map[string]string{"username": username, "password": reset.Password}); status != 401 {
		t.Errorf("a deactivated account signed in: %d", status)
	}
	admin.call("GET", "/api/admin/users/999999999", nil, 404, nil)

	// Administrators cannot lock themselves out.
	me := userID(t, "admin")
	admin.call("PATCH", fmt.Sprintf("/api/admin/users/%d", me), map[string]any{"admin": false}, 400, nil)
	admin.call("PATCH", fmt.Sprintf("/api/admin/users/%d", me), map[string]any{"active": false}, 400, nil)
	admin.call("POST", fmt.Sprintf("/api/admin/users/%d/reset-password", me), nil, 400, nil)
}

type groupDetail struct {
	ID          int64
	Name        string
	Description *string
	Members     []userRef
	Usage       struct{ Grants, Roles, SecurityLevels, Comments, Worklogs int }
}

func TestGroups(t *testing.T) {
	admin := login(t, "admin", "admin")
	hugo := login(t, "hugo.robert", jirafixture.Password)
	paul := login(t, "paul.martin", jirafixture.Password)
	name := unique("Equipe-")
	var g struct {
		ID          int64
		Name        string
		Description *string
		Members     int
	}
	admin.call("POST", "/api/admin/groups", map[string]any{"name": name, "description": "Pour les tests"}, 201, &g)
	if g.Name != strings.ToLower(name) || g.Description == nil || g.Members != 0 {
		t.Fatalf("created group: %+v", g)
	}
	path := fmt.Sprintf("/api/admin/groups/%d", g.ID)
	t.Cleanup(func() { admin.do("DELETE", path, nil) })
	admin.call("POST", "/api/admin/groups", map[string]any{"name": strings.ToUpper(name)}, 409, nil)
	admin.call("POST", "/api/admin/groups", map[string]any{"name": " "}, 400, nil)
	hugo.call("POST", "/api/admin/groups", map[string]any{"name": "pirates"}, 403, nil)
	hugo.call("GET", "/api/admin/groups", nil, 403, nil)

	var d groupDetail
	admin.call("POST", path+"/members", map[string]any{"users": []int64{userID(t, "hugo.robert"), userID(t, "paul.martin")}}, 200, &d)
	if len(d.Members) != 2 {
		t.Fatalf("members: %+v", d.Members)
	}
	admin.call("POST", path+"/members", map[string]any{"users": []int64{999999}}, 400, nil)
	admin.call("DELETE", fmt.Sprintf("%s/members/%d", path, userID(t, "hugo.robert")), nil, 204, nil)
	admin.call("DELETE", fmt.Sprintf("%s/members/%d", path, userID(t, "hugo.robert")), nil, 404, nil)
	var list []struct {
		ID      int64
		Members int
	}
	admin.call("GET", "/api/admin/groups?q="+strings.ToLower(name), nil, 200, &list)
	if len(list) != 1 || list[0].Members != 1 {
		t.Errorf("groups matching %s: %+v", name, list)
	}

	// A comment restricted to the group keeps it alive, and follows a rename.
	key := scratchIssue(t, paul, "Commentaire de groupe")
	var c commentOut
	paul.call("POST", "/api/issues/"+key+"/comments", map[string]any{"body": "Pour le groupe",
		"visibility": map[string]string{"group": strings.ToLower(name)}}, 201, &c)
	status, body := admin.do("DELETE", path, nil)
	if status != 409 || num(obj(t, body), "usage", "comments") != 1 {
		t.Fatalf("deleting a group with restricted comments: %d %s", status, body)
	}
	renamed := strings.ToLower(name) + "-bis"
	admin.call("PATCH", path, map[string]any{"name": renamed}, 200, &g)
	var comments []commentOut
	paul.call("GET", "/api/issues/"+key+"/comments", nil, 200, &comments)
	if len(comments) != 1 || comments[0].Visibility == nil || comments[0].Visibility.Group != renamed {
		t.Errorf("comment after the rename: %+v", comments)
	}
	// Outside the group, the comment does not exist.
	hugo.call("GET", "/api/issues/"+key+"/comments", nil, 200, &comments)
	if len(comments) != 0 {
		t.Errorf("hugo.robert, out of the group, reads %+v", comments)
	}
	paul.call("DELETE", fmt.Sprintf("/api/comments/%d", c.ID), nil, 204, nil)
	admin.call("DELETE", path, nil, 204, nil)
	admin.call("GET", path, nil, 404, nil)
}

func TestGroupMembersFailureAddsNone(t *testing.T) {
	admin := login(t, "admin", "admin")
	var g struct{ ID int64 }
	admin.call("POST", "/api/admin/groups", map[string]any{"name": unique("Atomique-")}, 201, &g)
	path := fmt.Sprintf("/api/admin/groups/%d", g.ID)
	t.Cleanup(func() { admin.do("DELETE", path, nil) })
	// The database refuses the second member, after the first was written.
	trigger := unique("refuse_member_")
	if _, err := testDB.Exec(fmt.Sprintf(`CREATE TRIGGER %s BEFORE INSERT ON group_members FOR EACH ROW
		BEGIN IF NEW.group_id = %d AND NEW.user_id = %d THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'refused by the test'; END IF; END`,
		trigger, g.ID, userID(t, "paul.martin"))); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { testDB.Exec(`DROP TRIGGER IF EXISTS ` + trigger) })

	if status, body := admin.do("POST", path+"/members", map[string]any{"users": []int64{userID(t, "hugo.robert"), userID(t, "paul.martin")}}); status != 500 {
		t.Errorf("add members: %d %s, want 500", status, body)
	}
	if n := dbInt(t, `SELECT COUNT(*) FROM group_members WHERE group_id = ?`, g.ID); n != 0 {
		t.Errorf("%d members added by a failed request", n)
	}
}

func TestRoles(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", jirafixture.Password)
	chloe := login(t, "chloe.simon", jirafixture.Password)
	type role struct {
		ID          int64
		Name        string
		Description *string
		Projects    int
		Grants      int
	}
	var r role
	name := unique("Relecteurs ")
	admin.call("POST", "/api/admin/roles", map[string]any{"name": name}, 201, &r)
	t.Cleanup(func() { admin.do("DELETE", fmt.Sprintf("/api/admin/roles/%d", r.ID), nil) })
	admin.call("POST", "/api/admin/roles", map[string]any{"name": strings.ToUpper(name)}, 409, nil)
	paul.call("POST", "/api/admin/roles", map[string]any{"name": "x"}, 403, nil)
	admin.call("PATCH", fmt.Sprintf("/api/admin/roles/%d", r.ID), map[string]any{"description": "Relisent"}, 200, &r)
	if r.Name != name || r.Description == nil || *r.Description != "Relisent" {
		t.Errorf("after PATCH: %+v", r)
	}

	// A project whose access is this role: its lead joins it, others are added.
	key := newProject(t, admin, map[string]any{"access": map[string]any{"holder": "role", "id": r.ID}})
	if status := chloe.get("/api/projects/"+key, nil); status != 404 {
		t.Errorf("chloe.simon sees %s before joining the role: %d", key, status)
	}
	type member struct {
		ID     int64
		Users  []userRef
		Groups []idName
	}
	var roles []member
	admin.call("GET", "/api/projects/"+key+"/roles", nil, 200, &roles)
	i := slices.IndexFunc(roles, func(m member) bool { return m.ID == r.ID })
	if i < 0 || len(roles[i].Users) != 1 || roles[i].Users[0].Username != "admin" {
		t.Fatalf("the role in %s: %+v", key, roles)
	}
	var m member
	rolePath := fmt.Sprintf("/api/projects/%s/roles/%d", key, r.ID)
	admin.call("PUT", rolePath, map[string]any{"users": []int64{userID(t, "admin"), userID(t, "chloe.simon")},
		"groups": []int64{}}, 200, &m)
	if len(m.Users) != 2 {
		t.Errorf("members after PUT: %+v", m)
	}
	admin.call("PUT", rolePath, map[string]any{"users": []int64{999999}}, 400, nil)
	if status := chloe.get("/api/projects/"+key, nil); status != 200 {
		t.Errorf("chloe.simon GET %s in the role: %d", key, status)
	}
	paul.call("GET", "/api/projects/"+key+"/roles", nil, 404, nil) // not found, or no access
	var all []role
	admin.call("GET", "/api/admin/roles", nil, 200, &all)
	if j := slices.IndexFunc(all, func(x role) bool { return x.ID == r.ID }); j < 0 || all[j].Projects != 1 || all[j].Grants == 0 {
		t.Errorf("roles list: %+v", all)
	}

	// Deleted: its members lose what it gave.
	admin.call("DELETE", fmt.Sprintf("/api/admin/roles/%d", r.ID), nil, 204, nil)
	if status := chloe.get("/api/projects/"+key, nil); status != 404 {
		t.Errorf("chloe.simon GET %s after the role went: %d", key, status)
	}
}

func TestProfile(t *testing.T) {
	admin := login(t, "admin", "admin")
	_, username := newAccount(t, admin, "premier-secret-123")
	// Wrong passwords lock the address out: a server of its own.
	base := ownServer(t)
	one, two := loginAt(t, base, username, "premier-secret-123"), loginAt(t, base, username, "premier-secret-123")

	var me struct {
		Username string
		Name     string
		Email    string
	}
	// Changing the address takes the current password, as the password does.
	one.call("PATCH", "/api/me", map[string]any{"email": "sans-mot-de-passe@exemple.fr"}, 400, nil)
	one.call("PATCH", "/api/me", map[string]any{"email": "mauvais-mot-de-passe@exemple.fr",
		"currentPassword": "faux"}, 400, nil)
	// The form always sends the address: unchanged, it needs no password.
	original := username + "@exemple.fr"
	one.call("PATCH", "/api/me", map[string]any{"name": "Nom Choisi", "email": original}, 200, &me)
	if me.Name != "Nom Choisi" || me.Email != original {
		t.Errorf("after PATCH: %+v", me)
	}
	// Surrounding spaces do not make it another address.
	one.call("PATCH", "/api/me", map[string]any{"email": "  " + original + "  "}, 200, &me)
	if me.Email != original {
		t.Errorf("after a padded, unchanged address: %+v", me)
	}
	one.call("PATCH", "/api/me", map[string]any{"email": "choisi@exemple.fr",
		"currentPassword": "premier-secret-123"}, 200, &me)
	if me.Email != "choisi@exemple.fr" {
		t.Errorf("after the address change: %+v", me)
	}
	for _, bad := range []map[string]any{
		{"email": "pas-une-adresse"},
		{"newPassword": "second-secret-456"},
		{"currentPassword": "faux", "newPassword": "second-secret-456"},
		{"currentPassword": "premier-secret-123", "newPassword": "court"},
	} {
		one.call("PATCH", "/api/me", bad, 400, nil)
	}
	one.call("PATCH", "/api/me", map[string]any{"currentPassword": "premier-secret-123", "newPassword": "second-secret-456"}, 200, nil)
	if status := one.get("/api/me", nil); status != 200 {
		t.Errorf("the session that changed the password: %d", status)
	}
	if status := two.get("/api/me", nil); status != 401 {
		t.Errorf("another session after a password change: %d", status)
	}
	loginAt(t, base, username, "second-secret-456")

	// A token may change neither the password nor the address.
	token := newToken(t, username, nil)
	if status, _, _ := call(t, nil, base, "PATCH", "/api/me", map[string]any{"currentPassword": "second-secret-456",
		"newPassword": "troisième-secret-789"}, func(r *http.Request) { bearer(token)(r); r.Header.Set(csrfHeader, "1") }); status != 403 {
		t.Errorf("password change with a token: %d", status)
	}
	if status, _, _ := call(t, nil, base, "PATCH", "/api/me", map[string]any{"email": "jeton@exemple.fr",
		"currentPassword": "second-secret-456"},
		func(r *http.Request) { bearer(token)(r); r.Header.Set(csrfHeader, "1") }); status != 403 {
		t.Errorf("address change with a token: %d", status)
	}
}

// A person's page shows the address only to administrators and to the person.
func TestPersonPageHidesTheAddress(t *testing.T) {
	admin := login(t, "admin", "admin")
	_, username := newAccount(t, admin, "une phrase assez longue")
	address := username + "@exemple.fr"
	var page struct{ Email string }
	login(t, username, "une phrase assez longue").call("GET", "/api/people/"+username, nil, 200, &page)
	if page.Email != address {
		t.Errorf("the person sees %q, want %q", page.Email, address)
	}
	login(t, "paul.martin", jirafixture.Password).call("GET", "/api/people/"+username, nil, 200, &page)
	if page.Email != "" {
		t.Errorf("another user sees the address %q", page.Email)
	}
	admin.call("GET", "/api/people/"+username, nil, 200, &page)
	if page.Email != address {
		t.Errorf("the administrator sees %q, want %q", page.Email, address)
	}
}
