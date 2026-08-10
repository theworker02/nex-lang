package host

import (
	"context"
	"errors"
	"strings"
	"time"

	"nex-lang/pkg/database"
	"nex-lang/pkg/evaluator"
)

func (h *Host) registerDBBuiltins(b map[string]*evaluator.Builtin) {
	ctx := func() context.Context { return context.Background() }
	requireDB := func(name string) *evaluator.Error {
		if h.DB == nil {
			return &evaluator.Error{Message: name + ": database not configured (set DATABASE_URL)"}
		}
		return nil
	}

	b["db_count_packages"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_count_packages"); err != nil {
			return err
		}
		n, err := h.DB.CountPackages(ctx())
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return &evaluator.Integer{Value: n}
	}}
	b["db_count_versions"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_count_versions"); err != nil {
			return err
		}
		n, err := h.DB.CountVersions(ctx())
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return &evaluator.Integer{Value: n}
	}}
	b["db_count_users"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_count_users"); err != nil {
			return err
		}
		n, err := h.DB.CountUsers(ctx())
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return &evaluator.Integer{Value: n}
	}}
	b["db_sum_downloads"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_sum_downloads"); err != nil {
			return err
		}
		n, err := h.DB.SumDownloads(ctx())
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return &evaluator.Integer{Value: n}
	}}
	b["db_top_tags"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_top_tags"); err != nil {
			return err
		}
		limit := int64(16)
		if len(args) > 0 {
			if n, ok := AsInt(args[0]); ok {
				limit = n
			}
		}
		rows, err := h.DB.TopTags(ctx(), int(limit))
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(rows)
	}}
	b["db_hub_stats"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_hub_stats"); err != nil {
			return err
		}
		stats, err := h.DB.HubStats(ctx())
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(stats)
	}}
	b["db_list_recent"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_list_recent"); err != nil {
			return err
		}
		limit := int64(10)
		if len(args) > 0 {
			if n, ok := AsInt(args[0]); ok {
				limit = n
			}
		}
		rows, err := h.DB.ListRecentPackages(ctx(), int(limit))
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(rows)
	}}
	b["db_list_popular"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		limit := int64(6)
		if len(args) > 0 {
			if n, ok := AsInt(args[0]); ok {
				limit = n
			}
		}
		rows, err := h.DB.ListPopularPackages(ctx(), int(limit))
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(rows)
	}}
	b["db_search"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_search"); err != nil {
			return err
		}
		if err := ExpectMinArgs("db_search", 1, args); err != nil {
			return err
		}
		params := database.SearchParams{Limit: 50, Sort: "relevance"}
		returnHash := false
		if m, ok := args[0].(*evaluator.Hash); ok {
			returnHash = true
			params.Query = HashGetString(m, "q")
			if params.Query == "" {
				params.Query = HashGetString(m, "query")
			}
			params.Category = HashGetString(m, "category")
			params.Keyword = HashGetString(m, "keyword")
			params.License = HashGetString(m, "license")
			params.Sort = HashGetString(m, "sort")
			params.Browse = AsBool(m.Get("browse"))
			if n, ok := AsInt(m.Get("limit")); ok {
				params.Limit = int(n)
			}
			if n, ok := AsInt(m.Get("offset")); ok {
				params.Offset = int(n)
			}
			if n, ok := AsInt(m.Get("page")); ok && n > 0 {
				if params.Limit <= 0 {
					params.Limit = 25
				}
				params.Offset = (int(n) - 1) * params.Limit
			}
			if raw := HashGetString(m, "updated_after"); raw != "" {
				t, err := database.ParseUpdatedAfter(raw)
				if err != nil {
					return &evaluator.Error{Message: err.Error()}
				}
				params.UpdatedAfter = t
			}
		} else {
			q, _ := AsString(args[0])
			params.Query = q
			if len(args) > 1 {
				params.Category, _ = AsString(args[1])
			}
			if len(args) > 2 {
				if n, ok := AsInt(args[2]); ok {
					params.Limit = int(n)
				}
			}
		}
		res, err := h.DB.Search(ctx(), params)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		if returnHash {
			out := evaluator.NewHash()
			out.SetString("packages", FromGo(res.Packages))
			out.SetString("total", &evaluator.Integer{Value: res.Total})
			out.SetString("query", &evaluator.String{Value: res.Params.Query})
			out.SetString("category", &evaluator.String{Value: res.Params.Category})
			out.SetString("keyword", &evaluator.String{Value: res.Params.Keyword})
			out.SetString("license", &evaluator.String{Value: res.Params.License})
			out.SetString("sort", &evaluator.String{Value: res.Params.Sort})
			out.SetString("limit", &evaluator.Integer{Value: int64(res.Params.Limit)})
			out.SetString("offset", &evaluator.Integer{Value: int64(res.Params.Offset)})
			return out
		}
		return FromGo(res.Packages)
	}}
	b["db_list_licenses"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_list_licenses"); err != nil {
			return err
		}
		limit := int64(24)
		if len(args) > 0 {
			if n, ok := AsInt(args[0]); ok {
				limit = n
			}
		}
		rows, err := h.DB.ListLicenses(ctx(), int(limit))
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(rows)
	}}
	b["db_top_keywords"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_top_keywords"); err != nil {
			return err
		}
		limit := int64(36)
		if len(args) > 0 {
			if n, ok := AsInt(args[0]); ok {
				limit = n
			}
		}
		rows, err := h.DB.TopKeywords(ctx(), int(limit))
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(rows)
	}}
	b["db_list_packages_page"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		page := int64(1)
		per := int64(50)
		if len(args) > 0 {
			if n, ok := AsInt(args[0]); ok {
				page = n
			}
		}
		if len(args) > 1 {
			if n, ok := AsInt(args[1]); ok {
				per = n
			}
		}
		rows, meta, err := h.DB.ListPackagesPage(ctx(), int(page), int(per))
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		out := evaluator.NewHash()
		out.SetString("packages", FromGo(rows))
		out.SetString("page", FromGo(meta))
		return out
	}}
	b["db_get_package"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("db_get_package", 1, args); err != nil {
			return err
		}
		name, ok := AsString(args[0])
		if !ok {
			return &evaluator.Error{Message: "name must be string"}
		}
		pkg, err := h.DB.GetPackageByName(ctx(), name)
		if err != nil {
			if errors.Is(err, database.ErrNotFound) {
				return evaluator.NULL
			}
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(pkg)
	}}
	b["db_list_versions"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("db_list_versions", 1, args); err != nil {
			return err
		}
		id, ok := AsInt(args[0])
		if !ok {
			return &evaluator.Error{Message: "package id must be int"}
		}
		vers, err := h.DB.ListPackageVersions(ctx(), id)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(vers)
	}}
	b["db_get_package_version"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("db_get_package_version", 2, args); err != nil {
			return err
		}
		name, ok1 := AsString(args[0])
		ver, ok2 := AsString(args[1])
		if !ok1 || !ok2 {
			return &evaluator.Error{Message: "expects (name, version)"}
		}
		pv, err := h.DB.GetPackageVersion(ctx(), name, ver)
		if err != nil {
			if errors.Is(err, database.ErrNotFound) {
				return evaluator.NULL
			}
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(pv)
	}}
	b["db_list_deps"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("db_list_deps", 1, args); err != nil {
			return err
		}
		id, ok := AsInt(args[0])
		if !ok {
			return &evaluator.Error{Message: "version id must be int"}
		}
		deps, err := h.DB.ListDependencies(ctx(), id)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(deps)
	}}
	b["db_increment_downloads"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("db_increment_downloads", 1, args); err != nil {
			return err
		}
		id, ok := AsInt(args[0])
		if !ok {
			return &evaluator.Error{Message: "package id must be int"}
		}
		if err := h.DB.IncrementDownloadCount(ctx(), id); err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return evaluator.TRUE
	}}
	b["db_get_user"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("db_get_user", 1, args); err != nil {
			return err
		}
		username, ok := AsString(args[0])
		if !ok {
			return &evaluator.Error{Message: "username must be string"}
		}
		u, err := h.DB.GetUserByUsername(ctx(), username)
		if err != nil {
			if errors.Is(err, database.ErrNotFound) {
				return evaluator.NULL
			}
			return &evaluator.Error{Message: err.Error()}
		}
		return publicUserHash(u)
	}}
	b["db_get_user_by_login"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("db_get_user_by_login", 1, args); err != nil {
			return err
		}
		login, ok := AsString(args[0])
		if !ok {
			return &evaluator.Error{Message: "login must be string"}
		}
		u, err := h.DB.GetUserByLogin(ctx(), login)
		if err != nil {
			if errors.Is(err, database.ErrNotFound) {
				return evaluator.NULL
			}
			return &evaluator.Error{Message: err.Error()}
		}
		return userToHash(u)
	}}
	b["db_create_user"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectMinArgs("db_create_user", 3, args); err != nil {
			return err
		}
		username, _ := AsString(args[0])
		email, _ := AsString(args[1])
		hash, _ := AsString(args[2])
		avatar := ""
		bio := ""
		useGravatar := false
		if len(args) > 3 {
			avatar, _ = AsString(args[3])
		}
		if len(args) > 4 {
			bio, _ = AsString(args[4])
		}
		if len(args) > 5 {
			useGravatar = AsBool(args[5])
		}
		u, err := h.DB.CreateUser(ctx(), username, email, hash, avatar, bio, useGravatar)
		if err != nil {
			if errors.Is(err, database.ErrConflict) {
				return &evaluator.Error{Message: "conflict"}
			}
			return &evaluator.Error{Message: err.Error()}
		}
		return userToHash(u)
	}}
	b["db_create_session"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("db_create_session", 1, args); err != nil {
			return err
		}
		uid, ok := AsInt(args[0])
		if !ok {
			return &evaluator.Error{Message: "user id must be int"}
		}
		token, _, err := h.DB.CreateSession(ctx(), uid, sessionTTL)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return &evaluator.String{Value: token}
	}}
	b["db_delete_session"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("db_delete_session", 1, args); err != nil {
			return err
		}
		token, ok := AsString(args[0])
		if !ok {
			return &evaluator.Error{Message: "token must be string"}
		}
		_ = h.DB.DeleteSession(ctx(), token)
		return evaluator.TRUE
	}}
	b["db_user_stats"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("db_user_stats", 1, args); err != nil {
			return err
		}
		uid, ok := AsInt(args[0])
		if !ok {
			return &evaluator.Error{Message: "user id must be int"}
		}
		stats, err := h.DB.UserStats(ctx(), uid)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(stats)
	}}
	b["db_list_packages_by_owner"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("db_list_packages_by_owner", 1, args); err != nil {
			return err
		}
		uid, ok := AsInt(args[0])
		if !ok {
			return &evaluator.Error{Message: "user id must be int"}
		}
		rows, err := h.DB.ListPackagesByOwner(ctx(), uid)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(rows)
	}}
	b["db_unlink_github"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("db_unlink_github", 1, args); err != nil {
			return err
		}
		uid, ok := AsInt(args[0])
		if !ok {
			return &evaluator.Error{Message: "user id must be int"}
		}
		u, err := h.DB.UnlinkGitHubAccount(ctx(), uid)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return userToHash(u)
	}}
	b["db_update_profile"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("db_update_profile", 4, args); err != nil {
			return err
		}
		uid, ok := AsInt(args[0])
		bio, _ := AsString(args[1])
		avatar, _ := AsString(args[2])
		useG := AsBool(args[3])
		if !ok {
			return &evaluator.Error{Message: "user id must be int"}
		}
		u, err := h.DB.UpdateUserProfile(ctx(), uid, bio, avatar, useG)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return userToHash(u)
	}}
	b["db_list_api_keys"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("db_list_api_keys", 1, args); err != nil {
			return err
		}
		uid, ok := AsInt(args[0])
		if !ok {
			return &evaluator.Error{Message: "user id must be int"}
		}
		keys, err := h.DB.ListAPIKeys(ctx(), uid)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(keys)
	}}
	b["db_create_api_key"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectMinArgs("db_create_api_key", 2, args); err != nil {
			return err
		}
		uid, ok := AsInt(args[0])
		name, _ := AsString(args[1])
		if !ok {
			return &evaluator.Error{Message: "user id must be int"}
		}
		scope := "publish"
		expiresDays := int64(0)
		if len(args) > 2 {
			if s, ok := AsString(args[2]); ok && s != "" {
				scope = s
			}
		}
		if len(args) > 3 {
			if n, ok := AsInt(args[3]); ok {
				expiresDays = n
			}
		}
		plain, key, err := h.DB.CreateAPIKey(ctx(), uid, name, scope, expiresDays)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		out := evaluator.NewHash()
		out.SetString("plaintext", &evaluator.String{Value: plain})
		out.SetString("key", FromGo(key))
		return out
	}}
	b["db_revoke_api_key"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("db_revoke_api_key", 2, args); err != nil {
			return err
		}
		uid, ok1 := AsInt(args[0])
		kid, ok2 := AsInt(args[1])
		if !ok1 || !ok2 {
			return &evaluator.Error{Message: "expects (user_id, key_id)"}
		}
		if err := h.DB.RevokeAPIKey(ctx(), uid, kid); err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return evaluator.TRUE
	}}
	b["db_list_trusted"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("db_list_trusted", 1, args); err != nil {
			return err
		}
		uid, ok := AsInt(args[0])
		if !ok {
			return &evaluator.Error{Message: "user id must be int"}
		}
		rows, err := h.DB.ListTrustedPublishers(ctx(), uid)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(rows)
	}}
	b["db_create_trusted"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("db_create_trusted", 1, args); err != nil {
			return err
		}
		m, ok := args[0].(*evaluator.Hash)
		if !ok {
			return &evaluator.Error{Message: "expects hash"}
		}
		tp := database.TrustedPublisher{
			UserID:           mustInt(m.Get("user_id"), 0),
			Provider:         HashGetString(m, "provider"),
			RepositoryOwner:  HashGetString(m, "repository_owner"),
			RepositoryName:   HashGetString(m, "repository_name"),
			WorkflowFilename: HashGetString(m, "workflow_filename"),
			Environment:      HashGetString(m, "environment"),
			PackageScope:     HashGetString(m, "package_scope"),
		}
		if tp.Provider == "" {
			tp.Provider = "github_actions"
		}
		created, err := h.DB.CreateTrustedPublisher(ctx(), tp)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(created)
	}}
	b["db_delete_trusted"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("db_delete_trusted", 2, args); err != nil {
			return err
		}
		uid, ok1 := AsInt(args[0])
		id, ok2 := AsInt(args[1])
		if !ok1 || !ok2 {
			return &evaluator.Error{Message: "expects (user_id, id)"}
		}
		if err := h.DB.DeleteTrustedPublisher(ctx(), uid, id); err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return evaluator.TRUE
	}}
	b["db_match_trusted"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("db_match_trusted", 6, args); err != nil {
			return err
		}
		uid, _ := AsInt(args[0])
		pkg, _ := AsString(args[1])
		owner, _ := AsString(args[2])
		repo, _ := AsString(args[3])
		wf, _ := AsString(args[4])
		env, _ := AsString(args[5])
		tp, err := h.DB.MatchTrustedPublisher(ctx(), uid, pkg, owner, repo, wf, env)
		if err != nil {
			if errors.Is(err, database.ErrNotFound) {
				return evaluator.NULL
			}
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(tp)
	}}
	b["db_match_trusted_claims"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("db_match_trusted_claims", 5, args); err != nil {
			return err
		}
		pkg, _ := AsString(args[0])
		owner, _ := AsString(args[1])
		repo, _ := AsString(args[2])
		wf, _ := AsString(args[3])
		env, _ := AsString(args[4])
		tp, err := h.DB.MatchTrustedPublisherByClaims(ctx(), pkg, owner, repo, wf, env)
		if err != nil {
			if errors.Is(err, database.ErrNotFound) {
				return evaluator.NULL
			}
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(tp)
	}}
	b["db_explain_trusted_mismatch"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("db_explain_trusted_mismatch", 5, args); err != nil {
			return err
		}
		pkg, _ := AsString(args[0])
		owner, _ := AsString(args[1])
		repo, _ := AsString(args[2])
		wf, _ := AsString(args[3])
		env, _ := AsString(args[4])
		return &evaluator.String{Value: h.DB.ExplainTrustedPublisherMismatch(ctx(), pkg, owner, repo, wf, env)}
	}}
	b["db_mint_publish_token"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectMinArgs("db_mint_publish_token", 1, args); err != nil {
			return err
		}
		uid, ok := AsInt(args[0])
		if !ok {
			return &evaluator.Error{Message: "user_id required"}
		}
		ttl := int64(900)
		if len(args) > 1 {
			if n, ok := AsInt(args[1]); ok && n > 0 {
				ttl = n
			}
		}
		tok, sess, err := h.DB.CreateTrustedPublishToken(ctx(), uid, time.Duration(ttl)*time.Second)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		out := evaluator.NewHash()
		out.SetString("token", &evaluator.String{Value: tok})
		out.SetString("expires_at", &evaluator.String{Value: sess.ExpiresAt.UTC().Format(time.RFC3339)})
		out.SetString("expires_in", &evaluator.Integer{Value: ttl})
		out.SetString("token_type", &evaluator.String{Value: "Bearer"})
		return out
	}}
	b["db_publish"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("db_publish", 1, args); err != nil {
			return err
		}
		m, ok := args[0].(*evaluator.Hash)
		if !ok {
			return &evaluator.Error{Message: "db_publish expects hash"}
		}
		in := database.PublishInput{
			Name:              HashGetString(m, "name"),
			Description:       HashGetString(m, "description"),
			Author:            HashGetString(m, "author"),
			License:           HashGetString(m, "license"),
			Repository:        HashGetString(m, "repository"),
			Homepage:          HashGetString(m, "homepage"),
			Readme:            HashGetString(m, "readme"),
			Version:           HashGetString(m, "version"),
			Checksum:          HashGetString(m, "checksum"),
			StoragePath:       HashGetString(m, "storage_path"),
			Filename:          HashGetString(m, "filename"),
			FileSize:          mustInt(m.Get("file_size"), 0),
			ContentType:       HashGetString(m, "content_type"),
			OwnerID:           mustInt(m.Get("owner_id"), 0),
			PublishedByUserID: mustInt(m.Get("published_by_user_id"), 0),
			PublishedVia:      HashGetString(m, "published_via"),
		}
		if in.ContentType == "" {
			in.ContentType = "application/x-nexus-package"
		}
		if k := m.Get("keywords"); k != evaluator.NULL {
			if arr, ok := ToGo(k).([]any); ok {
				for _, v := range arr {
					if s, ok := v.(string); ok {
						in.Keywords = append(in.Keywords, s)
					}
				}
			}
		}
		if c := m.Get("categories"); c != evaluator.NULL {
			if arr, ok := ToGo(c).([]any); ok {
				for _, v := range arr {
					if s, ok := v.(string); ok {
						in.Categories = append(in.Categories, s)
					}
				}
			}
		}
		if tpid := m.Get("trusted_publisher_id"); tpid != evaluator.NULL {
			if id, ok := AsInt(tpid); ok {
				in.TrustedPublisherID = &id
			}
		}
		if deps := m.Get("dependencies"); deps != evaluator.NULL {
			if arr, ok := ToGo(deps).([]any); ok {
				for _, raw := range arr {
					if dm, ok := raw.(map[string]any); ok {
						di := database.DependencyInput{
							Name:       strVal(dm["name"]),
							VersionReq: strVal(dm["version_req"]),
							Optional:   boolVal(dm["optional"]),
							Dev:        boolVal(dm["dev"]),
						}
						in.Dependencies = append(in.Dependencies, di)
					}
				}
			}
		}
		pv, err := h.DB.UpsertPackageVersion(ctx(), in)
		if err != nil {
			if errors.Is(err, database.ErrConflict) {
				return &evaluator.Error{Message: "conflict"}
			}
			return &evaluator.Error{Message: err.Error()}
		}
		if in.PublishedByUserID > 0 {
			if err := h.DB.RecordPublishSuccess(ctx(), in.PublishedByUserID); err != nil {
				h.Logger.Error("record publish rate limit", "error", err, "user_id", in.PublishedByUserID)
			}
		}
		h.IncPublish()
		return FromGo(pv)
	}}
	b["db_check_publish_rate_limit"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_check_publish_rate_limit"); err != nil {
			return err
		}
		if err := ExpectArgs("db_check_publish_rate_limit", 1, args); err != nil {
			return err
		}
		userID, ok := AsInt(args[0])
		if !ok {
			return &evaluator.Error{Message: "db_check_publish_rate_limit expects user id"}
		}
		cooldown := time.Duration(h.Cfg.PublishRateLimitMinutes) * time.Minute
		st, err := h.DB.CheckPublishRateLimit(ctx(), userID, cooldown)
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		out := evaluator.NewHash()
		if st.Allowed {
			out.SetString("allowed", evaluator.TRUE)
		} else {
			out.SetString("allowed", evaluator.FALSE)
		}
		out.SetString("retry_after_seconds", &evaluator.Integer{Value: st.RetryAfterSeconds})
		out.SetString("cooldown_minutes", &evaluator.Integer{Value: st.CooldownMinutes})
		out.SetString("seconds_since_last", &evaluator.Integer{Value: st.SecondsSinceLast})
		if st.LastSuccessAt != nil {
			out.SetString("last_success_at", &evaluator.String{Value: st.LastSuccessAt.UTC().Format(time.RFC3339)})
		}
		return out
	}}
	b["db_record_publish_success"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := requireDB("db_record_publish_success"); err != nil {
			return err
		}
		if err := ExpectArgs("db_record_publish_success", 1, args); err != nil {
			return err
		}
		userID, ok := AsInt(args[0])
		if !ok {
			return &evaluator.Error{Message: "db_record_publish_success expects user id"}
		}
		if err := h.DB.RecordPublishSuccess(ctx(), userID); err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return evaluator.NULL
	}}
	b["db_yank_version"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("db_yank_version", 4, args); err != nil {
			return err
		}
		name, ok := AsString(args[0])
		if !ok {
			return &evaluator.Error{Message: "package name must be string"}
		}
		version, ok := AsString(args[1])
		if !ok {
			return &evaluator.Error{Message: "version must be string"}
		}
		ownerID, ok := AsInt(args[2])
		if !ok {
			return &evaluator.Error{Message: "owner id must be int"}
		}
		reason, _ := AsString(args[3])
		pv, err := h.DB.YankPackageVersion(ctx(), name, version, ownerID, reason)
		if err != nil {
			if errors.Is(err, database.ErrNotFound) {
				return &evaluator.Error{Message: "not found"}
			}
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(pv)
	}}
	b["db_unyank_version"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectArgs("db_unyank_version", 3, args); err != nil {
			return err
		}
		name, ok1 := AsString(args[0])
		version, ok2 := AsString(args[1])
		ownerID, ok3 := AsInt(args[2])
		if !ok1 || !ok2 || !ok3 {
			return &evaluator.Error{Message: "expects (name, version, owner_id)"}
		}
		pv, err := h.DB.UnyankPackageVersion(ctx(), name, version, ownerID)
		if err != nil {
			if errors.Is(err, database.ErrNotFound) {
				return &evaluator.Error{Message: "not found"}
			}
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(pv)
	}}
	b["db_deprecate_version"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectMinArgs("db_deprecate_version", 4, args); err != nil {
			return err
		}
		name, ok1 := AsString(args[0])
		version, ok2 := AsString(args[1])
		ownerID, ok3 := AsInt(args[2])
		deprecated := AsBool(args[3])
		message := ""
		if len(args) > 4 {
			message, _ = AsString(args[4])
		}
		if !ok1 || !ok2 || !ok3 {
			return &evaluator.Error{Message: "expects (name, version, owner_id, deprecated[, message])"}
		}
		pv, err := h.DB.SetVersionDeprecated(ctx(), name, version, ownerID, deprecated, message)
		if err != nil {
			if errors.Is(err, database.ErrNotFound) {
				return &evaluator.Error{Message: "not found"}
			}
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(pv)
	}}
	b["db_list_reverse_deps"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectMinArgs("db_list_reverse_deps", 1, args); err != nil {
			return err
		}
		name, ok := AsString(args[0])
		if !ok {
			return &evaluator.Error{Message: "package name must be string"}
		}
		limit := int64(50)
		if len(args) > 1 {
			if n, ok := AsInt(args[1]); ok {
				limit = n
			}
		}
		deps, err := h.DB.ListReverseDependencies(ctx(), name, int(limit))
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(deps)
	}}
	b["db_list_daily_downloads"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		if err := ExpectMinArgs("db_list_daily_downloads", 1, args); err != nil {
			return err
		}
		id, ok := AsInt(args[0])
		if !ok {
			return &evaluator.Error{Message: "package id must be int"}
		}
		days := int64(30)
		if len(args) > 1 {
			if n, ok := AsInt(args[1]); ok {
				days = n
			}
		}
		rows, err := h.DB.ListDailyDownloads(ctx(), id, int(days))
		if err != nil {
			return &evaluator.Error{Message: err.Error()}
		}
		return FromGo(rows)
	}}

	_ = time.Now
	_ = strings.TrimSpace
}

func strVal(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func boolVal(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true" || t == "1" || t == "yes"
	default:
		return false
	}
}

func (h *Host) registerAuthBuiltins(b map[string]*evaluator.Builtin) {
	// placeholder for future auth helpers; resolveAuth is used from HTTP layer
}
