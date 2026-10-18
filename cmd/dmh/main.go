// Command dmh manages a deployment of the dead man's handle: it makes the
// app's setup QR code, checks in, shows and changes the settings, and gets
// and replaces the document. It works on AWS directly, not through the
// Lambdas, using the credentials in the environment.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

// globals are the flags given before the command
type globals struct {
	app    string // Terraform application_name, which prefixes the parameter names
	region string // empty for the environment's
}

// awsConfig loads the AWS configuration from the environment
func (g globals) awsConfig(ctx context.Context) (aws.Config, error) {
	var opts []func(*config.LoadOptions) error
	if g.region != "" {
		opts = append(opts, config.WithRegion(g.region))
	}
	return config.LoadDefaultConfig(ctx, opts...)
}

// parameter returns the name of one of the application's parameters
func (g globals) parameter(name string) string {
	return "/" + g.app + "/" + name
}

type command struct {
	summary string
	run     func(ctx context.Context, g globals, args []string) error
}

var commands = map[string]command{
	"qr": {"write the app's setup QR code", runQR},
}

// commandOrder is the order commands are listed in the usage
var commandOrder = []string{"qr"}

func main() {
	err := run(context.Background(), os.Args[1:])
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "dmh:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	var g globals
	fs := flag.NewFlagSet("dmh", flag.ContinueOnError)
	fs.StringVar(&g.app, "app", "deadmanshandle", "Terraform application_name, which prefixes the parameter names")
	fs.StringVar(&g.region, "region", "", "AWS region (default: from the environment)")
	fs.Usage = func() { usage(fs) }
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		fs.Usage()
		return errors.New("no command given")
	}
	name := fs.Arg(0)
	cmd, ok := commands[name]
	if !ok {
		fs.Usage()
		return fmt.Errorf("unknown command %q", name)
	}
	return cmd.run(ctx, g, fs.Args()[1:])
}

func usage(fs *flag.FlagSet) {
	out := fs.Output()
	_, _ = fmt.Fprintln(out, "Usage: dmh [-app name] [-region region] <command> [flags]")
	_, _ = fmt.Fprintln(out, "\nCommands:")
	for _, name := range commandOrder {
		_, _ = fmt.Fprintf(out, "  %-10s %s\n", name, commands[name].summary)
	}
	_, _ = fmt.Fprintln(out, "\nRun dmh <command> -h for a command's flags.\n\nGlobal flags:")
	fs.PrintDefaults()
}

// newFlagSet returns a flag set for a command, whose usage line is
// "dmh <name> <synopsis>"
func newFlagSet(name, synopsis string) *flag.FlagSet {
	fs := flag.NewFlagSet("dmh "+name, flag.ContinueOnError)
	fs.Usage = func() {
		_, _ = fmt.Fprintln(fs.Output(), strings.TrimSpace("Usage: dmh "+name+" "+synopsis))
		fs.PrintDefaults()
	}
	return fs
}

