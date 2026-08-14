//nolint:goheader // Source-file header normalization is still in progress during alpha.
package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	osanoclient "github.com/jflavan/pulumi-osano/provider/internal/osano"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

const (
	defaultPublicationInitialInterval = time.Second
	defaultPublicationMaxInterval     = 10 * time.Second
	defaultPublicationTimeout         = 20 * time.Minute
)

// CookieConsentPublication publishes an Osano Cookie Consent configuration and exposes its install script.
type CookieConsentPublication struct{}

// CookieConsentPublicationArgs controls when and how a Cookie Consent configuration is published.
type CookieConsentPublicationArgs struct {
	ConfigID                string  `pulumi:"configId"`
	ChangeToken             string  `pulumi:"changeToken"`
	KeepUnclassifiedTattles *bool   `pulumi:"keepUnclassifiedTattles,optional"`
	Description             *string `pulumi:"description,optional"`
	WebhookURL              *string `pulumi:"webhookUrl,optional"`
}

// CookieConsentPublicationState extends publication inputs with Osano metadata and public script outputs.
type CookieConsentPublicationState struct {
	CookieConsentPublicationArgs

	CustomerID        string `pulumi:"customerId"`
	PublishStatus     string `pulumi:"publishStatus"`
	LastPublished     int    `pulumi:"lastPublished"`
	PublishedRevision int    `pulumi:"publishedRevision"`
	ScriptSrc         string `pulumi:"scriptSrc"`
	ScriptTag         string `pulumi:"scriptTag"`
}

// Annotate documents the CookieConsentPublication resource.
func (r *CookieConsentPublication) Annotate(a infer.Annotator) {
	a.Describe(
		r,
		"Publishes an Osano Cookie Consent configuration, waits for completion, and returns its hosted CMP script. "+
			"Import with the Osano config ID; deleting this resource only removes Pulumi state and "+
			"does not unpublish the config.",
	)
}

// Annotate documents CookieConsentPublication inputs and their defaults.
func (args *CookieConsentPublicationArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.ConfigID, "The Osano Cookie Consent config ID to publish. This is also the import ID.")
	a.Describe(
		&args.ChangeToken,
		"A caller-managed desired-state token. Changing it queues a new publication; an unchanged token is a no-op.",
	)
	a.Describe(
		&args.KeepUnclassifiedTattles,
		"Whether publication preserves unclassified discoveries. Defaults to true to avoid unexpected deletion.",
	)
	a.SetDefault(&args.KeepUnclassifiedTattles, true)
	a.Describe(&args.Description, "Optional description sent with the publication request.")
	a.Describe(&args.WebhookURL, "Optional absolute HTTP or HTTPS URL notified by Osano after publication.")
}

// Annotate documents CookieConsentPublication outputs.
func (state *CookieConsentPublicationState) Annotate(a infer.Annotator) {
	a.Describe(&state.CustomerID, "The Osano customer ID that owns the published configuration.")
	a.Describe(&state.PublishStatus, "The publication status returned by Osano after completion.")
	a.Describe(&state.LastPublished, "Unix timestamp of the completed Osano publication.")
	a.Describe(&state.PublishedRevision, "The configuration revision most recently published by Osano.")
	a.Describe(&state.ScriptSrc, "The public hosted Osano CMP JavaScript URL for this customer and config.")
	a.Describe(
		&state.ScriptTag,
		"The complete public CMP script tag to place first in the site head, without async or defer attributes.",
	)
}

// Check validates and defaults CookieConsentPublication inputs.
func (r *CookieConsentPublication) Check(
	ctx context.Context, req infer.CheckRequest,
) (infer.CheckResponse[CookieConsentPublicationArgs], error) {
	args, failures, err := infer.DefaultCheck[CookieConsentPublicationArgs](ctx, req.NewInputs)
	if err != nil {
		return infer.CheckResponse[CookieConsentPublicationArgs]{Inputs: args, Failures: failures}, err
	}

	if !req.NewInputs.Get("configId").HasComputed() && strings.TrimSpace(args.ConfigID) == "" {
		failures = append(failures, p.CheckFailure{Property: "configId", Reason: "configId is required"})
	}
	if !req.NewInputs.Get("changeToken").HasComputed() && strings.TrimSpace(args.ChangeToken) == "" {
		failures = append(failures, p.CheckFailure{Property: "changeToken", Reason: "changeToken is required"})
	}
	if args.KeepUnclassifiedTattles == nil {
		keep := true
		args.KeepUnclassifiedTattles = &keep
	}
	if args.WebhookURL != nil && !validPublicationWebhookURL(*args.WebhookURL) {
		failures = append(failures, p.CheckFailure{
			Property: "webhookUrl",
			Reason:   "webhookUrl must be an absolute HTTP or HTTPS URL",
		})
	}

	return infer.CheckResponse[CookieConsentPublicationArgs]{Inputs: args, Failures: failures}, nil
}

