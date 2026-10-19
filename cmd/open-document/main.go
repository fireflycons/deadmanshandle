// Command open-document opens (decrypts) a document sent by the dead man's
// handle. It is for the recipients: they drop the emailed file onto it, or
// give its path, and type the password the owner gave them. If they have AWS
// access to the deployment, the password is read from there instead.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/fireflycons/deadmanshandle/internal/adapters"
	"github.com/fireflycons/deadmanshandle/internal/cli"
	"github.com/fireflycons/deadmanshandle/internal/opener"
	"github.com/fireflycons/deadmanshandle/internal/prompt"
)

func main() {
	app := flag.String("app", "deadmanshandle", "the deployment's name, for reading the password from AWS")
	region := flag.String("region", "", "the deployment's AWS region (default: from the AWS settings)")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: open-document [flags] [file]")
		fmt.Fprintln(os.Stderr, "Opens a document sent by the dead man's handle. Without a file, it asks for one.")
		flag.PrintDefaults()
	}
	flag.Parse()

	err := run(*app, *region, flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Sorry, "+err.Error())
	}
	pauseIfOwnWindow()
	if err != nil {
		os.Exit(1)
	}
}

func run(app, region, file string) error {
	fmt.Println("Open a document sent by the dead man's handle")
	fmt.Println()
	if file == "" {
		var err error
		file, err = prompt.Line("Drag the document (its name ends in .dmh) onto this window,\nor type where it is, then press Enter:\n> ")
		if err != nil {
			return err
		}
	}
	file = opener.CleanPath(file)
	if file == "" {
		return fmt.Errorf("no document was given")
	}

	out, err := opener.Open(context.Background(), file, awsPassword(app, region), prompt.Password)
	if err != nil {
		return err
	}
	fmt.Println()
	fmt.Println("Done. The opened document is saved as:")
	fmt.Println("  " + out)
	return nil
}

// awsPassword reads the password from the deployment's Parameter Store, for
// a recipient who has AWS access to it. Anyone else has no credentials, and
// is asked for the password.
func awsPassword(app, region string) opener.StoredPassword {
	return func(ctx context.Context) (string, bool, error) {
		ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		// Off an EC2 instance the metadata service only makes the search for
		// credentials slow
		if os.Getenv("AWS_EC2_METADATA_DISABLED") == "" {
			_ = os.Setenv("AWS_EC2_METADATA_DISABLED", "true")
		}
		var opts []func(*config.LoadOptions) error
		if region != "" {
			opts = append(opts, config.WithRegion(region))
		}
		cfg, err := config.LoadDefaultConfig(ctx, opts...)
		if err != nil || cfg.Region == "" {
			return "", false, err
		}
		if _, err := cfg.Credentials.Retrieve(ctx); err != nil {
			return "", false, err
		}
		store := adapters.NewSSMConfigStore(ssm.NewFromConfig(cfg))
		return cli.ReadDocumentPassword(ctx, store, "/"+app+"/document-password")
	}
}
