package commands

import (
	"fmt"
	"os"

	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	"github.com/argoproj/argo-cd/util/localconfig"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// NewLogoutCommand returns a new instance of `argocd logout` command
func NewLogoutCommand(globalClientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "logout CONTEXT",
		Short: "Log out from Argo CD",
		Long:  "Log out from Argo CD",
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			context := args[0]

			localCfg, err := localconfig.ReadLocalConfig(globalClientOpts.ConfigPath)
			errors.CheckError(err)
			if localCfg == nil {
				log.Fatalf("Nothing to logout from")
			}

			serverName, ok := localCfg.RemoveContext(context)
			if !ok {
				log.Fatalf("Context %s does not exist", context)
			}
			_ = localCfg.RemoveUser(context)
			_ = localCfg.RemoveServer(serverName)

			if localCfg.IsEmpty() {
				err = localconfig.DeleteLocalConfig(globalClientOpts.ConfigPath)
				errors.CheckError(err)
			} else {
				if localCfg.CurrentContext == context {
					localCfg.CurrentContext = localCfg.Contexts[0].Name
				}
				err = localconfig.ValidateLocalConfig(*localCfg)
				if err != nil {
					log.Fatalf("Error in logging out")
				}
				err = localconfig.WriteLocalConfig(*localCfg, globalClientOpts.ConfigPath)
				errors.CheckError(err)
			}
			fmt.Printf("Context '%s' deleted\n", context)
		},
	}
	return command
}