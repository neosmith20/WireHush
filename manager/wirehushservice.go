/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */
package manager

import (
	"context"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.zx2c4.com/wireguard/windows/conf"
	"golang.zx2c4.com/wireguard/windows/product"
	"golang.zx2c4.com/wireguard/windows/protocol"
	"google.golang.org/grpc"
	"time"
)

type wireHushV1Service struct{}

// RunV1 is the production service path. It never launches a UI, starts legacy
// gob IPC, invokes the inherited updater, or treats a disconnected UI as exit.
func RunV1() error { return svc.Run(product.ManagerServiceName, &wireHushV1Service{}) }

func (service *wireHushV1Service) Execute(_ []string, requests <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	changes <- svc.Status{State: svc.StartPending}
	root, err := conf.PrepareWireHushMachineData()
	if err != nil {
		return false, uint32(windows.ERROR_ACCESS_DENIED)
	}
	conf.PresetRootDirectory(root)
	server, err := newWireHushRPCServer()
	if err != nil {
		return false, uint32(windows.ERROR_SERVICE_SPECIFIC_ERROR)
	}
	listener, group, err := listenWireHushPipe()
	if err != nil {
		return false, uint32(windows.ERROR_ACCESS_DENIED)
	}
	defer listener.Close()
	transport := grpc.NewServer(grpc.Creds(wireHushPipeCredentials{group: group}), grpc.StatsHandler(server), grpc.UnaryInterceptor(server.unary), grpc.MaxRecvMsgSize(protocol.MaximumMessageBytes), grpc.MaxSendMsgSize(protocol.MaximumMessageBytes), grpc.MaxConcurrentStreams(16), grpc.ConnectionTimeout(protocol.PipeTimeout))
	protocol.RegisterManagerServer(transport, server)
	serveError := make(chan error, 1)
	go func() { serveError <- transport.Serve(listener) }()
	defer func() {
		close(server.shutdown)
		done := make(chan struct{})
		go func() { transport.GracefulStop(); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			transport.Stop()
			<-done
		}
	}()
	// A recovery inventory failure must not silently erase running network truth.
	// RPC admission/snapshots repeat the authoritative inventory and fail closed.
	if err := trackExistingWireHushTunnelServices(); err != nil {
		return false, uint32(windows.ERROR_SERVICE_SPECIFIC_ERROR)
	}
	current := svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	changes <- current
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var idleSince time.Time
	for {
		select {
		case <-server.stopRequested:
			changes <- svc.Status{State: svc.StopPending}
			return false, 0
		case <-serveError:
			// Unexpected manager transport failure does not kill independent tunnels.
			return false, uint32(windows.ERROR_SERVICE_SPECIFIC_ERROR)
		case request := <-requests:
			switch request.Cmd {
			case svc.Interrogate:
				changes <- current
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending, WaitHint: 10000}
				ctx, cancel := context.WithTimeout(context.Background(), protocol.RequestTimeout)
				err := server.cleanupAll(ctx)
				cancel()
				if err == nil {
					return false, 0
				}
				if request.Cmd == svc.Shutdown {
					return false, uint32(windows.ERROR_SERVICE_SPECIFIC_ERROR)
				}
				// Stop failure is observable as Running; operator may retry. It is
				// never reported as successful cleanup, and configs remain intact.
				changes <- current
			}
		case now := <-ticker.C:
			server.sessionsLock.Lock()
			clients := len(server.sessions)
			server.sessionsLock.Unlock()
			active, err := server.mutations.inventory()
			busy := err != nil
			for _, tunnel := range active {
				if tunnel.State != TunnelStopped {
					busy = true
				}
			}
			if clients != 0 || busy {
				idleSince = time.Time{}
				continue
			}
			if idleSince.IsZero() {
				idleSince = now
				continue
			}
			if now.Sub(idleSince) >= 30*time.Second {
				// Recheck under the mutation gate before committing to idle exit.
				ctx, cancel := context.WithTimeout(context.Background(), protocol.RequestTimeout)
				idle := server.commitIdleExit(ctx)
				cancel()
				if idle {
					changes <- svc.Status{State: svc.StopPending}
					return false, 0
				}
				idleSince = time.Time{}
			}
		}
	}
}

func (server *wireHushRPCServer) cleanupAll(ctx context.Context) error {
	if err := server.mutations.enter(ctx); err != nil {
		return err
	}
	defer server.mutations.leave()
	active, err := server.mutations.inventory()
	if err != nil {
		return err
	}
	for _, tunnel := range active {
		if tunnel.State == TunnelStopped {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := server.mutations.control.withContext(ctx).stopAndWait(tunnel.Locator); err != nil {
			return err
		}
	}
	server.mutations.closing = true
	return nil
}

func (server *wireHushRPCServer) commitIdleExit(ctx context.Context) bool {
	if err := server.mutations.enter(ctx); err != nil {
		return false
	}
	defer server.mutations.leave()
	active, err := server.mutations.inventory()
	if err != nil {
		return false
	}
	for _, tunnel := range active {
		if tunnel.State != TunnelStopped {
			return false
		}
	}
	server.sessionsLock.Lock()
	defer server.sessionsLock.Unlock()
	if len(server.sessions) != 0 {
		return false
	}
	server.stopping = true
	server.mutations.closing = true
	return true
}
