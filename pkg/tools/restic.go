package tools

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"

	"github.com/forbearing/restic"
)

var (
	ErrRepoNotSet   = errors.New("restic repository not set")
	ErrPasswdNotSet = errors.New("restic password not set")
	ErrNotFound     = errors.New(`command "restic" not found`)
)

type Restic struct {
	Repo   string
	Passwd string
	Hosts  []string
	Tags   []string

	Cmd string

	// restic command stdout and stderr output.
	stdout io.Writer
	stderr io.Writer

	// set the restic command output format to JSON, default to TEXT.
	JSON bool
}

func (t *Restic) check() error {
	if len(t.Repo) == 0 {
		return ErrRepoNotSet
	}
	if len(t.Passwd) == 0 {
		return ErrPasswdNotSet
	}
	if _, err := exec.LookPath("restic"); err != nil {
		return ErrNotFound
	}

	return nil
}
func (t *Restic) cleanup(ctx context.Context) {
	r, _ := restic.New(ctx, nil)
	r.Command(restic.Unlock{}).Run()
}

// DoBackup start backup data using the restic backup tool.
func (t *Restic) DoBackup(ctx context.Context, dst, src string) error {
	defer t.cleanup(ctx)

	if err := t.check(); err != nil {
		return err
	}

	r, err := restic.New(ctx, &restic.GlobalFlags{NoCache: true, Repo: t.Repo})
	if err != nil {
		return err
	}
	os.Setenv("RESTIC_REPOSITORY", t.Passwd)
	os.Setenv("RESTIC_REPOSITORY", t.Repo)
	r.Command(restic.Init{})
	t.Cmd = r.String()

	return r.Run()
}

// DoRestore start restore data using the restic backup tool.
func (t *Restic) DoRestore(ctx context.Context, dst, src string) error {
	defer t.cleanup(ctx)

	return nil
}

// DoMigration start migrate data using the restic backup tool.
func (t *Restic) DoMigration(ctx context.Context, dst, src string) error {
	defer t.cleanup(ctx)

	return nil
}

// DoClone start clone data using the restic backup tool.
func (t *Restic) DoClone(ctx context.Context, dst, src string) error {
	defer t.cleanup(ctx)

	return nil
}
