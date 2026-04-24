package main

import (
	"fmt"
	"os"

	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/apis/options"
	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/validation"
	"github.com/spf13/pflag"
)

func main() {
	log := logger.NewLogEntry()

	flagSet := pflag.NewFlagSet("oauth2-proxy", pflag.ExitOnError)

	// Define core flags
	config := flagSet.String("config", "", "path to config file")
	showVersion := flagSet.Bool("version", false, "print version string")
	alphaConfig := flagSet.String("alpha-config", "", "path to alpha config file (experimental)")

	// Add option flags from the options package
	options.AddFlags(flagSet)

	err := flagSet.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing flags: %v\n", err)
		os.Exit(1)
	}

	if *showVersion {
		fmt.Printf("oauth2-proxy %s (built with %s)\n", VERSION, runtime.Version())
		return
	}

	// Load configuration
	opts, err := loadConfiguration(*config, *alphaConfig, flagSet, os.Args[1:])
	if err != nil {
		log.Fatalf("ERROR: failed to load configuration: %v", err)
	}

	// Validate configuration
	if err := validation.Validate(opts); err != nil {
		log.Fatalf("ERROR: invalid configuration: %v", err)
	}

	// Initialize and run the proxy
	oauthProxy, err := proxy.NewOAuthProxy(opts, func(email string) bool {
		return opts.IsValidatedEmail(email)
	})
	if err != nil {
		log.Fatalf("ERROR: failed to initialise OAuth2 Proxy: %v", err)
	}

	server := server.NewServer(opts, oauthProxy)
	if err := server.Start(); err != nil {
		log.Fatalf("ERROR: server failed to start: %v", err)
	}
}

// loadConfiguration reads and merges configuration from file and flags.
// Priority order (highest to lowest): CLI flags > alpha config > config file > defaults
func loadConfiguration(configFile, alphaConfigFile string, flagSet *pflag.FlagSet, args []string) (*options.Options, error) {
	opts, err := options.NewOptions()
	if err != nil {
		return nil, fmt.Errorf("failed to create default options: %w", err)
	}

	// Look for config files in a few convenient locations so local dev doesn't
	// require passing --config every time. Check current dir first, then ~/.config.
	if configFile == "" {
		candidates := []string{
			"oauth2-proxy.cfg",
			os.Getenv("HOME") + "/.config/oauth2-proxy.cfg",
			// Also check XDG_CONFIG_HOME if set, which is more correct on Linux
			os.Getenv("XDG_CONFIG_HOME") + "/oauth2-proxy/oauth2-proxy.cfg",
		}
		for _, candidate := range candidates {
			if candidate == "/oauth2-proxy/oauth2-proxy.cfg" {
				// XDG_CONFIG_HOME was not set, skip this candidate
				continue
			}
			if _, statErr := os.Stat(candidate); statErr == nil {
				configFile = candidate
				break
			}
		}
	}

	if configFile != "" {
		if err := options.LoadConfig(configFile, opts); err != nil {
			return nil, fmt.Errorf("failed to load config file %q: %w", configFile, err)
		}
	}

	if alphaConfigFile != "" {
		alpha, err := options.LoadAlphaOptions(alphaConfigFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load alpha config file %q: %w", alphaConfigFile, err)
		}
		// Merge alpha options into the main options struct
		alpha.MergeInto(opts)
	}

	return opts, nil
}
