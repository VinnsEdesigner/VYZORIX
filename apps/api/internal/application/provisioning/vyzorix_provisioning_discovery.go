package provisioning

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// ManifestName is the canonical file that makes a directory a provisioned
// resource. The contract fixes the layout as devices/<device-id>/<manifest>: one
// file is one resource, and the directory names the device it binds to.
//
// The content is the same vyzorix.toml the deploy service parses, so .toml is the
// canonical name. ManifestAliasV is also accepted because the contract spells the
// path `devices/<id>/vyzorix.v`; .v is the *compiled* config's extension, so a
// file by that name is read as a manifest regardless, which costs nothing and
// keeps the documented layout working.
const (
	ManifestName   = "vyzorix.toml"
	ManifestAliasV = "vyzorix.v"
)

// isManifestName reports whether a file name is a provisioned resource.
func isManifestName(name string) bool {
	return name == ManifestName || name == ManifestAliasV
}

// Manifest is one discovered file, bound to a device.
//
// Field order follows alignment so the struct packs without padding.
type Manifest struct {
	// Err is a discovery-time problem (a malformed path, an unreadable file, a
	// symlink). It is a per-resource error, never a job failure: one bad file must
	// not stop the others from applying.
	Err         error
	Path        string
	DeviceID    string
	ContentHash string
	Bytes       []byte
}

// Discover walks root/syncPath and returns every manifest under it.
//
// A resource's Path is relative to the synced subdirectory, because `path` selects
// the tree being synced: with path "/apps" the repository declares
// apps/devices/<id>/<manifest>, and the resource is still addressed
// devices/<id>/<manifest>. That keeps the devices/ anchor meaningful whatever
// subdirectory is configured.
//
// Results are sorted by path so a sync's order is stable and its job results diff
// cleanly between runs.
func Discover(root, syncPath string) ([]*Manifest, error) {
	start, err := syncRoot(root, syncPath)
	if err != nil {
		return nil, err
	}

	var out []*Manifest

	walkErr := filepath.WalkDir(start, func(full string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// A symlinked directory is not part of the declared tree. WalkDir does not
			// follow one, but skipping it explicitly keeps the rule visible.
			if d.Type()&fs.ModeSymlink != 0 {
				return fs.SkipDir
			}

			return nil
		}
		if !isManifestName(d.Name()) {
			return nil
		}

		out = append(out, readManifest(start, full, d))

		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })

	markDuplicateDevices(out)

	return out, nil
}

// markDuplicateDevices flags a device that declares more than one manifest.
//
// The contract fixes one manifest per device, so devices/<id>/ holding both vyzorix.toml
// and vyzorix.v is a mistake. Left alone it produced two resources for one device: the
// second deploy hit the one-owner index and failed with "device is owned by another
// repository", which names no owner because the owner is this same repository — a
// confusing message for a problem that is really an ambiguous file layout.
func markDuplicateDevices(manifests []*Manifest) {
	byDevice := make(map[string][]*Manifest, len(manifests))
	for _, m := range manifests {
		if m.Err == nil && m.DeviceID != "" {
			byDevice[m.DeviceID] = append(byDevice[m.DeviceID], m)
		}
	}

	for deviceID, group := range byDevice {
		if len(group) < 2 {
			continue
		}

		paths := make([]string, len(group))
		for i, m := range group {
			paths[i] = m.Path
		}
		sort.Strings(paths)

		msg := fmt.Sprintf("device %s declares %d manifests (%s); keep exactly one",
			deviceID, len(group), strings.Join(paths, ", "))

		for _, m := range group {
			m.Err = errors.New(msg)
		}
	}
}

// syncRoot resolves a repository's configured subdirectory inside the checkout,
// refusing any path that escapes it.
func syncRoot(root, syncPath string) (string, error) {
	clean := path.Clean("/" + strings.TrimSpace(syncPath))
	if clean == "/" {
		return root, nil
	}

	joined := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(clean, "/")))

	// A configured path must stay inside the checkout. Without this a repository
	// record with "../.." would walk the host filesystem.
	rel, err := filepath.Rel(root, joined)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("sync path %q escapes the repository", syncPath)
	}

	return joined, nil
}

// DeviceDir is a directory under devices/ that holds no manifest.
//
// Full discovery lists these because the wizard's "which devices will this
// repository own" step has to show what the repository contains, not only what it
// would apply. A device directory with no manifest is present in the tree and
// deliberately unmanaged, which is different from one that is simply absent.
type DeviceDir struct {
	DeviceID string `json:"device_id"`
	Path     string `json:"path"`
}

