package deployhandlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/deploy"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/deploystate"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/deployconfig"
)

// The deploy config surface: POST /v1/deploy, GET /v1/deploy/{imei},
// GET /v1/deploy/{imei}/document, GET /v1/deploy/schema/{version}.
//
// The device segment is spelled `:imei` because gin refuses two different
// parameter names at the same path position, and the device-signed routes under
// /v1/device already use `:imei`. The value is a device id, not an IMEI.

// deployResponse is what the operator is told. It names the version and the
// authoriser, so the receipt can be checked against the device's config.
type deployResponse struct {
	AuthorizedBy string       `json:"authorizedBy"`
	Document     string       `json:"document,omitempty"`
	Apps         []appMarking `json:"apps"`
	Artifacts    int          `json:"artifacts"`
	Version      int          `json:"version"`
	DryRun       bool         `json:"dryRun"`
}

// appMarking is one app's catalog classification: the console marks an app
// the catalog does not know as custom, so an unreviewed app is visible.
type appMarking struct {
	Name   string `json:"name"`
	Custom bool   `json:"custom"`
}

// Deploy handles POST /v1/deploy.
// @Summary      Deploy a manifest to a device
// @Description  Compiles, authorises, signs and persists a device config. With
// @Description  ?dry_run=true it compiles and signs but stores nothing.
// @Tags         deploy
// @Accept       application/toml
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        dry_run  query  bool  false  "compile and sign without persisting"
// @Success      202  {object}  deployResponse  "deployed"
// @Failure      400  {object}  openapi.ErrorResponse  "manifest invalid"
// @Failure      403  {object}  openapi.ErrorResponse  "device.deploy required"
// @Failure      404  {object}  openapi.ErrorResponse  "device not in organization"
// @Failure      409  {object}  openapi.ErrorResponse  "device not registered"
// @Router       /deploy [post]
func (h *Handler) Deploy(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == "" {
		respondMissingOrg(c)

		return
	}

	caller, memberID, ok := h.resolveCaller(c, orgID)
	if !ok {
		respondNotAMember(c)

		return
	}

	body, err := c.GetRawData()
	if err != nil {
		respondBadBody(c)

		return
	}

	result, err := h.service.Deploy(c.Request.Context(), deploy.DeployRequest{
		Caller: caller,
		OrgID:  orgID,
		// POST /v1/deploy has no device segment; the manifest's device.id is the
		// target and the service reads it. Passing the path param here (always empty
		// on this route) is what made every deploy fail authorization.
		//
		// The member is the resolved caller's, not the context operator's: a CI deploy
		// authenticated by a credential has no operator in the context, and
		// requires the config to name the human who minted the credential.
		MemberID: memberID,
		Manifest: body,
		DryRun:   c.Query("dry_run") == "true",
	})
	if err != nil {
		h.respondDeployError(c, err)

		return
	}

	// A device that is connected right now is told immediately; one that is
	// offline re-fetches on reconnect. A push that finds no connection is
	// not an error — the deploy is stored either way — so its outcome is logged
	// and never changes the response.
	if !result.DryRun {
		h.pushConfig(c, result.DeviceID, result.Version)
	}

	// 202, not 200: the deploy is accepted and stored, and the device applies it
	// on its next connection. Nothing has run yet.
	c.JSON(http.StatusAccepted, deployResponse{
		Version:      result.Version,
		AuthorizedBy: result.AuthorizedBy,
		Apps:         markApps(result.Apps),
		Artifacts:    len(result.Artifacts),
		DryRun:       result.DryRun,
		Document:     result.Document,
	})
}

// pushConfig notifies a connected device of a new deploy. It is best-effort: the
// config is already stored, so a device that is not connected (or a handler with
// no pusher wired) simply re-fetches on reconnect.
func (h *Handler) pushConfig(c *gin.Context, deviceID string, version int) {
	if h.pusher == nil || deviceID == "" {
		return
	}

	if h.pusher.PushConfig(c.Request.Context(), deviceID) {
		h.log.Info("deploy pushed to a connected device",
			slog.String("deviceId", deviceID), slog.Int("version", version))
	}
}

// markApps converts the service's catalog markings to the wire shape.
func markApps(markings []deploy.AppMarking) []appMarking {
	out := make([]appMarking, 0, len(markings))

	for _, m := range markings {
		out = append(out, appMarking{Name: m.Name, Custom: m.Custom})
	}

	return out
}

// currentResponse is the GET /v1/deploy/{imei} body. It carries the document,
// because the console shows it and the operator can compare it to what the device
// reports.
type currentResponse struct {
	DeviceID  string `json:"deviceId"`
	Document  string `json:"document"`
	Kind      string `json:"kind"`
	Version   int    `json:"version"`
	CreatedAt int64  `json:"createdAt"`
	// Drift is how the device's *reported* state relates to this stored config. It
	// is the answer to "is the device running what the server expects", computed on
	// read so it is current on every call rather than only at apply time.
	//
	// `state` is one of in_sync | unknown | behind | ahead | app_changed. `unknown`
	// covers both "the device does not report state" and "no config is stored", and
	// is never rendered as healthy.
	Drift driftResponse `json:"drift"`
}

