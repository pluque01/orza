package integration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/catalog"
	"github.com/pluque01/orza/internal/catalogrepo"
	"github.com/pluque01/orza/internal/credential"
	"github.com/pluque01/orza/internal/hostkey"
	"github.com/pluque01/orza/internal/sshclient"
)

type forwardingAppFixture struct {
	service     *app.TunnelService
	connections *app.ConnectionService
	folders     *app.FolderService
	store       *credential.Fake
	trusted     app.TrustedHostRepository
	options     app.TunnelOptions
	path        string
}

func newForwardingAppFixture(t *testing.T, knownHosts string) *forwardingAppFixture {
	t.Helper()
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, catalog.CatalogFileName)
	db, err := catalog.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("catalog mode = %o", info.Mode().Perm())
		}
	}
	t.Cleanup(func() { _ = db.Close() })
	repository := catalogrepo.NewRepository(db)
	store := credential.NewFake()
	scope := credential.Scope("ffffffffffffffffffffffffffffffff")
	saga, err := app.NewCredentialSaga(app.NewCatalogCredentialOperationRepository(db), store, scope, nil)
	if err != nil {
		t.Fatal(err)
	}
	connections, err := app.NewConnectionService(repository, saga)
	if err != nil {
		t.Fatal(err)
	}
	folders, err := app.NewFolderService(repository, saga)
	if err != nil {
		t.Fatal(err)
	}
	trusted := hostkey.NewCatalogTrustedHostAdapter(catalog.NewTrustedHostRepository(db))
	knownPath := filepath.Join(directory, "known_hosts")
	if err := os.WriteFile(knownPath, []byte(knownHosts), 0o600); err != nil {
		t.Fatal(err)
	}
	policy, err := hostkey.NewPolicy(trusted, knownPath)
	if err != nil {
		t.Fatal(err)
	}
	options := app.TunnelOptions{Connections: repository, Credentials: saga, HostTrust: policy, TrustedHosts: trusted, Store: store, Scope: scope, Runner: sshclient.New(sshclient.Options{})}
	service, err := app.NewTunnelService(options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	return &forwardingAppFixture{service: service, connections: connections, folders: folders, store: store, trusted: trusted, options: options, path: path}
}

func (f *forwardingAppFixture) create(t *testing.T, server *forwardingTestServer, name string) app.Connection {
	t.Helper()
	endpoint := forwardingEndpoint(t, server.address)
	created, err := f.connections.Create(context.Background(), app.CreateConnectionRequest{
		Parent: app.ItemSelector{Path: "/"}, Name: name, Host: endpoint.Host, Port: endpoint.Port, Username: "tester", AuthMethod: app.AuthMethodPassword,
		CredentialIntent: app.CredentialRemember, Password: []byte(server.config.Password),
	})
	if err != nil {
		t.Fatal(err)
	}
	return created.Connection
}

func (f *forwardingAppFixture) trust(t *testing.T, server *forwardingTestServer) {
	t.Helper()
	endpoint := forwardingEndpoint(t, server.address)
	presented, err := hostkey.NewPresentedHost(app.HostEndpoint{CanonicalHost: endpoint.Host, Port: endpoint.Port}, server.address, server.signer.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.trusted.TrustHost(context.Background(), app.TrustHostRequest{Host: presented}); err != nil {
		t.Fatal(err)
	}
}

func forwardingAppRequest(connection app.Connection, config app.TunnelConfig) app.TunnelRequest {
	revision := connection.Revision
	return app.TunnelRequest{Connection: app.ItemSelector{ID: connection.ID}, Expected: &revision, Config: config, NonInteractive: true}
}

func forwardingAppState(t *testing.T, service *app.TunnelService, id uint64, state app.TunnelState) app.TunnelSnapshot {
	t.Helper()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		snapshot, ok := service.Get(id)
		if !ok {
			t.Fatalf("missing tunnel %d", id)
		}
		if snapshot.State == state {
			return snapshot
		}
		if snapshot.State == app.TunnelFailed && state != app.TunnelFailed {
			t.Fatalf("tunnel failed: %+v", snapshot)
		}
		select {
		case <-tick.C:
		case <-deadline.C:
			t.Fatalf("tunnel %d state = %s, want %s", id, snapshot.State, state)
		}
	}
}

