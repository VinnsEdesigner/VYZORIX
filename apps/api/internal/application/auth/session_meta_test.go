package auth

import (
        "context"
        "errors"
        "testing"

        "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/session"
)

// recordingSessionRepo records UpdateSessionMeta calls. It embeds the interface
// so only the methods the test exercises need a body.
type recordingSessionRepo struct {
        session.Repository
        calls []metaCall
        err   error
}

type metaCall struct {
        id, ip, ua string
}

func (r *recordingSessionRepo) UpdateSessionMeta(_ context.Context, id, ipAddress, userAgent string) error {
        if r.err != nil {
                return r.err
        }
        r.calls = append(r.calls, metaCall{id: id, ip: ipAddress, ua: userAgent})
        return nil
}

func TestBackfillSessionMeta_FillsMissingFields(t *testing.T) {
        repo := &recordingSessionRepo{}
        svc := &AuthService{sessionRepo: repo}

        sess := &session.Session{ID: "s-1"}
        svc.BackfillSessionMeta(context.Background(), sess, "203.0.113.5", "Mozilla/5.0")

        if len(repo.calls) != 1 {
                t.Fatalf("want 1 update, got %d", len(repo.calls))
        }
        if repo.calls[0] != (metaCall{id: "s-1", ip: "203.0.113.5", ua: "Mozilla/5.0"}) {
                t.Errorf("unexpected update: %+v", repo.calls[0])
        }
        // The in-request copy is updated so a handler in the same request sees it.
        if sess.IPAddress != "203.0.113.5" || sess.UserAgent != "Mozilla/5.0" {
                t.Errorf("session not updated in place: %q / %q", sess.IPAddress, sess.UserAgent)
        }
}

func TestBackfillSessionMeta_SkipsWhenAlreadySet(t *testing.T) {
        repo := &recordingSessionRepo{}
        svc := &AuthService{sessionRepo: repo}

        sess := &session.Session{ID: "s-2", IPAddress: "10.0.0.1", UserAgent: "UA"}
        svc.BackfillSessionMeta(context.Background(), sess, "203.0.113.5", "Mozilla/5.0")

        if len(repo.calls) != 0 {
                t.Fatalf("a fully-populated session must not be touched, got %d updates", len(repo.calls))
        }
}

func TestBackfillSessionMeta_SkipsWhenNothingToWrite(t *testing.T) {
        repo := &recordingSessionRepo{}
        svc := &AuthService{sessionRepo: repo}

        svc.BackfillSessionMeta(context.Background(), &session.Session{ID: "s-3"}, "", "")
        if len(repo.calls) != 0 {
                t.Fatalf("no source values means no write, got %d updates", len(repo.calls))
        }
}

func TestBackfillSessionMeta_SwallowsWriteError(t *testing.T) {
        repo := &recordingSessionRepo{err: errors.New("db down")}
        svc := &AuthService{sessionRepo: repo}

        sess := &session.Session{ID: "s-4"}
        // Must not panic or propagate; the request continues.
        svc.BackfillSessionMeta(context.Background(), sess, "203.0.113.5", "Mozilla/5.0")
        if sess.IPAddress != "" {
                t.Errorf("a failed write must not mutate the session: %q", sess.IPAddress)
        }
}

func TestBackfillSessionMeta_NilSessionIsNoop(t *testing.T) {
        repo := &recordingSessionRepo{}
        svc := &AuthService{sessionRepo: repo}
        svc.BackfillSessionMeta(context.Background(), nil, "203.0.113.5", "UA")
        if len(repo.calls) != 0 {
                t.Fatalf("nil session must be a no-op, got %d updates", len(repo.calls))
        }
}
