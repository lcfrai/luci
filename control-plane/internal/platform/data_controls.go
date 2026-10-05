package platform

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var dataNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)
var bucketPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)
var dataTypes = map[string]string{"text": "text", "uuid": "uuid", "boolean": "boolean", "integer": "integer", "bigint": "bigint", "numeric": "numeric", "timestamptz": "timestamptz", "jsonb": "jsonb"}

func readDataResponse(res *http.Response) ([]byte, error) {
	b, e := io.ReadAll(io.LimitReader(res.Body, (8<<20)+1))
	if e != nil || len(b) > 8<<20 {
		return nil, errors.New("data response exceeds limit")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, errors.New("data service rejected request")
	}
	return b, nil
}
func (s *Service) registerDataControls(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/platform/projects/{id}/data/users", s.dataUsers)
	mux.HandleFunc("POST /v1/platform/projects/{id}/data/users", s.dataUsers)
	mux.HandleFunc("GET /v1/platform/projects/{id}/data/tables", s.dataTables)
	mux.HandleFunc("POST /v1/platform/projects/{id}/data/tables", s.dataTables)
	mux.HandleFunc("GET /v1/platform/projects/{id}/data/tables/{table}/rows", s.dataRows)
	mux.HandleFunc("POST /v1/platform/projects/{id}/data/tables/{table}/rows", s.dataRows)
	mux.HandleFunc("GET /v1/platform/projects/{id}/data/tables/{table}/rls", s.dataRLS)
	mux.HandleFunc("POST /v1/platform/projects/{id}/data/tables/{table}/rls", s.dataRLS)
	mux.HandleFunc("GET /v1/platform/projects/{id}/data/buckets", s.dataBuckets)
	mux.HandleFunc("POST /v1/platform/projects/{id}/data/buckets", s.dataBuckets)
	mux.HandleFunc("GET /v1/platform/projects/{id}/data/buckets/{bucket}/objects", s.dataObjects)
	mux.HandleFunc("POST /v1/platform/projects/{id}/data/buckets/{bucket}/objects/{path...}", s.dataObjects)
	mux.HandleFunc("GET /v1/platform/projects/{id}/data/buckets/{bucket}/objects/{path...}", s.dataObjects)
	mux.HandleFunc("GET /v1/platform/projects/{id}/data/realtime", s.dataRealtime)
	mux.HandleFunc("POST /v1/platform/projects/{id}/data/realtime", s.dataRealtime)
	s.registerDataFunctions(mux)
}
func (s *Service) readyData(w http.ResponseWriter, r *http.Request) (dataInstallation, dataSecret, bool) {
	p, ok := s.project(w, r)
	if !ok {
		return dataInstallation{}, dataSecret{}, false
	}
	s.mu.Lock()
	v, e := s.loadData(p.ID)
	s.mu.Unlock()
	if e != nil || v.Status != "ready" || (v.Operation != nil && v.Operation.Status == "running") {
		problem(w, 409, "data_not_ready", "Start or refresh the data stack until its services are ready before using data controls.")
		return dataInstallation{}, dataSecret{}, false
	}
	var install dataInstallation
	if s.read(p.ID+"-data-installation", &install) != nil || install.ProjectID != p.ID {
		problem(w, 500, "data_state_unavailable", "The private data binding could not be read.")
		return install, dataSecret{}, false
	}
	secret, e := s.dataCredentials(install)
	if e != nil {
		problem(w, 500, "data_state_unavailable", "The private data binding could not be read.")
		return install, secret, false
	}
	return install, secret, true
}
func (s *Service) dataControlClaim(w http.ResponseWriter, r *http.Request, id string) bool {
	if !s.claim(id, operationLimit(r)) {
		problem(w, 409, "operation_running", "A platform operation is already running for this project.")
		return false
	}
	return true
}
func dataControlError(w http.ResponseWriter) {
	problem(w, 502, "data_operation_failed", "The data service rejected this operation. Check the supplied values and refresh service health. Existing data is retained.")
}
func sqlIdentifier(v string) string { return `"` + v + `"` }
func sqlLiteral(v string) string    { return `'` + strings.ReplaceAll(v, `'`, `''`) + `'` }
func jsonSQL(ctx context.Context, s *Service, install dataInstallation, sql string) (any, error) {
	b, e := s.dataSQL(ctx, install, sql)
	if e != nil {
		return nil, e
	}
	var out any
	if json.Unmarshal([]byte(strings.TrimSpace(string(b))), &out) != nil {
		return nil, errors.New("invalid database JSON response")
	}
	return out, nil
}

