package network

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TempDir(t *testing.T) string {
	dir, err := ioutil.TempDir("", "network test")
	assert.NoError(t, err)
	return dir
}

func TestNetworkAddNode(t *testing.T) {
	net, err := NewNetwork(TempDir(t))
	assert.NoError(t, err)
	defer net.ShutdownAll()

	n1, err := net.AddNode(AnyNodeType)
	assert.NoError(t, err)

	n2, err := net.AddNode(AnyNodeType)
	assert.NoError(t, err)

	n3, err := net.AddNode(AnyNodeType)
	assert.NoError(t, err)

	assert.True(t, n1 == net.GetNode(0))
	assert.True(t, n2 == net.GetNode(1))
	assert.True(t, n3 == net.GetNode(2))
}

func TestNetworkAddNodes(t *testing.T) {
	net, err := NewNetwork(TempDir(t))
	assert.NoError(t, err)
	defer net.ShutdownAll()

	err = net.AddNodes(AnyNodeType, 10)
	assert.NoError(t, err)
}

func TestNetworkConnectNodes(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	net, err := NewNetwork(TempDir(t))
	require.NoError(err)
	defer net.ShutdownAll()

	n1, err := net.AddNode(AnyNodeType)
	require.NoError(err)

	n2, err := net.AddNode(AnyNodeType)
	require.NoError(err)

	n3, err := net.AddNode(AnyNodeType)
	require.NoError(err)

	require.True(n1 == net.GetNode(0))
	require.True(n2 == net.GetNode(1))
	require.True(n3 == net.GetNode(2))

	_, err = n1.Connect(n2.Daemon)
	assert.NoError(err)

	_, err = n1.Connect(n3.Daemon)
	assert.NoError(err)

	_, err = n2.Connect(n3.Daemon)
	assert.NoError(err)
}

func TestLogging(t *testing.T) {
	// create a temp network
	net, err := NewNetwork(TempDir(t))
	assert.NoError(t, err)
	defer net.ShutdownAll()

	buf := bytes.NewBuffer(nil)
	go io.Copy(buf, net.Logs().Reader())

	// Add the nodes to the network
	n1, err := net.AddNode(MinerNodeType)
	assert.NoError(t, err)
	n2, err := net.AddNode(MinerNodeType)
	assert.NoError(t, err)
	n3, err := net.AddNode(MinerNodeType)
	assert.NoError(t, err)

	// Connect all Nodes
	_, err = n1.Connect(n3.Daemon)
	assert.NoError(t, err)
	_, err = n1.Connect(n2.Daemon)
	assert.NoError(t, err)
	_, err = n2.Connect(n3.Daemon)
	assert.NoError(t, err)

	// every node mines a block
	assert.NoError(t, n1.Daemon.MiningOnce())
	//assert.NoError(t, n2.Daemon.MiningOnce())
	//assert.NoError(t, n3.Daemon.MiningOnce())

	// check logs

	check := []map[string]string{
		{"type": "MinerJoins"},
		{"type": "MinerJoins"},
		{"type": "MinerJoins"},
		{"type": "Connected"},
		{"type": "Connected"},
		{"type": "Connected"},
		{"type": "NewBlockMined"},
		{"type": "BroadcastBlock"},
		/*
			{"type": "NewBlockMined"},
			{"type": "BroadcastBlock"},
			{"type": "NewBlockMined"},
			{"type": "BroadcastBlock"},
		*/
	}

	d := json.NewDecoder(buf)
	for _, m1 := range check {
		var m2 map[string]interface{}
		assert.NoError(t, d.Decode(&m2))

		for k, _ := range m1 {
			//assert.Equal(t, v, m2[k])
			fmt.Printf("%#v", m2[k])
		}
		//t.Log(m2)
	}
}
