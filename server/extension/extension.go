package extension

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
	"time"

	v1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	applisters "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/server/rbacpolicy"
	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/security"
	"github.com/argoproj/argo-cd/v2/util/settings"
	"github.com/ghodss/yaml"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

const (
	URLPrefix                    = "/extensions"
	DefaultConnectionTimeout     = 2 * time.Second
	DefaultKeepAlive             = 15 * time.Second
	DefaultIdleConnectionTimeout = 60 * time.Second
	DefaultMaxIdleConnections    = 30

	// HeaderArgoCDApplicationName defines the name of the
	// expected application header to be passed to the extension
	// handler. The header value must follow the format:
	//     "<namespace>:<app-name>"
	// Example:
	//     Argocd-Application-Name: "namespace:app-name"
	HeaderArgoCDApplicationName = "Argocd-Application-Name"

	// HeaderArgoCDProjectName defines the name of the expected
	// project header to be passed to the extension handler.
	// Example:
	//     Argocd-Project-Name: "default"
	HeaderArgoCDProjectName = "Argocd-Project-Name"
)

// RequestResources defines the authorization scope for
// an incoming request to a given extension. This struct
// is populated from pre-defined Argo CD headers.
type RequestResources struct {
	ApplicationName      string
	ApplicationNamespace string
	ProjectName          string
}

// ValidateHeaders will validate the pre-defined Argo CD
// request headers for extensions and extract the resources
// information populating and returning a RequestResources
// object.
// The pre-defined headers are:
// - Argocd-Application-Name
// - Argocd-Project-Name
// - Argocd-Resource-GVK-Name
//
// The headers expected format is documented in each of the constant
// types defined for them.
func ValidateHeaders(r *http.Request) (*RequestResources, error) {
	appHeader := r.Header.Get(HeaderArgoCDApplicationName)
	if appHeader == "" {
		return nil, fmt.Errorf("header %q must be provided", HeaderArgoCDApplicationName)
	}
	appNamespace, appName, err := getAppName(appHeader)
	if err != nil {
		return nil, fmt.Errorf("error getting app details: %s", err)
	}
	if !argo.IsValidNamespaceName(appNamespace) {
		return nil, errors.New("invalid value for namespace")
	}
	if !argo.IsValidAppName(appName) {
		return nil, errors.New("invalid value for application name")
	}

	projName := r.Header.Get(HeaderArgoCDProjectName)
	if projName == "" {
		return nil, fmt.Errorf("header %q must be provided", HeaderArgoCDProjectName)
	}
	if !argo.IsValidProjectName(projName) {
		return nil, errors.New("invalid value for project name")
	}
	return &RequestResources{
		ApplicationName:      appName,
		ApplicationNamespace: appNamespace,
		ProjectName:          projName,
	}, nil
}

func getAppName(appHeader string) (string, string, error) {
	parts := strings.Split(appHeader, ":")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid value for %q header: expected format: <namespace>:<app-name>", HeaderArgoCDApplicationName)
	}
	return parts[0], parts[1], nil
}

// ExtensionConfigs defines the configurations for all extensions
// retrieved from Argo CD configmap (argocd-cm).
type ExtensionConfigs struct {
	Extensions []ExtensionConfig `json:"extensions"`
}

// ExtensionConfig defines the configuration for one extension.
type ExtensionConfig struct {
	// Name defines the endpoint that will be used to register
	// the extension route. Mandatory field.
	Name    string        `json:"name"`
	Backend BackendConfig `json:"backend"`
}

// BackendConfig defines the backend service configurations that will
// be used by an specific extension. An extension can have multiple services
// associated. This is necessary when Argo CD is managing applications in
// external clusters. In this case, each cluster may have its own backend
// service.
type BackendConfig struct {
	ProxyConfig
	Services []ServiceConfig `json:"services"`
}

// ProxyConfig allows configuring connection behaviour between Argo CD
// API Server and the backend service.
type ProxyConfig struct {
	// ConnectionTimeout is the maximum amount of time a dial to
	// the extension server will wait for a connect to complete.
	// Default: 2 seconds
	ConnectionTimeout time.Duration `json:"connectionTimeout"`

	// KeepAlive specifies the interval between keep-alive probes
	// for an active network connection between the API server and
	// the extension server.
	// Default: 15 seconds
	KeepAlive time.Duration `json:"keepAlive"`

	// IdleConnectionTimeout is the maximum amount of time an idle
	// (keep-alive) connection between the API server and the extension
	// server will remain idle before closing itself.
	// Default: 60 seconds
	IdleConnectionTimeout time.Duration `json:"idleConnectionTimeout"`

	// MaxIdleConnections controls the maximum number of idle (keep-alive)
	// connections between the API server and the extension server.
	// Default: 30
	MaxIdleConnections int `json:"maxIdleConnections"`
}

