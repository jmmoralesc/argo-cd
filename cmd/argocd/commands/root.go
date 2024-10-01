package commands

import (
	"fmt"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"strings"

	"github.com/argoproj/argo-cd/v2/cmd/argocd/commands/admin"
	"github.com/argoproj/argo-cd/v2/cmd/argocd/commands/initialize"
	cmdutil "github.com/argoproj/argo-cd/v2/cmd/util"
	"github.com/argoproj/argo-cd/v2/common"
	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/config"
	"github.com/argoproj/argo-cd/v2/util/env"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/localconfig"
)

func init() {
	cobra.OnInitialize(initConfig)
}

func initConfig() {
	cli.SetLogFormat(cmdutil.LogFormat)
	cli.SetLogLevel(cmdutil.LogLevel)
}

func NewDefaultArgoCDCommand() *cobra.Command {
	return NewDefaultArgoCDCommandWithArgs(cmdutil.ArgoCDCLIOptions{
		PluginHandler: cmdutil.NewDefaultPluginHandler([]string{"argocd"}),
		Arguments:     os.Args,
	})
}

func NewDefaultArgoCDCommandWithArgs(o cmdutil.ArgoCDCLIOptions) *cobra.Command {
	cmd := NewCommand()

	if o.PluginHandler == nil {
		return cmd
	}

	// the first argument will be the binary, followed by other arguments
	if len(o.Arguments) > 1 {
		cmdPathPieces := o.Arguments[1:]

		// Try to find a valid Argo CD command that matches the arguments provided (e.g., foo in the case of argocd foo)
		// If it finds a command, it continues without invoking the plugin.
		// If it doesn't find the command (err is non-nil), it means foo isn't a built-in Argo CD command,
		// so it might be a plugin like argocd-foo.
		if _, _, err := cmd.Find(cmdPathPieces); err != nil {
			var cmdName string
			for _, arg := range cmdPathPieces {
				if !strings.HasPrefix(arg, "-") {
					cmdName = arg
					break
				}
			}

			switch cmdName {
			case "help", cobra.ShellCompRequestCmd, cobra.ShellCompNoDescRequestCmd:
				// Don't search for a plugin
			default:
				if err := HandlePluginCommand(o.PluginHandler, cmdPathPieces, 1); err != nil {
					fmt.Errorf("Error: %v\n", err)
					os.Exit(1)
				}
			}
		}
	}

	return cmd
}

// HandlePluginCommand is  responsible for finding and executing a plugin when a command isn't recognized as a built-in command
func HandlePluginCommand(pluginHandler cmdutil.PluginHandler, cmdArgs []string, minArgs int) error {
	var remainingArgs []string // this will contain all "non-flag" arguments
	for _, arg := range cmdArgs {
		// if you encounter a flag, break the loop
		// For eg. If cmdArgs is ["argocd", "foo", "-v"],
		// it will store ["argocd", "foo"] in remainingArgs
		// and stop when it hits the flag -v
		if strings.HasPrefix(arg, "-") {
			break
		}
		remainingArgs = append(remainingArgs, strings.Replace(arg, "-", "_", -1))
	}

	if len(remainingArgs) == 0 {
		// the length of cmdArgs is at least 1
		return fmt.Errorf("flags cannot be placed before plugin name: %s", cmdArgs[0])
	}

	foundPluginPath := ""

	for len(remainingArgs) > 0 {
		path, found := pluginHandler.LookForPlugin(strings.Join(remainingArgs, "-"))
		if !found {
			remainingArgs = remainingArgs[:len(remainingArgs)-1]
			if len(remainingArgs) < minArgs {
				break
			}

			continue
		}

		foundPluginPath = path
		break
	}

	if len(foundPluginPath) == 0 {
		return nil
	}

	// Execute the plugin that is found
	if err := pluginHandler.ExecutePlugin(foundPluginPath, cmdArgs[len(remainingArgs):], os.Environ()); err != nil {
		return err
	}

	return nil
}

