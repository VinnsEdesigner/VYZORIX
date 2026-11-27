package provisioning_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

// consoleVocabulary is the console's mirror of the domain enums.
func consoleVocabulary(t *testing.T) []byte {
	t.Helper()

	path := filepath.Join("..", "..", "..", "..", "..",
		"packages", "API_Client", "src", "domain", "provisioning", "provisioning-vocabulary.ts")

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("console vocabulary not readable at %s: %v", path, err)
	}

	return raw
}

// TestVocabularyMatchesConsole asserts the Go enums and the console's mirrored lists agree.
//
// The console switches on these strings and labels them for a person, so a value added on
// one side only is either a row the console cannot render (server adds) or a request the
// server rejects (client adds). The lists are one contract split across languages; this is
// the same cross-language guard the theme and preference keys already use.
func TestVocabularyMatchesConsole(t *testing.T) {
	raw := consoleVocabulary(t)

	cases := []struct {
		list   string
		server []string
	}{
		{"PROVIDER_TYPES", []string{
			string(domain.ConnectionGitHub), string(domain.ConnectionGitLab), string(domain.ConnectionGeneric),
		}},
		{"CONNECTION_STATUSES", []string{
			string(domain.ConnectionPending), string(domain.ConnectionReady), string(domain.ConnectionError),
		}},
		{"SYNC_STATES", []string{
			string(domain.SyncPending), string(domain.SyncSuccess), string(domain.SyncWarning),
			string(domain.SyncWorking), string(domain.SyncError), string(domain.SyncDisabled),
		}},
		{"RESOURCE_STATES", []string{
			string(domain.ResourceApplied), string(domain.ResourcePending), string(domain.ResourceDrifted),
			string(domain.ResourceMissing), string(domain.ResourceError), string(domain.ResourceIgnored),
		}},
		{"RESOURCE_KINDS", []string{
			string(domain.KindDevice), string(domain.KindAlerts), string(domain.KindContactPoints),
			string(domain.KindAnnotations), string(domain.KindGroups),
		}},
		{"JOB_OUTCOMES", []string{
			string(domain.ResultApplied), string(domain.ResultUpdated), string(domain.ResultUnchanged),
			string(domain.ResultMissing), string(domain.ResultIgnored), string(domain.ResultSkipped),
			string(domain.ResultFailed),
		}},
		{"ACTOR_KINDS", []string{
			string(domain.ActorUser), string(domain.ActorServiceAccount), string(domain.ActorScheduler),
			string(domain.ActorSystem), string(domain.ActorProvider),
		}},
		{"EVENT_TYPES", []string{
			string(domain.EventConnectionCreated), string(domain.EventConnectionUpdated),
			string(domain.EventConnectionDeleted), string(domain.EventConnectionChecked),
			string(domain.EventRepositoryCreated), string(domain.EventRepositoryUpdated),
			string(domain.EventRepositoryDeleted), string(domain.EventSyncTriggered),
			string(domain.EventSyncApplied), string(domain.EventDocumentPushed),
		}},
	}

	for _, tc := range cases {
		t.Run(tc.list, func(t *testing.T) {
			got := readTSList(t, raw, tc.list)

			want := append([]string(nil), tc.server...)
			sort.Strings(got)
			sort.Strings(want)

			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Fatalf("%s drifted from the server enum:\n  console: %v\n  server:  %v",
					tc.list, got, want)
			}
		})
	}
}

// readTSList extracts the quoted strings from `export const <name> = [ ... ] as const;`.
func readTSList(t *testing.T, raw []byte, name string) []string {
	t.Helper()

	re := regexp.MustCompile(`(?s)\b` + name + `\s*=\s*\[(.*?)\]\s*as const`)
	m := re.FindSubmatch(raw)
	if m == nil {
		t.Fatalf("could not find %s in the console vocabulary", name)
	}

	quoted := regexp.MustCompile(`'([^']*)'`)

	out := make([]string, 0, len(quoted.FindAllStringSubmatch(string(m[1]), -1)))
	for _, match := range quoted.FindAllStringSubmatch(string(m[1]), -1) {
		out = append(out, match[1])
	}

	return out
}
