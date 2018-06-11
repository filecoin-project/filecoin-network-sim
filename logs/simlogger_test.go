package logs

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSimLoggerFiles(t *testing.T) {
	f, err := os.Open("./eventlogs.ndjson")
	assert.NoError(t, err)
	defer f.Close()

	id := "fcqmtkkrxtuh7fh0s9as7lves4j25ry7gnrzw6xxs"
	l := NewSimLogger(id, f)

	buf := bytes.NewBuffer(nil)
	io.Copy(buf, l.Reader())

	f2, err := ioutil.ReadFile("./simlogs.ndjson")
	assert.NoError(t, err)

	fmt.Println(string(buf.Bytes()))
	_ = f2
	_ = fmt.Printf
	// assert.Exactly(t, buf.Bytes(), f2)
}
