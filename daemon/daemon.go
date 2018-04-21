package daemon

import (
  "encoding/json"
  "errors"
  "fmt"
  "io"
  "io/ioutil"
  "net"
  "net/http"
  "os"
  "os/exec"
  "path/filepath"
  "strings"
  "sync"
  "syscall"
  "time"
  "log"
)

// Output manages running, inprocess, a filecoin command.
type Output struct {
  lk sync.Mutex
  // Input is the the raw input we got.
  Input string
  // Args is the cleaned up version of the input.
  Args []string
  // Code is the unix style exit code, set after the command exited.
  Code int
  // Error is the error returned from the command, after it exited.
  Error  error
  Stdin  io.WriteCloser
  Stdout io.ReadCloser
  stdout []byte
  Stderr io.ReadCloser
  stderr []byte
}

func (o *Output) Close(code int, err error) {
  o.lk.Lock()
  defer o.lk.Unlock()

  o.Code = code
  o.Error = err
}

func (o *Output) ReadStderr() string {
  o.lk.Lock()
  defer o.lk.Unlock()

  return string(o.stderr)
}

func (o *Output) ReadStdout() string {
  o.lk.Lock()
  defer o.lk.Unlock()

  return string(o.stdout)
}

func (o *Output) ReadStdoutTrimNewlines() string {
  return strings.Trim(o.ReadStdout(), "\n")
}

func (o *Output) Failed() bool {
  if o.Error != nil {
    return true
  }
  oErr := o.ReadStderr()
  if o.Code != 0 {
    return true
  }

  if strings.Contains(oErr, "CRITICAL") ||
    strings.Contains(oErr, "ERROR") ||
    strings.Contains(oErr, "WARNING") {
    return true
  }

  return false
}

type Daemon struct {
  cmdAddr   string
  swarmAddr string
  repoDir   string

  init bool

  // The filecoin daemon process
  process *exec.Cmd

  lk     sync.Mutex
  Stdin  io.Writer
  Stdout io.Reader
  Stderr io.Reader
}

func (td *Daemon) Run(args ...string) (*Output, error) {
  return td.RunWithStdin(nil, args...)
}

func (td *Daemon) RunWithStdin(stdin io.Reader, args ...string) (*Output, error) {
  bin, err := GetFilecoinBinary()
  if err != nil {
    return nil, err
  }

  // handle Run("cmd subcmd")
  if len(args) == 1 {
    args = strings.Split(args[0], " ")
  }

  finalArgs := append(args, "--repodir="+td.repoDir, "--cmdapiaddr="+td.cmdAddr)

  log.Printf("run: %q", strings.Join(finalArgs, " "))
  cmd := exec.Command(bin, finalArgs...)

  if stdin != nil {
    cmd.Stdin = stdin
  }

  stderr, err := cmd.StderrPipe()
  if err != nil {
    return nil, err
  }

  stdout, err := cmd.StdoutPipe()
  if err != nil {
    return nil, err
  }

  err = cmd.Start()
  if err != nil {
    return nil, err
  }

  stderrBytes, err := ioutil.ReadAll(stderr)
  if err != nil {
    return nil, err
  }

  stdoutBytes, err := ioutil.ReadAll(stdout)
  if err != nil {
    return nil, err
  }

  o := &Output{
    Args:   args,
    Stdout: stdout,
    stdout: stdoutBytes,
    Stderr: stderr,
    stderr: stderrBytes,
  }

  err = cmd.Wait()

  switch err := err.(type) {
  case *exec.ExitError:
    // TODO: its non-trivial to get the 'exit code' cross platform...
    o.Code = 1
  default:
    o.Error = err
  case nil:
    // okay
  }

  if err != nil && o.Failed() {
    err = errors.New("command failed")
  }

  return o, err
}

func (td *Daemon) EventLogStream() io.Reader {
  r, w := io.Pipe()

  go func() {
    defer w.Close()

    url := fmt.Sprintf("http://127.0.0.1%s/api/log/tail", td.cmdAddr)
    res, err := http.Get(url)
    if err != nil {
      return
    }
    io.Copy(w, res.Body)
    defer res.Body.Close()
  }()

  return r
}

func (td *Daemon) ReadStdout() string {
  td.lk.Lock()
  defer td.lk.Unlock()
  out, err := ioutil.ReadAll(td.Stdout)
  if err != nil {
    panic(err)
  }
  return string(out)
}

func (td *Daemon) ReadStderr() string {
  td.lk.Lock()
  defer td.lk.Unlock()
  out, err := ioutil.ReadAll(td.Stderr)
  if err != nil {
    panic(err)
  }
  return string(out)
}

func (td *Daemon) Start() (*Daemon, error) {
  if err := td.process.Start(); err != nil {
    return td, err
  }
  err := td.WaitForAPI()
  return td, err
}

