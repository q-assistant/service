package service

import (
	"context"
	"fmt"
	"github.com/q-assistant/service/config"
	"github.com/q-assistant/service/discovery"
	"github.com/q-assistant/service/internal"
	"github.com/q-assistant/service/logger"
	"github.com/q-assistant/service/server"
	"github.com/q-assistant/service/update"
	"google.golang.org/grpc"
	"net"
	"os"
	"os/signal"
	"syscall"
)

type Service struct {
	name                   string
	tags                   []string
	meta                   map[string]string
	discovery              discovery.Discovery
	dependencies           []string
	config                 config.Config
	server                 grpc.Server
	serverConfig           *server.Config
	listener               net.Listener
	logger                 *logger.Logger
	onConfigUpdateFunc     update.UpdateFunc
	onDependencyUpdateFunc update.UpdateFunc
	updates                chan *update.Update
	ctx                    context.Context
	cancel                 context.CancelFunc
	allDependenciesOnline  chan bool
}

func New(name string, tags []string, meta map[string]string) (*Service, error) {
	ctx, cancel := context.WithCancel(context.Background())
	updates := make(chan *update.Update)

	lgr := logger.NewLogger(name)

	disc, err := discovery.NewConsulClient(ctx, lgr, updates)
	if err != nil {
		cancel()
		return nil, err
	}

	s := &Service{
		name:      name,
		discovery: disc,
		logger:    lgr,
		updates:   updates,
		ctx:       ctx,
		cancel:    cancel,
		tags:      tags,
		meta:      meta,
	}

	go s.handleUpdates()

	return s, nil
}

func (s *Service) WithDependencies(deps ...string) {
	s.dependencies = deps
	s.discovery.WithDependencies(s.dependencies)
}

func (s *Service) WithConfig(data map[string]interface{}) (config.Config, error) {
	var err error

	if s.config, err = config.NewConsulClient(s.ctx, s.logger, s.updates, data); err != nil {
		return nil, err
	}

	return s.config, nil
}

func (s *Service) WithServer(fn server.ServerFunc) error {
	address, err := internal.GetLocalIP()
	if err != nil {
		return fmt.Errorf("unable to obtain local ip: %w", err)
	}

	port, err := internal.GetFreePort()
	if err != nil {
		return fmt.Errorf("unable to obtain free port: %w", err)
	}

	s.serverConfig = &server.Config{
		Address: address,
		Port:    port,
		Fn:      fn,
	}

	return nil
}

// OnDependencyUpdate is a callback to handle dependency updates
func (s *Service) OnDependencyUpdate(fn update.UpdateFunc) {
	s.onDependencyUpdateFunc = fn
}

// OnConfigUpdate is a callback to handle config updates
func (s *Service) OnConfigUpdate(fn update.UpdateFunc) {
	s.onConfigUpdateFunc = fn
}

func (s *Service) Discovery() *discovery.Finder {
	return discovery.NewFinder(s.discovery)
}

func (s *Service) Logger() *logger.Logger {
	return s.logger
}

func (s *Service) Run() {
	stop := make(chan os.Signal)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	if s.dependencies != nil {
		s.logger.Info("waiting for all dependencies to be online")
		s.allDependenciesOnline = s.discovery.AllDependenciesOnline()

		<-s.allDependenciesOnline
		s.logger.Info("all dependencies are online")
	}

	if s.serverConfig != nil {
		s.discovery.Register(&discovery.Registration{
			Port:    s.serverConfig.Port,
			Name:    s.name,
			Address: s.serverConfig.Address,
			Tags:    s.tags,
			Meta:    s.meta,
		})
		lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", s.serverConfig.Address, s.serverConfig.Port))
		if err != nil {

		}

		grpcServer := grpc.NewServer()

		s.serverConfig.Fn(grpcServer)

		go func() {
			if err := grpcServer.Serve(lis); err != nil {
				s.logger.Fatal(fmt.Sprintf("unable to start grpc server: %s", err))
			}
		}()

		s.logger.Info(fmt.Sprintf("service running at %s:%d", s.serverConfig.Address, s.serverConfig.Port))
	}

	<-stop

	s.cancel()
	close(s.updates)
}

func (s *Service) handleUpdates() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case u := <-s.updates:
			switch u.Kind {
			case update.UpdateKindConfig:
				if s.onConfigUpdateFunc != nil {
					s.onConfigUpdateFunc(u)
				}
			case update.UpdateKindDependency:
				if s.onDependencyUpdateFunc != nil {
					s.onDependencyUpdateFunc(u)
				}
			}
		case <-s.allDependenciesOnline:
		}
	}
}