// Create queues and waits for a Cookie Consent publication.
func (r *CookieConsentPublication) Create(
	ctx context.Context, req infer.CreateRequest[CookieConsentPublicationArgs],
) (infer.CreateResponse[CookieConsentPublicationState], error) {
	if req.DryRun {
		return infer.CreateResponse[CookieConsentPublicationState]{
			ID:     "preview",
			Output: CookieConsentPublicationState{CookieConsentPublicationArgs: req.Inputs},
		}, nil
	}

	client, err := customerClientFromConfig(infer.GetConfig[Config](ctx))
	if err != nil {
		return infer.CreateResponse[CookieConsentPublicationState]{}, err
	}
	state, err := publishCookieConsent(ctx, client, req.Inputs, defaultPublicationPollOptions())
	if err != nil {
		return infer.CreateResponse[CookieConsentPublicationState]{}, err
	}
	return infer.CreateResponse[CookieConsentPublicationState]{ID: state.ConfigID, Output: state}, nil
}

// Read refreshes publication metadata without queuing another publication.
func (r *CookieConsentPublication) Read(
	ctx context.Context,
	req infer.ReadRequest[CookieConsentPublicationArgs, CookieConsentPublicationState],
) (infer.ReadResponse[CookieConsentPublicationArgs, CookieConsentPublicationState], error) {
	client, err := customerClientFromConfig(infer.GetConfig[Config](ctx))
	if err != nil {
		return infer.ReadResponse[CookieConsentPublicationArgs, CookieConsentPublicationState]{}, err
	}

	var current cmpConfigResponse
	err = client.DoJSON(ctx, http.MethodGet, cookieConsentConfigPath(req.ID), nil, nil, &current)
	if osanoclient.IsHTTPStatus(err, http.StatusNotFound) {
		return infer.ReadResponse[CookieConsentPublicationArgs, CookieConsentPublicationState]{ID: ""}, nil
	}
	if err != nil {
		return infer.ReadResponse[CookieConsentPublicationArgs, CookieConsentPublicationState]{},
			fmt.Errorf("read Cookie Consent publication %q: %w", req.ID, err)
	}

	args := req.Inputs
	if args.ChangeToken == "" {
		args.ChangeToken = fmt.Sprintf("import:%d:%d", current.LastPublished, current.PublishedRevision)
	}
	if args.KeepUnclassifiedTattles == nil {
		keep := true
		args.KeepUnclassifiedTattles = &keep
	}
	args.ConfigID = current.ConfigID

	state, err := cookieConsentPublicationState(args, current)
	if err != nil {
		return infer.ReadResponse[CookieConsentPublicationArgs, CookieConsentPublicationState]{},
			fmt.Errorf("read Cookie Consent publication %q: %w", req.ID, err)
	}
	return infer.ReadResponse[CookieConsentPublicationArgs, CookieConsentPublicationState]{
		ID:     current.ConfigID,
		Inputs: state.CookieConsentPublicationArgs,
		State:  state,
	}, nil
}

// Update queues and waits for another Cookie Consent publication.
func (r *CookieConsentPublication) Update(
	ctx context.Context,
	req infer.UpdateRequest[CookieConsentPublicationArgs, CookieConsentPublicationState],
) (infer.UpdateResponse[CookieConsentPublicationState], error) {
	if req.DryRun {
		preview := req.State
		preview.CookieConsentPublicationArgs = req.Inputs
		return infer.UpdateResponse[CookieConsentPublicationState]{Output: preview}, nil
	}

	client, err := customerClientFromConfig(infer.GetConfig[Config](ctx))
	if err != nil {
		return infer.UpdateResponse[CookieConsentPublicationState]{}, err
	}
	state, err := publishCookieConsent(ctx, client, req.Inputs, defaultPublicationPollOptions())
	if err != nil {
		return infer.UpdateResponse[CookieConsentPublicationState]{}, err
	}
	return infer.UpdateResponse[CookieConsentPublicationState]{Output: state}, nil
}

// Delete forgets publication state without calling Osano or unpublishing the config.
func (r *CookieConsentPublication) Delete(
	_ context.Context, _ infer.DeleteRequest[CookieConsentPublicationState],
) (infer.DeleteResponse, error) {
	return infer.DeleteResponse{}, nil
}