func (td *Daemon) Shutdown() error {
  if err := td.process.Process.Signal(syscall.SIGTERM); err != nil {
    log.Printf("Daemon Stderr:\n%s", td.ReadStderr())
    log.Printf("Failed to kill daemon %s", err)
    return err
  }

  if td.repoDir == "" {
    panic("daemon had no repodir set")
  }

  _ = os.RemoveAll(td.repoDir)
  return nil
}

func (td *Daemon) Kill() {
  if err := td.process.Process.Kill(); err != nil {
    log.Printf("Daemon Stderr:\n%s", td.ReadStderr())
    log.Printf("Failed to kill daemon %s", err)
  }
}

func (td *Daemon) WaitForAPI() error {
  for i := 0; i < 100; i++ {
    err := tryAPICheck(td)
    if err == nil {
      return nil
    }
    time.Sleep(time.Millisecond * 100)
  }
  return fmt.Errorf("filecoin node failed to come online in given time period (10 seconds)")
}


func tryAPICheck(td *Daemon) error {
  url := fmt.Sprintf("http://127.0.0.1%s/api/id", td.cmdAddr)
  resp, err := http.Get(url)
  if err != nil {
    return err
  }

  out := make(map[string]interface{})
  err = json.NewDecoder(resp.Body).Decode(&out)
  if err != nil {
    return fmt.Errorf("liveness check failed: %s", err)
  }

  _, ok := out["ID"]
  if !ok {
    return fmt.Errorf("liveness check failed: ID field not present in output")
  }

  return nil
}

func GetFilecoinBinary() (string, error) {
  bin := filepath.FromSlash(fmt.Sprintf("%s/src/github.com/filecoin-project/go-filecoin/go-filecoin", os.Getenv("GOPATH")))
  _, err := os.Stat(bin)
  if err == nil {
    return bin, nil
  }

  if os.IsNotExist(err) {
    return "", fmt.Errorf("You are missing the filecoin binary...try building, searched in '%s'", bin)
  }

  return "", err
}

func SwarmAddr(addr string) func(*Daemon) {
  return func(td *Daemon) {
    td.swarmAddr = addr
  }
}

func RepoDir(dir string) func(*Daemon) {
  return func(td *Daemon) {
    td.repoDir = dir
  }
}

func ShouldInit(i bool) func(*Daemon) {
  return func(td *Daemon) {
    td.init = i
  }
}

func NewDaemon(options ...func(*Daemon)) (*Daemon, error) {
  // Ensure we have the actual binary
  filecoinBin, err := GetFilecoinBinary()
  if err != nil {
    return nil, err
  }

  //Ask the kernel for a port to avoid conflicts
  cmdPort, err := GetFreePort()
  if err != nil {
    return nil, err
  }
  swarmPort, err := GetFreePort()
  if err != nil {
    return nil, err
  }

  dir, err := ioutil.TempDir("", "go-fil-test")
  if err != nil {
    return nil, err
  }

  td := &Daemon{
    cmdAddr:   fmt.Sprintf(":%d", cmdPort),
    swarmAddr: fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", swarmPort),
    repoDir:   dir,
    init:      true, // we want to init unless told otherwise
  }

  // configure Daemon options
  for _, option := range options {
    option(td)
  }

  repodirFlag := fmt.Sprintf("--repodir=%s", td.repoDir)
  if td.init {
    out, err := RunInit(repodirFlag)
    if err != nil {
      log.Println(string(out))
      return td, err
    }
  }

  // define filecoin daemon process
  td.process = exec.Command(filecoinBin, "daemon",
    fmt.Sprintf("--repodir=%s", td.repoDir),
    fmt.Sprintf("--cmdapiaddr=%s", td.cmdAddr),
    fmt.Sprintf("--swarmlisten=%s", td.swarmAddr),
  )

  // setup process pipes
  td.Stdout, err = td.process.StdoutPipe()
  if err != nil {
    return td, err
  }
  td.Stderr, err = td.process.StderrPipe()
  if err != nil {
    return td, err
  }
  td.Stdin, err = td.process.StdinPipe()
  if err != nil {
    return td, err
  }

  return td, nil
}

// Credit: https://github.com/phayes/freeport
func GetFreePort() (int, error) {
  addr, err := net.ResolveTCPAddr("tcp", "0.0.0.0:0")
  if err != nil {
    return 0, err
  }

  l, err := net.ListenTCP("tcp", addr)
  if err != nil {
    return 0, err
  }
  defer l.Close()
  return l.Addr().(*net.TCPAddr).Port, nil
}

func RunInit(opts ...string) ([]byte, error) {
  return RunCommand("init", opts...)
}

func RunCommand(cmd string, opts ...string) ([]byte, error) {
  filecoinBin, err := GetFilecoinBinary()
  if err != nil {
    return nil, err
  }

  process := exec.Command(filecoinBin, append([]string{cmd}, opts...)...)
  return process.CombinedOutput()
}

func ConfigExists(dir string) bool {
  _, err := os.Stat(filepath.Join(dir, "config.toml"))
  if os.IsNotExist(err) {
    return false
  }
  return err == nil
}
