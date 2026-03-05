package app

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"

	"github.com/Gthulhu/api/config"
	"github.com/Gthulhu/api/decisionmaker/rest"
	"github.com/Gthulhu/api/pkg/logger"
	"github.com/labstack/echo/v4"
	"go.uber.org/fx"
)

func NewRestApp(configName string, configDirPath string) (*fx.App, error) {
	cfg, err := config.InitDMConfig(configName, configDirPath)
	if err != nil {
		return nil, err
	}
	cfgModule, err := ConfigModule(cfg)
	if err != nil {
		return nil, err
	}
	svcModule, err := ServiceModule()
	handlerModule, err := HandlerModule(fx.Options(cfgModule, svcModule))
	if err != nil {
		return nil, err
	}

	app := fx.New(
		handlerModule,
		fx.Invoke(StartRestApp),
	)
	return app, nil
}

func StartRestApp(lc fx.Lifecycle, cfg config.ServerConfig, mtlsCfg config.MTLSConfig, handler *rest.Handler) error {
	engine := echo.New()
	if err := handler.SetupRoutes(engine); err != nil {
		return err
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			serverHost := cfg.Host
			if serverHost == "" {
				serverHost = ":8082"
			}
			go func() {
				if mtlsCfg.Enable {
					if err := startTLSServer(ctx, engine, serverHost, mtlsCfg); err != nil {
						logger.Logger(ctx).Fatal().Err(err).Msgf("start dm rest server with mTLS fail on port %s", serverHost)
					}
				} else {
					logger.Logger(ctx).Info().Msgf("starting dm server on port %s", serverHost)
					if err := engine.Start(serverHost); err != nil {
						logger.Logger(ctx).Fatal().Err(err).Msgf("start rest server fail on port %s", serverHost)
					}
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Logger(ctx).Info().Msg("shutting down dm server")
			return engine.Shutdown(ctx)
		},
	})

	return nil
}

// startTLSServer starts the Echo server with mTLS: the server presents its own certificate and
// requires the connecting client (Manager) to present a certificate signed by the shared CA.
func startTLSServer(ctx context.Context, engine *echo.Echo, addr string, mtlsCfg config.MTLSConfig) error {
	cert, err := tls.X509KeyPair([]byte(mtlsCfg.CertPem.Value()), []byte(mtlsCfg.KeyPem.Value()))
	if err != nil {
		return fmt.Errorf("load mTLS server certificate: %w", err)
	}

	caPool := x509.NewCertPool()
	caPEM := mtlsCfg.CAPem.Value()
	if caPEM == "" {
		return fmt.Errorf("mTLS server CA PEM is empty; cannot configure client certificate validation")
	}
	if !caPool.AppendCertsFromPEM([]byte(caPEM)) {
		return fmt.Errorf("no CA certificates found in mTLS server CA PEM; failed to parse CA bundle")
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caPool,
		MinVersion:   tls.VersionTLS12,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("create listener: %w", err)
	}
	tlsListener := tls.NewListener(ln, tlsCfg)
	engine.Listener = tlsListener

	logger.Logger(ctx).Info().Msgf("starting dm server with mTLS on port %s", addr)
	return engine.Start("")
}
