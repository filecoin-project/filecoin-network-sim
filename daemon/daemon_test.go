package daemon

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type TestDaemon struct {
	*Daemon

	test *testing.T
}

func NewTestDaemon(t *testing.T, options ...func(*Daemon)) *TestDaemon {
	daemon, err := NewDaemon()
	assert.NoError(t, err)
	return &TestDaemon{daemon, t}
}

func (td *TestDaemon) Start() *TestDaemon {
	_, err := td.Daemon.Start()
	assert.NoError(td.test, err)
	return td
}

func (td *TestDaemon) ShutdownSuccess() {
	assert.NoError(td.test, td.Daemon.Shutdown())
}

func TestDaemonStartupMessage(t *testing.T) {
	assert := assert.New(t)
	daemon := NewTestDaemon(t).Start()
	daemon.ShutdownSuccess()

	out := daemon.ReadStdout()
	assert.Regexp("^My peer ID is [a-zA-Z0-9]*", out)
	assert.Regexp("\\nSwarm listening on.*", out)
}

func TestSwarmConnectPeers(t *testing.T) {

	d1 := NewTestDaemon(t, SwarmAddr("/ip4/127.0.0.1/tcp/6000")).Start()
	defer d1.ShutdownSuccess()

	d2 := NewTestDaemon(t, SwarmAddr("/ip4/127.0.0.1/tcp/6001")).Start()
	defer d2.ShutdownSuccess()

	_, err := d1.Connect(d2.Daemon)
	assert.NoError(t, err)

	d3 := NewTestDaemon(t).Start()
	defer d3.ShutdownSuccess()

	d4 := NewTestDaemon(t).Start()
	defer d4.ShutdownSuccess()

	_, err = d1.Connect(d3.Daemon)
	assert.NoError(t, err)

	_, err = d1.Connect(d4.Daemon)
	assert.NoError(t, err)

	_, err = d2.Connect(d3.Daemon)
	assert.NoError(t, err)

	_, err = d2.Connect(d4.Daemon)
	assert.NoError(t, err)

	_, err = d3.Connect(d4.Daemon)
	assert.NoError(t, err)
}

func TestDaemonEventLogs(t *testing.T) {
	assert := assert.New(t)
	daemon := NewTestDaemon(t).Start()
	defer daemon.ShutdownSuccess()

	t.Log("setting up event log stream")
	logs := daemon.EventLogStream()
	blocks := 10

	done := make(chan struct{}, 2)
	go func() {
		d := json.NewDecoder(logs)
		var m map[string]interface{}

		blocksLeft := blocks
		eventsSeen := 0
		for ; blocksLeft > 0; eventsSeen++ {
			err := d.Decode(&m)
			assert.NoError(err)

			if m["Operation"].(string) == "AddNewBlock" {
				blocksLeft--
			}
		}

		t.Logf("Parsed %d events", eventsSeen)
		done <- struct{}{}
	}()

	go func() {
		time.Sleep(100 * time.Millisecond)
		for i := 0; i < blocks; i++ {
			t.Log("mining once...")
			err := daemon.MiningOnce()
			assert.NoError(err)
		}
		done <- struct{}{}
	}()

	select {
	case <-done:
		select {
		case <-done:
			return // success
		case <-time.After(5 * time.Second):
			t.Fail()
		}
	case <-time.After(5 * time.Second):
		t.Fail()
	}
}