// Diff reports config identity changes as replacements and publication option changes as updates.
func (r *CookieConsentPublication) Diff(
	_ context.Context,
	req infer.DiffRequest[CookieConsentPublicationArgs, CookieConsentPublicationState],
) (infer.DiffResponse, error) {
	diff := map[string]p.PropertyDiff{}
	if req.Inputs.ConfigID != req.State.ConfigID {
		diff["configId"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}
	if req.Inputs.ChangeToken != req.State.ChangeToken {
		diff["changeToken"] = p.PropertyDiff{Kind: p.Update}
	}
	if !boolPointersEqual(req.Inputs.KeepUnclassifiedTattles, req.State.KeepUnclassifiedTattles) {
		diff["keepUnclassifiedTattles"] = p.PropertyDiff{Kind: p.Update}
	}
	if !ptrStringEqual(req.Inputs.Description, req.State.Description) {
		diff["description"] = p.PropertyDiff{Kind: p.Update}
	}
	if !ptrStringEqual(req.Inputs.WebhookURL, req.State.WebhookURL) {
		diff["webhookUrl"] = p.PropertyDiff{Kind: p.Update}
	}
	return infer.DiffResponse{HasChanges: len(diff) > 0, DetailedDiff: diff}, nil
}

type publicationBaseline struct {
	Status            string
	LastPublished     int
	PublishedRevision int
}

type publicationPollOptions struct {
	InitialInterval time.Duration
	MaxInterval     time.Duration
	Timeout         time.Duration
	Sleep           func(context.Context, time.Duration) error
}

func defaultPublicationPollOptions() publicationPollOptions {
	return publicationPollOptions{
		InitialInterval: defaultPublicationInitialInterval,
		MaxInterval:     defaultPublicationMaxInterval,
		Timeout:         defaultPublicationTimeout,
		Sleep:           sleepForPublication,
	}
}

func publishCookieConsent(
	ctx context.Context,
	client jsonClient,
	args CookieConsentPublicationArgs,
	opts publicationPollOptions,
) (CookieConsentPublicationState, error) {
	if strings.TrimSpace(args.ConfigID) == "" {
		return CookieConsentPublicationState{}, errors.New("configId is required to publish Cookie Consent")
	}
	if strings.TrimSpace(args.ChangeToken) == "" {
		return CookieConsentPublicationState{}, errors.New("changeToken is required to publish Cookie Consent")
	}
	if args.KeepUnclassifiedTattles == nil {
		keep := true
		args.KeepUnclassifiedTattles = &keep
	}
	opts = normalizePublicationPollOptions(opts)
	ctx, cancel := withPublicationTimeout(ctx, opts.Timeout)
	defer cancel()

	var baselineResponse cmpConfigResponse
	if err := client.DoJSON(
		ctx, http.MethodGet, cookieConsentConfigPath(args.ConfigID), nil, nil, &baselineResponse,
	); err != nil {
		return CookieConsentPublicationState{},
			fmt.Errorf("read publication baseline for Cookie Consent config %q: %w", args.ConfigID, err)
	}
	if _, _, err := cookieConsentScript(baselineResponse.CustomerID, baselineResponse.ConfigID); err != nil {
		return CookieConsentPublicationState{}, fmt.Errorf("read publication baseline for config %q: %w", args.ConfigID, err)
	}
	switch baselineResponse.PublishStatus {
	case "published", "in-progress", "unpublished", "outdated", "error":
	default:
		return CookieConsentPublicationState{}, publicationStatusError(baselineResponse)
	}
	baseline := publicationBaseline{
		Status:            baselineResponse.PublishStatus,
		LastPublished:     baselineResponse.LastPublished,
		PublishedRevision: baselineResponse.PublishedRevision,
	}

	body := map[string]any{"keepUnclassifiedTattles": *args.KeepUnclassifiedTattles}
	if args.Description != nil {
		body["description"] = *args.Description
	}
	if args.WebhookURL != nil {
		body["webhookUrl"] = *args.WebhookURL
	}
	err := client.DoJSON(
		ctx,
		http.MethodPost,
		cookieConsentConfigPath(args.ConfigID)+"/publish",
		nil,
		body,
		nil,
	)
	if err != nil && !osanoclient.IsHTTPStatus(err, http.StatusConflict) {
		return CookieConsentPublicationState{},
			fmt.Errorf("queue Cookie Consent publication for config %q: %w", args.ConfigID, err)
	}

	interval := opts.InitialInterval
	seenInProgress := false
	for {
		var current cmpConfigResponse
		if err := client.DoJSON(
			ctx, http.MethodGet, cookieConsentConfigPath(args.ConfigID), nil, nil, &current,
		); err != nil {
			return CookieConsentPublicationState{},
				fmt.Errorf("poll Cookie Consent publication for config %q: %w", args.ConfigID, err)
		}
		if _, _, err := cookieConsentScript(current.CustomerID, current.ConfigID); err != nil {
			return CookieConsentPublicationState{},
				fmt.Errorf("poll Cookie Consent publication for config %q: %w", args.ConfigID, err)
		}

		switch current.PublishStatus {
		case "published":
			completed := seenInProgress ||
				baseline.Status != "published" ||
				current.LastPublished > baseline.LastPublished ||
				current.PublishedRevision > baseline.PublishedRevision
			if completed {
				return cookieConsentPublicationState(args, current)
			}
		case "in-progress":
			seenInProgress = true
		case "unpublished", "outdated":
		case "error":
			freshError := baseline.Status != "error" ||
				seenInProgress ||
				current.LastPublished != baseline.LastPublished ||
				current.PublishedRevision != baseline.PublishedRevision
			if freshError {
				return CookieConsentPublicationState{}, publicationStatusError(current)
			}
		default:
			return CookieConsentPublicationState{}, publicationStatusError(current)
		}

		if err := opts.Sleep(ctx, interval); err != nil {
			return CookieConsentPublicationState{},
				fmt.Errorf("wait for Cookie Consent publication of config %q: %w", args.ConfigID, err)
		}
		interval = nextPublicationInterval(interval, opts.MaxInterval)
	}
}

func cookieConsentPublicationState(
	args CookieConsentPublicationArgs, current cmpConfigResponse,
) (CookieConsentPublicationState, error) {
	src, tag, err := cookieConsentScript(current.CustomerID, current.ConfigID)
	if err != nil {
		return CookieConsentPublicationState{}, err
	}
	args.ConfigID = current.ConfigID
	if args.KeepUnclassifiedTattles == nil {
		keep := true
		args.KeepUnclassifiedTattles = &keep
	}
	return CookieConsentPublicationState{
		CookieConsentPublicationArgs: args,
		CustomerID:                   current.CustomerID,
		PublishStatus:                current.PublishStatus,
		LastPublished:                current.LastPublished,
		PublishedRevision:            current.PublishedRevision,
		ScriptSrc:                    src,
		ScriptTag:                    tag,
	}, nil
}

func cookieConsentScript(customerID, configID string) (scriptSrc, scriptTag string, err error) {
	if strings.TrimSpace(customerID) == "" || strings.TrimSpace(configID) == "" {
		return "", "", errors.New("customerId and configId are required to build the CMP script")
	}
	src := fmt.Sprintf(
		"https://cmp.osano.com/%s/%s/osano.js",
		url.PathEscape(customerID), url.PathEscape(configID),
	)
	return src, `<script src="` + src + `"></script>`, nil
}

func withPublicationTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = defaultPublicationTimeout
	}
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= timeout {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, timeout)
}

