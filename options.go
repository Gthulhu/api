package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
)

// CommandLineOptions contains all command line options
type CommandLineOptions struct {
	ConfigPath     string
	Port           string
	KubeConfigPath string
	InCluster      bool
}

// ParseCommandLineOptions parses command line arguments
func ParseCommandLineOptions() CommandLineOptions {
	options := CommandLineOptions{}

	// Define command line flags
	flag.StringVar(&options.ConfigPath, "config", "config.json", "Path to configuration file")
	flag.StringVar(&options.Port, "port", "", "Server port (overrides config file)")
	flag.StringVar(&options.KubeConfigPath, "kubeconfig", "", "Path to Kubernetes config file (defaults to $HOME/.kube/config)")
	flag.BoolVar(&options.InCluster, "in-cluster", false, "Run in Kubernetes in-cluster mode")

	// Parse flags
	flag.Parse()

	// If kubeconfig is not specified, check environment variable
	if options.KubeConfigPath == "" {
		options.KubeConfigPath = os.Getenv("KUBECONFIG")
	}

	// If still empty, use default path
	if options.KubeConfigPath == "" && !options.InCluster {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			options.KubeConfigPath = filepath.Join(homeDir, ".kube", "config")
		}
	}

	return options
}

// PrintCommandLineOptions prints the current command line options
func PrintCommandLineOptions(options CommandLineOptions) {
	log.Printf("Configuration:")
	log.Printf("  Config file: %s", options.ConfigPath)
	if options.Port != "" {
		log.Printf("  Port override: %s", options.Port)
	}

	if options.InCluster {
		log.Printf("  Kubernetes: In-cluster mode")
	} else if options.KubeConfigPath != "" {
		log.Printf("  Kubernetes config: %s", options.KubeConfigPath)
	} else {
		log.Printf("  Kubernetes: No config specified")
	}
}
