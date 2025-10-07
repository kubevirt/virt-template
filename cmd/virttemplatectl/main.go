package main

import (
	"os"

	_ "k8s.io/client-go/plugin/pkg/client/auth" // Import to initialize client auth plugins.

	virttemplattectl "kubevirt.io/virt-template/internal/virttemplatectl"
)

func main() {
	os.Exit(virttemplattectl.Execute())
}