// ServiceConfig provides the configuration for a backend service.
type ServiceConfig struct {
	// URL is the address where the extension backend must be available.
	// Mandatory field.
	URL string `json:"url"`

	// Cluster if provided, will have to match the application
	// destination name to have requests properly forwarded to this
	// service URL.
	Cluster string `json:"cluster"`
}

// SettingsGetter defines the contract to retrieve Argo CD Settings.
type SettingsGetter interface {
	Get() (*settings.ArgoCDSettings, error)
}

// DefaultSettingsGetter is the real settings getter implementation.
type DefaultSettingsGetter struct {
	settingsMgr *settings.SettingsManager
}

// NewDefaultSettingsGetter returns a new default settings getter.
func NewDefaultSettingsGetter(mgr *settings.SettingsManager) *DefaultSettingsGetter {
	return &DefaultSettingsGetter{
		settingsMgr: mgr,
	}
}

// Get will retrieve the Argo CD settings.
func (s *DefaultSettingsGetter) Get() (*settings.ArgoCDSettings, error) {
	return s.settingsMgr.GetSettings()
}

// ProjectGetter defines the contract to retrieve Argo CD Project.
type ProjectGetter interface {
	Get(name string) (*v1alpha1.AppProject, error)
	GetClusters(project string) ([]*v1alpha1.Cluster, error)
}

// DefaultProjectGetter is the real ProjectGetter implementation.
type DefaultProjectGetter struct {
	projLister applisters.AppProjectNamespaceLister
	db         db.ArgoDB
}

// NewDefaultProjectGetter returns a new default project getter
func NewDefaultProjectGetter(lister applisters.AppProjectNamespaceLister, db db.ArgoDB) *DefaultProjectGetter {
	return &DefaultProjectGetter{
		projLister: lister,
		db:         db,
	}
}

// Get will retrieve the live AppProject state.
func (p *DefaultProjectGetter) Get(name string) (*v1alpha1.AppProject, error) {
	return p.projLister.Get(name)
}

// GetClusters will retrieve the clusters configured by a project.
func (p *DefaultProjectGetter) GetClusters(project string) ([]*v1alpha1.Cluster, error) {
	return p.db.GetProjectClusters(context.TODO(), project)
}

// ApplicationGetter defines the contract to retrieve the application resource.
type ApplicationGetter interface {
	Get(ns, name string) (*v1alpha1.Application, error)
}

// DefaultApplicationGetter is the real application getter implementation.
type DefaultApplicationGetter struct {
	appLister applisters.ApplicationLister
}

// NewDefaultApplicationGetter returns the default application getter.
func NewDefaultApplicationGetter(al applisters.ApplicationLister) *DefaultApplicationGetter {
	return &DefaultApplicationGetter{
		appLister: al,
	}
}

// Get will retrieve the application resorce for the given namespace and name.
func (a *DefaultApplicationGetter) Get(ns, name string) (*v1alpha1.Application, error) {
	return a.appLister.Applications(ns).Get(name)
}

// RbacEnforcer defines the contract to enforce rbac rules
type RbacEnforcer interface {
	EnforceErr(rvals ...interface{}) error
}

// Manager is the object that will be responsible for registering
// and handling proxy extensions.
type Manager struct {
	log         *log.Entry
	settings    SettingsGetter
	application ApplicationGetter
	project     ProjectGetter
	rbac        RbacEnforcer
}

// NewManager will initialize a new manager.
func NewManager(log *log.Entry, sg SettingsGetter, ag ApplicationGetter, pg ProjectGetter, rbac RbacEnforcer) *Manager {
	return &Manager{
		log:         log,
		settings:    sg,
		application: ag,
		project:     pg,
		rbac:        rbac,
	}
}

