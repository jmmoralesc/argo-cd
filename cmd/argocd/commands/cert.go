package commands

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	//log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	certificatepkg "github.com/argoproj/argo-cd/pkg/apiclient/certificate"
	appsv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util"
	certutil "github.com/argoproj/argo-cd/util/cert"

	//"github.com/argoproj/argo-cd/util/cli"

	"crypto/x509"
)

// NewCertCommand returns a new instance of an `argocd repo` command
func NewCertCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "cert",
		Short: "Manage repository certificates and SSH known hosts entries",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}

	command.AddCommand(NewCertAddSSHCommand(clientOpts))
	command.AddCommand(NewCertAddTLSCommand(clientOpts))
	command.AddCommand(NewCertListCommand(clientOpts))
	command.AddCommand(NewCertRemoveCommand(clientOpts))
	return command
}

func NewCertAddTLSCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		sourceFile string
	)
	var command = &cobra.Command{
		Use:   "add-tls SERVERNAME",
		Short: "Add TLS certificate data for connecting to repository server SERVERNAME",
		Run: func(c *cobra.Command, args []string) {
			conn, certIf := argocdclient.NewClientOrDie(clientOpts).NewCertClientOrDie()
			defer util.Close(conn)

			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			var certificateArray []string
			var err error

			if sourceFile != "" {
				certificateArray, err = certutil.ParseTLSCertificatesFromPath(sourceFile)
			} else {
				certificateArray, err = certutil.ParseTLSCertificatesFromStream(os.Stdin)
			}

			errors.CheckError(err)

			fmt.Printf("Parsed %d possible PEM certificates from input stream.\n", len(certificateArray))

			certificateList := make([]appsv1.RepositoryCertificate, 0)
			subjectMap := make(map[string]*x509.Certificate)
			for _, entry := range certificateArray {
				// We want to make sure to only send valid certificate data to the
				// server, so we decode the certificate into X509 structure before
				// further processing it.
				x509cert, err := certutil.DecodePEMCertificateToX509(entry)
				errors.CheckError(err)

				// TODO: We need a better way to detect duplicates sent in the stream,
				// maybe by using fingerprints? For now, no two certs with the same
				// subject may be sent.
				if subjectMap[x509cert.Subject.String()] != nil {
					fmt.Printf("ERROR: Cert with subject '%s' already seen.\n", x509cert.Subject.String())
					continue
				} else {
					fmt.Printf("Found certificate with subject '%s'\n", x509cert.Subject.String())
					subjectMap[x509cert.Subject.String()] = x509cert
				}

				certificateList = append(certificateList, appsv1.RepositoryCertificate{
					ServerName: args[0],
					CertType:   "https",
					CertData:   []byte(entry),
				})
			}

			serverName := args[0]

			if len(certificateList) > 0 {
				certificates, err := certIf.Create(context.Background(), &certificatepkg.RepositoryCertificateCreateRequest{
					Certificates: &appsv1.RepositoryCertificateList{
						Items: certificateList,
					},
				})
				errors.CheckError(err)
				fmt.Printf("Created %d certificates for server %s\n", len(certificates.Items), serverName)
			} else {
				fmt.Printf("No valid certificate has been detected in the stream.\n")
			}
		},
	}
	command.Flags().StringVar(&sourceFile, "source-file", "", "load certificate from file instead of stdin")
	return command
}

// NewCertAddCommand returns a new instance of an `argocd cert add` command
func NewCertAddSSHCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		sourceFile    string
		batchProcess  bool
		sshServerName string
		sshKeyType    string
		sshKeyData    string
		certificates  []appsv1.RepositoryCertificate
	)

	var command = &cobra.Command{
		Use:   "add-ssh",
		Short: "Add SSH known host entries for repository servers",
		Run: func(c *cobra.Command, args []string) {

			conn, certIf := argocdclient.NewClientOrDie(clientOpts).NewCertClientOrDie()
			defer util.Close(conn)

			var sshKnownHostsLists []string
			var err error

			if batchProcess {
				if sourceFile != "" {
					sshKnownHostsLists, err = certutil.ParseSSHKnownHostsFromPath(sourceFile)
				} else {
					sshKnownHostsLists, err = certutil.ParseSSHKnownHostsFromStream(os.Stdin)
				}

				errors.CheckError(err)
				fmt.Printf("Parsed %d known host entries from input stream.\n", len(sshKnownHostsLists))
			} else {
				if sshServerName == "" || sshKeyData == "" {
					err := fmt.Errorf("You need to specify all of --ssh-server-name, --ssh-key-type and --ssh-key-data\n")
					errors.CheckError(err)
				}

				sshKnownHostsLists, err = certutil.ParseSSHKnownHostsFromData(fmt.Sprintf("%s %s", sshServerName, sshKeyData))
				errors.CheckError(err)
				fmt.Printf("Successfully parsed SSH key data.\n")
			}

			if len(sshKnownHostsLists) == 0 {
				errors.CheckError(fmt.Errorf("No valid SSH known hosts data found."))
			}

			for _, knownHostsEntry := range sshKnownHostsLists {
				hostname, certSubType, certData, err := certutil.TokenizeSSHKnownHostsEntry(knownHostsEntry)
				errors.CheckError(err)
				_, _, err = certutil.KnownHostsLineToPublicKey(knownHostsEntry)
				errors.CheckError(err)
				certificate := appsv1.RepositoryCertificate{
					ServerName: hostname,
					CertType:   "ssh",
					CertCipher: certSubType,
					CertData:   certData,
				}

				certificates = append(certificates, certificate)
			}

			certList := &appsv1.RepositoryCertificateList{Items: certificates}
			response, err := certIf.Create(context.Background(), &certificatepkg.RepositoryCertificateCreateRequest{Certificates: certList})
			errors.CheckError(err)
			fmt.Printf("Successfully created %d SSH known host entries\n", len(response.Items))
		},
	}
	command.Flags().StringVar(&sourceFile, "source-file", "", "Specify file to read SSH known hosts from (default is to read from stdin)")
	command.Flags().BoolVar(&batchProcess, "batch", false, "Perform batch processing by reading in SSH known hosts data")
	command.Flags().StringVar(&sshServerName, "ssh-server-name", "", "The name of the SSH server to store the key for")
	command.Flags().StringVar(&sshKeyType, "ssh-key-type", "", "The type of the key to add")
	command.Flags().StringVar(&sshKeyData, "ssh-key-data", "", "Actual key data, base64 encoded")
	return command
}