func (s *Service) dataUsers(w http.ResponseWriter, r *http.Request) {
	i, secret, ok := s.readyData(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	method, path, input := r.Method, "/auth/v1/admin/users?page=1&per_page=100", []byte(nil)
	if method == "POST" {
		var in struct {
			Email        string `json:"email"`
			Password     string `json:"password"`
			EmailConfirm bool   `json:"emailConfirm"`
		}
		if !body(w, r, &in) {
			return
		}
		address, e := mail.ParseAddress(in.Email)
		if e != nil || address.Address != in.Email || len(in.Email) > 254 || len(in.Password) < 12 || len(in.Password) > 256 || strings.ContainsAny(in.Password, "\x00\r\n") {
			problem(w, 400, "invalid_user", "Use a valid email address and a password between 12 and 256 characters.")
			return
		}
		if !in.EmailConfirm {
			problem(w, 400, "email_delivery_unconfigured", "Create a confirmed user for now. Configure SMTP before sending signup or recovery email.")
			return
		}
		if !s.dataControlClaim(w, r, i.ProjectID) {
			return
		}
		defer s.release(i.ProjectID)
		path = "/auth/v1/admin/users"
		input, _ = json.Marshal(map[string]any{"email": in.Email, "password": in.Password, "email_confirm": true})
	}
	b, e := s.dataHTTP(ctx, i, secret, method, path, input, "application/json")
	if e != nil {
		dataControlError(w)
		return
	}
	var all struct {
		Users []struct {
			ID          string `json:"id"`
			Email       string `json:"email"`
			CreatedAt   string `json:"created_at"`
			ConfirmedAt string `json:"email_confirmed_at"`
		}
	}
	if method == "GET" {
		if json.Unmarshal(b, &all) != nil {
			dataControlError(w)
			return
		}
		respond(w, 200, map[string]any{"users": all.Users, "page": 1, "limit": 100})
		return
	}
	var user struct {
		ID          string `json:"id"`
		Email       string `json:"email"`
		CreatedAt   string `json:"created_at"`
		ConfirmedAt string `json:"email_confirmed_at"`
	}
	if json.Unmarshal(b, &user) != nil || user.ID == "" {
		dataControlError(w)
		return
	}
	respond(w, 201, user)
}

type DataColumn struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
}

