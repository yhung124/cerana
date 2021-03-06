package zfs_test

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/cerana/cerana/acomm"
	"github.com/cerana/cerana/provider"
	zfsp "github.com/cerana/cerana/providers/zfs"
	libzfs "github.com/cerana/cerana/zfs"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/suite"
)

var (
	eagain       = syscall.EAGAIN.Error()
	ebadf        = syscall.EBADF.Error()
	ebusy        = syscall.EBUSY.Error()
	eexist       = syscall.EEXIST.Error()
	einval       = syscall.EINVAL.Error()
	enametoolong = syscall.ENAMETOOLONG.Error()
	enoent       = syscall.ENOENT.Error()
	epipe        = syscall.EPIPE.Error()
	exdev        = syscall.EXDEV.Error()

	longName = strings.Repeat("z", 257)
)

type zfs struct {
	suite.Suite
	pool         string
	files        []string
	dir          string
	config       *provider.Config
	tracker      *acomm.Tracker
	zfs          *zfsp.ZFS
	responseHook *url.URL
}

func TestZFS(t *testing.T) {
	suite.Run(t, new(zfs))
}

type props map[string]interface{}

func command(name string, arg ...string) *exec.Cmd {
	cmd := exec.Command(name, arg...)
	cmd.Stderr = os.Stderr
	return cmd
}

func (s *zfs) zfsSetup(pool string) {
	s.pool = pool
	files := make([]string, 5)
	for i := range files {
		f, err := ioutil.TempFile(s.dir, "zpool-img")
		if err != nil {
			panic(err)
		}
		files[i] = f.Name()
		_ = f.Close()
	}
	s.files = files

	script := []byte(`
    set -e
    pool=` + s.pool + `
    zpool list $pool &>/dev/null && zpool destroy $pool
    files=(` + strings.Join(files, " ") + `)
    for f in ${files[*]}; do
        truncate -s1G $f
    done
    zpool create $pool ${files[*]}

	zfs create $pool/fs
	zfs create $pool/fs/1snap
	zfs snapshot $pool/fs/1snap@snap

	zfs create $pool/fs/3snap
	zfs snapshot $pool/fs/3snap@snap1
	zfs snapshot $pool/fs/3snap@snap2
	zfs snapshot $pool/fs/3snap@snap3

	zfs create $pool/fs/hold_snap
	zfs snapshot $pool/fs/hold_snap@snap
	zfs hold hold $pool/fs/hold_snap@snap

	zfs create $pool/fs/unmounted
	zfs unmount $pool/fs/unmounted

	zfs create $pool/fs/unmounted_children
	zfs create $pool/fs/unmounted_children/1
	zfs create $pool/fs/unmounted_children/2
	zfs unmount $pool/fs/unmounted_children

	zfs snapshot $pool/fs@snap_with_clone
	zfs clone $pool/fs@snap_with_clone $pool/fs_clone
	zfs unmount $pool/fs_clone


	zfs create $pool/vol
	zfs create -V 8192 $pool/vol/1snap
	zfs snapshot $pool/vol/1snap@snap

	exit 0
    `)

	cmd := command("sudo", "bash", "-c", string(script))

	stdin, err := cmd.StdinPipe()
	s.Require().NoError(err)
	go func() {
		_, err := stdin.Write([]byte(script))
		s.Require().NoError(err)
	}()

	s.Require().NoError(cmd.Run())
}

func (s *zfs) zfsTearDown() {
	err := command("sudo", "zpool", "destroy", s.pool).Run()
	for i := range s.files {
		_ = os.Remove(s.files[i])
	}
	s.Require().NoError(err)
}

func unmount(ds string) error {
	return command("sudo", "zfs", "unmount", ds).Run()
}

func unhold(tag, snapshot string) error {
	return command("sudo", "zfs", "release", tag, snapshot).Run()
}

func (s *zfs) SetupSuite() {
	s.responseHook, _ = url.ParseRequestURI("unix:///tmp/foobar")
	dir, err := ioutil.TempDir("", "zfs-provider-test-")
	s.Require().NoError(err)
	s.dir = dir

	v := viper.New()
	flagset := pflag.NewFlagSet("zfs-provider", pflag.PanicOnError)
	config := provider.NewConfig(flagset, v)
	s.Require().NoError(flagset.Parse([]string{}))
	v.Set("service_name", "zfs-provider-test")
	v.Set("socket_dir", s.dir)
	v.Set("coordinator_url", "unix:///tmp/foobar")
	v.Set("log_level", "fatal")
	s.Require().NoError(config.LoadConfig())
	s.Require().NoError(config.SetupLogging())
	s.config = config

	tracker, err := acomm.NewTracker(filepath.Join(s.dir, "tracker.sock"), nil, nil, 5*time.Second)
	s.Require().NoError(err)
	s.Require().NoError(tracker.Start())
	s.tracker = tracker

	s.zfs = zfsp.New(config, tracker)
}

func (s *zfs) SetupTest() {
	s.zfsSetup("zfs-provider-test")
}

func (s *zfs) TearDownTest() {
	s.zfsTearDown()
}

func (s *zfs) TearDownSuite() {
	s.tracker.Stop()
	_ = os.RemoveAll(s.dir)
}

func (s *zfs) TestDatasetMoutpoint() {
	tests := []struct {
		name             string
		mountpointSource string
		mountpoint       string
		result           string
	}{
		{"foo", "", "", "foo"},
		{"foo", "foo", "bar", "bar"},
		{"foo/bar", "foo", "baz", "baz/bar"},
	}

	for _, test := range tests {
		testS := fmt.Sprintf("%+v", test)
		ds := &zfsp.Dataset{
			Name: test.name,
			Properties: &libzfs.DatasetProperties{
				MountpointSource: test.mountpointSource,
				Mountpoint:       test.mountpoint,
			},
		}

		s.Equal(test.result, ds.Mountpoint(), testS)
	}
}

func (s *zfs) TestRegisterTasks() {
	server, err := provider.NewServer(s.config)
	s.Require().NoError(err)

	s.zfs.RegisterTasks(server)

	s.True(len(server.RegisteredTasks()) > 0)
}
