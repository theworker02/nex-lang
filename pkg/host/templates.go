package host

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (h *Host) loadTemplates() error {
	if h.templates != nil {
		return nil
	}
	if h.Cfg.WebDir == "" {
		return fmt.Errorf("web directory not configured (set NEX_WEB_DIR or place ./web next to the app)")
	}

	tplDir := filepath.Join(h.Cfg.WebDir, "templates")
	entries, err := os.ReadDir(tplDir)
	if err != nil {
		return fmt.Errorf("read templates dir: %w", err)
	}
	pages := make([]string, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".html") {
			continue
		}
		// base.html is composed separately; skip non-page shells if any.
		if name == "base.html" {
			continue
		}
		pages = append(pages, name)
	}
	if len(pages) == 0 {
		return fmt.Errorf("no page templates found in %s", tplDir)
	}
	out := make(map[string]*template.Template, len(pages))

	baseSrc, err := os.ReadFile(filepath.Join(tplDir, "base.html"))
	if err != nil {
		return fmt.Errorf("read base template: %w", err)
	}

	partialDir := filepath.Join(tplDir, "partials")
	partialEntries, err := os.ReadDir(partialDir)
	if err != nil {
		return fmt.Errorf("read partials dir: %w", err)
	}
	partialSrcs := make([]string, 0, len(partialEntries))
	for _, pe := range partialEntries {
		if pe.IsDir() || !strings.HasSuffix(pe.Name(), ".html") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(partialDir, pe.Name()))
		if err != nil {
			return fmt.Errorf("read partial %s: %w", pe.Name(), err)
		}
		partialSrcs = append(partialSrcs, string(src))
	}

	funcMap := template.FuncMap{
		"formatBytes": formatBytes,
		"formatDate":  formatDate,
		"formatNumber": formatNumber,
		"initial": func(s string) string {
			s = strings.TrimSpace(s)
			if s == "" {
				return "?"
			}
			return strings.ToUpper(s[:1])
		},
	}

	for _, page := range pages {
		root := template.New("root").Funcs(funcMap)
		if _, err := root.Parse(string(baseSrc)); err != nil {
			return fmt.Errorf("parse base template: %w", err)
		}
		for i, src := range partialSrcs {
			if _, err := root.Parse(src); err != nil {
				return fmt.Errorf("parse partial %d: %w", i, err)
			}
		}
		cloned, err := root.Clone()
		if err != nil {
			return fmt.Errorf("clone base template: %w", err)
		}
		pageSrc, err := os.ReadFile(filepath.Join(tplDir, page))
		if err != nil {
			return fmt.Errorf("read template %s: %w", page, err)
		}
		if _, err := cloned.Parse(string(pageSrc)); err != nil {
			return fmt.Errorf("parse template %s: %w", page, err)
		}
		out[page] = cloned
	}
	h.templates = out
	return nil
}

