//go:build e2e && pipeline

package pipeline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

const (
	// mockCustomerID is the fixed Osano customer that owns every config the mock creates.
	mockCustomerID = "PipelineE2ECustomer"
	// mockAPIKey is the only key the mock accepts. It is a test fixture, not a credential.
	mockAPIKey = "pipeline-e2e-api-key" //nolint:gosec // Test fixture for a local mock server.

	apiKeyHeader = "x-osano-api-key" //nolint:gosec // HTTP header name, not a credential.

	statusUnpublished = "unpublished"
	statusInProgress  = "in-progress"
	statusPublished   = "published"
	statusOutdated    = "outdated"

	configsPath = "/v1/cookie-consent/configs"
	rulesPath   = "/v1/cookie-consent/rules"
)

// ruleStoreTypes maps the store type keys of a rule create body to the singular type Osano reports.
var ruleStoreTypes = map[string]string{
	"cookies":      "cookie",
	"scripts":      "script",
	"iframes":      "iframe",
	"localStorage": "localStorage",
}

var ruleClassifications = map[string]bool{
	"ANALYTICS": true, "BLACKLISTED": true, "ESSENTIAL": true,
	"HIDDEN": true, "MARKETING": true, "PERSONALIZATION": true,
}

// recordedRequest is one request the mock received, in arrival order.
type recordedRequest struct {
	Seq       int
	Method    string
	Path      string
	Query     string
	UserAgent string
	APIKeyOK  bool
	Body      string
	Status    int
	Routed    bool
}

func (r recordedRequest) String() string {
	return fmt.Sprintf("#%d %s %s?%s -> %d (UA %q)", r.Seq, r.Method, r.Path, r.Query, r.Status, r.UserAgent)
}

type mockConfig struct {
	ConfigID          string
	Name              string
	Domains           []string
	Mode              string
	OrgIDs            []string
	Configuration     map[string]any
	Created           int64
	Updated           int64
	PublishStatus     string
	LastPublished     int64
	PublishedRevision int

	// revision counts content changes (config PATCHes and rule changes); a publish records it.
	revision int
	// inProgressReads is how many more GETs report in-progress before the publication completes.
	inProgressReads int
	publishBodies   []map[string]any
}

type mockRule struct {
	RuleID         int
	ConfigID       string
	Type           string
	Classification string
	Rule           string
	Disclosure     bool
	Title          *string
	VendorName     *string
	RuleType       *string
	Description    *string
	Expiry         *string
	Created        string
	Updated        string
}

// mockOsano is a stateful in-process stand-in for the Osano Customer REST API endpoints the
// provider's Cookie Consent resources and functions call.
type mockOsano struct {
	server *httptest.Server

	mu         sync.Mutex
	configs    map[string]*mockConfig
	rules      map[int]*mockRule
	nextRuleID int
	defaults   map[string]any
	requests   []recordedRequest
	lastTick   int64
}

// serverDefaultConfiguration is what Osano adds to every configuration: a top-level key the program
// never sets, and a palette whose keys the program sets only partially.
func serverDefaultConfiguration() map[string]any {
	return map[string]any{
		"showWidget":               true,
		"managePreferencesEnabled": true,
		"timeoutSeconds":           10,
		"palette": map[string]any{
			"buttonBackgroundColor":  "#1A1A1A",
			"buttonForegroundColor":  "#FFFFFF",
			"dialogBackgroundColor":  "#FFFFFF",
			"dialogForegroundColor":  "#1A1A1A",
			"infoDialogOverlayColor": "rgba(0,0,0,0.45)",
			"widgetPosition":         "right",
		},
	}
}

