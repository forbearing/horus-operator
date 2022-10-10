package restic

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/forbearing/restic"
	res "github.com/forbearing/restic"
)

var (
	r      = restic.NewOrDie(context.TODO(), &restic.GlobalFlags{Json: true})
	cmdOut = new(bytes.Buffer)
)

// TestAll
func TestAll(t *testing.T) {
	t.Run("snapshot", testSnapshots)
	t.Run("find", testFind)

	// test failed
	// ref: https://github.com/restic/restic/blob/master/cmd/restic/cmd_ls.go
	//t.Run("ls", testLs)
}

// testSnapshots
func testSnapshots(t *testing.T) {
	cmdOut.Reset()
	if err := r.SetOutput(cmdOut, io.Discard).Command(res.Snapshots{}).Run(); err != nil {
		t.Fatalf(`run command "restic snapshot" failed: %s`, err.Error())
	}

	nodeSnapshots := []NodeSnapshot{}
	if err := json.Unmarshal(cmdOut.Bytes(), &nodeSnapshots); err != nil {
		t.Fatalf("json.Unmarshal error: %s", err.Error())
	}
	t.Log(nodeSnapshots)
}

// testFind
func testFind(t *testing.T) {
	cmdOut.Reset()
	if err := r.SetOutput(cmdOut, io.Discard).Command(res.Find{}.SetArgs("871dafac", "zshrc")).Run(); err != nil {
		t.Fatalf(`run command "restic find" failed: %s`, err.Error())
	}

	nodeFind := []NodeFind{}
	if err := json.Unmarshal(cmdOut.Bytes(), &nodeFind); err != nil {
		t.Fatalf("json.Unmarshal error: %s", err.Error())
	}
	t.Log(nodeFind)
}

// testLs
func testLs(t *testing.T) {
	cmdOut.Reset()
	if err := r.SetOutput(cmdOut, io.Discard).Command(res.Ls{}.SetArgs("871dafac")).Run(); err != nil {
		t.Fatalf(`run command "restic find" failed: %s`, err.Error())
	}

	nodeLs := []NodeLs{}
	if err := json.Unmarshal(cmdOut.Bytes(), &nodeLs); err != nil {
		t.Fatalf("json.Unmarshal error: %s", err.Error())
	}
	t.Log(nodeLs)
}
