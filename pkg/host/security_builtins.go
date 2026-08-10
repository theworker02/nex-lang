package host

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"nex-lang/pkg/database"
	"nex-lang/pkg/evaluator"
)

func (h *Host) registerSecurityBuiltins(b map[string]*evaluator.Builtin) {
	ctx := func() context.Context { return context.Background() }
	requireDB := func(name string) *evaluator.Error {
		if h.DB == nil {
			return &evaluator.Error{Message: name + ": database not configured (set DATABASE_URL)"}
		}
		return nil
	}

	b["totp_otpauth_url"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("totp_otpauth_url", 3, args); err != nil {
			return err
		}
		issuer, _ := AsString(args[0])
		account, _ := AsString(args[1])
		secret, _ := AsString(args[2])
		if issuer == "" {
			issuer = "Nexus Registry"
		}
		u := url.URL{
			Scheme: "otpauth",
			Host:   "totp",
			Path:   "/" + issuer + ":" + account,
		}
		q := u.Query()
		q.Set("secret", secret)
		q.Set("issuer", issuer)
		q.Set("algorithm", "SHA1")
		q.Set("digits", "6")
		q.Set("period", "30")
		u.RawQuery = q.Encode()
		return &evaluator.String{Value: u.String()}
	}}

	b["db_totp_begin"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_totp_begin"); err != nil {
			return err
		}
		if err := ExpectArgs("db_totp_begin", 1, args); err != nil {
			return err
		}
		uid, ok := AsInt(args[0])
		if !ok {
			return &evaluator.Error{Message: "user id required"}
		}
		secret, err := h.DB.BeginTOTPSetup(ctx(), uid)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return &evaluator.String{Value: secret}
	}}

	b["db_totp_confirm"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_totp_confirm"); err != nil {
			return err
		}
		if err := ExpectArgs("db_totp_confirm", 2, args); err != nil {
			return err
		}
		uid, ok := AsInt(args[0])
		code, _ := AsString(args[1])
		if !ok {
			return &evaluator.Error{Message: "user id required"}
		}
		u, err := h.DB.ConfirmTOTPSetup(ctx(), uid, code)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return userToHash(u)
	}}

	b["db_totp_disable"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_totp_disable"); err != nil {
			return err
		}
		if err := ExpectArgs("db_totp_disable", 2, args); err != nil {
			return err
		}
		uid, ok := AsInt(args[0])
		code, _ := AsString(args[1])
		if !ok {
			return &evaluator.Error{Message: "user id required"}
		}
		u, err := h.DB.DisableTOTP(ctx(), uid, code)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return userToHash(u)
	}}

	b["db_totp_challenge_create"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_totp_challenge_create"); err != nil {
			return err
		}
		if err := ExpectArgs("db_totp_challenge_create", 1, args); err != nil {
			return err
		}
		uid, ok := AsInt(args[0])
		if !ok {
			return &evaluator.Error{Message: "user id required"}
		}
		tok, err := h.DB.CreateTOTPChallenge(ctx(), uid)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return &evaluator.String{Value: tok}
	}}

	b["db_totp_challenge_consume"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_totp_challenge_consume"); err != nil {
			return err
		}
		if err := ExpectArgs("db_totp_challenge_consume", 2, args); err != nil {
			return err
		}
		challenge, _ := AsString(args[0])
		code, _ := AsString(args[1])
		u, err := h.DB.ConsumeTOTPChallenge(ctx(), challenge, code)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return userToHash(u)
	}}

	b["db_audit_log"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_audit_log"); err != nil {
			return err
		}
		if err := ExpectArgs("db_audit_log", 1, args); err != nil {
			return err
		}
		m, ok := args[0].(*evaluator.Hash)
		if !ok {
			return &evaluator.Error{Message: "expects hash"}
		}
		ev := database.AuditEvent{
			ActorUserID:  mustInt(m.Get("actor_user_id"), 0),
			Action:       HashGetString(m, "action"),
			ResourceType: HashGetString(m, "resource_type"),
			ResourceID:   HashGetString(m, "resource_id"),
			PackageName:  HashGetString(m, "package_name"),
			Version:      HashGetString(m, "version"),
			IP:           HashGetString(m, "ip"),
			UserAgent:    HashGetString(m, "user_agent"),
		}
		if meta := m.Get("metadata"); meta != evaluator.NULL {
			if gm, ok := ToGo(meta).(map[string]any); ok {
				ev.Metadata = gm
			}
		}
		if err := h.DB.InsertAuditLog(ctx(), ev); err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return evaluator.TRUE
	}}

	b["db_list_audit_logs"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_list_audit_logs"); err != nil {
			return err
		}
		if err := ExpectMinArgs("db_list_audit_logs", 1, args); err != nil {
			return err
		}
		uid, ok := AsInt(args[0])
		if !ok {
			return &evaluator.Error{Message: "user id required"}
		}
		limit := int64(50)
		if len(args) > 1 {
			if n, ok := AsInt(args[1]); ok {
				limit = n
			}
		}
		rows, err := h.DB.ListAuditLogsForUser(ctx(), uid, int(limit))
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(rows)
	}}

	b["db_list_audit_logs_admin"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_list_audit_logs_admin"); err != nil {
			return err
		}
		limit := int64(100)
		if len(args) > 0 {
			if n, ok := AsInt(args[0]); ok {
				limit = n
			}
		}
		rows, err := h.DB.ListAuditLogsAdmin(ctx(), int(limit))
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(rows)
	}}

	b["db_create_abuse_report"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_create_abuse_report"); err != nil {
			return err
		}
		if err := ExpectArgs("db_create_abuse_report", 1, args); err != nil {
			return err
		}
		m, ok := args[0].(*evaluator.Hash)
		if !ok {
			return &evaluator.Error{Message: "expects hash"}
		}
		r := database.AbuseReport{
			ReporterEmail: HashGetString(m, "reporter_email"),
			PackageName:   HashGetString(m, "package_name"),
			Version:       HashGetString(m, "version"),
			Category:      HashGetString(m, "category"),
			Details:       HashGetString(m, "details"),
		}
		if id := mustInt(m.Get("reporter_user_id"), 0); id > 0 {
			r.ReporterUserID = &id
		}
		created, err := h.DB.CreateAbuseReport(ctx(), r)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(created)
	}}

	b["db_list_abuse_reports"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_list_abuse_reports"); err != nil {
			return err
		}
		status := ""
		limit := int64(100)
		if len(args) > 0 {
			status, _ = AsString(args[0])
		}
		if len(args) > 1 {
			if n, ok := AsInt(args[1]); ok {
				limit = n
			}
		}
		rows, err := h.DB.ListAbuseReports(ctx(), status, int(limit))
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(rows)
	}}

	b["db_mark_trusted_verified"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_mark_trusted_verified"); err != nil {
			return err
		}
		if err := ExpectArgs("db_mark_trusted_verified", 1, args); err != nil {
			return err
		}
		id, ok := AsInt(args[0])
		if !ok {
			return &evaluator.Error{Message: "id required"}
		}
		if err := h.DB.MarkTrustedPublisherVerified(ctx(), id); err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return evaluator.TRUE
	}}

	b["db_record_trusted_failure"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_record_trusted_failure"); err != nil {
			return err
		}
		if err := ExpectArgs("db_record_trusted_failure", 3, args); err != nil {
			return err
		}
		owner, _ := AsString(args[0])
		repo, _ := AsString(args[1])
		reason, _ := AsString(args[2])
		if err := h.DB.RecordTrustedPublisherFailure(ctx(), owner, repo, reason); err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return evaluator.TRUE
	}}

	b["db_set_version_provenance"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_set_version_provenance"); err != nil {
			return err
		}
		if err := ExpectArgs("db_set_version_provenance", 3, args); err != nil {
			return err
		}
		vid, ok := AsInt(args[0])
		source, _ := AsString(args[1])
		raw, _ := AsString(args[2])
		if !ok {
			return &evaluator.Error{Message: "version id required"}
		}
		bytes := []byte(strings.TrimSpace(raw))
		if h.provenance != nil {
			if err := h.provenance.Verify(ctx(), source, bytes); err != nil {
				return &evaluator.Error{Message: err.Error()}
			}
		}
		if err := h.DB.SetVersionProvenance(ctx(), vid, source, json.RawMessage(bytes)); err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return evaluator.TRUE
	}}

	b["db_unpublish_version"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_unpublish_version"); err != nil {
			return err
		}
		if err := ExpectArgs("db_unpublish_version", 3, args); err != nil {
			return err
		}
		name, _ := AsString(args[0])
		version, _ := AsString(args[1])
		ownerID, ok := AsInt(args[2])
		if !ok {
			return &evaluator.Error{Message: "owner id required"}
		}
		path, err := h.DB.UnpublishPackageVersion(ctx(), name, version, ownerID)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		out := evaluator.NewHash()
		out.SetString("storage_path", &evaluator.String{Value: path})
		out.SetString("message", &evaluator.String{Value: "unpublished"})
		return out
	}}

	b["api_key_allows_publish"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("api_key_allows_publish", 1, args); err != nil {
			return err
		}
		scope, _ := AsString(args[0])
		return FromGo(database.APIKeyAllowsPublish(scope))
	}}
}