// driftResponse is the drift verdict as the console reads it.
type driftResponse struct {
	State string `json:"state"`
	// Detail names what differs, for a human: "app vfd: expected 3f2a…, running 9c11…".
	Detail string `json:"detail"`
	// ReportedVersion is what the device says it holds, or null when it does not
	// report state. Null and 0 are different: null is unknown, 0 is "holds no config".
	ReportedVersion *int `json:"reportedVersion"`
	// ReportedAt is when the device last reported, epoch millis, or 0 when never.
	ReportedAt int64 `json:"reportedAt"`
}

// Current handles GET /v1/deploy/{imei}.
// @Summary      Get a device's current config
// @Description  Returns the signed config the device should be running.
// @Tags         deploy
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        imei  path  string  true  "device id"
// @Success      200  {object}  currentResponse  "current config"
// @Failure      403  {object}  openapi.ErrorResponse  "device.deploy required"
// @Failure      404  {object}  openapi.ErrorResponse  "device or config not found"
// @Router       /deploy/{imei} [get]
func (h *Handler) Current(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == "" {
		respondMissingOrg(c)

		return
	}

	caller, _, ok := h.resolveCaller(c, orgID)
	if !ok {
		respondNotAMember(c)

		return
	}

	record, err := h.service.Current(c.Request.Context(), caller, orgID, c.Param("imei"))
	if err != nil {
		h.respondDeployError(c, err)

		return
	}

	c.JSON(http.StatusOK, currentResponse{
		DeviceID:  record.DeviceID,
		Version:   record.Version,
		Kind:      string(record.Kind),
		Document:  record.Document,
		CreatedAt: record.CreatedAt.UnixMilli(),
		Drift:     h.driftFor(c.Request.Context(), orgID, record),
	})
}

// driftFor compares the config the server expects against what the device says it is
// running, and returns the verdict for the console.
//
// It is computed on read rather than stored, so it is current on every call — a
// device that drifted ten minutes ago and has since re-applied reads as in-sync,
// without anything having to notice the recovery. The comparison itself is one pure
// function (`deploystate.Compare`), so this method only gathers its inputs.
//
// A device that cannot be loaded reads as unknown rather than in-sync: an error must
// never be reported as health.
func (h *Handler) driftFor(
	ctx context.Context,
	orgID string,
	record *deployconfig.Record,
) driftResponse {
	expected := deploystate.ExpectedFromDocument(record.Document)
	// The stored config's own version is the authority on what is expected — the
	// document's `version` node and the record agree, and the record is what the
	// device is told to fetch.
	expected.Version = record.Version

	dev, err := h.devices.FindForOrg(ctx, record.DeviceID, orgID)
	if err != nil || dev == nil {
		return driftResponse{
			State:  string(deploystate.DriftUnknown),
			Detail: "the device could not be read",
		}
	}

	reported := deploystate.Reported{
		Version:       versionOrZero(dev.ReportedConfigVersion),
		AppDigests:    dev.ReportedAppDigests,
		ReportedState: dev.ReportedConfigVersion != nil,
	}

	drift := deploystate.Compare(expected, reported)

	return driftResponse{
		State:           string(drift.State),
		Detail:          drift.Detail,
		ReportedVersion: dev.ReportedConfigVersion,
		ReportedAt:      dev.ReportedAt,
	}
}

// versionOrZero dereferences an optional version. A nil pointer is "not reported",
// and the caller has already marked that as such; this only avoids a nil deref.
func versionOrZero(v *int) int {
	if v == nil {
		return 0
	}

	return *v
}

// Document handles GET /v1/deploy/{imei}/document: the signed config verbatim,
// as the device fetches it. It is separate from Current so a caller that only
// needs the bytes the device verifies does not parse a JSON envelope.
// @Summary      Get a device's signed config document
// @Description  Returns the signed vyzorix.v text verbatim.
// @Tags         deploy
// @Produce      text/plain
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        imei  path  string  true  "device id"
// @Success      200  {string}  string  "signed config"
// @Failure      403  {object}  openapi.ErrorResponse  "device.deploy required"
// @Failure      404  {object}  openapi.ErrorResponse  "device or config not found"
// @Router       /deploy/{imei}/document [get]
func (h *Handler) Document(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == "" {
		respondMissingOrg(c)

		return
	}

	caller, _, ok := h.resolveCaller(c, orgID)
	if !ok {
		respondNotAMember(c)

		return
	}

	record, err := h.service.Current(c.Request.Context(), caller, orgID, c.Param("imei"))
	if err != nil {
		h.respondDeployError(c, err)

		return
	}

	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(record.Document))
}

// Schema handles GET /v1/deploy/schema/{version}. It serves the manifest
// JSON Schema the CLI validates a manifest against before uploading, so a
// manifest the CLI accepts is the manifest the server accepts.
//
// It takes no caller: the schema is public, versioned and identical for every
// tenant. The {version} segment is the pin, so a client that validated against
// version 1 is refused rather than silently served version 2.
// @Summary      The manifest JSON Schema
// @Description  The versioned JSON Schema for vyzorix.toml. Generated from the vyzorix-spec module.
// @Tags         deploy
// @Produce      json
// @Param        version  path  string  true  "schema version"
// @Success      200  {object}  map[string]any  "the JSON Schema"
// @Failure      404  {object}  openapi.ErrorResponse  "unknown schema version"
// @Router       /deploy/schema/{version} [get]
func (h *Handler) Schema(c *gin.Context) {
	schema, err := deploy.ManifestSchema(c.Param("version"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})

		return
	}

	c.Data(http.StatusOK, "application/schema+json; charset=utf-8", schema)
}