// Survey is one walk of a repository: the manifests it declares and the device
// directories it does not manage.
type Survey struct {
	Manifests []*Manifest
	Unmanaged []DeviceDir
}

// SurveyRepository walks the repository once and returns both what it declares and
// what it contains but does not manage.
//
// It is one traversal rather than two so the tree view and the plan cannot disagree
// about which paths exist: a second walk could observe a different checkout state.
// Nothing is written and no manifest is applied — this is the read that feeds the
// wizard's preview.
func SurveyRepository(root, syncPath string) (*Survey, error) {
	manifests, err := Discover(root, syncPath)
	if err != nil {
		return nil, err
	}

	start, err := syncRoot(root, syncPath)
	if err != nil {
		return nil, err
	}

	declared := make(map[string]bool, len(manifests))
	for _, m := range manifests {
		declared[m.DeviceID] = true
	}

	unmanaged, err := unmanagedDeviceDirs(start, declared)
	if err != nil {
		return nil, err
	}

	return &Survey{Manifests: manifests, Unmanaged: unmanaged}, nil
}

// unmanagedDeviceDirs lists devices/<id> directories that hold no manifest.
//
// A directory whose own manifest failed to parse is NOT reported as unmanaged: the
// manifest exists, so the device is managed and the parse error is what the plan
// reports. Only a directory with no manifest file at all is unmanaged.
func unmanagedDeviceDirs(start string, declared map[string]bool) ([]DeviceDir, error) {
	devicesRoot := filepath.Join(start, "devices")

	entries, err := os.ReadDir(devicesRoot)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var out []DeviceDir

	for _, entry := range entries {
		// A symlinked directory is not part of the declared tree.
		if !entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
			continue
		}

		deviceID := entry.Name()
		// A device is unmanaged only when it has no manifest file at all. One whose
		// manifest exists but failed to parse is managed: the plan reports the error.
		if declared[deviceID] || hasManifest(filepath.Join(devicesRoot, deviceID)) {
			continue
		}

		out = append(out, DeviceDir{
			DeviceID: deviceID,
			Path:     relativePath(start, filepath.Join(devicesRoot, deviceID)),
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })

	return out, nil
}

// hasManifest reports whether a device directory contains any manifest file,
// regardless of whether it parses.
func hasManifest(dir string) bool {
	for _, name := range []string{ManifestName, ManifestAliasV} {
		if info, err := os.Lstat(filepath.Join(dir, name)); err == nil && !info.IsDir() {
			return true
		}
	}

	return false
}

// readManifest reads one file and resolves the device from its parent directory.
func readManifest(root, full string, d fs.DirEntry) *Manifest {
	m := &Manifest{Path: relativePath(root, full)}

	// A repository is untrusted content. git materializes a committed symlink as a
	// symlink, and os.ReadFile would follow it to any host file the server can read
	// (/etc/shadow, the env file holding the session secret). Only a regular file is
	// a manifest, so a symlink is refused before anything is opened.
	if d.Type()&fs.ModeSymlink != 0 {
		m.Err = fmt.Errorf("%s is a symlink, which is not allowed in a provisioning repository", m.Path)

		return m
	}

	data, err := os.ReadFile(full) // #nosec G304 -- full is a path walked from the checkout root.
	if err != nil {
		m.Err = fmt.Errorf("could not read %s: %w", m.Path, err)

		return m
	}

	m.Bytes = data

	sum := sha256.Sum256(data)
	m.ContentHash = hex.EncodeToString(sum[:])

	deviceID, err := deviceFromPath(m.Path)
	if err != nil {
		m.Err = err

		return m
	}
	m.DeviceID = deviceID

	return m
}

// deviceFromPath extracts the device id from devices/<device-id>/<manifest>.
//
// The contract fixes one device per file. A manifest not under devices/ is a
// mistake worth naming rather than silently ignoring: the user put a file where
// the sync will never apply it, and telling them is cheaper than letting them
// wonder why nothing happened.
func deviceFromPath(rel string) (string, error) {
	parts := strings.Split(rel, "/")
	if len(parts) < 3 || parts[0] != "devices" {
		return "", fmt.Errorf("%s is not under devices/<device-id>/", rel)
	}

	deviceID := strings.TrimSpace(parts[1])
	if deviceID == "" || deviceID == "." || deviceID == ".." {
		return "", fmt.Errorf("%s names no device", rel)
	}

	return deviceID, nil
}

// relativePath makes a path discovered under the checkout relative to the
// checkout root, so a resource is addressed the same way whatever subdirectory a
// repository syncs.
func relativePath(root, full string) string {
	rel, err := filepath.Rel(root, full)
	if err != nil {
		return filepath.ToSlash(full)
	}

	return filepath.ToSlash(rel)
}