func (s *Service) dataTables(w http.ResponseWriter, r *http.Request) {
	i, _, ok := s.readyData(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	if r.Method == "POST" {
		var in struct {
			Name      string       `json:"name"`
			Columns   []DataColumn `json:"columns"`
			EnableRLS bool         `json:"enableRls"`
		}
		if !body(w, r, &in) {
			return
		}
		if !dataNamePattern.MatchString(in.Name) || len(in.Columns) < 1 || len(in.Columns) > 32 {
			problem(w, 400, "invalid_table", "Use a lower-case table name and between 1 and 32 columns.")
			return
		}
		seen := map[string]bool{"id": true}
		definitions := []string{`"id" uuid PRIMARY KEY DEFAULT gen_random_uuid()`}
		for _, c := range in.Columns {
			typ := dataTypes[c.Type]
			if !dataNamePattern.MatchString(c.Name) || seen[c.Name] || typ == "" {
				problem(w, 400, "invalid_column", "Column names must be unique and lower-case. Choose one of the supported data types; id is automatically generated.")
				return
			}
			seen[c.Name] = true
			value := sqlIdentifier(c.Name) + " " + typ
			if !c.Nullable {
				value += " NOT NULL"
			}
			definitions = append(definitions, value)
		}
		if !in.EnableRLS {
			problem(w, 400, "rls_required", "New API tables require row-level security. Add explicit policies before exposing their rows to your application.")
			return
		}
		if !s.dataControlClaim(w, r, i.ProjectID) {
			return
		}
		defer s.release(i.ProjectID)
		sql := "BEGIN; CREATE TABLE public." + sqlIdentifier(in.Name) + " (" + strings.Join(definitions, ",") + "); ALTER TABLE public." + sqlIdentifier(in.Name) + " ENABLE ROW LEVEL SECURITY; GRANT SELECT,INSERT,UPDATE,DELETE ON public." + sqlIdentifier(in.Name) + " TO anon,authenticated; NOTIFY pgrst,'reload schema'; COMMIT;"
		if _, e := s.dataSQL(ctx, i, sql); e != nil {
			dataControlError(w)
			return
		}
		respond(w, 201, map[string]any{"name": in.Name, "rlsEnabled": true, "columns": append([]DataColumn{{Name: "id", Type: "uuid"}}, in.Columns...)})
		return
	}
	out, e := jsonSQL(ctx, s, i, `SELECT COALESCE(json_agg(t ORDER BY name),'[]'::json) FROM (SELECT c.relname AS name,c.relrowsecurity AS "rlsEnabled",(SELECT COALESCE(json_agg(json_build_object('name',a.attname,'type',format_type(a.atttypid,a.atttypmod),'nullable',NOT a.attnotnull) ORDER BY a.attnum),'[]'::json) FROM pg_attribute a WHERE a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped) AS columns FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relkind='r') t;`)
	if e != nil {
		dataControlError(w)
		return
	}
	respond(w, 200, map[string]any{"tables": out})
}
func (s *Service) tableExists(ctx context.Context, i dataInstallation, name string) bool {
	if !dataNamePattern.MatchString(name) {
		return false
	}
	b, e := s.dataSQL(ctx, i, "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relkind='r' AND c.relname="+sqlLiteral(name)+") THEN 1 ELSE 0 END;")
	return e == nil && strings.TrimSpace(string(b)) == "1"
}
func (s *Service) dataRows(w http.ResponseWriter, r *http.Request) {
	i, _, ok := s.readyData(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	table := r.PathValue("table")
	if !s.tableExists(ctx, i, table) {
		problem(w, 404, "table_unavailable", "Choose an existing public project table.")
		return
	}
	if r.Method == "POST" {
		var row map[string]json.RawMessage
		if !body(w, r, &row) {
			return
		}
		if len(row) < 1 || len(row) > 32 {
			problem(w, 400, "invalid_row", "Supply between 1 and 32 column values.")
			return
		}
		columns := []string{}
		for name := range row {
			if !dataNamePattern.MatchString(name) {
				problem(w, 400, "invalid_row", "Use valid column names.")
				return
			}
			columns = append(columns, sqlIdentifier(name))
		}
		data, _ := json.Marshal(row)
		if len(data) > 64<<10 {
			problem(w, 400, "row_too_large", "Rows may contain at most 64 KiB of JSON.")
			return
		}
		if !s.dataControlClaim(w, r, i.ProjectID) {
			return
		}
		defer s.release(i.ProjectID)
		list := strings.Join(columns, ",")
		query := "WITH inserted AS (INSERT INTO public." + sqlIdentifier(table) + " (" + list + ") SELECT " + list + " FROM json_populate_record(NULL::public." + sqlIdentifier(table) + "," + sqlLiteral(string(data)) + "::json) RETURNING *) SELECT row_to_json(inserted) FROM inserted;"
		out, e := jsonSQL(ctx, s, i, query)
		if e != nil {
			dataControlError(w)
			return
		}
		respond(w, 201, out)
		return
	}
	out, e := jsonSQL(ctx, s, i, "SELECT COALESCE(json_agg(t),'[]'::json) FROM (SELECT * FROM public."+sqlIdentifier(table)+" LIMIT 100) t;")
	if e != nil {
		dataControlError(w)
		return
	}
	respond(w, 200, map[string]any{"rows": out, "limit": 100, "access": "project-administrator"})
}
func (s *Service) dataRLS(w http.ResponseWriter, r *http.Request) {
	i, _, ok := s.readyData(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	table := r.PathValue("table")
	if !s.tableExists(ctx, i, table) {
		problem(w, 404, "table_unavailable", "Choose an existing public project table.")
		return
	}
	if r.Method == "POST" {
		var in struct {
			Enabled bool `json:"enabled"`
			Policy  *struct {
				Name        string `json:"name"`
				Operation   string `json:"operation"`
				Role        string `json:"role"`
				Using       string `json:"using"`
				OwnerColumn string `json:"ownerColumn,omitempty"`
			} `json:"policy,omitempty"`
		}
		if !body(w, r, &in) {
			return
		}
		sql := "ALTER TABLE public." + sqlIdentifier(table)
		if in.Enabled {
			sql += " ENABLE ROW LEVEL SECURITY;"
		} else {
			if in.Policy != nil {
				problem(w, 400, "invalid_policy", "Enable row-level security before adding a policy.")
				return
			}
			sql += " DISABLE ROW LEVEL SECURITY;"
		}
		if in.Policy != nil {
			p := in.Policy
			if !dataNamePattern.MatchString(p.Name) || (p.Role != "anon" && p.Role != "authenticated") || (p.Operation != "select" && p.Operation != "insert" && p.Operation != "update" && p.Operation != "delete" && p.Operation != "all") || (p.Using != "authenticated" && p.Using != "owner") {
				problem(w, 400, "invalid_policy", "Use a named policy with a supported operation, role and access rule.")
				return
			}
			expression := "(auth.uid() IS NOT NULL)"
			if p.Using == "owner" {
				if !dataNamePattern.MatchString(p.OwnerColumn) {
					problem(w, 400, "invalid_policy", "Owner policies need a valid UUID owner column.")
					return
				}
				expression = "(auth.uid() = " + sqlIdentifier(p.OwnerColumn) + ")"
			}
			if p.Role == "anon" && p.Using == "authenticated" {
				problem(w, 400, "invalid_policy", "An authenticated-user rule cannot grant anonymous access.")
				return
			}
			sql += " CREATE POLICY " + sqlIdentifier(p.Name) + " ON public." + sqlIdentifier(table) + " FOR " + strings.ToUpper(p.Operation) + " TO " + sqlIdentifier(p.Role)
			if p.Operation != "insert" {
				sql += " USING " + expression
			}
			if p.Operation == "insert" || p.Operation == "update" || p.Operation == "all" {
				sql += " WITH CHECK " + expression
			}
			sql += ";"
		}
		if !s.dataControlClaim(w, r, i.ProjectID) {
			return
		}
		defer s.release(i.ProjectID)
		if _, e := s.dataSQL(ctx, i, "BEGIN;"+sql+"COMMIT;"); e != nil {
			dataControlError(w)
			return
		}
	}
	out, e := jsonSQL(ctx, s, i, "SELECT json_build_object('enabled',(SELECT relrowsecurity FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND relname="+sqlLiteral(table)+"),'policies',(SELECT COALESCE(json_agg(json_build_object('name',policyname,'operation',cmd,'roles',roles,'using',qual,'check',with_check)),'[]'::json) FROM pg_policies WHERE schemaname='public' AND tablename="+sqlLiteral(table)+"));")
	if e != nil {
		dataControlError(w)
		return
	}
	respond(w, 200, out)
}
func (s *Service) dataBuckets(w http.ResponseWriter, r *http.Request) {
	i, secret, ok := s.readyData(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	input := []byte(nil)
	if r.Method == "POST" {
		var in struct {
			Name   string `json:"name"`
			Public bool   `json:"public"`
		}
		if !body(w, r, &in) {
			return
		}
		if !bucketPattern.MatchString(in.Name) || strings.Contains(in.Name, "..") {
			problem(w, 400, "invalid_bucket", "Use a bucket name of up to 63 lower-case letters, digits, dots, dashes or underscores.")
			return
		}
		if !s.dataControlClaim(w, r, i.ProjectID) {
			return
		}
		defer s.release(i.ProjectID)
		input, _ = json.Marshal(map[string]any{"id": in.Name, "name": in.Name, "public": in.Public, "file_size_limit": 8 << 20})
	}
	b, e := s.dataHTTP(ctx, i, secret, r.Method, "/storage/v1/bucket", input, "application/json")
	if e != nil {
		dataControlError(w)
		return
	}
	var out any
	if json.Unmarshal(b, &out) != nil {
		dataControlError(w)
		return
	}
	code := 200
	if r.Method == "POST" {
		code = 201
	}
	respond(w, code, out)
}
func objectPath(value string) (string, bool) {
	if value == "" || len(value) > 512 || strings.ContainsAny(value, "\\\x00\r\n") {
		return "", false
	}
	parts := strings.Split(value, "/")
	for index, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", false
		}
		parts[index] = url.PathEscape(part)
	}
	return strings.Join(parts, "/"), true
}
func (s *Service) dataObjects(w http.ResponseWriter, r *http.Request) {
	i, secret, ok := s.readyData(w, r)
	if !ok {
		return
	}
	bucket := r.PathValue("bucket")
	if !bucketPattern.MatchString(bucket) || strings.Contains(bucket, "..") {
		problem(w, 400, "invalid_bucket", "Choose a valid project bucket.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	path := r.PathValue("path")
	if path == "" && r.Method == "GET" {
		prefix := r.URL.Query().Get("prefix")
		if prefix != "" {
			if _, ok := objectPath(strings.TrimSuffix(prefix, "/")); !ok {
				problem(w, 400, "invalid_prefix", "Choose a relative folder prefix with no parent or empty segments.")
				return
			}
		}
		input, _ := json.Marshal(map[string]any{"prefix": prefix, "limit": 100, "offset": 0, "sortBy": map[string]string{"column": "name", "order": "asc"}})
		b, e := s.dataHTTP(ctx, i, secret, "POST", "/storage/v1/object/list/"+url.PathEscape(bucket), input, "application/json")
		if e != nil {
			dataControlError(w)
			return
		}
		var out any
		if json.Unmarshal(b, &out) != nil {
			dataControlError(w)
			return
		}
		respond(w, 200, map[string]any{"objects": out, "limit": 100, "prefix": prefix})
		return
	}
	escaped, ok := objectPath(path)
	if !ok {
		problem(w, 400, "invalid_object", "Choose a relative object path with no empty or parent segments.")
		return
	}
	upstream := "/storage/v1/object/" + url.PathEscape(bucket) + "/" + escaped
	if r.Method == "POST" {
		if !s.dataControlClaim(w, r, i.ProjectID) {
			return
		}
		defer s.release(i.ProjectID)
		b, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 8<<20))
		if e != nil {
			problem(w, 413, "object_too_large", "Objects uploaded through Luci are limited to 8 MiB.")
			return
		}
		contentType := r.Header.Get("Content-Type")
		if typ, _, e := mime.ParseMediaType(contentType); e != nil || typ == "" {
			contentType = http.DetectContentType(b)
		}
		out, e := s.dataHTTP(ctx, i, secret, "POST", upstream, b, contentType)
		if e != nil {
			dataControlError(w)
			return
		}
		var result any
		if json.Unmarshal(out, &result) != nil {
			dataControlError(w)
			return
		}
		respond(w, 201, result)
		return
	}
	b, e := s.dataHTTP(ctx, i, secret, "GET", upstream, nil, "")
	if e != nil {
		dataControlError(w)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="object-download"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(b)
}

func (s *Service) dataRealtime(w http.ResponseWriter, r *http.Request) {
	i, _, ok := s.readyData(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	if r.Method == "POST" {
		var in struct {
			Table   string `json:"table"`
			Enabled bool   `json:"enabled"`
		}
		if !body(w, r, &in) {
			return
		}
		if !s.tableExists(ctx, i, in.Table) {
			problem(w, 404, "table_unavailable", "Choose an existing public project table.")
			return
		}
		if !s.dataControlClaim(w, r, i.ProjectID) {
			return
		}
		defer s.release(i.ProjectID)
		exists, e := s.dataSQL(ctx, i, "SELECT CASE WHEN EXISTS(SELECT 1 FROM pg_publication_tables WHERE pubname='supabase_realtime' AND schemaname='public' AND tablename="+sqlLiteral(in.Table)+") THEN 1 ELSE 0 END;")
		if e != nil {
			dataControlError(w)
			return
		}
		present := strings.TrimSpace(string(exists)) == "1"
		if present != in.Enabled {
			action := "ADD"
			if !in.Enabled {
				action = "DROP"
			}
			if _, e = s.dataSQL(ctx, i, "ALTER PUBLICATION supabase_realtime "+action+" TABLE public."+sqlIdentifier(in.Table)+";"); e != nil {
				dataControlError(w)
				return
			}
		}
	}
	out, e := jsonSQL(ctx, s, i, `SELECT COALESCE(json_agg(tablename ORDER BY tablename),'[]'::json) FROM pg_publication_tables WHERE pubname='supabase_realtime' AND schemaname='public';`)
	if e != nil {
		dataControlError(w)
		return
	}
	respond(w, 200, map[string]any{"tables": out, "publication": "supabase_realtime", "rlsRequired": true})
}
