package util

import (
	"fmt"
	"strconv"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// MakeDirOnSftp create directory on sftp server.
// If port is empty, default to 22.
func MakeDirOnSftp(addr string, port uint32, user, pass string, dirpath string) (err error) {
	var (
		sshClient  *ssh.Client
		sftpClient *sftp.Client
	)

	sshConfig := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(pass)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		ClientVersion:   "",
		Timeout:         10 * time.Second,
	}
	if sshClient, err = ssh.Dial("tcp", fmt.Sprintf("%s:%s", addr, strconv.Itoa(int(port))), sshConfig); err != nil {
		return err
	}
	if sftpClient, err = sftp.NewClient(sshClient); err != nil {
		return err
	}
	if err = sftpClient.MkdirAll(dirpath); err != nil {
		return err
	}

	return nil
}