func newMockOsano(t *testing.T) *mockOsano {
	t.Helper()
	m := &mockOsano{
		configs:    map[string]*mockConfig{},
		rules:      map[int]*mockRule{},
		nextRuleID: 4100,
		defaults:   serverDefaultConfiguration(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST "+configsPath, m.createConfig)
	mux.HandleFunc("GET "+configsPath, m.listConfigs)
	mux.HandleFunc("GET "+configsPath+"/{configId}", m.getConfig)
	mux.HandleFunc("PATCH "+configsPath+"/{configId}", m.patchConfig)
	mux.HandleFunc("POST "+configsPath+"/{configId}/publish", m.publishConfig)
	mux.HandleFunc("GET "+configsPath+"/{configId}/rules", m.listRules)
	mux.HandleFunc("POST "+rulesPath, m.createRules)
	mux.HandleFunc("PATCH "+rulesPath+"/{ruleId}", m.patchRule)
	mux.HandleFunc("DELETE "+rulesPath+"/{ruleId}", m.deleteRule)

	m.server = httptest.NewServer(m.record(mux))
	t.Cleanup(m.server.Close)
	// A call the mock does not model, or one without the configured key, would otherwise surface
	// only indirectly (for example a 404 that the provider reads as a deleted resource).
	t.Cleanup(func() {
		for _, r := range m.RequestsSince(0) {
			if !r.Routed || !r.APIKeyOK {
				t.Errorf("unexpected request to the mock Osano API (routed=%v, apiKeyOK=%v): %s", r.Routed, r.APIKeyOK, r)
			}
		}
	})
	return m
}

// URL is the Customer REST API base URL to configure as osano:customerBaseUrl.
func (m *mockOsano) URL() string {
	return m.server.URL
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(status int) {
	s.status = status
	s.ResponseWriter.WriteHeader(status)
}

func (m *mockOsano) record(next *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))

		_, pattern := next.Handler(r)
		entry := recordedRequest{
			Method:    r.Method,
			Path:      r.URL.Path,
			Query:     r.URL.RawQuery,
			UserAgent: r.Header.Get("User-Agent"),
			APIKeyOK:  r.Header.Get(apiKeyHeader) == mockAPIKey,
			Body:      string(body),
			Routed:    pattern != "",
		}

		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		if entry.APIKeyOK {
			next.ServeHTTP(recorder, r)
		} else {
			writeJSON(recorder, http.StatusUnauthorized, map[string]any{"message": "invalid API key"})
		}
		entry.Status = recorder.status

		m.mu.Lock()
		entry.Seq = len(m.requests)
		m.requests = append(m.requests, entry)
		m.mu.Unlock()
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"statusCode": status, "message": message})
}

// tick returns a unix timestamp that is strictly greater than the previous one, so `updated`
// visibly changes on every write even within the same second.
func (m *mockOsano) tick() int64 {
	now := time.Now().Unix()
	if now <= m.lastTick {
		now = m.lastTick + 1
	}
	m.lastTick = now
	return now
}

func (m *mockOsano) isoTick() string {
	return time.Unix(m.tick(), 0).UTC().Format(time.RFC3339)
}

func (m *mockOsano) configView(c *mockConfig) map[string]any {
	orgIDs := c.OrgIDs
	if orgIDs == nil {
		orgIDs = []string{}
	}
	return map[string]any{
		"configId":            c.ConfigID,
		"customerId":          mockCustomerID,
		"name":                c.Name,
		"domains":             c.Domains,
		"mode":                c.Mode,
		"orgIds":              orgIDs,
		"configuration":       mergeDefaults(m.defaults, c.Configuration),
		"created":             c.Created,
		"updated":             c.Updated,
		"publishStatus":       c.PublishStatus,
		"lastPublished":       c.LastPublished,
		"publishedRevision":   c.PublishedRevision,
		"tattleRecordStopped": false,
	}
}

func ruleView(r *mockRule) map[string]any {
	return map[string]any{
		"ruleId":         r.RuleID,
		"configId":       r.ConfigID,
		"type":           r.Type,
		"classification": r.Classification,
		"rule":           r.Rule,
		"disclosure":     r.Disclosure,
		"title":          r.Title,
		"vendorName":     r.VendorName,
		"ruleType":       r.RuleType,
		"description":    r.Description,
		"expiry":         r.Expiry,
		"vendorId":       nil,
		"created":        r.Created,
		"updated":        r.Updated,
	}
}

