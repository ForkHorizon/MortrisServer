package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/ForkHorizon/Mortris/internal/adminauth"
	"github.com/ForkHorizon/Mortris/internal/config"
	"github.com/ForkHorizon/Mortris/internal/store"
)

// runCreateAdmin implements "CLI creates the first administrator"
// (section 10.3) — there is no public account creation endpoint, this is
// the only way an admin_users row is ever created.
func runCreateAdmin(ctx context.Context, cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("create-admin", flag.ExitOnError)
	email := fs.String("email", "", "admin email (required)")
	role := fs.String("role", "", "admin or viewer (required)")
	projects := fs.String("projects", "", "comma-separated project IDs this account may access (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *email == "" || *role == "" || *projects == "" {
		return fmt.Errorf("--email, --role, and --projects are all required")
	}
	if *role != "admin" && *role != "viewer" {
		return fmt.Errorf("--role must be \"admin\" or \"viewer\"")
	}
	projectIDs := strings.Split(*projects, ",")
	for i := range projectIDs {
		projectIDs[i] = strings.TrimSpace(projectIDs[i])
	}

	password, err := readPasswordTwice()
	if err != nil {
		return err
	}

	if cfg.WriterDSN == "" {
		return fmt.Errorf("MORTRIS_WRITER_DSN is required")
	}
	pool, err := store.NewPool(ctx, cfg.WriterDSN, 2)
	if err != nil {
		return err
	}
	defer pool.Close()

	for _, id := range projectIDs {
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM projects WHERE id = $1)`, id).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("project %q does not exist", id)
		}
	}

	passwordHash, err := adminauth.HashPassword(password)
	if err != nil {
		return err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var adminUserID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO admin_users (email, password_hash, role) VALUES ($1, $2, $3) RETURNING id
	`, *email, passwordHash, *role).Scan(&adminUserID); err != nil {
		return fmt.Errorf("insert admin_users (email already exists?): %w", err)
	}
	for _, id := range projectIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO admin_user_projects (admin_user_id, project_id) VALUES ($1, $2)`, adminUserID, id); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO admin_audit_log (admin_user_id, action, detail)
		VALUES ($1, 'admin_created', jsonb_build_object('email', $2::text, 'role', $3::text, 'projects', $4::text))
	`, adminUserID, *email, *role, *projects); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	fmt.Printf("created %s admin user %s with access to: %s\n", *role, *email, strings.Join(projectIDs, ", "))
	return nil
}

func readPasswordTwice() (string, error) {
	interactive := term.IsTerminal(int(os.Stdin.Fd()))
	// One shared reader for the non-interactive path — a fresh
	// bufio.Reader per call would silently drop whatever it had already
	// buffered past the first line's delimiter.
	lineReader := bufio.NewReader(os.Stdin)

	read := func(prompt string) (string, error) {
		fmt.Fprint(os.Stderr, prompt)
		if interactive {
			b, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Fprintln(os.Stderr)
			return string(b), err
		}
		// Piped stdin (scripting/automation) — no TTY to suppress echo on.
		line, err := lineReader.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", err
		}
		return strings.TrimRight(line, "\r\n"), nil
	}

	p1, err := read("Password: ")
	if err != nil {
		return "", err
	}
	p2, err := read("Confirm password: ")
	if err != nil {
		return "", err
	}
	if p1 != p2 {
		return "", fmt.Errorf("passwords do not match")
	}
	if len(p1) < 12 {
		return "", fmt.Errorf("password must be at least 12 characters")
	}
	return p1, nil
}
