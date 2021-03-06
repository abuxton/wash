package docker

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/puppetlabs/wash/activity"
	"github.com/puppetlabs/wash/plugin"
	vol "github.com/puppetlabs/wash/volume"
)

type container struct {
	plugin.EntryBase
	id     string
	client *client.Client
}

func (c *container) Metadata(ctx context.Context) (plugin.EntryMetadata, error) {
	// Use raw to also get the container size.
	_, raw, err := c.client.ContainerInspectWithRaw(ctx, c.id, true)
	if err != nil {
		return nil, err
	}

	return plugin.ToMeta(raw), nil
}

func (c *container) List(ctx context.Context) ([]plugin.Entry, error) {
	// TODO: May be worth creating a helper that makes it easy to create
	// read-only files. Lots of shared code between these two.
	cm := &containerMetadata{plugin.NewEntry("metadata.json"), c}
	cm.DisableDefaultCaching()
	content, err := cm.Open(ctx)
	if err != nil {
		return nil, err
	}
	cmAttr := plugin.EntryAttributes{}
	cmAttr.SetSize(uint64(content.Size()))
	cm.SetAttributes(cmAttr)

	clf := &containerLogFile{plugin.NewEntry("log"), c.id, c.client}
	clf.DisableCachingFor(plugin.MetadataOp)
	content, err = clf.Open(ctx)
	if err != nil {
		return nil, err
	}
	clfAttr := plugin.EntryAttributes{}
	clfAttr.SetSize(uint64(content.Size()))
	clf.SetAttributes(clfAttr)

	// Include a view of the remote filesystem using volume.FS
	return []plugin.Entry{cm, clf, vol.NewFS("fs", c)}, nil
}

func (c *container) Exec(ctx context.Context, cmd string, args []string, opts plugin.ExecOptions) (plugin.ExecResult, error) {
	execResult := plugin.ExecResult{}

	command := append([]string{cmd}, args...)
	activity.Record(ctx, "Exec %v on %v", command, c.Name())

	cfg := types.ExecConfig{Cmd: command, AttachStdout: true, AttachStderr: true, Tty: opts.Tty}
	if opts.Stdin != nil || opts.Tty {
		cfg.AttachStdin = true
	}
	created, err := c.client.ContainerExecCreate(ctx, c.id, cfg)
	if err != nil {
		return execResult, err
	}

	resp, err := c.client.ContainerExecAttach(ctx, created.ID, types.ExecStartCheck{})
	if err != nil {
		return execResult, err
	}

	// Asynchronously copy container exec output to an exec output channel.
	outputCh, stdout, stderr := plugin.CreateExecOutputStreams(ctx)
	go func() {
		_, err := stdcopy.StdCopy(stdout, stderr, resp.Reader)
		activity.Record(ctx, "Exec on %v complete: %v", c.Name(), err)
		stdout.CloseWithError(err)
		stderr.CloseWithError(err)
		resp.Close()
	}()

	// If stdin is supplied, asynchronously copy it to container exec input.
	var writeErr error
	if opts.Stdin != nil {
		go func() {
			_, writeErr = io.Copy(resp.Conn, opts.Stdin)
			// If using a Tty, wait until done reading in case we need to send Ctrl-C; when a Tty is
			// allocated commands expect user input and will respond to control signals. Otherwise close
			// input now to ensure commands that depend on EOF execute correctly.
			if !opts.Tty {
				respErr := resp.CloseWrite()
				activity.Record(ctx, "Closed execution input stream for %v: %v, %v", c.Name(), writeErr, respErr)
			}
		}()
	}

	execResult.OutputCh = outputCh
	execResult.CancelFunc = func() {
		// Close the response on cancellation. Copying will block until there's more to read from the
		// exec output. For an action with no more output it may never return.
		if opts.Tty {
			// If resp.Conn is still open, send Ctrl-C over resp.Conn before closing it.
			_, err := resp.Conn.Write([]byte{0x03})
			activity.Record(ctx, "Sent ETX on context termination: %v", err)
		}
		resp.Close()
	}
	execResult.ExitCodeCB = func() (int, error) {
		if writeErr != nil {
			return 0, err
		}

		resp, err := c.client.ContainerExecInspect(ctx, created.ID)
		if err != nil {
			return 0, err
		}

		if resp.Running {
			return 0, fmt.Errorf("the command was marked as 'Running' even though the output streams reached EOF")
		}

		activity.Record(ctx, "Exec on %v exited %v", c.Name(), resp.ExitCode)
		return resp.ExitCode, nil
	}

	return execResult, nil
}