// mergeDefaults returns a deep copy of defaults overlaid with the configuration the client stored.
func mergeDefaults(defaults, stored map[string]any) map[string]any {
	merged := deepCopyMap(defaults)
	for key, value := range stored {
		storedMap, storedIsMap := value.(map[string]any)
		defaultMap, defaultIsMap := merged[key].(map[string]any)
		if storedIsMap && defaultIsMap {
			merged[key] = mergeDefaults(defaultMap, storedMap)
			continue
		}
		merged[key] = deepCopyValue(value)
	}
	return merged
}

func deepCopyMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = deepCopyValue(value)
	}
	return out
}

func deepCopyValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return deepCopyMap(typed)
	case []any:
		out := make([]any, len(typed))
		for i := range typed {
			out[i] = deepCopyValue(typed[i])
		}
		return out
	default:
		return value
	}
}

// markChanged records a content change; a published config becomes outdated until republished.
func (c *mockConfig) markChanged() {
	c.revision++
	if c.PublishStatus == statusPublished {
		c.PublishStatus = statusOutdated
	}
}

type configBody struct {
	Name          *string        `json:"name"`
	Domains       []string       `json:"domains"`
	Mode          *string        `json:"mode"`
	OrgIDs        []string       `json:"orgIds"`
	Configuration map[string]any `json:"configuration"`
}

func validMode(mode string) bool {
	return mode == "debug" || mode == "permissive" || mode == "production"
}