func parseAndValidateConfig(config string) (*ExtensionConfigs, error) {
	configs := ExtensionConfigs{}
	err := yaml.Unmarshal([]byte(config), &configs)
	if err != nil {
		return nil, fmt.Errorf("invalid yaml: %s", err)
	}
	err = validateConfigs(&configs)
	if err != nil {
		return nil, fmt.Errorf("validation error: %s", err)
	}
	return &configs, nil
}

func validateConfigs(configs *ExtensionConfigs) error {
	nameSafeRegex := regexp.MustCompile(`^[A-Za-z0-9-_]+$`)
	for _, ext := range configs.Extensions {
		if ext.Name == "" {
			return fmt.Errorf("extensions.name must be configured")
		}
		if !nameSafeRegex.MatchString(ext.Name) {
			return fmt.Errorf("invalid extensions.name: only alphanumeric characters, hyphens, and underscores are allowed")
		}
		svcTotal := len(ext.Backend.Services)
		for _, svc := range ext.Backend.Services {
			if svc.URL == "" {
				return fmt.Errorf("extensions.backend.services.url must be configured")
			}
			if svcTotal > 1 && svc.Cluster == "" {
				return fmt.Errorf("extensions.backend.services.cluster must be configured when defining more than one service per extension")
			}
		}
	}
	return nil
}

// NewProxy will instantiate a new reverse proxy based on the provided
// targetURL and config.
func NewProxy(targetURL string, config ProxyConfig) (*httputil.ReverseProxy, error) {
	url, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse proxy URL: %s", err)
	}
	proxy := &httputil.ReverseProxy{
		Transport: newTransport(config),
		Director: func(req *http.Request) {
			req.Host = url.Host
			req.URL.Scheme = url.Scheme
			req.URL.Host = url.Host
			req.Header.Set("Host", url.Host)
		},
	}
	return proxy, nil
}

// newTransport will build a new transport to be used in the proxy
// applying default values if not defined in the given config.
func newTransport(config ProxyConfig) *http.Transport {
	applyProxyConfigDefaults(&config)
	return &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   config.ConnectionTimeout,
			KeepAlive: config.KeepAlive,
		}).DialContext,
		MaxIdleConns:          config.MaxIdleConnections,
		IdleConnTimeout:       config.IdleConnectionTimeout,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

func applyProxyConfigDefaults(c *ProxyConfig) {
	if c.ConnectionTimeout == 0 {
		c.ConnectionTimeout = DefaultConnectionTimeout
	}
	if c.KeepAlive == 0 {
		c.KeepAlive = DefaultKeepAlive
	}
	if c.IdleConnectionTimeout == 0 {
		c.IdleConnectionTimeout = DefaultIdleConnectionTimeout
	}
	if c.MaxIdleConnections == 0 {
		c.MaxIdleConnections = DefaultMaxIdleConnections
	}
}

// RegisterHandlers will retrieve all configured extensions
// and register the respective http handlers in the given
// router.
func (m *Manager) RegisterHandlers(r *mux.Router) error {
	m.log.Info("Registering extension handlers...")
	config, err := m.settings.Get()
	if err != nil {
		return fmt.Errorf("error getting settings: %s", err)
	}

	if config.ExtensionConfig == "" {
		return fmt.Errorf("No extensions configurations found")
	}

	extConfigs, err := parseAndValidateConfig(config.ExtensionConfig)
	if err != nil {
		return fmt.Errorf("error parsing extension config: %s", err)
	}
	return m.registerExtensions(r, extConfigs)
}

// registerExtensions will iterate over the given extConfigs and register
// http handlers for every extension. It also registers a list extensions
// handler under the "/extensions/" endpoint.
func (m *Manager) registerExtensions(r *mux.Router, extConfigs *ExtensionConfigs) error {
	extRouter := r.PathPrefix(fmt.Sprintf("%s/", URLPrefix)).Subrouter()
	for _, ext := range extConfigs.Extensions {
		proxyByCluster := make(map[string]*httputil.ReverseProxy)
		for _, service := range ext.Backend.Services {
			proxy, err := NewProxy(service.URL, ext.Backend.ProxyConfig)
			if err != nil {
				return fmt.Errorf("error creating proxy: %s", err)
			}
			proxyByCluster[service.Cluster] = proxy
		}
		m.log.Infof("Registering handler for %s/%s...", URLPrefix, ext.Name)
		extRouter.PathPrefix(fmt.Sprintf("/%s/", ext.Name)).
			HandlerFunc(m.CallExtension(ext.Name, proxyByCluster))
	}
	return nil
}