// NewCertRemoveCommand returns a new instance of an `argocd cert rm` command
func NewCertRemoveCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		removeAllCerts bool
		certType       string
		certSubType    string
		certQuery      certificatepkg.RepositoryCertificateQuery
	)
	var command = &cobra.Command{
		Use:   "rm REPOSERVER",
		Short: "Remove certificate of TYPE for REPOSERVER",
		Run: func(c *cobra.Command, args []string) {
			if len(args) < 1 && !removeAllCerts {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, certIf := argocdclient.NewClientOrDie(clientOpts).NewCertClientOrDie()
			defer util.Close(conn)
			if removeAllCerts {
				certQuery = certificatepkg.RepositoryCertificateQuery{
					HostNamePattern: "*",
					CertType:        "*",
					CertSubType:     "*",
				}
			} else {
				certQuery = certificatepkg.RepositoryCertificateQuery{
					HostNamePattern: args[0],
					CertType:        certType,
					CertSubType:     certSubType,
				}
			}
			removed, err := certIf.Delete(context.Background(), &certQuery)
			errors.CheckError(err)
			if len(removed.Items) > 0 {
				for _, cert := range removed.Items {
					fmt.Printf("Removed cert for '%s' of type '%s' (subtype '%s')\n", cert.ServerName, cert.CertType, cert.CertCipher)
				}
			} else {
				fmt.Println("No certificates were removed (none matched the given patterns)")
			}
		},
	}
	command.Flags().BoolVar(&removeAllCerts, "remove-all", false, "Remove all configured certificates of all types from server (DANGER: use with care!)")
	command.Flags().StringVar(&certType, "cert-type", "", "Only remove certs of given type (ssh, https)")
	command.Flags().StringVar(&certSubType, "cert-sub-type", "", "Only remove certs of given sub-type (only for ssh)")
	return command
}

// NewCertListCommand returns a new instance of an `argocd cert rm` command
func NewCertListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		certType        string
		hostNamePattern string
		sortOrder       string
	)
	var command = &cobra.Command{
		Use:   "list",
		Short: "List configured certificates",
		Run: func(c *cobra.Command, args []string) {
			if certType != "" {
				switch certType {
				case "ssh":
				case "https":
				default:
					fmt.Println("cert-type must be either ssh or https")
					os.Exit(1)
				}
			}

			conn, certIf := argocdclient.NewClientOrDie(clientOpts).NewCertClientOrDie()
			defer util.Close(conn)
			certificates, err := certIf.List(context.Background(), &certificatepkg.RepositoryCertificateQuery{HostNamePattern: hostNamePattern, CertType: certType})
			errors.CheckError(err)
			printCertTable(certificates.Items, sortOrder)
		},
	}

	command.Flags().StringVar(&sortOrder, "sort", "", "set display sort order, valid: 'hostname', 'type'")
	command.Flags().StringVar(&certType, "cert-type", "", "only list certificates of given type, valid: 'ssh','https'")
	command.Flags().StringVar(&hostNamePattern, "hostname-pattern", "", "only list certificates for hosts matching given glob-pattern")
	return command
}

// Print table of certificate info
func printCertTable(certs []appsv1.RepositoryCertificate, sortOrder string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "HOSTNAME\tTYPE\tSUBTYPE\tFINGERPRINT/SUBJECT\n")

	if sortOrder == "hostname" || sortOrder == "" {
		sort.Slice(certs, func(i, j int) bool {
			return certs[i].ServerName < certs[j].ServerName
		})
	} else if sortOrder == "type" {
		sort.Slice(certs, func(i, j int) bool {
			return certs[i].CertType < certs[j].CertType
		})
	}

	for _, c := range certs {
		if c.CertType == "ssh" {
			_, pubKey, err := certutil.TokenizedDataToPublicKey(c.ServerName, string(c.CertData))
			errors.CheckError(err)
			fmt.Fprintf(w, "%s\t%s\t%s\tSHA256:%s\n", c.ServerName, c.CertType, c.CertCipher, certutil.SSHFingerprintSHA256(pubKey))
		} else if c.CertType == "https" {
			x509Data, err := certutil.DecodePEMCertificateToX509(string(c.CertData))
			var subject string
			keyType := "-?-"
			if err != nil {
				subject = err.Error()
			} else {
				subject = x509Data.Subject.String()
				keyType = x509Data.PublicKeyAlgorithm.String()
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", c.ServerName, c.CertType, strings.ToLower(keyType), subject)
		}
	}
	_ = w.Flush()
}
