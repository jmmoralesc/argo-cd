package commands

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"text/tabwriter"

	"github.com/argoproj/gitops-engine/pkg/utils/errors"
	"github.com/argoproj/gitops-engine/pkg/utils/io"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	repocredspkg "github.com/argoproj/argo-cd/pkg/apiclient/repocreds"
	appsv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/git"
)

// NewRepoCredsCommand returns a new instance of an `argocd repocreds` command
func NewRepoCredsCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "repocreds",
		Short: "Manage repository connection parameters",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(errors.ErrorCommandSpecific)
		},
	}

	command.AddCommand(NewRepoCredsAddCommand(clientOpts))
	command.AddCommand(NewRepoCredsListCommand(clientOpts))
	command.AddCommand(NewRepoCredsRemoveCommand(clientOpts))
	return command
}

// NewRepoCredsAddCommand returns a new instance of an `argocd repocreds add` command
func NewRepoCredsAddCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		repo                 appsv1.RepoCreds
		upsert               bool
		sshPrivateKeyPath    string
		tlsClientCertPath    string
		tlsClientCertKeyPath string
	)

	// For better readability and easier formatting
	var repocredsAddExamples = `  # Add credentials with user/pass authentication to use for all repositories under https://git.example.com/repos
  argocd repocreds add https://git.example.com/repos/ --username git --password secret

  # Add credentials with SSH private key authentication to use for all repositories under ssh://git@git.example.com/repos
  argocd repocreds add ssh://git@git.example.com/repos/ --ssh-private-key-path ~/.ssh/id_rsa
`

	var command = &cobra.Command{
		Use:     "add REPOURL",
		Short:   "Add git repository connection parameters",
		Example: repocredsAddExamples,
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(errors.ErrorCommandSpecific)
			}

			// Repository URL
			repo.URL = args[0]

			// Specifying ssh-private-key-path is only valid for SSH repositories
			if sshPrivateKeyPath != "" {
				if ok, _ := git.IsSSHURL(repo.URL); ok {
					keyData, err := ioutil.ReadFile(sshPrivateKeyPath)
					if err != nil {
						errors.Fatal(err)
					}
					repo.SSHPrivateKey = string(keyData)
				} else {
					err := fmt.Errorf("--ssh-private-key-path is only supported for SSH repositories.")
					errors.CheckErrorWithCode(err, errors.ErrorCommandSpecific)
				}
			}

			// tls-client-cert-path and tls-client-cert-key-key-path must always be
			// specified together
			if (tlsClientCertPath != "" && tlsClientCertKeyPath == "") || (tlsClientCertPath == "" && tlsClientCertKeyPath != "") {
				err := fmt.Errorf("--tls-client-cert-path and --tls-client-cert-key-path must be specified together")
				errors.CheckErrorWithCode(err, errors.ErrorCommandSpecific)
			}

			// Specifying tls-client-cert-path is only valid for HTTPS repositories
			if tlsClientCertPath != "" {
				if git.IsHTTPSURL(repo.URL) {
					tlsCertData, err := ioutil.ReadFile(tlsClientCertPath)
					errors.CheckErrorWithCode(err, errors.ErrorCommandSpecific)
					tlsCertKey, err := ioutil.ReadFile(tlsClientCertKeyPath)
					errors.CheckErrorWithCode(err, errors.ErrorCommandSpecific)
					repo.TLSClientCertData = string(tlsCertData)
					repo.TLSClientCertKey = string(tlsCertKey)
				} else {
					err := fmt.Errorf("--tls-client-cert-path is only supported for HTTPS repositories")
					errors.CheckErrorWithCode(err, errors.ErrorCommandSpecific)
				}
			}

			conn, repoIf := argocdclient.NewClientOrDie(clientOpts).NewRepoCredsClientOrDie()
			defer io.Close(conn)

			// If the user set a username, but didn't supply password via --password,
			// then we prompt for it
			if repo.Username != "" && repo.Password == "" {
				repo.Password = cli.PromptPassword(repo.Password)
			}

			repoCreateReq := repocredspkg.RepoCredsCreateRequest{
				Creds:  &repo,
				Upsert: upsert,
			}

			createdRepo, err := repoIf.CreateRepositoryCredentials(context.Background(), &repoCreateReq)
			errors.CheckErrorWithCode(err, errors.ErrorAPIResponse)
			fmt.Printf("repository credentials for '%s' added\n", createdRepo.URL)
		},
	}
	command.Flags().StringVar(&repo.Username, "username", "", "username to the repository")
	command.Flags().StringVar(&repo.Password, "password", "", "password to the repository")
	command.Flags().StringVar(&sshPrivateKeyPath, "ssh-private-key-path", "", "path to the private ssh key (e.g. ~/.ssh/id_rsa)")
	command.Flags().StringVar(&tlsClientCertPath, "tls-client-cert-path", "", "path to the TLS client cert (must be PEM format)")
	command.Flags().StringVar(&tlsClientCertKeyPath, "tls-client-cert-key-path", "", "path to the TLS client cert's key path (must be PEM format)")
	command.Flags().BoolVar(&upsert, "upsert", false, "Override an existing repository with the same name even if the spec differs")
	return command
}

// NewRepoCredsRemoveCommand returns a new instance of an `argocd repocreds rm` command
func NewRepoCredsRemoveCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "rm CREDSURL",
		Short: "Remove repository credentials",
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(errors.ErrorCommandSpecific)
			}
			conn, repoIf := argocdclient.NewClientOrDie(clientOpts).NewRepoCredsClientOrDie()
			defer io.Close(conn)
			for _, repoURL := range args {
				_, err := repoIf.DeleteRepositoryCredentials(context.Background(), &repocredspkg.RepoCredsDeleteRequest{Url: repoURL})
				errors.CheckErrorWithCode(err, errors.ErrorAPIResponse)
			}
		},
	}
	return command
}

// Print the repository credentials as table
func printRepoCredsTable(repos []appsv1.RepoCreds) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "URL PATTERN\tUSERNAME\tSSH_CREDS\tTLS_CREDS\n")
	for _, r := range repos {
		if r.Username == "" {
			r.Username = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%v\t%v\n", r.URL, r.Username, r.SSHPrivateKey != "", r.TLSClientCertData != "")
	}
	_ = w.Flush()
}

// Print list of repo urls or url patterns for repository credentials
func printRepoCredsUrls(repos []appsv1.RepoCreds) {
	for _, r := range repos {
		fmt.Println(r.URL)
	}
}

// NewRepoCredsListCommand returns a new instance of an `argocd repo list` command
func NewRepoCredsListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		output string
	)
	var command = &cobra.Command{
		Use:   "list",
		Short: "List configured repository credentials",
		Run: func(c *cobra.Command, args []string) {
			conn, repoIf := argocdclient.NewClientOrDie(clientOpts).NewRepoCredsClientOrDie()
			defer io.Close(conn)
			repos, err := repoIf.ListRepositoryCredentials(context.Background(), &repocredspkg.RepoCredsQuery{})
			errors.CheckErrorWithCode(err, errors.ErrorAPIResponse)
			switch output {
			case "yaml", "json":
				err := PrintResourceList(repos.Items, output, false)
				errors.CheckErrorWithCode(err, errors.ErrorCommandSpecific)
			case "url":
				printRepoCredsUrls(repos.Items)
			case "wide", "":
				printRepoCredsTable(repos.Items)
			default:
				errors.CheckErrorWithCode(fmt.Errorf("unknown output format: %s", output), errors.ErrorCommandSpecific)
			}
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: json|yaml|wide|url")
	return command
}