func (m *mockOsano) createConfig(w http.ResponseWriter, r *http.Request) {
	var body configBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	switch {
	case body.Name == nil || *body.Name == "":
		writeError(w, http.StatusBadRequest, "name is required")
		return
	case len(body.Domains) == 0:
		writeError(w, http.StatusBadRequest, "domains is required")
		return
	case body.Mode == nil || !validMode(*body.Mode):
		writeError(w, http.StatusBadRequest, "mode must be debug, permissive, or production")
		return
	case body.Configuration["storagePolicyHref"] == nil:
		writeError(w, http.StatusBadRequest, "configuration.storagePolicyHref is required")
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.tick()
	config := &mockConfig{
		ConfigID:      uuid.NewString(),
		Name:          *body.Name,
		Domains:       body.Domains,
		Mode:          *body.Mode,
		OrgIDs:        body.OrgIDs,
		Configuration: deepCopyMap(body.Configuration),
		Created:       now,
		Updated:       now,
		PublishStatus: statusUnpublished,
		revision:      1,
	}
	m.configs[config.ConfigID] = config
	writeJSON(w, http.StatusCreated, m.configView(config))
}

func (m *mockOsano) listConfigs(w http.ResponseWriter, _ *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	items := make([]map[string]any, 0, len(m.configs))
	for _, id := range m.sortedConfigIDsLocked() {
		items = append(items, m.configView(m.configs[id]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "next": ""})
}

func (m *mockOsano) getConfig(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	config, ok := m.configs[r.PathValue("configId")]
	if !ok {
		writeError(w, http.StatusNotFound, "config not found")
		return
	}
	// A queued publication reports in-progress for one read, then completes on the next one.
	if config.PublishStatus == statusInProgress {
		if config.inProgressReads > 0 {
			config.inProgressReads--
		} else {
			config.PublishStatus = statusPublished
			config.LastPublished = m.tick()
			config.PublishedRevision = config.revision
		}
	}
	writeJSON(w, http.StatusOK, m.configView(config))
}

func (m *mockOsano) patchConfig(w http.ResponseWriter, r *http.Request) {
	var body configBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if body.Mode != nil && !validMode(*body.Mode) {
		writeError(w, http.StatusBadRequest, "mode must be debug, permissive, or production")
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	config, ok := m.configs[r.PathValue("configId")]
	if !ok {
		writeError(w, http.StatusNotFound, "config not found")
		return
	}
	if body.Name != nil {
		config.Name = *body.Name
	}
	if body.Domains != nil {
		config.Domains = body.Domains
	}
	if body.Mode != nil {
		config.Mode = *body.Mode
	}
	if body.OrgIDs != nil {
		config.OrgIDs = body.OrgIDs
	}
	if body.Configuration != nil {
		config.Configuration = deepCopyMap(body.Configuration)
	}
	config.Updated = m.tick()
	config.markChanged()
	writeJSON(w, http.StatusOK, m.configView(config))
}

func (m *mockOsano) publishConfig(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	config, ok := m.configs[r.PathValue("configId")]
	if !ok {
		writeError(w, http.StatusNotFound, "config not found")
		return
	}
	if config.PublishStatus == statusInProgress {
		writeError(w, http.StatusConflict, "a publication is already in progress")
		return
	}
	config.PublishStatus = statusInProgress
	config.inProgressReads = 1
	config.publishBodies = append(config.publishBodies, body)
	w.WriteHeader(http.StatusNoContent)
}

func (m *mockOsano) listRules(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	configID := r.PathValue("configId")
	if _, ok := m.configs[configID]; !ok {
		writeError(w, http.StatusNotFound, "config not found")
		return
	}
	query := r.URL.Query()
	limit, err := strconv.Atoi(query.Get("limit"))
	if err != nil || limit <= 0 || limit > 500 {
		limit = 500
	}
	offset := 0
	if next := query.Get("next"); next != "" {
		if offset, err = strconv.Atoi(next); err != nil || offset < 0 {
			writeError(w, http.StatusBadRequest, "invalid next cursor")
			return
		}
	}

	matching := []*mockRule{}
	for _, id := range m.sortedRuleIDsLocked() {
		rule := m.rules[id]
		if rule.ConfigID != configID {
			continue
		}
		if filter := query.Get("type"); filter != "" && rule.Type != filter {
			continue
		}
		if filter := query.Get("classification"); filter != "" && rule.Classification != filter {
			continue
		}
		matching = append(matching, rule)
	}

	items := []map[string]any{}
	next := ""
	for i := offset; i < len(matching); i++ {
		if len(items) == limit {
			next = strconv.Itoa(i)
			break
		}
		items = append(items, ruleView(matching[i]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "next": next})
}

// applyRuleFields copies the rule fields present in body onto rule. A JSON null clears an optional field.
func applyRuleFields(rule *mockRule, body map[string]any) error {
	if value, ok := body["classification"]; ok {
		classification, isString := value.(string)
		if !isString || !ruleClassifications[classification] {
			return fmt.Errorf("invalid classification %v", value)
		}
		rule.Classification = classification
	}
	if value, ok := body["rule"]; ok {
		pattern, isString := value.(string)
		if !isString || len(pattern) < 3 {
			return fmt.Errorf("rule must be a string of at least 3 characters, got %v", value)
		}
		rule.Rule = pattern
	}
	if value, ok := body["disclosure"]; ok {
		disclosure, isBool := value.(bool)
		if !isBool {
			return fmt.Errorf("disclosure must be a boolean, got %v", value)
		}
		rule.Disclosure = disclosure
	}
	for key, field := range map[string]**string{
		"title": &rule.Title, "vendorName": &rule.VendorName, "ruleType": &rule.RuleType,
		"description": &rule.Description, "expiry": &rule.Expiry,
	} {
		value, ok := body[key]
		if !ok {
			continue
		}
		if value == nil {
			*field = nil
			continue
		}
		text, isString := value.(string)
		if !isString {
			return fmt.Errorf("%s must be a string or null, got %v", key, value)
		}
		*field = &text
	}
	return nil
}

func (m *mockOsano) createRules(w http.ResponseWriter, r *http.Request) {
	var body map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	var configIDs []string
	if err := json.Unmarshal(body["configIds"], &configIDs); err != nil || len(configIDs) == 0 {
		writeError(w, http.StatusBadRequest, "configIds is required")
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for _, configID := range configIDs {
		if _, ok := m.configs[configID]; !ok {
			writeError(w, http.StatusNotFound, "config not found: "+configID)
			return
		}
	}

	created := []*mockRule{}
	for key := range body {
		if key == "configIds" {
			continue
		}
		ruleType, ok := ruleStoreTypes[key]
		if !ok {
			writeError(w, http.StatusBadRequest, "unsupported rule store type "+key)
			return
		}
		var definitions []map[string]any
		if err := json.Unmarshal(body[key], &definitions); err != nil {
			writeError(w, http.StatusBadRequest, "invalid "+key+" rules: "+err.Error())
			return
		}
		for _, configID := range configIDs {
			for _, definition := range definitions {
				rule := &mockRule{ConfigID: configID, Type: ruleType}
				if err := applyRuleFields(rule, definition); err != nil {
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				if rule.Classification == "" || rule.Rule == "" {
					writeError(w, http.StatusBadRequest, "classification and rule are required")
					return
				}
				created = append(created, rule)
			}
		}
	}
	if len(created) == 0 {
		writeError(w, http.StatusBadRequest, "no rules supplied")
		return
	}

	items := make([]map[string]any, 0, len(created))
	for _, rule := range created {
		m.nextRuleID++
		rule.RuleID = m.nextRuleID
		rule.Created = m.isoTick()
		rule.Updated = rule.Created
		m.rules[rule.RuleID] = rule
		m.configs[rule.ConfigID].markChanged()
		items = append(items, ruleView(rule))
	}
	writeJSON(w, http.StatusCreated, map[string]any{"items": items})
}

func (m *mockOsano) lookupRuleLocked(w http.ResponseWriter, r *http.Request) (*mockRule, bool) {
	ruleID, err := strconv.Atoi(r.PathValue("ruleId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "ruleId must be an integer")
		return nil, false
	}
	rule, ok := m.rules[ruleID]
	if !ok {
		writeError(w, http.StatusNotFound, "rule not found")
		return nil, false
	}
	return rule, true
}

func (m *mockOsano) patchRule(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	rule, ok := m.lookupRuleLocked(w, r)
	if !ok {
		return
	}
	updated := *rule
	if err := applyRuleFields(&updated, body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated.Updated = m.isoTick()
	*rule = updated
	m.configs[rule.ConfigID].markChanged()
	writeJSON(w, http.StatusOK, ruleView(rule))
}

func (m *mockOsano) deleteRule(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rule, ok := m.lookupRuleLocked(w, r)
	if !ok {
		return
	}
	delete(m.rules, rule.RuleID)
	m.configs[rule.ConfigID].markChanged()
	w.WriteHeader(http.StatusNoContent)
}

func (m *mockOsano) sortedConfigIDsLocked() []string {
	ids := make([]string, 0, len(m.configs))
	for id := range m.configs {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		left, right := m.configs[ids[i]], m.configs[ids[j]]
		if left.Created != right.Created {
			return left.Created < right.Created
		}
		return ids[i] < ids[j]
	})
	return ids
}

func (m *mockOsano) sortedRuleIDsLocked() []int {
	ids := make([]int, 0, len(m.rules))
	for id := range m.rules {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

// AddServerDefaults simulates Osano rolling out new configuration defaults after configs exist.
func (m *mockOsano) AddServerDefaults(defaults map[string]any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaults = mergeDefaults(m.defaults, defaults)
}

// EditPalette simulates a dashboard edit of one palette key outside Pulumi.
func (m *mockOsano) EditPalette(t *testing.T, configID, key string, value any) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	config, ok := m.configs[configID]
	if !ok {
		t.Fatalf("mock has no config %s", configID)
	}
	palette, _ := config.Configuration["palette"].(map[string]any)
	if palette == nil {
		palette = map[string]any{}
		config.Configuration["palette"] = palette
	}
	palette[key] = value
	config.Updated = m.tick()
	config.markChanged()
}

// Mark returns a position in the request log for RequestsSince.
func (m *mockOsano) Mark() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.requests)
}

// RequestsSince returns the requests received after mark, in arrival order.
func (m *mockOsano) RequestsSince(mark int) []recordedRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]recordedRequest(nil), m.requests[mark:]...)
}

// configSnapshot is a copy of a config's server-side state.
type configSnapshot struct {
	ConfigID          string
	Name              string
	Mode              string
	Configuration     map[string]any
	Updated           int64
	PublishStatus     string
	LastPublished     int64
	PublishedRevision int
	PublishCount      int
	LastPublishBody   map[string]any
}

// ConfigIDs lists every config the mock created, oldest first.
func (m *mockOsano) ConfigIDs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sortedConfigIDsLocked()
}

// Config returns a snapshot of one config, or false when the mock never created it.
func (m *mockOsano) Config(configID string) (configSnapshot, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	config, ok := m.configs[configID]
	if !ok {
		return configSnapshot{}, false
	}
	snapshot := configSnapshot{
		ConfigID:          config.ConfigID,
		Name:              config.Name,
		Mode:              config.Mode,
		Configuration:     deepCopyMap(config.Configuration),
		Updated:           config.Updated,
		PublishStatus:     config.PublishStatus,
		LastPublished:     config.LastPublished,
		PublishedRevision: config.PublishedRevision,
		PublishCount:      len(config.publishBodies),
	}
	if n := len(config.publishBodies); n > 0 {
		snapshot.LastPublishBody = config.publishBodies[n-1]
	}
	return snapshot, true
}

// Rules returns a copy of every rule the mock currently holds, ordered by rule ID.
func (m *mockOsano) Rules() []mockRule {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]mockRule, 0, len(m.rules))
	for _, id := range m.sortedRuleIDsLocked() {
		out = append(out, *m.rules[id])
	}
	return out
}