func normalizePublicationPollOptions(opts publicationPollOptions) publicationPollOptions {
	if opts.Timeout <= 0 {
		opts.Timeout = defaultPublicationTimeout
	}
	if opts.Sleep == nil {
		opts.Sleep = sleepForPublication
		if opts.InitialInterval <= 0 {
			opts.InitialInterval = defaultPublicationInitialInterval
		}
		if opts.MaxInterval <= 0 {
			opts.MaxInterval = defaultPublicationMaxInterval
		}
	}
	if opts.MaxInterval > 0 && opts.InitialInterval > opts.MaxInterval {
		opts.InitialInterval = opts.MaxInterval
	}
	return opts
}

func nextPublicationInterval(current, maximum time.Duration) time.Duration {
	if current <= 0 || maximum <= 0 {
		return current
	}
	if current >= maximum || current > maximum/2 {
		return maximum
	}
	return current * 2
}

func sleepForPublication(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func publicationStatusError(current cmpConfigResponse) error {
	switch current.PublishStatus {
	case "published", "in-progress", "unpublished", "outdated":
		return nil
	default:
		return fmt.Errorf(
			"Cookie Consent publication reached terminal status=%s lastPublished=%d publishedRevision=%d",
			current.PublishStatus,
			current.LastPublished,
			current.PublishedRevision,
		)
	}
}

func validPublicationWebhookURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	return err == nil && parsed.IsAbs() && parsed.Host != "" &&
		(parsed.Scheme == "http" || parsed.Scheme == "https")
}

func boolPointersEqual(left, right *bool) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}
