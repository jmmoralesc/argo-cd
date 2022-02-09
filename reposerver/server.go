package reposerver

import (
	"crypto/tls"
	"fmt"
	"os"
	"path/filepath"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_logrus "github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/argoproj/argo-cd/v2/common"
	versionpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/version"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	reposervercache "github.com/argoproj/argo-cd/v2/reposerver/cache"
	"github.com/argoproj/argo-cd/v2/reposerver/metrics"
	"github.com/argoproj/argo-cd/v2/reposerver/repository"
	"github.com/argoproj/argo-cd/v2/server/version"
	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/env"
	"github.com/argoproj/argo-cd/v2/util/git"
	grpc_util "github.com/argoproj/argo-cd/v2/util/grpc"
	tlsutil "github.com/argoproj/argo-cd/v2/util/tls"
)

// ArgoCDRepoServer is the repo server implementation
type ArgoCDRepoServer struct {
	log           *log.Entry
	repoService   *repository.Service
	metricsServer *metrics.MetricsServer
	gitCredsStore git.CredsStore
	cache         *reposervercache.Cache
	opts          []grpc.ServerOption
	initConstants repository.RepoServerInitConstants
}

// The hostnames to generate self-signed issues with
var tlsHostList []string = []string{"localhost", "reposerver"}

// NewServer returns a new instance of the Argo CD Repo server
func NewServer(metricsServer *metrics.MetricsServer, cache *reposervercache.Cache, tlsConfCustomizer tlsutil.ConfigCustomizer, initConstants repository.RepoServerInitConstants, gitCredsStore git.CredsStore) (*ArgoCDRepoServer, error) {
	var tlsConfig *tls.Config

	// Generate or load TLS server certificates to use with this instance of
	// repository server.
	if tlsConfCustomizer != nil {
		var err error
		certPath := fmt.Sprintf("%s/reposerver/tls/tls.crt", env.StringFromEnv(common.EnvAppConfigPath, common.DefaultAppConfigPath))
		keyPath := fmt.Sprintf("%s/reposerver/tls/tls.key", env.StringFromEnv(common.EnvAppConfigPath, common.DefaultAppConfigPath))
		tlsConfig, err = tlsutil.CreateServerTLSConfig(certPath, keyPath, tlsHostList)
		if err != nil {
			return nil, err
		}
		tlsConfCustomizer(tlsConfig)
	}

	if os.Getenv(common.EnvEnableGRPCTimeHistogramEnv) == "true" {
		grpc_prometheus.EnableHandlingTimeHistogram()
	}

	serverLog := log.NewEntry(log.StandardLogger())
	streamInterceptors := []grpc.StreamServerInterceptor{grpc_logrus.StreamServerInterceptor(serverLog), grpc_prometheus.StreamServerInterceptor, grpc_util.PanicLoggerStreamServerInterceptor(serverLog)}
	unaryInterceptors := []grpc.UnaryServerInterceptor{
		grpc_logrus.UnaryServerInterceptor(serverLog),
		grpc_prometheus.UnaryServerInterceptor, grpc_util.PanicLoggerUnaryServerInterceptor(serverLog),
		grpc_util.ErrorSanitizerUnaryServerInterceptor(),
	}

	serverOpts := []grpc.ServerOption{
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(unaryInterceptors...)),
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(streamInterceptors...)),
		grpc.MaxRecvMsgSize(apiclient.MaxGRPCMessageSize),
		grpc.MaxSendMsgSize(apiclient.MaxGRPCMessageSize),
	}

	// We do allow for non-TLS servers to be created, in case of mTLS will be
	// implemented by e.g. a sidecar container.
	if tlsConfig != nil {
		serverOpts = append(serverOpts, grpc.Creds(credentials.NewTLS(tlsConfig)))
	}
	repoService := repository.NewService(metricsServer, cache, initConstants, argo.NewResourceTracking(), filepath.Join(os.TempDir(), "_argocd-repo"))
	if err := repoService.Init(); err != nil {
		return nil, err
	}

	return &ArgoCDRepoServer{
		log:           serverLog,
		metricsServer: metricsServer,
		cache:         cache,
		initConstants: initConstants,
		opts:          serverOpts,
<<<<<<< HEAD
		gitCredsStore: gitCredsStore,
=======
		repoService:   repoService,
>>>>>>> afe616a79 (support restoring URLs from file system; sanitize error messages)
	}, nil
}

// CreateGRPC creates new configured grpc server
func (a *ArgoCDRepoServer) CreateGRPC() *grpc.Server {
	server := grpc.NewServer(a.opts...)
	versionpkg.RegisterVersionServiceServer(server, version.NewServer(nil, func() (bool, error) {
		return true, nil
	}))
<<<<<<< HEAD
	manifestService := repository.NewService(a.metricsServer, a.cache, a.initConstants, argo.NewResourceTracking(), a.gitCredsStore)
	apiclient.RegisterRepoServerServiceServer(server, manifestService)
=======
	apiclient.RegisterRepoServerServiceServer(server, a.repoService)
>>>>>>> afe616a79 (support restoring URLs from file system; sanitize error messages)

	healthService := health.NewServer()
	grpc_health_v1.RegisterHealthServer(server, healthService)

	// Register reflection service on gRPC server.
	reflection.Register(server)

	return server
}
