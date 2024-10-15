package main

import (
	"fmt"
	"os"
	"os/exec"
)

func mergeConfigs(localKubeconfigPath string, newCfg []byte) ([]byte, error) {
	// Create a temporary kubeconfig to store the config
	file, err := os.CreateTemp(os.TempDir(), "lke-temp-*")
	if err != nil {
		return nil, fmt.Errorf("could not generate a temporary file to store the kubeconfig: %s", err)
	}
	defer file.Close()

	if writeErr := writeConfig(file.Name(), newCfg); writeErr != nil {
		return nil, writeErr
	}

	fmt.Printf("Merged with main kubernetes config: %s\n", localKubeconfigPath)

	// Merge the two kubeconfigs and read the output into 'data'
	var cmd *exec.Cmd

	// Append KUBECONFIGS in ENV Vars
	appendKubeConfigENV := fmt.Sprintf("KUBECONFIG=%s:%s", file.Name(), localKubeconfigPath)
	cmd = exec.Command("kubectl", "config", "view", "--merge", "--flatten")
	cmd.Env = append(os.Environ(), appendKubeConfigENV)

	data, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("could not merge kubeconfigs: %s", err)
	}

	err = file.Close()
	if err != nil {
		return nil, fmt.Errorf("could not close temporary kubeconfig file: %s, %s", file.Name(), err)
	}

	err = os.Remove(file.Name())
	if err != nil {
		return nil, fmt.Errorf("could not remove temporary kubeconfig file: %s, %s", file.Name(), err)
	}

	return data, nil
}

func writeConfig(path string, data []byte) error {
	var _, err = os.Stat(path)

	// create file if not exists
	if os.IsNotExist(err) {
		var file, err = os.Create(path)
		if err != nil {
			return err
		}
		defer file.Close()
	}

	writeErr := os.WriteFile(path, data, 0600)
	if writeErr != nil {
		return writeErr
	}
	return nil
}
