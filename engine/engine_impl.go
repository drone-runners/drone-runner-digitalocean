// Copyright 2019 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by the Polyform License
// that can be found in the LICENSE file.

package engine

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/drone-runners/drone-runner-digitalocean/internal/platform"
	"github.com/drone/runner-go/logger"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

const (
	// the time to wait for ssh connections to be established on a single
	// ssh connection attempt.
	sshDialTimeout = time.Second * 10

	// the time to wait for our overall setup routine to connect to a recently
	// launched droplet.
	networkTimeout = time.Minute * 10
)

// New returns a new engine.
func New(publickeyFile, privatekeyFile string) (Engine, error) {
	publickey, err := ioutil.ReadFile(publickeyFile)
	if err != nil {
		return nil, err
	}
	privatekey, err := ioutil.ReadFile(privatekeyFile)
	if err != nil {
		return nil, err
	}
	fingerprint, err := calcFingerprint(publickey)
	if err != nil {
		return nil, err
	}
	return &engine{
		publickey:   string(publickey),
		privatekey:  string(privatekey),
		fingerprint: fingerprint,
	}, err
}

type engine struct {
	privatekey  string
	publickey   string
	fingerprint string
}

// Setup the pipeline environment.
func (e *engine) Setup(ctx context.Context, spec *Spec) error {
	err := platform.RegisterKey(ctx, platform.RegisterArgs{
		Fingerprint: e.fingerprint,
		Name:        "drone_runner_key",
		Data:        e.publickey,
		Token:       spec.Token,
	})
	if err != nil {
		return err
	}

	// provision the server instance.
	instance, err := platform.Provision(ctx, platform.ProvisionArgs{
		Key:    e.fingerprint,
		Image:  spec.Server.Image,
		Name:   spec.Server.Name,
		Region: spec.Server.Region,
		Size:   spec.Server.Size,
		Token:  spec.Token,
	})
	if instance.ID > 0 {
		spec.id = instance.ID
		spec.ip = instance.IP
	}
	if err != nil {
		return err
	}

	// establish an ssh connection with the server instance
	// to setup the build environment (upload build scripts, etc)

	client, err := dialRetry(
		ctx,
		spec.ip,
		spec.Server.User,
		e.privatekey,
	)
	if err != nil {
		return err
	}
	defer client.Close()

	clientftp, err := sftp.NewClient(client)
	if err != nil {
		logger.FromContext(ctx).
			WithError(err).
			WithField("hostname", spec.Server.Name).
			WithField("ip", instance.IP).
			WithField("id", instance.ID).
			Debug("failed to create sftp client")
		return err
	}
	if clientftp != nil {
		defer clientftp.Close()
	}

	// the pipeline workspace is created before pipeline
	// execution begins. All files and folders created during
	// pipeline execution are isolated to this workspace.
	err = mkdir(clientftp, spec.Root, 0777)
	if err != nil {
		logger.FromContext(ctx).
			WithError(err).
			WithField("path", spec.Root).
			Error("cannot create workspace directory")
		return err
	}

	// the pipeline specification may define global folders, such
	// as the pipeline working directory, wich must be created
	// before pipeline execution begins.
	for _, file := range spec.Files {
		if file.IsDir == false {
			continue
		}
		err = mkdir(clientftp, file.Path, file.Mode)
		if err != nil {
			logger.FromContext(ctx).
				WithError(err).
				WithField("path", file.Path).
				Error("cannot create directory")
			return err
		}
	}

	// the pipeline specification may define global files such
	// as authentication credentials that should be uploaded
	// before pipeline execution begins.
	for _, file := range spec.Files {
		if file.IsDir == true {
			continue
		}
		err = upload(clientftp, file.Path, file.Data, file.Mode)
		if err != nil {
			logger.FromContext(ctx).
				WithError(err).
				Error("cannot write file")
			return err
		}
	}

	logger.FromContext(ctx).
		WithField("hostname", spec.Server.Name).
		WithField("ip", instance.IP).
		WithField("id", instance.ID).
		Debug("server configuration complete")
	return nil
}

// Destroy the pipeline environment.
func (e *engine) Destroy(ctx context.Context, spec *Spec) error {
	// if the server was not successfully created
	// exit since there is no droplet to delete.
	if spec.id == 0 {
		return nil
	}
	logger.FromContext(ctx).
		WithField("hostname", spec.Server.Name).
		WithField("ip", spec.ip).
		WithField("id", spec.id).
		Debug("terminating server")
	return platform.Destroy(ctx, platform.DestroyArgs{
		ID:    spec.id,
		IP:    spec.ip,
		Token: spec.Token,
	})
}

