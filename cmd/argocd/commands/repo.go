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

	"github.com/argoproj/argo-cd/common"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	repositorypkg "github.com/argoproj/argo-cd/pkg/apiclient/repository"
	appsv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/git"
)

// NewRepoCommand returns a new instance of an `argocd repo` command
func NewRepoCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "repo",
		Short: "Manage repository connection parameters",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(errors.ErrorCommandSpecific)
		},
	}

	command.AddCommand(NewRepoAddCommand(clientOpts))
	command.AddCommand(NewRepoGetCommand(clientOpts))
	command.AddCommand(NewRepoListCommand(clientOpts))
	command.AddCommand(NewRepoRemoveCommand(clientOpts))
	return command
}

// NewRepoAddCommand returns a new instance of an `argocd repo add` command
func NewRepoAddCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		repo                           appsv1.Repository
		upsert                         bool
		sshPrivateKeyPath              string
		insecureIgnoreHostKey          bool
		insecureSkipServerVerification bool
		tlsClientCertPath              string
		tlsClientCertKeyPath           string
		enableLfs                      bool
	)

	// For better readability and easier formatting
	var repoAddExamples = `  # Add a Git repository via SSH using a private key for authentication, ignoring the server's host key:
	argocd repo add git@git.example.com:repos/repo --insecure-ignore-host-key --ssh-private-key-path ~/id_rsa

	# Add a Git repository via SSH on a non-default port - need to use ssh:// style URLs here
	argocd repo add ssh://git@git.example.com:2222/repos/repo --ssh-private-key-path ~/id_rsa

  # Add a private Git repository via HTTPS using username/password and TLS client certificates:
  argocd repo add https://git.example.com/repos/repo --username git --password secret --tls-client-cert-path ~/mycert.crt --tls-client-cert-key-path ~/mycert.key

  # Add a private Git repository via HTTPS using username/password without verifying the server's TLS certificate
  argocd repo add https://git.example.com/repos/repo --username git --password secret --insecure-skip-server-verification

  # Add a public Helm repository named 'stable' via HTTPS
  argocd repo add https://kubernetes-charts.storage.googleapis.com --type helm --name stable  

  # Add a private Helm repository named 'stable' via HTTPS
  argocd repo add https://kubernetes-charts.storage.googleapis.com --type helm --name stable --username test --password test
`

	var command = &cobra.Command{
		Use:     "add REPOURL",
		Short:   "Add git repository connection parameters",
		Example: repoAddExamples,
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(errors.ErrorCommandSpecific)
			}

			// Repository URL
			repo.Repo = args[0]

			// Specifying ssh-private-key-path is only valid for SSH repositories
			if sshPrivateKeyPath != "" {
				if ok, _ := git.IsSSHURL(repo.Repo); ok {
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
				if git.IsHTTPSURL(repo.Repo) {
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

			// Set repository connection properties only when creating repository, not
			// when creating repository credentials.
			// InsecureIgnoreHostKey is deprecated and only here for backwards compat
			repo.InsecureIgnoreHostKey = insecureIgnoreHostKey
			repo.Insecure = insecureSkipServerVerification
			repo.EnableLFS = enableLfs

			if repo.Type == "helm" && repo.Name == "" {
				errors.CheckErrorWithCode(fmt.Errorf("Must specify --name for repos of type 'helm'"), errors.ErrorCommandSpecific)
			}

			conn, repoIf := argocdclient.NewClientOrDie(clientOpts).NewRepoClientOrDie()
			defer io.Close(conn)

			// If the user set a username, but didn't supply password via --password,
			// then we prompt for it
			if repo.Username != "" && repo.Password == "" {
				repo.Password = cli.PromptPassword(repo.Password)
			}

			// We let the server check access to the repository before adding it. If
			// it is a private repo, but we cannot access with with the credentials
			// that were supplied, we bail out.
			//
			// Skip validation if we are just adding credentials template, chances
			// are high that we do not have the given URL pointing to a valid Git
			// repo anyway.
			repoAccessReq := repositorypkg.RepoAccessQuery{
				Repo:              repo.Repo,
				Type:              repo.Type,
				Name:              repo.Name,
				Username:          repo.Username,
				Password:          repo.Password,
				SshPrivateKey:     repo.SSHPrivateKey,
				TlsClientCertData: repo.TLSClientCertData,
				TlsClientCertKey:  repo.TLSClientCertKey,
				Insecure:          repo.IsInsecure(),
			}
			_, err := repoIf.ValidateAccess(context.Background(), &repoAccessReq)
			errors.CheckErrorWithCode(err, errors.ErrorAPIResponse)

			repoCreateReq := repositorypkg.RepoCreateRequest{
				Repo:   &repo,
				Upsert: upsert,
			}

			createdRepo, err := repoIf.Create(context.Background(), &repoCreateReq)
			errors.CheckErrorWithCode(err, errors.ErrorAPIResponse)
			fmt.Printf("repository '%s' added\n", createdRepo.Repo)
		},
	}
	command.Flags().StringVar(&repo.Type, "type", common.DefaultRepoType, "type of the repository, \"git\" or \"helm\"")
	command.Flags().StringVar(&repo.Name, "name", "", "name of the repository, mandatory for repositories of type helm")
	command.Flags().StringVar(&repo.Username, "username", "", "username to the repository")
	command.Flags().StringVar(&repo.Password, "password", "", "password to the repository")
	command.Flags().StringVar(&sshPrivateKeyPath, "ssh-private-key-path", "", "path to the private ssh key (e.g. ~/.ssh/id_rsa)")
	command.Flags().StringVar(&tlsClientCertPath, "tls-client-cert-path", "", "path to the TLS client cert (must be PEM format)")
	command.Flags().StringVar(&tlsClientCertKeyPath, "tls-client-cert-key-path", "", "path to the TLS client cert's key path (must be PEM format)")
	command.Flags().BoolVar(&insecureIgnoreHostKey, "insecure-ignore-host-key", false, "disables SSH strict host key checking (deprecated, use --insecure-skip-server-verification instead)")
	command.Flags().BoolVar(&insecureSkipServerVerification, "insecure-skip-server-verification", false, "disables server certificate and host key checks")
	command.Flags().BoolVar(&enableLfs, "enable-lfs", false, "enable git-lfs (Large File Support) on this repository")
	command.Flags().BoolVar(&upsert, "upsert", false, "Override an existing repository with the same name even if the spec differs")
	return command
}

