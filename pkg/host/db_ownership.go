package host

import (
	"context"
	"errors"
	"time"

	"nex-lang/pkg/database"
	"nex-lang/pkg/evaluator"
)

func (h *Host) registerOwnershipBuiltins(b map[string]*evaluator.Builtin) {
	ctx := func() context.Context { return context.Background() }
	requireDB := func(name string) *evaluator.Error {
		if h.DB == nil {
			return &evaluator.Error{Message: name + ": database not configured (set DATABASE_URL)"}
		}
		return nil
	}

	b["db_user_can_publish"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_user_can_publish"); err != nil {
			return err
		}
		if err := ExpectArgs("db_user_can_publish", 2, args); err != nil {
			return err
		}
		uid, ok1 := AsInt(args[0])
		name, ok2 := AsString(args[1])
		if !ok1 || !ok2 {
			return &evaluator.Error{Message: "expects (user_id, package_name)"}
		}
		ok, err := h.DB.UserCanPublish(ctx(), uid, name)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		if ok {
			return evaluator.TRUE
		}
		return evaluator.FALSE
	}}

	b["db_list_package_owners"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_list_package_owners"); err != nil {
			return err
		}
		if err := ExpectArgs("db_list_package_owners", 1, args); err != nil {
			return err
		}
		pid, ok := AsInt(args[0])
		if !ok {
			return &evaluator.Error{Message: "package id required"}
		}
		rows, err := h.DB.ListPackageOwners(ctx(), pid)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(rows)
	}}

	b["db_invite_package_owner"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_invite_package_owner"); err != nil {
			return err
		}
		if err := ExpectMinArgs("db_invite_package_owner", 3, args); err != nil {
			return err
		}
		m, ok := args[0].(*evaluator.Hash)
		if !ok {
			return &evaluator.Error{Message: "expects hash"}
		}
		pkgID := mustInt(m.Get("package_id"), 0)
		actorID := mustInt(m.Get("actor_id"), 0)
		username := HashGetString(m, "username")
		email := HashGetString(m, "email")
		role := HashGetString(m, "role")
		owner, token, err := h.DB.InvitePackageOwner(ctx(), pkgID, actorID, username, email, role)
		if err != nil {
			if errors.Is(err, database.ErrConflict) {
				return &evaluator.Error{Message: "conflict"}
			}
			if errors.Is(err, database.ErrNotFound) {
				return &evaluator.Error{Message: "not found"}
			}
			return &evaluator.Error{Message: err.Error()}
		}
		out := evaluator.NewHash()
		out.SetString("owner", FromGo(owner))
		out.SetString("invite_token", &evaluator.String{Value: token})
		return out
	}}

	b["db_add_org_package_owner"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_add_org_package_owner"); err != nil {
			return err
		}
		if err := ExpectArgs("db_add_org_package_owner", 4, args); err != nil {
			return err
		}
		pkgID, _ := AsInt(args[0])
		orgID, _ := AsInt(args[1])
		actorID, _ := AsInt(args[2])
		role, _ := AsString(args[3])
		owner, err := h.DB.AddOrgAsPackageOwner(ctx(), pkgID, orgID, actorID, role)
		if err != nil {
			if errors.Is(err, database.ErrConflict) {
				return &evaluator.Error{Message: "conflict"}
			}
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(owner)
	}}

	b["db_remove_package_owner"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_remove_package_owner"); err != nil {
			return err
		}
		if err := ExpectArgs("db_remove_package_owner", 3, args); err != nil {
			return err
		}
		pkgID, _ := AsInt(args[0])
		ownerID, _ := AsInt(args[1])
		actorID, _ := AsInt(args[2])
		if err := h.DB.RemovePackageOwner(ctx(), pkgID, ownerID, actorID); err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return evaluator.TRUE
	}}

	b["db_accept_owner_invite"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_accept_owner_invite"); err != nil {
			return err
		}
		if err := ExpectArgs("db_accept_owner_invite", 2, args); err != nil {
			return err
		}
		token, _ := AsString(args[0])
		uid, _ := AsInt(args[1])
		owner, err := h.DB.AcceptOwnerInvite(ctx(), token, uid)
		if err != nil {
			if errors.Is(err, database.ErrNotFound) {
				return evaluator.NULL
			}
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(owner)
	}}

	b["db_create_ownership_transfer"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_create_ownership_transfer"); err != nil {
			return err
		}
		if err := ExpectArgs("db_create_ownership_transfer", 1, args); err != nil {
			return err
		}
		m, ok := args[0].(*evaluator.Hash)
		if !ok {
			return &evaluator.Error{Message: "expects hash"}
		}
		t, token, err := h.DB.CreateOwnershipTransfer(
			ctx(),
			mustInt(m.Get("package_id"), 0),
			mustInt(m.Get("from_user_id"), 0),
			HashGetString(m, "to_username"),
			HashGetString(m, "to_org_slug"),
		)
		if err != nil {
			if errors.Is(err, database.ErrNotFound) {
				return &evaluator.Error{Message: "not found"}
			}
			return &evaluator.Error{Message: err.Error()}
		}
		out := evaluator.NewHash()
		out.SetString("transfer", FromGo(t))
		out.SetString("token", &evaluator.String{Value: token})
		return out
	}}

	b["db_accept_ownership_transfer"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_accept_ownership_transfer"); err != nil {
			return err
		}
		if err := ExpectArgs("db_accept_ownership_transfer", 2, args); err != nil {
			return err
		}
		token, _ := AsString(args[0])
		uid, _ := AsInt(args[1])
		if err := h.DB.AcceptOwnershipTransfer(ctx(), token, uid); err != nil {
			if errors.Is(err, database.ErrNotFound) {
				return evaluator.NULL
			}
			return &evaluator.Error{Message: err.Error()}
		}
		return evaluator.TRUE
	}}

	b["db_create_org"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_create_org"); err != nil {
			return err
		}
		if err := ExpectMinArgs("db_create_org", 2, args); err != nil {
			return err
		}
		slug, _ := AsString(args[0])
		uid, _ := AsInt(args[1])
		display := ""
		desc := ""
		avatar := ""
		if len(args) > 2 {
			display, _ = AsString(args[2])
		}
		if len(args) > 3 {
			desc, _ = AsString(args[3])
		}
		if len(args) > 4 {
			avatar, _ = AsString(args[4])
		}
		org, err := h.DB.CreateOrganization(ctx(), slug, display, desc, avatar, uid)
		if err != nil {
			if errors.Is(err, database.ErrConflict) {
				return &evaluator.Error{Message: "conflict"}
			}
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(org)
	}}

	b["db_get_org"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_get_org"); err != nil {
			return err
		}
		if err := ExpectArgs("db_get_org", 1, args); err != nil {
			return err
		}
		slug, _ := AsString(args[0])
		org, err := h.DB.GetOrganizationBySlug(ctx(), slug)
		if err != nil {
			if errors.Is(err, database.ErrNotFound) {
				return evaluator.NULL
			}
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(org)
	}}

	b["db_list_user_orgs"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_list_user_orgs"); err != nil {
			return err
		}
		if err := ExpectArgs("db_list_user_orgs", 1, args); err != nil {
			return err
		}
		uid, _ := AsInt(args[0])
		rows, err := h.DB.ListOrganizationsForUser(ctx(), uid)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(rows)
	}}

	b["db_list_org_members"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_list_org_members"); err != nil {
			return err
		}
		if err := ExpectArgs("db_list_org_members", 1, args); err != nil {
			return err
		}
		oid, _ := AsInt(args[0])
		rows, err := h.DB.ListOrgMembers(ctx(), oid)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(rows)
	}}

	b["db_org_member_role"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_org_member_role"); err != nil {
			return err
		}
		if err := ExpectArgs("db_org_member_role", 2, args); err != nil {
			return err
		}
		oid, _ := AsInt(args[0])
		uid, _ := AsInt(args[1])
		role, err := h.DB.OrgMemberRole(ctx(), oid, uid)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return &evaluator.String{Value: role}
	}}

	b["db_add_org_member"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_add_org_member"); err != nil {
			return err
		}
		if err := ExpectArgs("db_add_org_member", 4, args); err != nil {
			return err
		}
		oid, _ := AsInt(args[0])
		uid, _ := AsInt(args[1])
		role, _ := AsString(args[2])
		actor, _ := AsInt(args[3])
		m, err := h.DB.AddOrgMember(ctx(), oid, uid, role, actor)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(m)
	}}

	b["db_remove_org_member"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_remove_org_member"); err != nil {
			return err
		}
		if err := ExpectArgs("db_remove_org_member", 3, args); err != nil {
			return err
		}
		oid, _ := AsInt(args[0])
		uid, _ := AsInt(args[1])
		actor, _ := AsInt(args[2])
		if err := h.DB.RemoveOrgMember(ctx(), oid, uid, actor); err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return evaluator.TRUE
	}}

	b["db_list_org_packages"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_list_org_packages"); err != nil {
			return err
		}
		if err := ExpectArgs("db_list_org_packages", 1, args); err != nil {
			return err
		}
		oid, _ := AsInt(args[0])
		rows, err := h.DB.ListPackagesByOrg(ctx(), oid)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(rows)
	}}

	b["db_create_team"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_create_team"); err != nil {
			return err
		}
		if err := ExpectMinArgs("db_create_team", 2, args); err != nil {
			return err
		}
		oid, _ := AsInt(args[0])
		name, _ := AsString(args[1])
		desc := ""
		if len(args) > 2 {
			desc, _ = AsString(args[2])
		}
		t, err := h.DB.CreateTeam(ctx(), oid, name, desc)
		if err != nil {
			if errors.Is(err, database.ErrConflict) {
				return &evaluator.Error{Message: "conflict"}
			}
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(t)
	}}

	b["db_list_teams"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_list_teams"); err != nil {
			return err
		}
		if err := ExpectArgs("db_list_teams", 1, args); err != nil {
			return err
		}
		oid, _ := AsInt(args[0])
		rows, err := h.DB.ListTeams(ctx(), oid)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(rows)
	}}

	b["db_add_team_member"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_add_team_member"); err != nil {
			return err
		}
		if err := ExpectMinArgs("db_add_team_member", 2, args); err != nil {
			return err
		}
		tid, _ := AsInt(args[0])
		uid, _ := AsInt(args[1])
		role := "member"
		if len(args) > 2 {
			role, _ = AsString(args[2])
		}
		m, err := h.DB.AddTeamMember(ctx(), tid, uid, role)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(m)
	}}

	b["db_list_team_members"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_list_team_members"); err != nil {
			return err
		}
		if err := ExpectArgs("db_list_team_members", 1, args); err != nil {
			return err
		}
		tid, _ := AsInt(args[0])
		rows, err := h.DB.ListTeamMembers(ctx(), tid)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(rows)
	}}

	b["db_list_user_activity"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_list_user_activity"); err != nil {
			return err
		}
		if err := ExpectMinArgs("db_list_user_activity", 1, args); err != nil {
			return err
		}
		uid, _ := AsInt(args[0])
		limit := int64(25)
		if len(args) > 1 {
			if n, ok := AsInt(args[1]); ok {
				limit = n
			}
		}
		rows, err := h.DB.ListUserActivity(ctx(), uid, int(limit))
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(rows)
	}}

	b["db_list_org_activity"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_list_org_activity"); err != nil {
			return err
		}
		if err := ExpectMinArgs("db_list_org_activity", 1, args); err != nil {
			return err
		}
		oid, _ := AsInt(args[0])
		limit := int64(25)
		if len(args) > 1 {
			if n, ok := AsInt(args[1]); ok {
				limit = n
			}
		}
		rows, err := h.DB.ListOrgActivity(ctx(), oid, int(limit))
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(rows)
	}}

	b["db_create_auth_token"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_create_auth_token"); err != nil {
			return err
		}
		if err := ExpectMinArgs("db_create_auth_token", 2, args); err != nil {
			return err
		}
		uid, _ := AsInt(args[0])
		purpose, _ := AsString(args[1])
		ttl := int64(86400)
		if len(args) > 2 {
			if n, ok := AsInt(args[2]); ok && n > 0 {
				ttl = n
			}
		}
		tok, err := h.DB.CreateAuthToken(ctx(), uid, purpose, time.Duration(ttl)*time.Second)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return &evaluator.String{Value: tok}
	}}

	b["db_consume_auth_token"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_consume_auth_token"); err != nil {
			return err
		}
		if err := ExpectArgs("db_consume_auth_token", 2, args); err != nil {
			return err
		}
		tok, _ := AsString(args[0])
		purpose, _ := AsString(args[1])
		u, err := h.DB.ConsumeAuthToken(ctx(), tok, purpose)
		if err != nil {
			if errors.Is(err, database.ErrNotFound) {
				return evaluator.NULL
			}
			return &evaluator.Error{Message: err.Error()}
		}
		return userToHash(u)
	}}

	b["db_peek_auth_token"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_peek_auth_token"); err != nil {
			return err
		}
		if err := ExpectArgs("db_peek_auth_token", 2, args); err != nil {
			return err
		}
		tok, _ := AsString(args[0])
		purpose, _ := AsString(args[1])
		u, err := h.DB.PeekAuthToken(ctx(), tok, purpose)
		if err != nil {
			if errors.Is(err, database.ErrNotFound) {
				return evaluator.NULL
			}
			return &evaluator.Error{Message: err.Error()}
		}
		return userToHash(u)
	}}

	b["db_mark_email_verified"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_mark_email_verified"); err != nil {
			return err
		}
		if err := ExpectArgs("db_mark_email_verified", 1, args); err != nil {
			return err
		}
		uid, _ := AsInt(args[0])
		u, err := h.DB.MarkEmailVerified(ctx(), uid)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return userToHash(u)
	}}

	b["db_set_password"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_set_password"); err != nil {
			return err
		}
		if err := ExpectArgs("db_set_password", 2, args); err != nil {
			return err
		}
		uid, _ := AsInt(args[0])
		hash, _ := AsString(args[1])
		if err := h.DB.SetUserPassword(ctx(), uid, hash); err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return evaluator.TRUE
	}}

	b["db_get_user_by_email"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_get_user_by_email"); err != nil {
			return err
		}
		if err := ExpectArgs("db_get_user_by_email", 1, args); err != nil {
			return err
		}
		email, _ := AsString(args[0])
		u, err := h.DB.GetUserByEmail(ctx(), email)
		if err != nil {
			if errors.Is(err, database.ErrNotFound) {
				return evaluator.NULL
			}
			return &evaluator.Error{Message: err.Error()}
		}
		return userToHash(u)
	}}

	b["db_package_owner_role"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_package_owner_role"); err != nil {
			return err
		}
		if err := ExpectArgs("db_package_owner_role", 2, args); err != nil {
			return err
		}
		pid, _ := AsInt(args[0])
		uid, _ := AsInt(args[1])
		role, err := h.DB.UserIsPackageOwnerRole(ctx(), pid, uid)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return &evaluator.String{Value: role}
	}}

	b["send_email"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("send_email", 3, args); err != nil {
			return err
		}
		to, _ := AsString(args[0])
		subject, _ := AsString(args[1])
		body, _ := AsString(args[2])
		if err := h.SendEmail(to, subject, body); err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return evaluator.TRUE
	}}
}