// Run runs the pipeline step.
func (e *engine) Run(ctx context.Context, spec *Spec, step *Step, output io.Writer) (*State, error) {
	// we should not need dialRetry here, since we've already confirmed we
	// can connect via the Setup method.
	client, err := dial(
		spec.ip,
		spec.Server.User,
		e.privatekey,
	)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	clientftp, err := sftp.NewClient(client)
	if err != nil {
		return nil, err
	}
	defer clientftp.Close()

	// unlike os/exec there is no good way to set environment
	// the working directory or configure environment variables.
	// we work around this by pre-pending these configurations
	// to the pipeline execution script.
	for _, file := range step.Files {
		w := new(bytes.Buffer)
		writeWorkdir(w, step.WorkingDir)
		writeSecrets(w, spec.Platform.OS, step.Secrets)
		writeEnviron(w, spec.Platform.OS, step.Envs)
		w.Write(file.Data)
		err = upload(clientftp, file.Path, w.Bytes(), file.Mode)
		if err != nil {
			logger.FromContext(ctx).
				WithError(err).
				WithField("path", file.Path).
				Error("cannot write file")
			return nil, err
		}
	}

	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	session.Stdout = output
	session.Stderr = output
	cmd := step.Command + " " + strings.Join(step.Args, " ")

	log := logger.FromContext(ctx)
	log.Debug("ssh session started")

	done := make(chan error)
	go func() {
		done <- session.Run(cmd)
	}()

	select {
	case err = <-done:
	case <-ctx.Done():
		// BUG(bradrydzewski): openssh does not support the signal
		// command and will not signal remote processes. This may
		// be resolved in openssh 7.9 or higher. Please subscribe
		// to https://github.com/golang/go/issues/16597.
		if err := session.Signal(ssh.SIGKILL); err != nil {
			log.WithError(err).Debug("kill remote process")
		}

		log.Debug("ssh session killed")
		return nil, ctx.Err()
	}

	state := &State{
		ExitCode:  0,
		Exited:    true,
		OOMKilled: false,
	}
	if err != nil {
		state.ExitCode = 255
	}
	if exiterr, ok := err.(*ssh.ExitError); ok {
		state.ExitCode = exiterr.ExitStatus()
	}

	log.WithField("ssh.exit", state.ExitCode).
		Debug("ssh session finished")
	return state, err
}

// helper function configures and dials the ssh server.
func dial(server, username, privatekey string) (*ssh.Client, error) {
	if !strings.HasSuffix(server, ":22") {
		server = server + ":22"
	}
	config := &ssh.ClientConfig{
		User:            username,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	pem := []byte(privatekey)
	signer, err := ssh.ParsePrivateKey(pem)
	if err != nil {
		return nil, err
	}
	config.Auth = append(config.Auth, ssh.PublicKeys(signer))
	return ssh.Dial("tcp", server, config)
}

// helper function configures and dials the ssh server and retries if there is
// an error connecting.
func dialRetry(ctx context.Context, server, username, privatekey string) (*ssh.Client, error) {
	var err error
	var client *ssh.Client
	client, err = dial(server, username, privatekey)
	if err == nil {
		return client, nil
	}

	ctx, cancel := context.WithTimeout(ctx, networkTimeout)
	defer cancel()

	for i := 1; ; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		logger.FromContext(ctx).
			WithField("host", server).
			WithField("user", username).
			WithField("retry_attempt", i).
			Debug("dialing the vm")

		client, err = dial(server, username, privatekey)
		if err == nil {
			return client, nil
		}

		logger.FromContext(ctx).
			WithError(err).
			WithField("ip", server).
			WithField("retry_attempt", i).
			Trace("failed to re-dial vm")

		if client != nil {
			client.Close()
		}

		select {
		case <-ctx.Done():
			// we've been cancelled
			return nil, ctx.Err()
		case <-time.After(time.Second * 10):
			// waiting 10 seconds before retry
		}
	}
	return client, err
}

// helper function writes the file to the remote server and then
// configures the file permissions.
func upload(client *sftp.Client, path string, data []byte, mode uint32) error {
	f, err := client.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return err
	}
	err = f.Chmod(os.FileMode(mode))
	if err != nil {
		return err
	}
	return nil
}

// helper function creates the folder on the remote server and
// then configures the folder permissions.
func mkdir(client *sftp.Client, path string, mode uint32) error {
	err := client.MkdirAll(path)
	if err != nil {
		return err
	}
	return client.Chmod(path, os.FileMode(mode))
}
