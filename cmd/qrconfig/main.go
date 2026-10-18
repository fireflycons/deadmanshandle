// Command qrconfig writes a PNG QR code that sets up the Android check-in
// app. By default it reads the API endpoint from the Terraform state and the
// API key from the config file. With -aws it reads the active values from
// Parameter Store instead, using the credentials in the environment. With
// -firebase it adds the Firebase project's client settings, from its
// google-services.json, so the app can receive push notifications.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/fireflycons/deadmanshandle/internal/adapters"
	"github.com/fireflycons/deadmanshandle/internal/qrconfig"
	"github.com/skip2/go-qrcode"
)

func main() {
	statePath := flag.String("state", "terraform/terraform.tfstate", "Terraform state file (not with -aws)")
	configPath := flag.String("config", "config.json", "config file holding the apiKey (not with -aws)")
	fromAWS := flag.Bool("aws", false, "read the endpoint and API key from Parameter Store")
	app := flag.String("app", "deadmanshandle", "Terraform application_name, which prefixes the parameter names (with -aws only)")
	firebase := flag.String("firebase", "", "the Firebase project's google-services.json, to turn on push notifications")
	outPath := flag.String("out", "checkin-qr.png", "PNG file to write")
	size := flag.Int("size", 1024, "width and height of the PNG in pixels")
	flag.Parse()

	set := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { set[f.Name] = true })

	var endpoint, apiKey string
	var err error
	switch {
	case *fromAWS && (set["state"] || set["config"]):
		err = errors.New("-aws cannot be used with -state or -config")
	case !*fromAWS && set["app"]:
		err = errors.New("-app is only used with -aws")
	case *fromAWS:
		endpoint, apiKey, err = readAWS(context.Background(), *app)
	default:
		endpoint, apiKey, err = readFiles(*statePath, *configPath)
	}
	var push *qrconfig.Push
	if err == nil && *firebase != "" {
		push, err = readFirebase(*firebase)
	}
	if err == nil {
		err = write(endpoint, apiKey, push, *outPath, *size)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "qrconfig:", err)
		os.Exit(1)
	}
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
func readAWS(ctx context.Context, app string) (endpoint, apiKey string, err error) {
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return "", "", err
	}
	store := adapters.NewSSMConfigStore(ssm.NewFromConfig(awsCfg))

	endpointParam := "/" + app + "/api-endpoint"
	value, err := store.GetConfig(ctx, endpointParam)
	if err != nil {
		return "", "", fmt.Errorf("%s: %w", endpointParam, err)
	}
	endpoint = string(value)
	if err := qrconfig.CheckEndpoint(endpoint); err != nil {
		return "", "", fmt.Errorf("%s: %w", endpointParam, err)
	}

	configParam := "/" + app + "/config"
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

func write(endpoint, apiKey string, push *qrconfig.Push, outPath string, size int) error {
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