// NewCommand returns a new instance of an argocd command
func NewCommand() *cobra.Command {
	var (
		clientOpts argocdclient.ClientOptions
		pathOpts   = clientcmd.NewDefaultPathOptions()
	)

	command := &cobra.Command{
		Use:   cliName,
		Short: "argocd controls a Argo CD server",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
		DisableAutoGenTag: true,
		SilenceUsage:      true,
	}

	command.AddCommand(NewCompletionCommand())
	command.AddCommand(initialize.InitCommand(NewVersionCmd(&clientOpts, nil)))
	command.AddCommand(initialize.InitCommand(NewClusterCommand(&clientOpts, pathOpts)))
	command.AddCommand(initialize.InitCommand(NewApplicationCommand(&clientOpts)))
	command.AddCommand(initialize.InitCommand(NewAppSetCommand(&clientOpts)))
	command.AddCommand(NewLoginCommand(&clientOpts))
	command.AddCommand(NewReloginCommand(&clientOpts))
	command.AddCommand(initialize.InitCommand(NewRepoCommand(&clientOpts)))
	command.AddCommand(initialize.InitCommand(NewRepoCredsCommand(&clientOpts)))
	command.AddCommand(NewContextCommand(&clientOpts))
	command.AddCommand(initialize.InitCommand(NewProjectCommand(&clientOpts)))
	command.AddCommand(initialize.InitCommand(NewAccountCommand(&clientOpts)))
	command.AddCommand(NewLogoutCommand(&clientOpts))
	command.AddCommand(initialize.InitCommand(NewCertCommand(&clientOpts)))
	command.AddCommand(initialize.InitCommand(NewGPGCommand(&clientOpts)))
	command.AddCommand(admin.NewAdminCommand(&clientOpts))

	defaultLocalConfigPath, err := localconfig.DefaultLocalConfigPath()
	errors.CheckError(err)
	command.PersistentFlags().StringVar(&clientOpts.ConfigPath, "config", config.GetFlag("config", defaultLocalConfigPath), "Path to Argo CD config")
	command.PersistentFlags().StringVar(&clientOpts.ServerAddr, "server", config.GetFlag("server", ""), "Argo CD server address")
	command.PersistentFlags().BoolVar(&clientOpts.PlainText, "plaintext", config.GetBoolFlag("plaintext"), "Disable TLS")
	command.PersistentFlags().BoolVar(&clientOpts.Insecure, "insecure", config.GetBoolFlag("insecure"), "Skip server certificate and domain verification")
	command.PersistentFlags().StringVar(&clientOpts.CertFile, "server-crt", config.GetFlag("server-crt", ""), "Server certificate file")
	command.PersistentFlags().StringVar(&clientOpts.ClientCertFile, "client-crt", config.GetFlag("client-crt", ""), "Client certificate file")
	command.PersistentFlags().StringVar(&clientOpts.ClientCertKeyFile, "client-crt-key", config.GetFlag("client-crt-key", ""), "Client certificate key file")
	command.PersistentFlags().StringVar(&clientOpts.AuthToken, "auth-token", config.GetFlag("auth-token", env.StringFromEnv(common.EnvAuthToken, "")), fmt.Sprintf("Authentication token; set this or the %s environment variable", common.EnvAuthToken))
	command.PersistentFlags().BoolVar(&clientOpts.GRPCWeb, "grpc-web", config.GetBoolFlag("grpc-web"), "Enables gRPC-web protocol. Useful if Argo CD server is behind proxy which does not support HTTP2.")
	command.PersistentFlags().StringVar(&clientOpts.GRPCWebRootPath, "grpc-web-root-path", config.GetFlag("grpc-web-root-path", ""), "Enables gRPC-web protocol. Useful if Argo CD server is behind proxy which does not support HTTP2. Set web root.")
	command.PersistentFlags().StringVar(&cmdutil.LogFormat, "logformat", config.GetFlag("logformat", "text"), "Set the logging format. One of: text|json")
	command.PersistentFlags().StringVar(&cmdutil.LogLevel, "loglevel", config.GetFlag("loglevel", "info"), "Set the logging level. One of: debug|info|warn|error")
	command.PersistentFlags().StringSliceVarP(&clientOpts.Headers, "header", "H", config.GetStringSliceFlag("header", []string{}), "Sets additional header to all requests made by Argo CD CLI. (Can be repeated multiple times to add multiple headers, also supports comma separated headers)")
	command.PersistentFlags().BoolVar(&clientOpts.PortForward, "port-forward", config.GetBoolFlag("port-forward"), "Connect to a random argocd-server port using port forwarding")
	command.PersistentFlags().StringVar(&clientOpts.PortForwardNamespace, "port-forward-namespace", config.GetFlag("port-forward-namespace", ""), "Namespace name which should be used for port forwarding")
	command.PersistentFlags().IntVar(&clientOpts.HttpRetryMax, "http-retry-max", config.GetIntFlag("http-retry-max", 0), "Maximum number of retries to establish http connection to Argo CD server")
	command.PersistentFlags().BoolVar(&clientOpts.Core, "core", config.GetBoolFlag("core"), "If set to true then CLI talks directly to Kubernetes instead of talking to Argo CD API server")
	command.PersistentFlags().StringVar(&clientOpts.Context, "argocd-context", "", "The name of the Argo-CD server context to use")
	command.PersistentFlags().StringVar(&clientOpts.ServerName, "server-name", env.StringFromEnv(common.EnvServerName, common.DefaultServerName), fmt.Sprintf("Name of the Argo CD API server; set this or the %s environment variable when the server's name label differs from the default, for example when installing via the Helm chart", common.EnvServerName))
	command.PersistentFlags().StringVar(&clientOpts.AppControllerName, "controller-name", env.StringFromEnv(common.EnvAppControllerName, common.DefaultApplicationControllerName), fmt.Sprintf("Name of the Argo CD Application controller; set this or the %s environment variable when the controller's name label differs from the default, for example when installing via the Helm chart", common.EnvAppControllerName))
	command.PersistentFlags().StringVar(&clientOpts.RedisHaProxyName, "redis-haproxy-name", env.StringFromEnv(common.EnvRedisHaProxyName, common.DefaultRedisHaProxyName), fmt.Sprintf("Name of the Redis HA Proxy; set this or the %s environment variable when the HA Proxy's name label differs from the default, for example when installing via the Helm chart", common.EnvRedisHaProxyName))
	command.PersistentFlags().StringVar(&clientOpts.RedisName, "redis-name", env.StringFromEnv(common.EnvRedisName, common.DefaultRedisName), fmt.Sprintf("Name of the Redis deployment; set this or the %s environment variable when the Redis's name label differs from the default, for example when installing via the Helm chart", common.EnvRedisName))
	command.PersistentFlags().StringVar(&clientOpts.RepoServerName, "repo-server-name", env.StringFromEnv(common.EnvRepoServerName, common.DefaultRepoServerName), fmt.Sprintf("Name of the Argo CD Repo server; set this or the %s environment variable when the server's name label differs from the default, for example when installing via the Helm chart", common.EnvRepoServerName))

	clientOpts.KubeOverrides = &clientcmd.ConfigOverrides{}
	command.PersistentFlags().StringVar(&clientOpts.KubeOverrides.CurrentContext, "kube-context", "", "Directs the command to the given kube-context")

	return command
}
