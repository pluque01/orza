package app

import (
	"context"
	"errors"
	"testing"
)

func TestTunnelStopJoinsAndRejectsLateReady(t *testing.T) {
	entered := make(chan TunnelRunRequest, 1)
	canceled := make(chan struct{})
	release := make(chan struct{})
	s, repo, _, _, _ := tunnelTestService(t, func(ctx context.Context, r TunnelRunRequest) error {
		entered <- r
		<-ctx.Done()
		close(canceled)
		<-release
		r.Ready()
		return ctx.Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	snap, err := s.Start(ctx, tunnelTestRequest(repo))
	if err != nil {
		t.Fatal(err)
	}
	r := <-entered
	cancel()
	r.Ready()
	if got, _ := s.Get(snap.ID); got.State != TunnelActive {
		t.Fatal("operation context owns runtime")
	}
	done := make(chan error, 1)
	go func() { done <- s.Stop(context.Background(), snap.ID) }()
	<-canceled
	// Once stop owns cancellation it must join even if its caller is canceled.
	if got, _ := s.Get(snap.ID); got.State != TunnelStopping {
		t.Fatalf("state before cleanup = %s", got.State)
	}
	select {
	case <-done:
		t.Fatal("stop returned before cleanup")
	default:
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get(snap.ID); got.State != TunnelStopped {
		t.Fatalf("late activation = %s", got.State)
	}
	if err := s.Stop(context.Background(), snap.ID); err != nil {
		t.Fatal(err)
	}
}

func TestTunnelDiagnosticsCoalesceWithoutConsumer(t *testing.T) {
	requests := make(chan TunnelRunRequest, 1)
	s, repo, _, _, _ := tunnelTestService(t, func(ctx context.Context, r TunnelRunRequest) error { requests <- r; <-ctx.Done(); return ctx.Err() })
	snap, err := s.Start(context.Background(), tunnelTestRequest(repo))
	if err != nil {
		t.Fatal(err)
	}
	r := <-requests
	r.Ready()
	active, _ := s.Get(snap.ID)
	for range 10000 {
		r.Ready()
		r.Diagnostic("destination unavailable")
	}
	got, _ := s.Get(snap.ID)
	if got.State != TunnelActive || got.Version != active.Version+1 || got.Diagnostic != "destination unavailable" {
		t.Fatalf("not coalesced: %#v", got)
	}
	for _, unsafe := range []string{"password-canary", "\x1b[31mtransport lost", "transport lost\n", "transport\u202elost", string(make([]byte, 4096))} {
		r.Diagnostic(unsafe)
	}
	if current, _ := s.Get(snap.ID); current != got {
		t.Fatal("unsafe diagnostic accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Wait(ctx, snap.ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("wait cancel = %v", err)
	}
	if current, _ := s.Get(snap.ID); current.State != TunnelActive {
		t.Fatal("wait canceled runtime")
	}
}

func TestTunnelAdapterFailureDiagnosticReplacesDestinationNotice(t *testing.T) {
	for _, diagnostic := range []string{"Forwarding transport lost.", "Forwarding protocol cleanup stalled."} {
		t.Run(diagnostic, func(t *testing.T) {
			requests := make(chan TunnelRunRequest, 1)
			fail := make(chan struct{})
			s, repo, _, _, _ := tunnelTestService(t, func(ctx context.Context, request TunnelRunRequest) error {
				requests <- request
				<-fail
				request.Diagnostic(diagnostic)
				return errors.New("raw-transport-canary")
			})
			snap, err := s.Start(context.Background(), tunnelTestRequest(repo))
			if err != nil {
				t.Fatal(err)
			}
			request := <-requests
			request.Ready()
			request.Diagnostic("Forwarding destination unavailable.")
			before, _ := s.Get(snap.ID)
			if before.Diagnostic != "Forwarding destination unavailable." {
				t.Fatalf("destination notice = %#v", before)
			}
			close(fail)
			if err := s.Wait(context.Background(), snap.ID); err == nil {
				t.Fatal("missing runtime failure")
			}
			got, _ := s.Get(snap.ID)
			if got.State != TunnelFailed || got.Diagnostic != diagnostic || got.Version != before.Version+2 {
				t.Fatalf("failure did not replace destination notice: %#v", got)
			}
		})
	}
}

func TestTunnelStopJoinsAfterCallerCancellation(t *testing.T) {
	entered := make(chan struct{})
	canceled := make(chan struct{})
	release := make(chan struct{})
	s, repo, _, _, _ := tunnelTestService(t, func(ctx context.Context, r TunnelRunRequest) error {
		close(entered)
		<-ctx.Done()
		close(canceled)
		<-release
		return ctx.Err()
	})
	snap, err := s.Start(context.Background(), tunnelTestRequest(repo))
	if err != nil {
		t.Fatal(err)
	}
	<-entered
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.Stop(ctx, snap.ID) }()
	<-canceled
	cancel()
	select {
	case <-done:
		t.Fatal("canceled caller bypassed cleanup join")
	default:
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestTunnelDelayedStopAttemptDoesNotStopRetry(t *testing.T) {
	requests := make(chan TunnelRunRequest, 2)
	s, repo, _, _, _ := tunnelTestService(t, func(ctx context.Context, request TunnelRunRequest) error {
		requests <- request
		<-ctx.Done()
		return ctx.Err()
	})
	first, err := s.Start(context.Background(), tunnelTestRequest(repo))
	if err != nil {
		t.Fatal(err)
	}
	<-requests
	delayed := make(chan struct{})
	commandStarted := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		close(commandStarted)
		<-delayed
		done <- s.StopAttempt(context.Background(), first.ID, first.Attempt)
	}()
	<-commandStarted
	if err := s.Stop(context.Background(), first.ID); err != nil {
		t.Fatal(err)
	}
	retry, err := s.Retry(context.Background(), first.ID, tunnelTestRequest(repo))
	if err != nil {
		t.Fatal(err)
	}
	current := <-requests
	current.Ready()
	before, _ := s.Get(retry.ID)
	close(delayed)
	if err := <-done; err != nil {
		t.Fatalf("stale stop = %v", err)
	}
	if after, _ := s.Get(retry.ID); after != before || after.State != TunnelActive {
		t.Fatalf("stale command changed retry: before=%#v after=%#v", before, after)
	}
	if err := s.StopAttempt(context.Background(), retry.ID, retry.Attempt); err != nil {
		t.Fatal(err)
	}
	if err := s.Dismiss(retry.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.StopAttempt(context.Background(), first.ID, first.Attempt); err != nil {
		t.Fatalf("dismissed attempt cleanup = %v", err)
	}
}

func TestTunnelStopAttemptJoinsMatchingEntryOnly(t *testing.T) {
	for _, active := range []bool{false, true} {
		t.Run(map[bool]string{false: "starting", true: "active"}[active], func(t *testing.T) {
			requests := make(chan TunnelRunRequest, 1)
			canceled := make(chan struct{})
			release := make(chan struct{})
			defer close(release)
			s, repo, _, _, _ := tunnelTestService(t, func(ctx context.Context, request TunnelRunRequest) error {
				requests <- request
				<-ctx.Done()
				close(canceled)
				<-release
				request.Ready()
				return ctx.Err()
			})
			snap, err := s.Start(context.Background(), tunnelTestRequest(repo))
			if err != nil {
				t.Fatal(err)
			}
			request := <-requests
			if active {
				request.Ready()
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- s.StopAttempt(ctx, snap.ID, snap.Attempt) }()
			<-canceled
			cancel()
			stopping, _ := s.Get(snap.ID)
			if stopping.State != TunnelStopping {
				t.Fatalf("matching stop state = %s", stopping.State)
			}
			if err := s.StopAttempt(context.Background(), snap.ID, snap.Attempt+1); err != nil {
				t.Fatalf("mismatched stop = %v", err)
			}
			if got, _ := s.Get(snap.ID); got != stopping {
				t.Fatal("mismatched stop altered matching cleanup")
			}
			select {
			case <-done:
				t.Fatal("matching stop returned before cleanup")
			default:
			}
			release <- struct{}{}
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			if got, _ := s.Get(snap.ID); got.State != TunnelStopped {
				t.Fatalf("state after join = %s", got.State)
			}
			if err := s.StopAttempt(context.Background(), snap.ID, snap.Attempt); err != nil {
				t.Fatalf("repeated matching stop = %v", err)
			}
		})
	}
}

func TestTunnelRetryAtCapacityPreservesTerminalState(t *testing.T) {
	s, repo, _, _, _ := tunnelTestService(t, func(ctx context.Context, r TunnelRunRequest) error { <-ctx.Done(); return ctx.Err() })
	first, err := s.Start(context.Background(), tunnelTestRequest(repo))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(context.Background(), first.ID); err != nil {
		t.Fatal(err)
	}
	before, _ := s.Get(first.ID)
	for range 16 {
		if _, err := s.Start(context.Background(), tunnelTestRequest(repo)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Retry(context.Background(), first.ID, tunnelTestRequest(repo)); !errors.Is(err, ErrConflict) {
		t.Fatalf("capacity retry = %v", err)
	}
	if after, _ := s.Get(first.ID); after != before {
		t.Fatal("capacity retry changed snapshot")
	}
}

func TestTunnelScopeAndIndependentStop(t *testing.T) {
	requests := make(chan TunnelRunRequest, 2)
	s, repo, _, _, _ := tunnelTestService(t, func(ctx context.Context, r TunnelRunRequest) error { requests <- r; <-ctx.Done(); return ctx.Err() })
	remote := tunnelTestRequest(repo)
	remote.Config.Mode = TunnelRemote
	a, err := s.Start(context.Background(), remote)
	if err != nil {
		t.Fatal(err)
	}
	ar := <-requests
	ar.Ready()
	dynamic := tunnelTestRequest(repo)
	dynamic.Config.Mode = TunnelDynamic
	dynamic.Config.Destination = TunnelEndpoint{}
	dynamic.Config.Listen.Host = "0.0.0.0"
	dynamic.Config.ExposureAcknowledged = true
	b, err := s.Start(context.Background(), dynamic)
	if err != nil {
		t.Fatal(err)
	}
	br := <-requests
	br.Ready()
	if a, _ := s.Get(a.ID); a.Scope != "unverified" || a.Warning == "" {
		t.Fatalf("remote scope = %#v", a)
	}
	if b, _ := s.Get(b.ID); b.Scope != "local_bound" || b.Warning == "" {
		t.Fatalf("dynamic scope = %#v", b)
	}
	if err := s.Stop(context.Background(), a.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get(b.ID); got.State != TunnelActive {
		t.Fatal("stop affected unrelated tunnel")
	}
}

func TestTunnelEvictsOldestCompletionNotOldestCreation(t *testing.T) {
	s, repo, _, _, _ := tunnelTestService(t, func(ctx context.Context, r TunnelRunRequest) error { <-ctx.Done(); return ctx.Err() })
	first, err := s.Start(context.Background(), tunnelTestRequest(repo))
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Start(context.Background(), tunnelTestRequest(repo))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(context.Background(), second.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(context.Background(), first.ID); err != nil {
		t.Fatal(err)
	}
	for range 31 {
		snap, err := s.Start(context.Background(), tunnelTestRequest(repo))
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Stop(context.Background(), snap.ID); err != nil {
			t.Fatal(err)
		}
	}
	if _, ok := s.Get(first.ID); !ok {
		t.Fatal("evicted oldest creation")
	}
	if _, ok := s.Get(second.ID); ok {
		t.Fatal("retained oldest completion")
	}
}

func TestTunnelRetryPinnedTargetVersionsAndDismiss(t *testing.T) {
	requests := make(chan TunnelRunRequest, 3)
	s, repo, _, _, _ := tunnelTestService(t, func(ctx context.Context, r TunnelRunRequest) error { requests <- r; <-ctx.Done(); return ctx.Err() })
	snap, err := s.Start(context.Background(), tunnelTestRequest(repo))
	if err != nil {
		t.Fatal(err)
	}
	old := <-requests
	repo.connection.Path = "/renamed"
	repo.connection.Revision++
	if got, _ := s.Get(snap.ID); got.Connection.Path != snap.Connection.Path {
		t.Fatal("running target relabeled")
	}
	if err := s.Dismiss(snap.ID); !errors.Is(err, ErrConflict) {
		t.Fatal("dismissed live runtime")
	}
	if err := s.Stop(context.Background(), snap.ID); err != nil {
		t.Fatal(err)
	}
	retry, err := s.Retry(context.Background(), snap.ID, tunnelTestRequest(repo))
	if err != nil {
		t.Fatal(err)
	}
	current := <-requests
	old.Ready()
	old.Diagnostic("transport lost")
	if got, _ := s.Get(snap.ID); got.State != TunnelStarting || got.Diagnostic != "" {
		t.Fatal("stale callbacks altered retry")
	}
	current.Ready()
	if retry.ID != snap.ID || retry.Attempt <= snap.Attempt || retry.Version <= snap.Version || retry.Connection.Path != "/renamed" {
		t.Fatalf("retry = %#v", retry)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get(snap.ID); got.State != TunnelStopped {
		t.Fatal("close did not join")
	}
	if err := s.Dismiss(snap.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Get(snap.ID); ok {
		t.Fatal("dismiss retained entry")
	}
	if _, err := s.Start(context.Background(), tunnelTestRequest(repo)); err == nil {
		t.Fatal("started after close")
	}
}

func TestTunnelCapacityAndCompletionEviction(t *testing.T) {
	s, repo, _, _, _ := tunnelTestService(t, func(ctx context.Context, r TunnelRunRequest) error { <-ctx.Done(); return ctx.Err() })
	var ids []uint64
	for range 16 {
		snap, err := s.Start(context.Background(), tunnelTestRequest(repo))
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, snap.ID)
	}
	if _, err := s.Start(context.Background(), tunnelTestRequest(repo)); err == nil {
		t.Fatal("live cap not enforced")
	}
	for _, id := range ids {
		if err := s.Stop(context.Background(), id); err != nil {
			t.Fatal(err)
		}
	}
	for range 20 {
		snap, err := s.Start(context.Background(), tunnelTestRequest(repo))
		if err != nil {
			t.Fatal(err)
		}
		if snap.ID <= ids[len(ids)-1] {
			t.Fatal("ID reused")
		}
		ids = append(ids, snap.ID)
		if err := s.Stop(context.Background(), snap.ID); err != nil {
			t.Fatal(err)
		}
	}
	got := s.Snapshots()
	if len(got) != 32 || got[0].ID != ids[4] {
		t.Fatalf("eviction = %#v", got)
	}
	got[0].Diagnostic = "mutated"
	if current, _ := s.Get(got[0].ID); current.Diagnostic != "" {
		t.Fatal("mutable snapshots")
	}
}
