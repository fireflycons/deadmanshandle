package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/fireflycons/deadmanshandle/internal/cli"
	"github.com/fireflycons/deadmanshandle/internal/passphrase"
)

var documentCommands = map[string]command{
	"get":      {"download the document to the current directory", runDocumentGet},
	"put":      {"upload a new document, and point the settings at it", runDocumentPut},
	"password": {"set or show the password that encrypts the document", runDocumentPassword},
}

func runDocument(ctx context.Context, g globals, args []string) error {
	return subcommand(ctx, g, "document", documentCommands, []string{"get", "put", "password"}, args)
}

var passwordCommands = map[string]command{
	"set":  {"make a new password (or choose one), re-encrypting the document", runPasswordSet},
	"show": {"show the password", runPasswordShow},
}

func runDocumentPassword(ctx context.Context, g globals, args []string) error {
	return subcommand(ctx, g, "document password", passwordCommands, []string{"set", "show"}, args)
}

func runPasswordSet(ctx context.Context, g globals, args []string) error {
	fs := newFlagSet("document password set", "[flags]")
	words := fs.Int("words", passphrase.DefaultWords, "number of words in the generated passphrase")
	prompt := fs.Bool("prompt", false, fmt.Sprintf("type your own password (at least %d characters) instead", cli.MinPasswordLength))
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("document password set: unexpected argument %q", fs.Arg(0))
	}

	var password string
	if *prompt {
		first, err := readPassword("New password: ")
		if err != nil {
			return err
		}
		again, err := readPassword("Again: ")
		if err != nil {
			return err
		}
		if first != again {
			return errors.New("document password set: the passwords don't match")
		}
		password = first
	} else {
		generated, err := passphrase.Generate(*words)
		if err != nil {
			return fmt.Errorf("document password set: %w", err)
		}
		password = generated
		// Shown before anything changes, so that it isn't lost if a step fails
		fmt.Println("Document password:", password)
		fmt.Println()
	}

	s, err := newSession(ctx, g)
	if err != nil {
		return err
	}
	docs, bucket, err := s.documentStore(ctx)
	if err != nil {
		return fmt.Errorf("document password set: %w", err)
	}
	change, err := cli.SetDocumentPassword(ctx, s.ssm, s.passwordParameter(), s.configParameter(), docs, bucket, password)
	if err != nil {
		return fmt.Errorf("document password set: %w", err)
	}

	fmt.Println("Saved in", s.passwordParameter())
	switch {
	case change.Reencrypted:
		fmt.Printf("s3://%s/%s is now encrypted with it. You'll be emailed that the document\n", bucket, change.Key)
		fmt.Println("changed, and delivery is held for 24 hours.")
	case change.Plain:
		fmt.Printf("s3://%s/%s isn't encrypted; dmh document put encrypts it.\n", bucket, change.Key)
	case change.Missing:
		fmt.Println("No document is stored yet; dmh document put encrypts one with it.")
	}
	fmt.Println()
	fmt.Println("Give the password to your recipients now, by hand or on paper, never by email:")
	fmt.Println("they need it to open the document. If it replaced an older one, they need this")
	fmt.Println("new one instead.")
	return nil
}

func runPasswordShow(ctx context.Context, g globals, args []string) error {
	fs := newFlagSet("document password show", "[flags]")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("document password show: unexpected argument %q", fs.Arg(0))
	}

	s, err := newSession(ctx, g)
	if err != nil {
		return err
	}
	password, found, err := cli.ReadDocumentPassword(ctx, s.ssm, s.passwordParameter())
	if err != nil {
		return fmt.Errorf("document password show: %w", err)
	}
	if !found {
		return fmt.Errorf("document password show: no password is set; dmh document password set makes one")
	}
	fmt.Println(password)
	return nil
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