func forwardingAppStop(t *testing.T, service *app.TunnelService, id uint64) {
	t.Helper()
	result := make(chan error, 1)
	go func() { result <- service.Stop(context.Background(), id) }()
	if err := forwardingFixtureWait(t, result); err != nil {
		t.Fatal(err)
	}
	forwardingAppState(t, service, id, app.TunnelStopped)
}

func TestTunnelManagementForwardingPinnedCatalogRetryAndIndependentStop(t *testing.T) {
	server := startForwardingServer(t, forwardingServerConfig{})
	f := newForwardingAppFixture(t, "")
	f.trust(t, server)
	first := f.create(t, server, "first")
	second := f.create(t, server, "second")
	address, _ := startForwardingDestination(t)
	configs := []app.TunnelConfig{
		{Mode: app.TunnelLocal, Listen: forwardingFreeEndpoint(t), Destination: forwardingEndpoint(t, address)},
		{Mode: app.TunnelLocal, Listen: forwardingFreeEndpoint(t), Destination: forwardingEndpoint(t, address)},
	}
	snapshots := make([]app.TunnelSnapshot, 2)
	var firstTransport forwardingTransportID
	for i, connection := range []app.Connection{first, second} {
		snapshot, err := f.service.Start(context.Background(), forwardingAppRequest(connection, configs[i]))
		if err != nil {
			t.Fatal(err)
		}
		snapshots[i] = forwardingAppState(t, f.service, snapshot.ID, app.TunnelActive)
		id := server.WaitTransport(t)
		if i == 0 {
			firstTransport = id
		}
	}
	name := "renamed"
	updated, err := f.connections.Update(context.Background(), app.UpdateConnectionRequest{Connection: app.ItemSelector{ID: first.ID}, Expected: &first.Revision, Name: &name})
	if err != nil {
		t.Fatal(err)
	}
	folder, err := f.folders.Create(context.Background(), app.CreateFolderRequest{Parent: app.ItemSelector{Path: "/"}, Name: "moved"})
	if err != nil {
		t.Fatal(err)
	}
	moved, err := f.connections.Move(context.Background(), app.MoveConnectionRequest{Connection: app.ItemSelector{ID: first.ID}, Expected: &updated.Connection.Revision, Destination: app.ItemSelector{ID: folder.Folder.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if captured, _ := f.service.Get(snapshots[0].ID); captured.Connection != snapshots[0].Connection {
		t.Fatal("running target relabeled by catalog update/move")
	}
	if _, err := f.service.Start(context.Background(), forwardingAppRequest(first, app.TunnelConfig{Mode: app.TunnelLocal, Listen: forwardingFreeEndpoint(t), Destination: configs[0].Destination})); !errors.Is(err, app.ErrConflict) {
		t.Fatalf("stale start = %v", err)
	}
	for _, config := range configs {
		if err := forwardingExchange(forwardingDial(t, config, config.Destination), []byte("catalog-independent")); err != nil {
			t.Fatal(err)
		}
	}
	server.CloseTransport(firstTransport)
	failed := forwardingAppState(t, f.service, snapshots[0].ID, app.TunnelFailed)
	forwardingPortReleased(t, configs[0].Listen)
	retried, err := f.service.Retry(context.Background(), failed.ID, forwardingAppRequest(moved.Connection, configs[0]))
	if err != nil {
		t.Fatal(err)
	}
	retried = forwardingAppState(t, f.service, retried.ID, app.TunnelActive)
	if retried.ID != failed.ID || retried.Attempt <= failed.Attempt || retried.Connection.Path != moved.Connection.Path || retried.Connection.Revision != moved.Connection.Revision {
		t.Fatalf("retry snapshot = %+v", retried)
	}
	server.WaitTransport(t)
	forwardingAppStop(t, f.service, retried.ID)
	forwardingPortReleased(t, configs[0].Listen)
	if _, err := f.connections.Delete(context.Background(), app.DeleteConnectionRequest{Connection: app.ItemSelector{ID: second.ID}, Expected: &second.Revision}); err != nil {
		t.Fatal(err)
	}
	if captured, _ := f.service.Get(snapshots[1].ID); captured.Connection != snapshots[1].Connection {
		t.Fatal("deleted running target changed")
	}
	if err := forwardingExchange(forwardingDial(t, configs[1], configs[1].Destination), []byte("still-live-after-delete")); err != nil {
		t.Fatal(err)
	}
	forwardingAppStop(t, f.service, snapshots[1].ID)
	if _, err := f.service.Retry(context.Background(), snapshots[1].ID, forwardingAppRequest(second, configs[1])); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("missing retry = %v", err)
	}
	forwardingPortReleased(t, configs[1].Listen)
}

func TestTunnelManagementForwardingCapacityEvictionAndSessionOnly(t *testing.T) {
	server := startForwardingServer(t, forwardingServerConfig{})
	f := newForwardingAppFixture(t, "")
	f.trust(t, server)
	connection := f.create(t, server, "capacity")
	address, _ := startForwardingDestination(t)
	config := app.TunnelConfig{Mode: app.TunnelLocal, Destination: forwardingEndpoint(t, address)}
	ids := make([]uint64, 0, 16)
	ports := make([]app.TunnelEndpoint, 0, 16)
	for range 16 {
		config.Listen = forwardingFreeEndpoint(t)
		snapshot, err := f.service.Start(context.Background(), forwardingAppRequest(connection, config))
		if err != nil {
			t.Fatal(err)
		}
		forwardingAppState(t, f.service, snapshot.ID, app.TunnelActive)
		server.WaitTransport(t)
		ids = append(ids, snapshot.ID)
		ports = append(ports, config.Listen)
	}
	config.Listen = forwardingFreeEndpoint(t)
	if _, err := f.service.Start(context.Background(), forwardingAppRequest(connection, config)); !errors.Is(err, app.ErrConflict) {
		t.Fatalf("capacity admission = %v", err)
	}
	if err := forwardingExchange(forwardingDial(t, app.TunnelConfig{Mode: app.TunnelLocal, Listen: ports[0]}, config.Destination), []byte("capacity-does-not-evict")); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		forwardingAppStop(t, f.service, id)
	}
	for _, port := range ports {
		forwardingPortReleased(t, port)
	}
	for range 18 {
		config.Listen = forwardingFreeEndpoint(t)
		snapshot, err := f.service.Start(context.Background(), forwardingAppRequest(connection, config))
		if err != nil {
			t.Fatal(err)
		}
		forwardingAppState(t, f.service, snapshot.ID, app.TunnelActive)
		server.WaitTransport(t)
		forwardingAppStop(t, f.service, snapshot.ID)
		forwardingPortReleased(t, config.Listen)
	}
	snapshots := f.service.Snapshots()
	if len(snapshots) != 32 || snapshots[0].ID != 3 || snapshots[31].ID != 34 {
		t.Fatalf("terminal eviction: %+v", snapshots)
	}
	if err := f.service.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := app.NewTunnelService(f.options)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if len(reopened.Snapshots()) != 0 {
		t.Fatal("tunnels persisted across session restart")
	}
	if _, err := f.connections.Get(context.Background(), app.ItemSelector{ID: connection.ID}); err != nil {
		t.Fatal("catalog connection did not survive session restart")
	}
}
