package provisioning

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// OrgResource is one discovered org file: an organisation's declaration of a kind.
//
// It is a separate type from Manifest because it carries no device and no deploy bytes.
// The two share the sync pipeline (scope, plan, apply, drift) but not their payload, and
// conflating them would make every device field meaningless on an org file.
type OrgResource struct {
	// Err is a discovery-time problem (unreadable file, a symlink, an unknown kind).
	Err error
	// Path is the resource path within the repository, e.g. orgs/acme/alerts.toml.
	Path string
	// Org is the organisation name from the directory.
	Org string
	// Kind is what the file declares.
	Kind ResourceKind
	// ContentHash is the SHA-256 of the file's bytes, so drift is detectable without
	// re-parsing.
	ContentHash string
	// Bytes is the file content, already read.
	Bytes []byte
}

// DiscoverOrgFiles walks root/syncPath and returns every recognised org file.
//
// It mirrors Discover's rules: only regular files, never a symlink (a repository is
// untrusted content and a committed symlink would let it read a host file), sorted by path
// so a run is stable. A file under orgs/ whose name is not a known kind is reported as an
// error on that file rather than skipped, so a typo is visible.
func DiscoverOrgFiles(root, syncPath string) ([]*OrgResource, error) {
	start, err := syncRoot(root, syncPath)
	if err != nil {
		return nil, err
	}

	orgsRoot := filepath.Join(start, "orgs")

	// No orgs/ directory is the common case: most repositories hold devices only. Any other
	// stat failure (permissions, I/O) is a real error and must not read as "nothing to sync",
	// or a broken checkout would silently report a clean run.
	if _, statErr := os.Stat(orgsRoot); errors.Is(statErr, fs.ErrNotExist) {
		return nil, nil
	} else if statErr != nil {
		return nil, fmt.Errorf("could not read %s: %w", orgsRoot, statErr)
	}

	var out []*OrgResource

	walkErr := filepath.WalkDir(orgsRoot, func(full string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Type()&fs.ModeSymlink != 0 {
				return fs.SkipDir
			}

			return nil
		}

		if _, known := OrgFileNames[d.Name()]; !known {
			// Only a file that looks like a declaration is reported; a README in orgs/ is
			// ordinary and must not fail a sync.
			if filepath.Ext(d.Name()) != ".toml" {
				return nil
			}
		}

		out = append(out, readOrgResource(start, full, d))

		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })

	markDuplicateOrgFiles(out)

	return out, nil
}

// markDuplicateOrgFiles flags two files of the same kind for one organisation.
//
// The layout is one file per kind per org, so orgs/acme/alerts.toml and
// orgs/acme/nested/alerts.toml is a mistake. Left alone both would apply, last writer
// winning, and which one won would depend on walk order. Both are flagged with a message
// naming the conflict, mirroring the one-manifest-per-device rule.
func markDuplicateOrgFiles(files []*OrgResource) {
	type key struct {
		kind ResourceKind
		org  string
	}

	groups := make(map[key][]*OrgResource, len(files))

	for _, f := range files {
		if f.Err != nil || f.Org == "" {
			continue
		}

		k := key{kind: f.Kind, org: f.Org}
		groups[k] = append(groups[k], f)
	}

	for k, group := range groups {
		if len(group) < 2 {
			continue
		}

		paths := make([]string, len(group))
		for i, f := range group {
			paths[i] = f.Path
		}
		sort.Strings(paths)

		msg := fmt.Sprintf("organisation %s declares %d %s files (%s); keep exactly one",
			k.org, len(group), k.kind, strings.Join(paths, ", "))

		for _, f := range group {
			f.Err = errors.New(msg)
		}
	}
}

// readOrgResource reads one org file and resolves its kind and organisation.
func readOrgResource(root, full string, d fs.DirEntry) *OrgResource {
	rel := relativePath(root, full)
	r := &OrgResource{Path: rel}

	if d.Type()&fs.ModeSymlink != 0 {
		r.Err = fmt.Errorf("%s is a symlink, which is not allowed in a provisioning repository", rel)

		return r
	}

	kind, org, err := ClassifyPath(rel)
	if err != nil {
		r.Err = err

		return r
	}

	data, readErr := os.ReadFile(full) // #nosec G304 -- full is a path walked from the checkout root.
	if readErr != nil {
		r.Err = fmt.Errorf("could not read %s: %w", rel, readErr)

		return r
	}

	sum := sha256.Sum256(data)
	r.ContentHash = hex.EncodeToString(sum[:])
	r.Bytes = data
	r.Kind = kind
	r.Org = org

	return r
}
