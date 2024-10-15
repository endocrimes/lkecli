# lkecli

A Lightweight convenience CLI for interacting with Linode Kubernetes Engine
(LKE).

It currently only works on linux/macos, but windows support would be a welcome
addition.

## Usage

If you are authenticated with `linode-cli`, `lkecli` will automatically re-use
your authentication data, otherwise you can set `LINODE_CLI_TOKEN`/ pass
`--linode-cli-token` to authenticate.

### Merge Kubeconfigs

`lkecli kubeconfig {clustername}` will merge the kubeconfig for the given
cluster into your $HOME/.kube/config.
