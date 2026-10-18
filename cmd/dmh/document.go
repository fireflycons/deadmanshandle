package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/fireflycons/deadmanshandle/internal/cli"
)

var documentCommands = map[string]command{
	"get": {"download the document to the current directory", runDocumentGet},
	"put": {"upload a new document, and point the settings at it", runDocumentPut},
}

func runDocument(ctx context.Context, g globals, args []string) error {
	return subcommand(ctx, g, "document", documentCommands, []string{"get", "put"}, args)
}

func runDocumentGet(ctx context.Context, g globals, args []string) error {
	fs := newFlagSet("document get", "[flags]")
	force := fs.Bool("force", false, "replace a file of the same name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("document get: unexpected argument %q", fs.Arg(0))
	}

	s, err := newSession(ctx, g)
	if err != nil {
		return err
	}
	docs, bucket, err := s.documentStore(ctx)
	if err != nil {
		return fmt.Errorf("document get: %w", err)
	}
	file, err := cli.GetDocument(ctx, s.ssm, s.configParameter(), docs, bucket, ".", *force)
	if err != nil {
		return fmt.Errorf("document get: %w", err)
	}
	fmt.Println("Wrote", file)
	return nil
}

func runDocumentPut(ctx context.Context, g globals, args []string) error {
	fs := newFlagSet("document put", "[flags] <file>")
	key := fs.String("key", "", "S3 key to upload to (default: the file's name)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("document put: give the file to upload")
	}

	s, err := newSession(ctx, g)
	if err != nil {
		return err
	}
	docs, bucket, err := s.documentStore(ctx)
	if err != nil {
		return fmt.Errorf("document put: %w", err)
	}
	result, err := cli.PutDocument(ctx, s.ssm, s.configParameter(), docs, bucket, fs.Arg(0), *key)
	if err != nil {
		return fmt.Errorf("document put: %w", err)
	}

	fmt.Printf("Uploaded %s to s3://%s/%s\n", fs.Arg(0), bucket, result.Key)
	if result.KeyChanged() {
		fmt.Printf("The settings now name it as the document. The old one is still at s3://%s/%s;\n", bucket, result.PreviousKey)
		fmt.Println("delete it if you no longer need it. The change is reported by the next daily run")
		fmt.Println("if the upload's own notice doesn't come first.")
	}
	fmt.Println("You'll be emailed that the document changed. If the deadline passes within")
	fmt.Println("24 hours of the change, delivery is held until the 24 hours are up.")
	return nil
}