// NewRepoRemoveCommand returns a new instance of an `argocd repo list` command
func NewRepoRemoveCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "rm REPO",
		Short: "Remove repository credentials",
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(errors.ErrorCommandSpecific)
			}
			conn, repoIf := argocdclient.NewClientOrDie(clientOpts).NewRepoClientOrDie()
			defer io.Close(conn)
			for _, repoURL := range args {
				_, err := repoIf.Delete(context.Background(), &repositorypkg.RepoQuery{Repo: repoURL})
				errors.CheckErrorWithCode(err, errors.ErrorAPIResponse)
			}
		},
	}
	return command
}

// Print table of repo info
func printRepoTable(repos appsv1.Repositories) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, "TYPE\tNAME\tREPO\tINSECURE\tLFS\tCREDS\tSTATUS\tMESSAGE\n")
	for _, r := range repos {
		var hasCreds string
		if !r.HasCredentials() {
			hasCreds = "false"
		} else {
			if r.InheritedCreds {
				hasCreds = "inherited"
			} else {
				hasCreds = "true"
			}
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%v\t%v\t%s\t%s\t%s\n", r.Type, r.Name, r.Repo, r.IsInsecure(), r.EnableLFS, hasCreds, r.ConnectionState.Status, r.ConnectionState.Message)
	}
	_ = w.Flush()
}

// Print list of repo urls or url patterns for repository credentials
func printRepoUrls(repos appsv1.Repositories) {
	for _, r := range repos {
		fmt.Println(r.Repo)
	}
}

// NewRepoListCommand returns a new instance of an `argocd repo rm` command
func NewRepoListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		output  string
		refresh string
	)
	var command = &cobra.Command{
		Use:   "list",
		Short: "List configured repositories",
		Run: func(c *cobra.Command, args []string) {
			conn, repoIf := argocdclient.NewClientOrDie(clientOpts).NewRepoClientOrDie()
			defer io.Close(conn)
			forceRefresh := false
			switch refresh {
			case "":
			case "hard":
				forceRefresh = true
			default:
				err := fmt.Errorf("--refresh must be one of: 'hard'")
				errors.CheckErrorWithCode(err, errors.ErrorAPIResponse)
			}
			repos, err := repoIf.List(context.Background(), &repositorypkg.RepoQuery{ForceRefresh: forceRefresh})
			errors.CheckErrorWithCode(err, errors.ErrorAPIResponse)
			switch output {
			case "yaml", "json":
				err := PrintResourceList(repos.Items, output, false)
				errors.CheckErrorWithCode(err, errors.ErrorAPIResponse)
			case "url":
				printRepoUrls(repos.Items)
				// wide is the default
			case "wide", "":
				printRepoTable(repos.Items)
			default:
				errors.CheckErrorWithCode(fmt.Errorf("unknown output format: %s", output), errors.ErrorCommandSpecific)
			}
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: json|yaml|wide|url")
	command.Flags().StringVar(&refresh, "refresh", "", "Force a cache refresh on connection status")
	return command
}

// NewRepoGetCommand returns a new instance of an `argocd repo rm` command
func NewRepoGetCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		output  string
		refresh string
	)
	var command = &cobra.Command{
		Use:   "get",
		Short: "Get a configured repository by URL",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(errors.ErrorCommandSpecific)
			}

			// Repository URL
			repoURL := args[0]
			conn, repoIf := argocdclient.NewClientOrDie(clientOpts).NewRepoClientOrDie()
			defer io.Close(conn)
			forceRefresh := false
			switch refresh {
			case "":
			case "hard":
				forceRefresh = true
			default:
				err := fmt.Errorf("--refresh must be one of: 'hard'")
				errors.CheckErrorWithCode(err, errors.ErrorAPIResponse)
			}
			repo, err := repoIf.Get(context.Background(), &repositorypkg.RepoQuery{Repo: repoURL, ForceRefresh: forceRefresh})
			errors.CheckErrorWithCode(err, errors.ErrorAPIResponse)
			switch output {
			case "yaml", "json":
				err := PrintResource(repo, output)
				errors.CheckErrorWithCode(err, errors.ErrorCommandSpecific)
			case "url":
				fmt.Println(repo.Repo)
				// wide is the default
			case "wide", "":
				printRepoTable(appsv1.Repositories{repo})
			default:
				errors.CheckErrorWithCode(fmt.Errorf("unknown output format: %s", output), errors.ErrorCommandSpecific)
			}
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: json|yaml|wide|url")
	command.Flags().StringVar(&refresh, "refresh", "", "Force a cache refresh on connection status")
	return command
}
