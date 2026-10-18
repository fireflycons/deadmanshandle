package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/fireflycons/deadmanshandle/internal/adapters"
	"github.com/fireflycons/deadmanshandle/internal/qrconfig"
	"github.com/skip2/go-qrcode"
)

// runQR writes a PNG QR code that sets up the Android check-in app. By
// default it reads the API endpoint from the Terraform state and the API key
// from the config file. With -aws it reads the active values from Parameter
// Store instead. With -firebase it adds the Firebase project's client
// settings, from its google-services.json, so the app can receive push
// notifications.
func runQR(ctx context.Context, g globals, args []string) error {
	fs := newFlagSet("qr", "[flags]")
	statePath := fs.String("state", "terraform/terraform.tfstate", "Terraform state file (not with -aws)")
	configPath := fs.String("config", "config.json", "config file holding the apiKey (not with -aws)")
	fromAWS := fs.Bool("aws", false, "read the endpoint and API key from Parameter Store")
	firebase := fs.String("firebase", "", "the Firebase project's google-services.json, to turn on push notifications")
	outPath := fs.String("out", "checkin-qr.png", "PNG file to write")
	size := fs.Int("size", 1024, "width and height of the PNG in pixels")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("qr: unexpected argument %q", fs.Arg(0))
	}

	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })

	var endpoint, apiKey string
	var err error
	switch {
	case *fromAWS && (set["state"] || set["config"]):
		err = errors.New("-aws cannot be used with -state or -config")
	case *fromAWS:
		endpoint, apiKey, err = readAWS(ctx, g)
	default:
		endpoint, apiKey, err = readFiles(*statePath, *configPath)
	}
	var push *qrconfig.Push
	if err == nil && *firebase != "" {
		push, err = readFirebase(*firebase)
	}
	if err == nil {
		err = writeQR(endpoint, apiKey, push, *outPath, *size)
	}
	if err != nil {
		return fmt.Errorf("qr: %w", err)
	}
	return nil
}

func readFiles(statePath, configPath string) (endpoint, apiKey string, err error) {
	state, err := os.ReadFile(statePath)
	if err != nil {
		return "", "", err
	}
	endpoint, err = qrconfig.EndpointFromState(state)
	if err != nil {
		return "", "", fmt.Errorf("%s: %w", statePath, err)
	}

	cfg, err := os.ReadFile(configPath)
	if err != nil {
		return "", "", err
	}
	apiKey, err = qrconfig.APIKeyFromConfig(cfg)
	if err != nil {
		return "", "", fmt.Errorf("%s: %w", configPath, err)
	}
	return endpoint, apiKey, nil
}

// readAWS reads the parameters that Terraform creates in
// terraform/parameter_store.tf
func readAWS(ctx context.Context, g globals) (endpoint, apiKey string, err error) {
	awsCfg, err := g.awsConfig(ctx)
	if err != nil {
		return "", "", err
	}
	store := adapters.NewSSMConfigStore(ssm.NewFromConfig(awsCfg))

	endpointParam := g.parameter("api-endpoint")
	value, err := store.GetConfig(ctx, endpointParam)
	if err != nil {
		return "", "", fmt.Errorf("%s: %w", endpointParam, err)
	}
	endpoint = string(value)
	if err := qrconfig.CheckEndpoint(endpoint); err != nil {
		return "", "", fmt.Errorf("%s: %w", endpointParam, err)
	}

	configParam := g.parameter("config")
	value, err = store.GetConfig(ctx, configParam)
	if err != nil {
		return "", "", fmt.Errorf("%s: %w", configParam, err)
	}
	apiKey, err = qrconfig.APIKeyFromConfig(value)
	if err != nil {
		return "", "", fmt.Errorf("%s: %w", configParam, err)
	}
	return endpoint, apiKey, nil
}

func readFirebase(path string) (*qrconfig.Push, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	push, err := qrconfig.PushFromGoogleServices(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return push, nil
}

func writeQR(endpoint, apiKey string, push *qrconfig.Push, outPath string, size int) error {
	payload, err := qrconfig.Encode(endpoint, apiKey, push)
	if err != nil {
		return err
	}
	png, err := qrcode.Encode(string(payload), qrcode.Medium, size)
	if err != nil {
		return err
	}
	// Owner-only, as it holds the API key
	if err := os.WriteFile(outPath, png, 0o600); err != nil {
		return err
	}

	fmt.Printf("Wrote %s for %s\n", outPath, endpoint)
	if push != nil {
		fmt.Printf("Push notifications: Firebase project %s\n", push.ProjectID)
	} else {
		fmt.Println("Push notifications: off (no -firebase)")
	}
	fmt.Println("It contains the API key: delete it once the app has scanned it.")
	return nil
}