// requestFilter selects recorded requests.
type requestFilter func(recordedRequest) bool

func isPublish(r recordedRequest) bool {
	return r.Method == http.MethodPost && strings.HasPrefix(r.Path, configsPath+"/") &&
		strings.HasSuffix(r.Path, "/publish")
}

func isConfigPatch(r recordedRequest) bool {
	return r.Method == http.MethodPatch && strings.HasPrefix(r.Path, configsPath+"/")
}

func isConfigCreate(r recordedRequest) bool {
	return r.Method == http.MethodPost && r.Path == configsPath
}

func isConfigDelete(r recordedRequest) bool {
	return r.Method == http.MethodDelete && strings.HasPrefix(r.Path, configsPath)
}

func isRuleCreate(r recordedRequest) bool {
	return r.Method == http.MethodPost && r.Path == rulesPath
}

func isRulePatch(r recordedRequest) bool {
	return r.Method == http.MethodPatch && strings.HasPrefix(r.Path, rulesPath+"/")
}

func isRuleDelete(r recordedRequest) bool {
	return r.Method == http.MethodDelete && strings.HasPrefix(r.Path, rulesPath+"/")
}

// isWrite matches every request that changes Osano state.
func isWrite(r recordedRequest) bool {
	return r.Method != http.MethodGet
}

func countRequests(requests []recordedRequest, filter requestFilter) int {
	count := 0
	for _, r := range requests {
		if filter(r) {
			count++
		}
	}
	return count
}

func describeRequests(requests []recordedRequest) string {
	if len(requests) == 0 {
		return "  (no requests)"
	}
	lines := make([]string, 0, len(requests))
	for _, r := range requests {
		lines = append(lines, "  "+r.String())
	}
	return strings.Join(lines, "\n")
}
