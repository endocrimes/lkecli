package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/alecthomas/kong"
	configparser "github.com/bigkevmcd/go-configparser"
	"github.com/linode/linodego"
	"golang.org/x/oauth2"
)

// A flag with a hook that, if triggered, will set the debug loggers output to stdout.
type debugFlag bool

func (d debugFlag) BeforeApply(logger *log.Logger) error {
	logger.SetOutput(os.Stdout)
	return nil
}

type cfg struct {
	LinodeCLIToken   string    `name:"linode-cli-token" env:"LINODE_CLI_TOKEN" help:"Override the Linode API Token"`
	LinodeConfigPath string    `name:"linode-config-path" env:"LINODE_CONFIG_PATH" type:"path" default:"~/.config/linode-cli" help:"Path to your Linode CLI configuration"`
	LinodeUser       string    `name:"linode-user" env:"LINODE_USER" help:"Override the user configuration when using a linode-cli config"`
	Debug            debugFlag `help:"Enable debug logging."`
}

func (c *cfg) resolveAPIToken() (string, error) {
	if c.LinodeCLIToken != "" {
		return c.LinodeCLIToken, nil
	}

	p, err := configparser.NewConfigParserFromFile(c.LinodeConfigPath)
	if err != nil {
		return "", err
	}

	user := c.LinodeUser
	if user == "" {
		user = p.Defaults()["default-user"]
	}

	hasUserSection := p.HasSection(user)
	if !hasUserSection {
		return "", fmt.Errorf("missing configuration for user %q", user)
	}

	token, err := p.Get(user, "token")
	if err != nil {
		return "", fmt.Errorf("failed to resolve token for user %q: %w", user, err)
	}

	return token, nil
}

func (c *cfg) linodeAPIClient() (linodego.Client, error) {
	token, err := c.resolveAPIToken()
	if err != nil {
		return linodego.Client{}, err
	}
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})

	oauth2Client := &http.Client{
		Transport: &oauth2.Transport{
			Source: tokenSource,
		},
	}

	linodeClient := linodego.NewClient(oauth2Client)
	linodeClient.SetDebug(bool(c.Debug))

	return linodeClient, nil
}

type cLI struct {
	cfg

	Kubeconfig kubeconfigCmd `cmd:"" help:"Fetch the Kubeconfig for a cluster"`
}

type kubeconfigCmd struct {
	Cluster string `arg:"cluster" help:"Name of the cluster to fetch the kubeconfig for"`
	Output  string `name:"output" type:"path" default:"~/.kube/config" help:"Path to save the Kubeconfig to"`
	Merge   bool   `name:"merge" default:"true" help:"Whether the kubeconfig should merge with an existing one, or overwrite"`
}

func (k *kubeconfigCmd) Run(cfg *cfg) error {
	apiClient, err := cfg.linodeAPIClient()
	if err != nil {
		return err
	}

	ctx := context.Background()

	clusters, err := apiClient.ListLKEClusters(ctx, linodego.NewListOptions(0, ""))
	if err != nil {
		return err
	}

	var cluster *linodego.LKECluster
	for _, c := range clusters {
		if c.Label == k.Cluster {
			cluster = &c
			break
		}
	}

	if cluster == nil {
		return fmt.Errorf("could not find cluster named %q in list:\n%s", k.Cluster, fmtClusterNames(clusters))
	}

	kc, err := apiClient.GetLKEClusterKubeconfig(ctx, cluster.ID)
	if err != nil {
		return err
	}

	data, err := base64.StdEncoding.DecodeString(kc.KubeConfig)
	if err != nil {
		return err
	}

	newCfg, err := mergeConfigs(k.Output, data)
	if err != nil {
		return err
	}

	err = writeConfig(k.Output, newCfg)
	if err != nil {
		return err
	}

	fmt.Print("\nAccess your cluster with:\n")
	fmt.Printf("kubectl config use-context %s\n", fmt.Sprintf("lke%d", cluster.ID))
	fmt.Println("kubectl get node")

	return nil
}

func fmtClusterNames(clusters []linodego.LKECluster) string {
	var sb strings.Builder
	for _, cluster := range clusters {
		sb.WriteString("- ")
		sb.WriteString(cluster.Label)
		sb.WriteString("\n")
	}
	return sb.String()
}

func main() {
	cli := cLI{}
	logger := log.New(io.Discard, "", log.LstdFlags)
	kctx := kong.Parse(&cli,
		kong.Name("lkecli"),
		kong.Description("Helper CLI for interacting with LKE"),
		kong.UsageOnError(),
		kong.Bind(logger))
	err := kctx.Run(&cli.cfg)
	kctx.FatalIfErrorf(err)
}