func (h *Host) writeHTML(w http.ResponseWriter, r *http.Request, page string, status int, data map[string]any) {
	if err := h.loadTemplates(); err != nil {
		h.Logger.Error("load templates", "error", err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	tmpl, ok := h.templates[page]
	if !ok {
		http.Error(w, "template not found", http.StatusInternalServerError)
		return
	}

	// Normalize keys for templates that expect exported struct fields.
	normalized := normalizeTemplateData(data)
	if _, ok := normalized["GitHubAuthEnabled"]; !ok {
		normalized["GitHubAuthEnabled"] = h.githubOAuthConfigured()
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := tmpl.ExecuteTemplate(w, "base", normalized); err != nil {
		h.Logger.Error("execute template", "page", page, "error", err)
	}
}

func normalizeTemplateData(data map[string]any) map[string]any {
	out := make(map[string]any, len(data))
	for k, v := range data {
		out[k] = v
	}
	// Support both Title and title style from Nexus hashes.
	aliases := map[string]string{
		"title": "Title", "description": "Description", "active_nav": "ActiveNav",
		"query": "Query", "category": "Category", "result_count": "ResultCount",
		"package_count": "PackageCount", "version_count": "VersionCount",
		"packages": "Packages", "featured": "Featured", "recent": "Recent",
		"package": "Package", "versions": "Versions", "selected": "Selected",
		"dependencies": "Dependencies", "download_url": "DownloadURL",
		"install_command": "InstallCommand", "filename": "Filename",
		"readme_html": "ReadmeHTML", "doc_id": "DocID", "doc_title": "DocTitle",
		"doc_lead": "DocLead", "doc_html": "DocHTML", "status": "Status",
		"message": "Message", "is_search": "IsSearch", "is_docs": "IsDocs",
		"doc_section": "DocSection", "user": "User", "profile_user": "ProfileUser",
		"stats": "Stats", "api_keys": "APIKeys", "trusted_publishers": "TrustedPublishers",
		"audit_logs": "AuditLogs", "totp_pending_secret": "TOTPPendingSecret",
		"totp_otpauth_url": "TOTPOtpauthURL", "reports": "Reports",
		"status_filter": "StatusFilter", "submitted": "Submitted", "report_id": "ReportID",
		"current_user": "CurrentUser", "form_error": "FormError",
		"form_username": "FormUsername", "form_email": "FormEmail",
		"next": "Next", "flash": "Flash", "flash_err": "FlashErr", "new_api_key": "NewAPIKey", "page": "Page",
		"challenge": "Challenge",
		"download_count": "DownloadCount", "user_count": "UserCount", "tags": "Tags",
		"prev_page": "PrevPage", "next_page": "NextPage",
		"categories": "Categories", "keywords": "Keywords", "docs_url": "DocsURL",
		"docs_html": "DocsHTML", "has_docs": "HasDocs", "package_url": "PackageURL",
		"is_version_docs": "IsVersionDocs", "is_legal": "IsLegal", "legal_id": "LegalID",
		"owners": "Owners", "activity": "Activity", "org": "Org", "orgs": "Orgs",
		"members": "Members", "teams": "Teams", "can_manage": "CanManage",
		"package_name": "PackageName", "token": "Token",
		"legal_title": "LegalTitle", "legal_lead": "LegalLead", "legal_html": "LegalHTML",
		"show_dmca_form": "ShowDmcaForm", "dmca_submitted": "DmcaSubmitted",
		"dmca_error": "DmcaError", "mailto_url": "MailtoURL",
		"is_curated": "IsCurated", "selected_slug": "SelectedSlug",
		"selected_category": "SelectedCategory", "is_category_detail": "IsCategoryDetail",
		"is_filtered": "IsFiltered", "is_keyword_detail": "IsKeywordDetail",
		"keyword": "Keyword", "license": "License", "licenses": "Licenses",
		"sort": "Sort", "updated_after": "UpdatedAfter", "form_action": "FormAction",
		"page_num": "PageNum", "per_page": "PerPage",
	}
	for from, to := range aliases {
		if _, has := out[to]; !has {
			if v, ok := out[from]; ok {
				out[to] = v
			}
		}
	}
	if rh, ok := out["ReadmeHTML"].(string); ok {
		out["ReadmeHTML"] = template.HTML(rh)
	}
	if dh, ok := out["DocHTML"].(string); ok {
		out["DocHTML"] = template.HTML(dh)
	}
	if lh, ok := out["LegalHTML"].(string); ok {
		out["LegalHTML"] = template.HTML(lh)
	}
	if vh, ok := out["DocsHTML"].(string); ok {
		out["DocsHTML"] = template.HTML(vh)
	}
	return out
}

func formatBytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	units := []string{"KiB", "MiB", "GiB"}
	val := float64(n)
	for _, unit := range units {
		val /= 1024
		if val < 1024 {
			return fmt.Sprintf("%.1f %s", val, unit)
		}
	}
	return fmt.Sprintf("%.1f GiB", val)
}

func formatDate(v any) string {
	switch t := v.(type) {
	case time.Time:
		if t.IsZero() {
			return ""
		}
		return t.UTC().Format("2006-01-02")
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return ""
		}
		if parsed, err := time.Parse(time.RFC3339, s); err == nil {
			return parsed.UTC().Format("2006-01-02")
		}
		if len(s) >= 10 {
			return s[:10]
		}
		return s
	default:
		return fmt.Sprint(v)
	}
}

func formatNumber(v any) string {
	var n int64
	switch t := v.(type) {
	case int64:
		n = t
	case int:
		n = int64(t)
	case float64:
		n = int64(t)
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		if err != nil {
			return t
		}
		n = parsed
	default:
		return fmt.Sprint(v)
	}
	if n < 0 {
		return fmt.Sprintf("%d", n)
	}
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	rem := len(s) % 3
	if rem == 0 {
		rem = 3
	}
	b.WriteString(s[:rem])
	for i := rem; i < len(s); i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	return b.String()
}