// authorize will enforce rbac rules are satified for the given RequestResources.
// The following validations are executed:
//   - enforce the subject has permission to read application/project provided
//     in HeaderArgoCDApplicationName and HeaderArgoCDProjectName.
//   - enforce the subject has permission to invoke the extension identified by
//     extName.
//   - enforce that all resources provided in the HeaderArgoCDResourceGVKName belong
//     to the application identified by HeaderArgoCDApplicationName.
//
// If all validations are satified it will return the Application resource
func (m *Manager) authorize(ctx context.Context, rr *RequestResources, extName string) (*v1alpha1.Application, error) {
	if m.rbac == nil {
		return nil, fmt.Errorf("rbac enforcer not set in extension manager")
	}
	appRBACName := security.AppRBACName(rr.ApplicationNamespace, rr.ProjectName, rr.ApplicationNamespace, rr.ApplicationName)
	if err := m.rbac.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, appRBACName); err != nil {
		return nil, fmt.Errorf("application authorization error: %s", err)
	}

	if err := m.rbac.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceExtensions, rbacpolicy.ActionInvoke, extName); err != nil {
		return nil, fmt.Errorf("unauthorized to invoke extension %q: %s", extName, err)
	}

	// just retrieve the app after checking if subject has access to it
	app, err := m.application.Get(rr.ApplicationNamespace, rr.ApplicationName)
	if err != nil {
		return nil, fmt.Errorf("error getting application: %s", err)
	}
	if app == nil {
		return nil, fmt.Errorf("invalid Application provided in the %q header", HeaderArgoCDApplicationName)
	}

	if app.Spec.GetProject() != rr.ProjectName {
		return nil, fmt.Errorf("project mismatch provided in the %q header", HeaderArgoCDProjectName)
	}

	proj, err := m.project.Get(app.Spec.GetProject())
	if err != nil {
		return nil, fmt.Errorf("error getting project: %s", err)
	}
	if proj == nil {
		return nil, fmt.Errorf("invalid project provided in the %q header", HeaderArgoCDProjectName)
	}
	permitted, err := proj.IsDestinationPermitted(app.Spec.Destination, m.project.GetClusters)
	if err != nil {
		return nil, fmt.Errorf("error validating project destinations: %s", err)
	}
	if !permitted {
		return nil, fmt.Errorf("the provided project is not allowed to access the cluster configured in the Application destination")
	}

	return app, nil
}

// CallExtension returns a handler func responsible for forwarding requests to the
// extension service. The request will be sanitized by removing sensitive headers.
func (m *Manager) CallExtension(extName string, proxyByCluster map[string]*httputil.ReverseProxy) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		reqResources, err := ValidateHeaders(r)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid headers: %s", err), http.StatusBadRequest)
			return
		}
		app, err := m.authorize(r.Context(), reqResources, extName)
		if err != nil {
			m.log.Infof("unauthorized extension request: %s", err)
			http.Error(w, "Unauthorized extension request", http.StatusUnauthorized)
			return
		}

		sanitizeRequest(r, extName)

		// This is the case where there is only one proxy configured
		// for this extension.
		if len(proxyByCluster) == 1 {
			for cName, proxy := range proxyByCluster {
				m.log.Debugf("proxing request to cluster %q", cName)
				// in this case we just forward the request to the single
				// proxy and return
				proxy.ServeHTTP(w, r)
				return
			}
		}
		clusterName := app.Spec.Destination.Name
		if clusterName == "" {
			clusterName = app.Spec.Destination.Server
		}

		// This is the case where there are more than one proxy configured
		// for this extension. In this case we need to get the proper proxy
		// instance configured for the target cluster.
		proxy, ok := proxyByCluster[clusterName]
		if !ok {
			msg := fmt.Sprintf("No extension configured for cluster %q", clusterName)
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		m.log.Debugf("proxing request to cluster %q", clusterName)
		proxy.ServeHTTP(w, r)
	}
}

// sanitizeRequest is reponsible for preparing and cleaning the given
// request, removing sensitive information before forwarding it to the
// proxy extension.
func sanitizeRequest(r *http.Request, extName string) {
	r.URL.Path = strings.TrimPrefix(r.URL.Path, fmt.Sprintf("%s/%s", URLPrefix, extName))
	r.Header.Del("Cookie")
	r.Header.Del("Authorization")
}
